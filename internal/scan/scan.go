// Package scan reads the state of a site over SSH with WP-CLI: versions, pending updates and
// translations, whether each update check really reached wordpress.org, and what a security scan
// finds, with outside lookups made from the user's machine. It only reads.
package scan

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// Job is one site to scan, with what its host gives it.
type Job struct {
	Site    model.Site
	Target  sshx.Target
	HostKey string // shares the host's limit of commands at a time (the host ID)
	Max     int    // commands at a time on the host
	WPCLI   string // the site's effective WP-CLI command
	// Previous is the site's latest scan, whose comparison of the files is carried over while it
	// is less than a day old; CheckFiles asks for a new one anyway.
	Previous   model.Scan
	CheckFiles bool
}

// Progress tells how far a scan is.
type Progress struct {
	SiteID      model.ID
	Done, Total int
}

// Scanner runs scans.
type Scanner struct {
	Runner  sshx.Runner
	Limiter *sshx.Limiter
	Now     func() time.Time
	// Lookups makes the outside lookups; nil makes none, and their checks are "not checked".
	Lookups Lookups
}

// filesSteps are the commands of the comparison of the files: WordPress's and the plugins'
// checksums, the PHP files in its uploads.
const filesSteps = 3

// steps is the number of commands of a scan known before it starts, for the progress: 25; a
// network reads its subsites' list too, and its administrators in one go (ADR-0029). The comparison
// of the files, each administrator of a single site, each constant read and Wordfence add theirs
// once known. A theme in a plugin's folder takes one more, the themes read again with the plugins.
func (j Job) steps() int {
	if j.Site.Multisite {
		return 26
	}
	return 25
}

// Site scans one site. The scan is always returned, also when it failed or was cancelled; for a
// network, the subsites read are returned too.
func (s Scanner) Site(ctx context.Context, job Job, progress func(Progress)) (model.Scan, []model.Subsite) {
	r := &run{scanner: s, job: job, progress: progress, total: job.steps()}
	result := model.Scan{ID: model.NewID(), SiteID: job.Site.ID, Started: s.Now()}
	subsites, err := r.scan(ctx, &result)
	result.Finished = s.Now()
	var connErr *sshx.ConnectionError
	switch {
	case err == nil:
		result.Outcome = model.ScanCompleted
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		result.Outcome, result.Error = model.ScanCancelled, "cancelled"
	case errors.Is(err, wpcli.ErrNotWordPress):
		result.Outcome, result.Failure, result.Error = model.ScanCannotLoad, FailureNotWordPress, err.Error()
	case errors.Is(err, wpcli.ErrCannotLoad), errors.Is(err, errNotInstalled):
		result.Outcome, result.Error = model.ScanCannotLoad, err.Error()
	case errors.As(err, &connErr):
		result.Outcome, result.Failure, result.Error = model.ScanFailed, string(connErr.Failure), connErr.Message
	case errors.Is(err, wpcli.ErrUnexpectedOutput):
		result.Outcome, result.Failure, result.Error = model.ScanFailed, FailureUnreadable, err.Error()
	default:
		result.Outcome, result.Error = model.ScanFailed, err.Error()
	}
	if result.Outcome != model.ScanCompleted {
		subsites = nil
	}
	return result, subsites
}

var errNotInstalled = errors.New("WordPress is not installed at this path")

// FailureUnreadable is the failure of a scan whose WP-CLI output Vigie could not read; its text
// in the catalogs is "failure.unreadable".
const FailureUnreadable = "unreadable"

// FailureNotWordPress is why a site could not load when its folder is gone or holds no WordPress
// any more: most likely removed from the server. Its text in the catalogs is "outcome.notWordPress".
const FailureNotWordPress = "notWordPress"

// run is one scan in progress.
type run struct {
	scanner  Scanner
	job      Job
	progress func(Progress)
	done     int
	total    int // grows as the scan learns what it reads
	// themesInPlugins: the active theme lives in a plugin's folder, so the theme reads load the
	// plugins.
	themesInPlugins bool
	root            string // the site's real folder, once read
	// languageUpdates are the translation updates read with the update checks.
	languageUpdates []model.ScanItem
}

