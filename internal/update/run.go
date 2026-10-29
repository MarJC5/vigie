package update

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/httpcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// ErrNotOffered is returned for a selection the preview did not offer.
var ErrNotOffered = errors.New("not offered by the preview")

// errOutside is returned for a folder WP-CLI gave outside the site's own.
var errOutside = errors.New("folder outside the site")

// keptBackups is how many runs keep their backups on the server, per site.
const keptBackups = 3

// Store records the operations, step by step (store.DB).
type Store interface {
	StartOperation(ctx context.Context, op model.Operation) error
	SaveOperationStep(ctx context.Context, operationID model.ID, step model.OperationStep) error
	FinishOperation(ctx context.Context, id model.ID, outcome model.OperationOutcome, finished time.Time, summary string) error
}

// Checker watches pages after a change (httpcheck.Checker).
type Checker interface {
	Watch(ctx context.Context, urls []string) httpcheck.Result
}

// Selection is what the user chose to update among the preview.
type Selection struct {
	Core      bool
	Plugins   []string
	Themes    []string
	Languages bool
}

// Run updates the items selected: backups first, then WordPress in maintenance mode, the plugins,
// the themes and the translations, one at a time, each checked; it stops at the first failure.
// The operation is returned with its steps, whatever the outcome; an error means nothing started.
func (u Updater) Run(ctx context.Context, job Job, preview Preview, sel Selection) (model.Operation, error) {
	if !preview.DumpTool {
		return model.Operation{}, fmt.Errorf("%s: %w", job.Site.Name, wpcli.ErrNoDumpTool)
	}
	items, err := chosen(preview, sel)
	if err != nil {
		return model.Operation{}, err
	}
	op := model.Operation{
		ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationUpdate, Started: u.Now(),
		Outcome: model.OperationRunning, BackupDir: remote.BackupDir(job.Site.ID, u.Now()), PID: u.PID,
	}
	if err := u.Store.StartOperation(ctx, op); err != nil {
		return op, err
	}
	r := &run{s: session{u: u, job: job}, op: &op}
	outcome, summary := r.update(ctx, items, preview.Languages, sel.Languages)
	// The end is recorded, and old backups removed, even when the run was cancelled.
	end := context.WithoutCancel(ctx)
	r.prune(end)
	op.Outcome, op.Finished, op.Summary = outcome, u.Now(), summary
	if err := u.Store.FinishOperation(end, op.ID, outcome, op.Finished, summary); err != nil {
		return op, err
	}
	return op, nil
}

// chosen returns the items selected, in the order they are updated: WordPress, plugins, themes.
func chosen(p Preview, sel Selection) ([]Item, error) {
	var items []Item
	if sel.Core {
		if p.Core == nil {
			return nil, fmt.Errorf("WordPress: %w", ErrNotOffered)
		}
		items = append(items, *p.Core)
	}
	for _, kind := range []struct {
		slugs   []string
		offered []Item
	}{{sel.Plugins, p.Plugins}, {sel.Themes, p.Themes}} {
		for _, slug := range kind.slugs {
			i := slices.IndexFunc(kind.offered, func(it Item) bool { return it.Slug == slug })
			if i < 0 {
				return nil, fmt.Errorf("%q: %w", slug, ErrNotOffered)
			}
			items = append(items, kind.offered[i])
		}
	}
	if sel.Languages && len(p.Languages) == 0 {
		return nil, fmt.Errorf("translations: %w", ErrNotOffered)
	}
	return items, nil
}

// run is one update in progress.
type run struct {
	s        session
	op       *model.Operation
	root     string // the site's folder, links resolved
	debugLog string // where WordPress writes its PHP errors; empty when it does not
}

// update runs the backups, then the items; it returns the outcome and a summary.
func (r *run) update(ctx context.Context, items []Item, languages []Item, withLanguages bool) (model.OperationOutcome, string) {
	if err := r.prepare(ctx); err != nil {
		r.fail(ctx, model.OperationStep{Step: model.StepBackupDatabase}, err)
		return model.OperationFailed, "0 of " + fmt.Sprint(len(items))
	}
	if !r.backups(ctx, items) {
		return model.OperationFailed, fmt.Sprintf("0 of %d", len(items))
	}
	done := 0
	for _, it := range items {
		if ctx.Err() != nil {
			return model.OperationCancelled, fmt.Sprintf("%d of %d", done, len(items))
		}
		// An item is never left halfway: a cancel waits for its end.
		if !r.item(context.WithoutCancel(ctx), it) {
			return model.OperationFailed, fmt.Sprintf("%d of %d", done, len(items))
		}
		done++
	}
	if withLanguages && len(languages) > 0 {
		if ctx.Err() != nil {
			return model.OperationCancelled, fmt.Sprintf("%d of %d", done, len(items))
		}
		if !r.translations(context.WithoutCancel(ctx)) {
			return model.OperationFailed, fmt.Sprintf("%d of %d", done, len(items))
		}
	}
	return model.OperationCompleted, fmt.Sprintf("%d of %d", done, len(items))
}

