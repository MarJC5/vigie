// Package scan reads the state of a site over SSH with WP-CLI: versions, pending updates and
// translations, and whether each update check really reached wordpress.org. It only reads.
package scan

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/MarJC5/vigie/internal/model"
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
}

// steps is the number of commands of a scan, for the progress: 14, and the subsites of a network.
// A theme in a plugin's folder takes two more, read again with the plugins.
func (j Job) steps() int {
	if j.Site.Multisite {
		return 15
	}
	return 14
}

// Site scans one site. The scan is always returned, also when it failed or was cancelled; for a
// network, the subsites read are returned too.
func (s Scanner) Site(ctx context.Context, job Job, progress func(Progress)) (model.Scan, []model.Subsite) {
	r := &run{scanner: s, job: job, progress: progress}
	result := model.Scan{ID: model.NewID(), SiteID: job.Site.ID, Started: s.Now()}
	subsites, err := r.scan(ctx, &result)
	result.Finished = s.Now()
	var connErr *sshx.ConnectionError
	switch {
	case err == nil:
		result.Outcome = model.ScanCompleted
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		result.Outcome, result.Error = model.ScanCancelled, "cancelled"
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

// run is one scan in progress.
type run struct {
	scanner  Scanner
	job      Job
	progress func(Progress)
	done     int
	// themesInPlugins: the active theme lives in a plugin's folder, so the theme reads load the
	// plugins.
	themesInPlugins bool
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
	var res sshx.Result
	err = r.scanner.Limiter.Do(ctx, r.job.HostKey, r.job.Max, func() error {
		var runErr error
		res, runErr = r.scanner.Runner.Run(ctx, r.job.Target, remote)
		return runErr
	})
	r.done++
	if r.progress != nil {
		r.progress(Progress{SiteID: r.job.Site.ID, Done: min(r.done, r.job.steps()), Total: r.job.steps()})
	}
	return res, err
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

	if err := r.core(ctx, result); err != nil {
		return nil, err
	}
	if err := r.extensions(ctx, result); err != nil {
		return nil, err
	}
	if err := r.languages(ctx, result); err != nil {
		return nil, err
	}
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
		r.progress(Progress{SiteID: r.job.Site.ID, Done: r.job.steps(), Total: r.job.steps()})
	}
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
		for _, e := range list {
			result.Items = append(result.Items, model.ScanItem{
				Kind: kind.item, Slug: e.Name, Title: e.Title, Status: e.Status, Version: e.Version,
				Update: updateState(e.Update), UpdateVersion: e.UpdateVersion,
				AutoUpdate: e.AutoUpdate, OnWordPressOrg: kind.item == model.ItemPlugin && e.OnWordPressOrg,
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

// languages reads the translation updates; WP-CLI fails when wordpress.org cannot be reached,
// which leaves the check unproven rather than stopping the scan.
func (r *run) languages(ctx context.Context, result *model.Scan) error {
	result.LanguagesChecked = true
	for _, of := range []wpcli.Languages{wpcli.LanguagesOfCore, wpcli.LanguagesOfPlugins, wpcli.LanguagesOfThemes} {
		cmd := wpcli.LanguageUpdates(of)
		if of == wpcli.LanguagesOfThemes && r.themesInPlugins {
			cmd = cmd.WithPlugins()
		}
		out, err := r.read(ctx, cmd)
		if errors.Is(err, wpcli.ErrCannotLoad) {
			result.LanguagesChecked = false
			continue
		}
		if err != nil {
			return err
		}
		updates, err := wpcli.ParseLanguageUpdates(out)
		if err != nil {
			return err
		}
		for _, u := range updates {
			result.Items = append(result.Items, model.ScanItem{
				Kind: model.ItemLanguage, Slug: u.Item, Language: u.Language, Update: model.UpdateAvailable,
			})
		}
	}
	return nil
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