// exec runs one WP-CLI command within the host's limit and reports the progress.
func (r *run) exec(ctx context.Context, cmd wpcli.Command, path string) (sshx.Result, error) {
	remote, err := cmd.Remote(r.job.WPCLI, path)
	if err != nil {
		return sshx.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return sshx.Result{}, err
	}
	return r.run(ctx, remote)
}

// run runs one command line within the host's limit and reports the progress.
func (r *run) run(ctx context.Context, remote string) (sshx.Result, error) {
	return r.runInput(ctx, remote, nil)
}

// runInput is run with a fixed text of Vigie's on the command's standard input.
func (r *run) runInput(ctx context.Context, remote string, input []byte) (sshx.Result, error) {
	var res sshx.Result
	err := r.scanner.Limiter.Do(ctx, r.job.HostKey, r.job.Max, func() error {
		var runErr error
		if input != nil {
			res, runErr = r.scanner.Runner.RunInput(ctx, r.job.Target, remote, input)
		} else {
			res, runErr = r.scanner.Runner.Run(ctx, r.job.Target, remote)
		}
		return runErr
	})
	r.done++
	if r.progress != nil {
		r.progress(Progress{SiteID: r.job.Site.ID, Done: min(r.done, r.total), Total: r.total})
	}
	return res, err
}

// shell runs a shell command of the remote package and returns its output; a failure is an error.
func (r *run) shell(ctx context.Context, line func() (string, error)) ([]byte, error) {
	remote, err := line()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res, err := r.run(ctx, remote)
	if err != nil {
		return nil, err
	}
	if err := wpcli.Done(res); err != nil {
		return nil, err
	}
	return res.Stdout, nil
}

// read runs a read command of the site and returns its output; a WP-CLI error is an error.
func (r *run) read(ctx context.Context, cmd wpcli.Command) ([]byte, error) {
	res, err := r.exec(ctx, cmd, r.job.Site.Path)
	if err != nil {
		return nil, err
	}
	if err := wpcli.Check(res); err != nil {
		return nil, err
	}
	return res.Stdout, nil
}

func (r *run) scan(ctx context.Context, result *model.Scan) ([]model.Subsite, error) {
	defer r.finish()
	res, err := r.exec(ctx, wpcli.CoreIsInstalled(false), r.job.Site.Path)
	if err != nil {
		return nil, err
	}
	installed, err := wpcli.IsInstalled(res)
	if err != nil {
		return nil, err
	}
	if !installed {
		return nil, errNotInstalled
	}

	res, err = r.exec(ctx, wpcli.CLIInfo(), "")
	if err != nil {
		return nil, err
	}
	if err := wpcli.Check(res); err != nil {
		return nil, err
	}
	info, err := wpcli.ParseCLIInfo(res.Stdout)
	if err != nil {
		return nil, err
	}
	result.PHPCLI, result.WPCLI = info.PHPVersion, info.WPCLIVersion

	out, err := r.read(ctx, wpcli.CoreVersion())
	if err != nil {
		return nil, err
	}
	if result.WordPress, err = wpcli.ParseCoreVersion(out); err != nil {
		return nil, err
	}
	if out, err = r.read(ctx, wpcli.OptionGet("home")); err != nil {
		return nil, err
	}
	if result.Home, err = wpcli.ParseHome(out); err != nil {
		return nil, err
	}
	if out, err = r.read(ctx, wpcli.OptionGet("siteurl")); err != nil {
		return nil, err
	}
	if result.SiteURL, err = wpcli.ParseSiteURL(out); err != nil {
		return nil, err
	}

	if err := r.core(ctx, result); err != nil {
		return nil, err
	}
	if err := r.extensions(ctx, result); err != nil {
		return nil, err
	}
	r.languages(result)
	// Before the outside lookups, which check the PHP serving the pages when it is known.
	if err := r.phpWeb(ctx, result); err != nil {
		return nil, err
	}

	// The outside lookups run from the machine while the server is read.
	found := make(chan lookupsDone, 1)
	snapshot := *result
	go func() { found <- r.scanner.lookups(ctx, snapshot) }()
	err = r.security(ctx, result)
	done := <-found
	result.Findings = append(result.Findings, done.findings...)
	sameName(result.Findings)
	sortFindings(result.Findings)
	// wordpress.org's own answer, from the machine, replaces what the server read: a server that
	// cannot reach wordpress.org reads every plugin as closed. Without an answer, the server's stays.
	for i, item := range result.Items {
		if known, ok := done.onWordPressOrg[item.Slug]; ok && item.Kind == model.ItemPlugin {
			result.Items[i].OnWordPressOrg = known
		}
	}
	if err != nil {
		return nil, err
	}
	r.backups(ctx, result)
	if !r.job.Site.Multisite {
		return nil, nil
	}
	if out, err = r.read(ctx, wpcli.SiteList()); err != nil {
		return nil, err
	}
	return wpcli.ParseSites(out)
}