// prepare reads the site's folder and where its PHP errors go.
func (r *run) prepare(ctx context.Context) error {
	line, err := remote.RealRoot(r.s.job.Site.Path)
	if err != nil {
		return err
	}
	res, err := r.s.shell(ctx, line)
	if err != nil {
		return err
	}
	if err := wpcli.Done(res); err != nil {
		return err
	}
	if r.root, err = wpcli.ParseDir(res.Stdout); err != nil {
		return err
	}
	debug, err := r.s.run(ctx, wpcli.ConfigConstant("WP_DEBUG"))
	if err != nil {
		return err
	}
	log, err := r.s.run(ctx, wpcli.ConfigConstant("WP_DEBUG_LOG"))
	if err != nil {
		return err
	}
	d, err := wpcli.ParseDebugLog(debug, log)
	if err != nil {
		return err
	}
	if d.Enabled {
		r.debugLog = d.Path
		if r.debugLog == "" {
			r.debugLog = r.root + "/wp-content/debug.log"
		}
	}
	return nil
}

// backups dumps the database and archives what the items change; false when one failed.
func (r *run) backups(ctx context.Context, items []Item) bool {
	dump := r.op.BackupDir + "/database.sql"
	exec := r.shellIn(ctx)
	ok := r.step(ctx, model.OperationStep{Step: model.StepBackupDatabase}, func() (string, error) {
		if err := exec(remote.MakeDir(r.op.BackupDir)); err != nil {
			return "", err
		}
		res, err := r.s.run(ctx, wpcli.DatabaseExport(dump))
		if err != nil {
			return "", err
		}
		if _, err := wpcli.ParseDatabaseExport(res); err != nil {
			return "", err
		}
		return dump + ".gz", exec(remote.Compress(dump))
	})
	if !ok {
		return false
	}
	for _, it := range items {
		ok := r.step(ctx, model.OperationStep{Step: model.StepBackupFiles, ItemKind: it.Kind, Slug: it.Slug}, func() (string, error) {
			if it.Kind == model.ItemCore {
				return r.root, exec(remote.ArchiveCore(r.op.BackupDir+"/core.tar.gz", r.root))
			}
			dir, err := r.folder(ctx, it)
			if err != nil {
				return "", err
			}
			return dir, exec(remote.Archive(archiveOf(r.op.BackupDir, it), dir))
		})
		if !ok {
			return false
		}
	}
	return true
}

// archiveOf is the archive of an item's folder in a run's backups.
func archiveOf(dir string, it Item) string {
	if it.Kind == model.ItemCore {
		return dir + "/core.tar.gz"
	}
	return dir + "/" + string(it.Kind) + "-" + it.Slug + ".tar.gz"
}

// folder asks WP-CLI where a plugin or theme is, which must be inside the site's folder.
func (r *run) folder(ctx context.Context, it Item) (string, error) {
	of := wpcli.ExtensionPlugins
	if it.Kind == model.ItemTheme {
		of = wpcli.ExtensionThemes
	}
	out, err := r.s.wp(ctx, wpcli.ExtensionDir(of, it.Slug))
	if err != nil {
		return "", err
	}
	dir, err := wpcli.ParseDir(out)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(dir, r.root+"/") || path.Dir(dir) == r.root {
		return "", fmt.Errorf("%s in %s: %w", dir, r.root, errOutside)
	}
	return dir, nil
}

// item updates one item and checks the site; false when either failed.
func (r *run) item(ctx context.Context, it Item) bool {
	offset := r.logSize(ctx)
	if it.Kind == model.ItemCore {
		if !r.core(ctx, it) {
			return false
		}
		return r.check(ctx, it, offset)
	}
	if !r.step(ctx, model.OperationStep{Step: model.StepUpdate, ItemKind: it.Kind, Slug: it.Slug,
		VersionBefore: it.From, VersionAfter: it.To}, func() (string, error) { return r.extension(ctx, it) }) {
		return false
	}
	return r.check(ctx, it, offset)
}

