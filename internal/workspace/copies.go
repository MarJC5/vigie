package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// errNoStream is returned when the workspace's runner cannot stream an output (a test's).
var errNoStream = errors.New("the runner cannot stream a command's output")

// CopyRequest is a copy the user asked for, with the settings of the copies (ADR-0041).
type CopyRequest struct {
	Root    string // the folder of the copies
	Content sitecopy.Content
	LeftOut []string
	Limit   int64 // the size of an archive
}

// copyServer runs a copy's commands on a site's account, within its host's limit of commands at a
// time: a stream holds its place for as long as it runs.
type copyServer struct {
	w    *Workspace
	host model.Host
}

func (s copyServer) Run(ctx context.Context, line string, input []byte) (sshx.Result, error) {
	var res sshx.Result
	err := s.w.limiter.Do(ctx, s.host.ID.String(), maxConcurrent(s.host), func() error {
		var err error
		res, err = s.w.deps.Runner.RunInput(ctx, sshx.TargetFor(s.host), line, input)
		return err
	})
	return res, err
}

func (s copyServer) Stream(ctx context.Context, line string, input io.Reader, out io.Writer) (sshx.Result, error) {
	streamer, ok := s.w.deps.Runner.(sshx.StreamRunner)
	if !ok {
		return sshx.Result{}, errNoStream
	}
	var res sshx.Result
	err := s.w.limiter.Do(ctx, s.host.ID.String(), maxConcurrent(s.host), func() error {
		var err error
		res, err = streamer.RunStream(ctx, sshx.TargetFor(s.host), line, input, out)
		return err
	})
	return res, err
}

// copier is the copier of a site, over its host's account.
func (w *Workspace) copier(ctx context.Context, siteID model.ID) (sitecopy.Copier, model.Site, model.Host, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return sitecopy.Copier{}, model.Site{}, model.Host{}, err
	}
	site, ok := tree.Site(siteID)
	if !ok {
		return sitecopy.Copier{}, model.Site{}, model.Host{}, fmt.Errorf("site %s: %w", siteID, model.ErrNotFound)
	}
	host, _ := tree.Host(site.HostID)
	command := tree.EffectiveWPCLI(site.ID)
	free := w.deps.DiskFree
	if free == nil {
		free = sitecopy.DiskFree
	}
	return sitecopy.Copier{
		Server:  copyServer{w: w, host: host},
		WP:      func(c wpcli.Command) (string, error) { return c.Remote(command, site.Path) },
		Now:     w.deps.Now,
		Version: buildinfo.Version(),
		Free:    free,
	}, site, host, nil
}

// PrepareCopy reads, read-only, what the copy of a site needs: what the copy dialog shows.
func (w *Workspace) PrepareCopy(ctx context.Context, siteID model.ID) (sitecopy.Preview, error) {
	c, _, _, err := w.copier(ctx, siteID)
	if err != nil {
		return sitecopy.Preview{}, err
	}
	return c.Prepare(ctx)
}

// CopySite copies a site to the computer, from what PrepareCopy read, and keeps the copy for the
// journal, done, failed or stopped. It answers the copy's folder, also when it failed halfway.
func (w *Workspace) CopySite(ctx context.Context, siteID model.ID, req CopyRequest, p sitecopy.Preview, progress func(sitecopy.Progress)) (string, error) {
	c, site, host, err := w.copier(ctx, siteID)
	if err != nil {
		return "", err
	}
	record := model.Copy{ID: model.NewID(), SiteID: site.ID, SiteName: site.Name, HostName: host.Name,
		Content: string(req.Content), Started: w.deps.Now()}
	address := site.URL
	if address == "" {
		address = site.Name
	}
	job := sitecopy.Job{Root: req.Root, Host: host.Name, Site: site.Name, Address: address,
		Content: req.Content, LeftOut: req.LeftOut, Limit: req.Limit}
	dir, err := c.Start(ctx, job, p, progress)
	if dir == "" {
		// Refused before anything was read (the disk too small): nothing to tell in the journal.
		return "", err
	}
	return dir, w.recordCopy(ctx, record, dir, 0, 0, err)
}

// ResumeCopy reads the archives missing of a cut copy of a site, and keeps the resume for the
// journal.
func (w *Workspace) ResumeCopy(ctx context.Context, siteID model.ID, dir string, progress func(sitecopy.Progress)) error {
	c, site, host, err := w.copier(ctx, siteID)
	if err != nil {
		return err
	}
	before, err := sitecopy.ReadManifest(dir)
	if err != nil {
		return err
	}
	record := model.Copy{ID: model.NewID(), SiteID: site.ID, SiteName: site.Name, HostName: host.Name,
		Content: string(before.Content), Resumed: true, Started: w.deps.Now()}
	err = c.Resume(ctx, dir, progress)
	if errors.Is(err, sitecopy.ErrComplete) {
		return err
	}
	var bytes int64
	for _, r := range before.Archives {
		bytes += r.Bytes
	}
	return w.recordCopy(ctx, record, dir, len(before.Archives), bytes, err)
}

// recordCopy keeps a copy or a resume in the journal: the archives and bytes it wrote, besides
// those there before it, and how it ended.
func (w *Workspace) recordCopy(ctx context.Context, record model.Copy, dir string, archivesBefore int, bytesBefore int64, err error) error {
	record.Folder, record.Finished = dir, w.deps.Now()
	if m, merr := sitecopy.ReadManifest(dir); merr == nil {
		record.Archives = len(m.Archives) - archivesBefore
		for _, r := range m.Archives {
			record.Bytes += r.Bytes
		}
		record.Bytes -= bytesBefore
	}
	switch {
	case err == nil:
		record.Outcome = model.CopyCompleted
	case errors.Is(err, context.Canceled):
		record.Outcome, record.Message = model.CopyCancelled, err.Error()
	default:
		record.Outcome, record.Message = model.CopyFailed, err.Error()
	}
	if serr := w.db.SaveCopy(context.WithoutCancel(ctx), record); serr != nil && err == nil {
		return serr
	}
	return err
}