// finish reports the end, whatever the number of steps run.
func (r *run) finish() {
	if r.progress != nil {
		r.progress(Progress{SiteID: r.job.Site.ID, Done: r.total, Total: r.total})
	}
}

// sameName makes the closure of a plugin on wordpress.org only shown when its files say it is
// another plugin of the same name: the closed one is not the one installed.
func sameName(findings []model.Finding) {
	other := map[string]bool{}
	for _, f := range findings {
		if f.Kind == model.KindPluginFile && f.Facts["mismatch"] == "true" {
			other[f.Subject] = true
		}
	}
	for i, f := range findings {
		if f.Kind == model.KindPluginClosed && other[f.Subject] {
			findings[i].Level = model.LevelInfo
			findings[i].Facts["other-plugin"] = "true"
		}
	}
}

// levelOrder ranks the levels, the worst first.
var levelOrder = []model.FindingLevel{model.LevelUrgent, model.LevelAttention, model.LevelNotChecked, model.LevelInfo}

// sortFindings orders the findings the worst first, then by key, the same from one scan to the next.
func sortFindings(findings []model.Finding) {
	slices.SortStableFunc(findings, func(a, b model.Finding) int {
		if c := cmp.Compare(slices.Index(levelOrder, a.Level), slices.Index(levelOrder, b.Level)); c != 0 {
			return c
		}
		// The super administrators of a network before the other administrators (ADR-0029).
		if a.Kind == model.KindAdministrator && b.Kind == model.KindAdministrator {
			if c := cmp.Compare(b.Facts["super"], a.Facts["super"]); c != 0 {
				return c
			}
		}
		return cmp.Compare(a.Key(), b.Key())
	})
}

// core reads the WordPress releases, then whether the check reached wordpress.org.
func (r *run) core(ctx context.Context, result *model.Scan) error {
	out, err := r.read(ctx, wpcli.CoreCheckUpdate())
	if err != nil {
		return err
	}
	offers, err := wpcli.ParseCoreUpdates(out)
	if err != nil {
		return err
	}
	if out, err = r.read(ctx, wpcli.UpdateData(wpcli.UpdatesOfCore)); err != nil {
		return err
	}
	result.CoreChecked = wpcli.CoreCheckProven(out, result.WordPress)
	if err := r.translations(out); err != nil {
		return err
	}
	if len(offers) == 0 {
		result.Items = append(result.Items, model.ScanItem{Kind: model.ItemCore, Version: result.WordPress, Update: model.UpdateNone})
	}
	for _, o := range offers {
		result.Items = append(result.Items, model.ScanItem{
			Kind: model.ItemCore, Status: o.Type, Version: result.WordPress,
			Update: model.UpdateAvailable, UpdateVersion: o.Version,
		})
	}
	return nil
}

// extensions reads the plugins and themes, each list followed by the proof of its check.
func (r *run) extensions(ctx context.Context, result *model.Scan) error {
	for _, kind := range []struct {
		list    func(context.Context) ([]wpcli.Extension, bool, error)
		data    wpcli.Updates
		item    model.ItemKind
		checked *bool
	}{
		{r.plugins, wpcli.UpdatesOfPlugins, model.ItemPlugin, &result.PluginsChecked},
		{r.themes, wpcli.UpdatesOfThemes, model.ItemTheme, &result.ThemesChecked},
	} {
		list, complete, err := kind.list(ctx)
		if err != nil {
			return err
		}
		out, err := r.read(ctx, wpcli.UpdateData(kind.data))
		if err != nil {
			return err
		}
		*kind.checked = complete && wpcli.ExtensionsCheckProven(out)
		if err := r.translations(out); err != nil {
			return err
		}
		known := map[string]bool{}
		if kind.item == model.ItemPlugin {
			known = wpcli.PluginsKnown(out)
		}
		for _, e := range list {
			result.Items = append(result.Items, model.ScanItem{
				Kind: kind.item, Slug: e.Name, Title: e.Title, Status: e.Status, Version: e.Version,
				Update: updateState(e.Update), UpdateVersion: e.UpdateVersion,
				AutoUpdate: e.AutoUpdate, OnWordPressOrg: known[e.Name],
			})
		}
	}
	return nil
}