// core updates WordPress, then its database, in maintenance mode, always turned off at the end.
// WordPress's own update removes the maintenance file it writes while copying: the mode is turned
// on again for the database.
func (r *run) core(ctx context.Context, it Item) (ok bool) {
	if !r.maintenance(ctx, true) {
		return false
	}
	defer func() { ok = r.maintenance(ctx, false) && ok }()
	if !r.step(ctx, model.OperationStep{Step: model.StepUpdate, ItemKind: model.ItemCore, VersionBefore: it.From, VersionAfter: it.To},
		func() (string, error) { return "", r.wpDone(ctx, wpcli.UpdateCore(it.To)) }) {
		return false
	}
	if !r.maintenance(ctx, true) {
		return false
	}
	return r.step(ctx, model.OperationStep{Step: model.StepUpdateDatabase}, func() (string, error) {
		return "", r.wpDone(ctx, wpcli.UpdateDatabase(r.s.job.Site.Multisite))
	})
}

// maintenance turns the maintenance mode on or off; already so is no failure.
func (r *run) maintenance(ctx context.Context, on bool) bool {
	st := model.OperationStep{Step: model.StepMaintenanceOff}
	if on {
		st.Step = model.StepMaintenanceOn
	}
	return r.step(ctx, st, func() (string, error) {
		res, err := r.s.run(ctx, wpcli.Maintenance(on))
		if err != nil {
			return "", err
		}
		return "", wpcli.MaintenanceSet(res)
	})
}

// extension updates one plugin or theme.
func (r *run) extension(ctx context.Context, it Item) (string, error) {
	of := wpcli.ExtensionPlugins
	if it.Kind == model.ItemTheme {
		of = wpcli.ExtensionThemes
	}
	res, err := r.s.run(ctx, wpcli.UpdateOne(of, it.Slug))
	if err != nil {
		return "", err
	}
	u, err := wpcli.ParseUpdateOne(res)
	if err != nil {
		return "", err
	}
	switch u.Outcome {
	case wpcli.UpdateFailed:
		return "", fmt.Errorf("%w: %s", wpcli.ErrFailed, u.Message)
	case wpcli.UpdateNothing:
		return "already up to date", nil
	}
	return "", nil
}

// translations updates the translations of WordPress, the plugins and the themes.
func (r *run) translations(ctx context.Context) bool {
	for _, of := range []wpcli.Languages{wpcli.LanguagesOfCore, wpcli.LanguagesOfPlugins, wpcli.LanguagesOfThemes} {
		ok := r.step(ctx, model.OperationStep{Step: model.StepUpdate, ItemKind: model.ItemLanguage, Slug: string(of)}, func() (string, error) {
			return "", r.wpDone(ctx, wpcli.UpdateLanguages(of))
		})
		if !ok {
			return false
		}
	}
	return true
}

// check watches the site after an item: its pages, WordPress still installed, and no new fatal
// error in debug.log since offset; a negative offset skips the log (after a rollback, requests to
// the code still in OPcache may log errors of the version just removed).
func (r *run) check(ctx context.Context, it Item, offset int64) bool {
	return r.step(ctx, model.OperationStep{Step: model.StepCheck, ItemKind: it.Kind, Slug: it.Slug}, func() (string, error) {
		if res := r.s.u.Checker.Watch(ctx, r.urls()); res.Problem != httpcheck.ProblemNone {
			return "", checkFailure(res)
		}
		if it.Kind == model.ItemCore {
			res, err := r.s.run(ctx, wpcli.CoreIsInstalled(false))
			if err != nil {
				return "", err
			}
			if installed, err := wpcli.IsInstalled(res); err != nil || !installed {
				return "", errors.Join(errors.New("WordPress no longer answers as installed"), err)
			}
		}
		if offset < 0 {
			return "", nil
		}
		if fatals := r.fatalsSince(ctx, offset); len(fatals) > 0 {
			return "", fmt.Errorf("new fatal error in debug.log: %s", strings.Join(fatals, "\n"))
		}
		return "", nil
	})
}

func checkFailure(res httpcheck.Result) error {
	switch res.Problem {
	case httpcheck.ProblemUnreachable:
		return fmt.Errorf("%s does not answer: %s", res.URL, res.Detail)
	case httpcheck.ProblemCriticalError:
		return fmt.Errorf("%s shows the critical error page of WordPress (status %d)", res.URL, res.Status)
	}
	return fmt.Errorf("%s answers %d (%s)", res.URL, res.Status, res.Problem)
}

