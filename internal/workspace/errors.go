package workspace

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/errorlog"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// ErrorsState is what a read found of a site's log.
type ErrorsState string

const (
	ErrorsOff   ErrorsState = "off"   // WordPress writes no log, and none is there
	ErrorsEmpty ErrorsState = "empty" // WordPress writes one, with nothing in it yet
	ErrorsLeft  ErrorsState = "left"  // WordPress no longer writes it: what it holds is old
	ErrorsRead  ErrorsState = "read"  // WordPress writes it, and it was read
)

// BeforeErrors is how long before an error's first time what changed on the site is told.
const BeforeErrors = 7 * 24 * time.Hour

// SiteErrors is what a read of a site's errors found: the one answer of the window, the command and
// an assistant. Its texts from the log are already masked; nothing of it is stored.
type SiteErrors struct {
	State  ErrorsState
	File   string // where the log is: relative to the site's folder, absolute outside it
	Size   int64
	Whole  bool // the whole file was searched
	More   bool // older errors than those read were left out
	Read   time.Time
	Lines  int       // the errors and warnings read
	Since  time.Time // the oldest of them: how far back the read goes
	Groups []errorlog.Group
	Before []Before // newest first
}

// Before is one thing that changed on the site just before an error's first time: an update or a
// rollback of Vigie, or a change seen between two scans. What happened before, never a cause.
type Before struct {
	At        time.Time
	Operation *model.Operation // an update or a rollback
	Change    *model.Change    // seen by the scan that ended at At
}

// SiteErrors reads a site's errors when asked, and only then: whether WordPress writes a log and
// where, then the fatal errors and warnings of its last megabyte, grouped and masked, with what
// changed on the site just before. It only reads on the server.
func (w *Workspace) SiteErrors(ctx context.Context, siteID model.ID) (SiteErrors, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return SiteErrors{}, err
	}
	site, ok := tree.Site(siteID)
	if !ok {
		return SiteErrors{}, fmt.Errorf("site %s: %w", siteID, model.ErrNotFound)
	}
	host, _ := tree.Host(site.HostID)
	command := tree.EffectiveWPCLI(site.ID)
	run := func(line string, err error) (sshx.Result, error) {
		if err != nil {
			return sshx.Result{}, err
		}
		return w.runOn(ctx, host, line)
	}

	debug, err := run(wpcli.ConfigConstant("WP_DEBUG").Remote(command, site.Path))
	if err != nil {
		return SiteErrors{}, err
	}
	logValue, err := run(wpcli.ConfigConstant("WP_DEBUG_LOG").Remote(command, site.Path))
	if err != nil {
		return SiteErrors{}, err
	}
	log, err := wpcli.ParseDebugLog(debug, logValue)
	if err != nil {
		return SiteErrors{}, err
	}
	res, err := run(remote.RealRoot(site.Path))
	if err == nil {
		err = wpcli.Done(res)
	}
	if err != nil {
		return SiteErrors{}, err
	}
	root, err := wpcli.ParseDir(res.Stdout)
	if err != nil {
		return SiteErrors{}, err
	}
	file, err := wpcli.DebugLogFile(root, log)
	if err != nil {
		return SiteErrors{}, err
	}
	if res, err = run(remote.ErrorLines(file)); err == nil {
		err = wpcli.Done(res)
	}
	if err != nil {
		return SiteErrors{}, err
	}
	read, err := remote.ParseErrorLines(res.Stdout)
	if err != nil {
		return SiteErrors{}, err
	}

	out := SiteErrors{File: strings.TrimPrefix(file, root+"/"), Size: read.Size, Whole: read.Whole(), More: read.More, Read: w.deps.Now().UTC()}
	switch {
	case !log.Enabled && !read.Exists:
		out.State = ErrorsOff
	case !log.Enabled:
		out.State = ErrorsLeft
	case !read.Exists || read.Size == 0:
		out.State = ErrorsEmpty
	default:
		out.State = ErrorsRead
	}
	if len(read.Lines) == 0 {
		return out, nil
	}
	s := errorlog.Read(read.Lines, root)
	out.Lines, out.Since, out.Groups = s.Lines, s.Since, s.Groups
	if out.Before, err = w.before(ctx, site.ID, s.Groups); err != nil {
		return SiteErrors{}, err
	}
	return out, nil
}

// before lists what changed on a site in the BeforeErrors before each group's first time, from what
// Vigie keeps: its updates and rollbacks, and the changes its scans noted. A version Vigie changed is
// told by its operation, which says when; the scan after it would only repeat it.
func (w *Workspace) before(ctx context.Context, siteID model.ID, groups []errorlog.Group) ([]Before, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	in := func(t time.Time) bool {
		return slices.ContainsFunc(groups, func(g errorlog.Group) bool {
			return !t.Before(g.First.Add(-BeforeErrors)) && !t.After(g.First)
		})
	}
	start := groups[0].First
	for _, g := range groups {
		start = minTime(start, g.First)
	}
	start = start.Add(-BeforeErrors)

	var out []Before
	operations, err := w.db.Operations(ctx, siteID)
	if err != nil {
		return nil, err
	}
	for i := range operations {
		if op := &operations[i]; in(op.Started) {
			out = append(out, Before{At: op.Started, Operation: op})
		}
	}
	scans, err := w.db.JournalScans(ctx, start, []model.ID{siteID})
	if err != nil {
		return nil, err
	}
	for _, s := range scans {
		if !s.Recorded || !in(s.Finished) {
			continue
		}
		for i := range s.RecordedChanges {
			if c := &s.RecordedChanges[i]; c.Kind != model.ChangeVersion || !c.ByVigie {
				out = append(out, Before{At: s.Finished, Change: c})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Before) int { return b.At.Compare(a.At) })
	return out, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