// plugins reads the plugin list; it is always complete.
func (r *run) plugins(ctx context.Context) ([]wpcli.Extension, bool, error) {
	out, err := r.read(ctx, wpcli.PluginList())
	if err != nil {
		return nil, false, err
	}
	list, err := wpcli.ParsePlugins(out)
	return list, true, err
}

// themes reads the theme list without the plugins, then with them when the active theme is
// missing: it lives in a plugin's folder. When the plugins cannot be loaded, the list is
// incomplete and holds the active theme with an unknown state.
func (r *run) themes(ctx context.Context) ([]wpcli.Extension, bool, error) {
	out, err := r.read(ctx, wpcli.ThemeList())
	if err != nil {
		return nil, false, err
	}
	list, err := wpcli.ParseThemes(out)
	if err != nil {
		return nil, false, err
	}
	if out, err = r.read(ctx, wpcli.OptionGet("stylesheet")); err != nil {
		return nil, false, err
	}
	active, err := wpcli.ParseActiveTheme(out)
	if err != nil {
		return nil, false, err
	}
	if active == "" || slices.ContainsFunc(list, func(e wpcli.Extension) bool { return e.Name == active }) {
		return list, true, nil
	}
	r.themesInPlugins = true
	out, err = r.read(ctx, wpcli.ThemeList().WithPlugins())
	if errors.Is(err, wpcli.ErrCannotLoad) {
		return append(list, wpcli.Extension{Name: active, Status: "active", Update: wpcli.UpdateUnknown}), false, nil
	}
	if err != nil {
		return nil, false, err
	}
	list, err = wpcli.ParseThemes(out)
	return list, true, err
}

// backups reads the site's backups on the server, Vigie's own before each update (ADR-0033): one
// command that only reads. Unread, they stay unknown; the scan goes on.
func (r *run) backups(ctx context.Context, result *model.Scan) {
	line, err := remote.BackupSizes(r.job.Site.ID)
	if err != nil {
		return
	}
	res, err := r.run(ctx, line)
	if err != nil || res.ExitCode != 0 {
		return
	}
	result.Backups = []model.Backup{}
	for _, b := range remote.ParseBackupSizes(res.Stdout) {
		if at, err := time.Parse(remote.BackupTimeLayout, b.Run); err == nil {
			result.Backups = append(result.Backups, model.Backup{At: at, Bytes: b.Bytes})
		}
	}
}

// translations keeps the translation updates of an update check's data, for languages.
func (r *run) translations(data []byte) error {
	updates, err := wpcli.ParseUpdateTranslations(data)
	if err != nil {
		return err
	}
	for _, u := range updates {
		r.languageUpdates = append(r.languageUpdates, model.ScanItem{
			Kind: model.ItemLanguage, Slug: u.Item, Language: u.Language, Update: model.UpdateAvailable,
		})
	}
	return nil
}

// languages adds the translation updates WordPress stored with its checks of WordPress, the
// plugins and the themes (ADR-0032): proven when the three checks are.
func (r *run) languages(result *model.Scan) {
	result.Items = append(result.Items, r.languageUpdates...)
	result.LanguagesChecked = result.CoreChecked && result.PluginsChecked && result.ThemesChecked
}

func updateState(s wpcli.UpdateState) model.UpdateState {
	switch s {
	case wpcli.UpdateNone:
		return model.UpdateNone
	case wpcli.UpdateAvailable:
		return model.UpdateAvailable
	case wpcli.UpdateUnavailable:
		return model.UpdateUnavailable
	case wpcli.UpdateNewerThanSource:
		return model.UpdateNewerThanSource
	}
	return model.UpdateUnknown
}
