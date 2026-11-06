// Package update updates a site: a preview of what would change, then backups on the server, one
// item at a time with checks after each, a rollback when an item breaks the site, every step
// recorded.
package update

import (
	"context"
	"fmt"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// Job is one site to update, with what its host gives it.
type Job struct {
	Site    model.Site
	Target  sshx.Target
	HostKey string // shares the host's limit of commands at a time (the host ID)
	Max     int    // commands at a time on the host
	WPCLI   string // the site's effective WP-CLI command
	// KeepFor is how long a run's backups stay on the server, 0 for no limit; the last KeptBackups
	// runs of a site at most, whatever the limit (ADR-0033).
	KeepFor time.Duration
	// Siblings are the other sites of this Vigie on the account, whose backups the limits apply to
	// at the end of an update too.
	Siblings []model.ID
	// AutoRollback puts back on its own a plugin or theme whose check fails, then goes on with the
	// next items (ADR-0042); off, the run stops there and the rollback is offered.
	AutoRollback bool
	// By is who started the update, the window when empty; ByClient the assistant's client
	// (ADR-0043). Its operations, a rollback on its own among them, carry them.
	By       model.RunBy
	ByClient string
}

// Updater previews and runs updates.
type Updater struct {
	Runner  sshx.Runner
	Limiter *sshx.Limiter
	Store   Store   // records every operation and step
	Checker Checker // watches the pages after each item
	Now     func() time.Time
	PID     int // this process, so an operation left by a process that ended is told apart
	// OnStep, when set, follows the steps as they start and end, for example in the window.
	OnStep func(op model.Operation, step model.OperationStep)
}

// Item is one thing an update changes: WordPress, a plugin, a theme or a translation.
type Item struct {
	Kind     model.ItemKind
	Slug     string // plugin or theme; empty for WordPress and its translations
	Language string // translations only, for example "fr_FR"
	From, To string // versions; empty for translations
}

// Preview is what an update of a site would change.
type Preview struct {
	Core      *Item // nil when WordPress is up to date
	Plugins   []Item
	Themes    []Item
	Languages []Item
	// DumpTool tells whether the server can dump the database; without it, nothing is updated.
	DumpTool bool
}

// All selects everything the preview offers, as a bulk run does.
func All(p Preview) Selection {
	var sel Selection
	sel.Core = p.Core != nil
	for _, it := range p.Plugins {
		sel.Plugins = append(sel.Plugins, it.Slug)
	}
	for _, it := range p.Themes {
		sel.Themes = append(sel.Themes, it.Slug)
	}
	sel.Languages = len(p.Languages) > 0
	return sel
}

// Pending counts the items to update.
func (p Preview) Pending() int {
	n := len(p.Plugins) + len(p.Themes) + len(p.Languages)
	if p.Core != nil {
		n++
	}
	return n
}

// Preview reads what an update of the site would change, changing nothing.
func (u Updater) Preview(ctx context.Context, job Job) (Preview, error) {
	s := session{u: u, job: job}
	var p Preview

	out, err := s.wp(ctx, wpcli.CoreVersion())
	if err != nil {
		return p, err
	}
	installed, err := wpcli.ParseCoreVersion(out)
	if err != nil {
		return p, err
	}
	if out, err = s.wp(ctx, wpcli.CoreCheckUpdate()); err != nil {
		return p, err
	}
	offers, err := wpcli.ParseCoreUpdates(out)
	if err != nil {
		return p, err
	}
	for _, o := range offers {
		if model.CompareVersions(o.Version, installed) > 0 && (p.Core == nil || model.CompareVersions(o.Version, p.Core.To) > 0) {
			p.Core = &Item{Kind: model.ItemCore, From: installed, To: o.Version}
		}
	}

	for _, kind := range []struct {
		of   wpcli.Extensions
		item model.ItemKind
		list *[]Item
	}{
		{wpcli.ExtensionPlugins, model.ItemPlugin, &p.Plugins},
		{wpcli.ExtensionThemes, model.ItemTheme, &p.Themes},
	} {
		if out, err = s.wp(ctx, wpcli.UpdatePreview(kind.of)); err != nil {
			return p, err
		}
		pending, err := wpcli.ParseUpdatePreview(out)
		if err != nil {
			return p, err
		}
		for _, e := range pending {
			*kind.list = append(*kind.list, Item{Kind: kind.item, Slug: e.Name, From: e.Version, To: e.UpdateVersion})
		}
	}

	for _, cmd := range []wpcli.Command{
		wpcli.LanguageUpdates(wpcli.LanguagesOfCore),
		wpcli.LanguageUpdates(wpcli.LanguagesOfPlugins),
		// A theme can live in a plugin's folder.
		wpcli.LanguageUpdates(wpcli.LanguagesOfThemes).WithPlugins(),
	} {
		if out, err = s.wp(ctx, cmd); err != nil {
			return p, err
		}
		updates, err := wpcli.ParseLanguageUpdates(out)
		if err != nil {
			return p, err
		}
		for _, l := range updates {
			p.Languages = append(p.Languages, Item{Kind: model.ItemLanguage, Slug: l.Item, Language: l.Language})
		}
	}

	res, err := s.shell(ctx, remote.DumpTool())
	if err != nil {
		return p, err
	}
	switch res.ExitCode {
	case 0:
		p.DumpTool = true
	case 1:
	default:
		return p, wpcli.Done(res)
	}
	return p, nil
}

// session runs the commands of one site within its host's limit.
type session struct {
	u   Updater
	job Job
}

// shell runs a command line on the account as is.
func (s session) shell(ctx context.Context, line string) (sshx.Result, error) {
	if err := ctx.Err(); err != nil {
		return sshx.Result{}, err
	}
	var res sshx.Result
	err := s.u.Limiter.Do(ctx, s.job.HostKey, s.job.Max, func() error {
		var runErr error
		res, runErr = s.u.Runner.Run(ctx, s.job.Target, line)
		return runErr
	})
	return res, err
}

// run runs a WP-CLI command on the site and returns its result as is.
func (s session) run(ctx context.Context, cmd wpcli.Command) (sshx.Result, error) {
	line, err := cmd.Remote(s.job.WPCLI, s.job.Site.Path)
	if err != nil {
		return sshx.Result{}, err
	}
	return s.shell(ctx, line)
}

// wp runs a WP-CLI command that reads and returns its output; a failure is ErrCannotLoad.
func (s session) wp(ctx context.Context, cmd wpcli.Command) ([]byte, error) {
	res, err := s.run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if err := wpcli.Check(res); err != nil {
		return nil, fmt.Errorf("%s: %w", s.job.Site.Name, err)
	}
	return res.Stdout, nil
}