// urls are the pages checked: the home and login pages of the site, and of its pinned subsites.
func (r *run) urls() []string {
	home := strings.TrimSuffix(r.s.job.Site.URL, "/")
	urls := []string{home + "/", home + "/wp-login.php"}
	for _, sub := range r.s.job.Site.Subsites {
		if sub.Pinned {
			urls = append(urls, strings.TrimSuffix(sub.URL, "/")+"/")
		}
	}
	return urls
}

// logSize is the size of debug.log before an item, 0 when WordPress keeps no log.
func (r *run) logSize(ctx context.Context) int64 {
	if r.debugLog == "" {
		return 0
	}
	line, err := remote.FileSize(r.debugLog)
	if err != nil {
		return 0
	}
	res, err := r.s.shell(ctx, line)
	if err != nil {
		return 0
	}
	n, err := remote.ParseSize(res)
	if err != nil {
		return 0
	}
	return n
}

// fatalsSince returns the fatal errors written to debug.log after offset.
func (r *run) fatalsSince(ctx context.Context, offset int64) []string {
	if r.debugLog == "" {
		return nil
	}
	line, err := remote.FatalsSince(r.debugLog, offset)
	if err != nil {
		return nil
	}
	res, err := r.s.shell(ctx, line)
	if err != nil {
		return nil
	}
	return remote.ParseFatals(res)
}

// prune removes the backups of older runs, keeping the last few; a failure is ignored.
func (r *run) prune(ctx context.Context) {
	line, err := remote.ListBackups(r.s.job.Site.ID)
	if err != nil {
		return
	}
	res, err := r.s.shell(ctx, line)
	if err != nil || res.ExitCode != 0 {
		return
	}
	runs := remote.ParseBackups(res.Stdout)
	for _, old := range runs[:max(0, len(runs)-keptBackups)] {
		line, err := remote.RemoveBackup(path.Dir(r.op.BackupDir) + "/" + old)
		if err != nil {
			continue
		}
		// A backup left behind is removed at the next run.
		if _, err := r.s.shell(ctx, line); err != nil {
			return
		}
	}
}

// step records a step before and after it runs; false when it failed. A step that cannot be
// recorded does not run: Vigie writes nothing on a server it cannot account for.
func (r *run) step(ctx context.Context, st model.OperationStep, fn func() (string, error)) bool {
	st.Position = len(r.op.Steps)
	st.Started, st.Outcome = r.s.u.Now(), model.StepRunning
	r.op.Steps = append(r.op.Steps, st)
	if err := r.s.u.Store.SaveOperationStep(context.WithoutCancel(ctx), r.op.ID, st); err != nil {
		st.Outcome, st.Message = model.StepFailed, "not recorded: "+err.Error()
		r.op.Steps[st.Position] = st
		return false
	}
	r.report(st)
	msg, err := fn()
	st.Finished, st.Outcome, st.Message = r.s.u.Now(), model.StepDone, msg
	if err != nil {
		st.Outcome, st.Message = model.StepFailed, err.Error()
	}
	r.op.Steps[st.Position] = st
	if saveErr := r.s.u.Store.SaveOperationStep(context.WithoutCancel(ctx), r.op.ID, st); saveErr != nil {
		return false
	}
	r.report(st)
	return err == nil
}

// report tells OnStep about a step, when it is set.
func (r *run) report(st model.OperationStep) {
	if r.s.u.OnStep != nil {
		r.s.u.OnStep(*r.op, st)
	}
}

// fail records a step that failed before it could start.
func (r *run) fail(ctx context.Context, st model.OperationStep, err error) {
	r.step(ctx, st, func() (string, error) { return "", err })
}

// shellIn returns a function that runs a command line built by package remote and says whether it
// worked: exec(remote.MakeDir(dir)).
func (r *run) shellIn(ctx context.Context) func(line string, err error) error {
	return func(line string, err error) error {
		if err != nil {
			return err
		}
		res, err := r.s.shell(ctx, line)
		if err != nil {
			return err
		}
		return wpcli.Done(res)
	}
}

func (r *run) wpDone(ctx context.Context, cmd wpcli.Command) error {
	res, err := r.s.run(ctx, cmd)
	if err != nil {
		return err
	}
	return wpcli.Done(res)
}
