package scan_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

// replay answers every command of a scan of fixture with its recorded output.
func replay(t *testing.T, fixture string) *sshxtest.Fake {
	t.Helper()
	fake := sshxtest.NewFake()
	wpclitest.Replay(t, fake, fixture, "~/sites/"+fixture, "wp")
	return fake
}

func job(fixture string) scan.Job {
	return scan.Job{
		Site:   model.Site{ID: model.NewID(), Name: fixture + ".example.test", Path: "~/sites/" + fixture},
		Target: sshx.Target{Alias: "bench"}, HostKey: "host-1", Max: 2, WPCLI: "wp",
	}
}

func scanner(runner sshx.Runner) scan.Scanner {
	return scan.Scanner{Runner: runner, Limiter: sshx.NewLimiter(), Now: ticking(time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC))}
}

// ticking is a clock that moves a second each time it is read, from start; the lookups read it
// at the same time as the scan.
func ticking(start time.Time) func() time.Time {
	var mu sync.Mutex
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		start = start.Add(time.Second)
		return start
	}
}

func item(s model.Scan, kind model.ItemKind, slug string) (model.ScanItem, bool) {
	for _, i := range s.Items {
		if i.Kind == kind && i.Slug == slug {
			return i, true
		}
	}
	return model.ScanItem{}, false
}

func TestScanOfAnOutdatedSite(t *testing.T) {
	j := job("outdated")
	got, _ := scanner(replay(t, "outdated")).Site(context.Background(), j, nil)

	if got.Outcome != model.ScanCompleted || got.SiteID != j.Site.ID || got.ID == model.NoID {
		t.Fatalf("scan = %+v, want a completed scan of the site", got)
	}
	if got.WordPress != "7.1.3" || got.WPCLI != "2.12.0" || !strings.HasPrefix(got.PHPCLI, "8.4.") || got.Home != "https://outdated.example.test" {
		t.Errorf("versions = %q, %q, %q, %q", got.WordPress, got.WPCLI, got.PHPCLI, got.Home)
	}
	if !got.CoreChecked || !got.PluginsChecked || !got.ThemesChecked || !got.LanguagesChecked {
		t.Errorf("checks = %v %v %v %v, want all proven", got.CoreChecked, got.PluginsChecked, got.ThemesChecked, got.LanguagesChecked)
	}
	if !got.Finished.After(got.Started) {
		t.Errorf("started %v, finished %v", got.Started, got.Finished)
	}
	if core, _ := item(got, model.ItemCore, ""); core.Update != model.UpdateAvailable || core.UpdateVersion != "7.1.4" || core.Version != "7.1.3" {
		t.Errorf("core = %+v, want 7.1.3 with 7.1.4 available", core)
	}
	if akismet, _ := item(got, model.ItemPlugin, "akismet"); akismet.Update != model.UpdateAvailable || !akismet.OnWordPressOrg {
		t.Errorf("akismet = %+v", akismet)
	}
	if premium, _ := item(got, model.ItemPlugin, "fixture-premium"); !premium.UpdateSourceUnknown() {
		t.Errorf("fixture-premium = %+v, want its update source unknown", premium)
	}
	if theme, _ := item(got, model.ItemTheme, "twentytwentyfour"); theme.Update != model.UpdateAvailable {
		t.Errorf("twentytwentyfour = %+v", theme)
	}
	if counts := got.PendingUpdates(); counts != (model.UpdateCounts{Core: 1, Plugins: 1, Themes: 1, Languages: 2}) {
		t.Errorf("PendingUpdates() = %+v", counts)
	}
	// The bench's image adds wp-config-docker.php where only WordPress's own files belong.
	if got.Status() != model.StatusUrgent || find(t, got, model.KindCoreFile, "core", "wp-config-docker.php").Level != model.LevelUrgent {
		t.Errorf("Status() = %q, want urgent for the added file", got.Status())
	}
}

func TestScanWithoutOutboundHTTPNeverSaysUpToDate(t *testing.T) {
	got, _ := scanner(replay(t, "offline")).Site(context.Background(), job("offline"), nil)
	if got.Outcome != model.ScanCompleted {
		t.Fatalf("outcome = %q (%s), want completed", got.Outcome, got.Error)
	}
	if got.CoreChecked || got.PluginsChecked || got.ThemesChecked || got.LanguagesChecked {
		t.Errorf("checks = %v %v %v %v, want none proven", got.CoreChecked, got.PluginsChecked, got.ThemesChecked, got.LanguagesChecked)
	}
	if got.Status() == model.StatusUpToDate {
		t.Errorf("Status() = %q, want anything but up to date", got.Status())
	}
}

func TestScanOfASiteThatCannotLoad(t *testing.T) {
	got, _ := scanner(replay(t, "broken")).Site(context.Background(), job("broken"), nil)
	if got.Outcome != model.ScanCannotLoad || !strings.Contains(got.Error, "Error establishing a database connection") {
		t.Fatalf("scan = %q, %q; want cannot-load with WP-CLI's message", got.Outcome, got.Error)
	}
	if got.Status() != model.StatusUnknown {
		t.Errorf("Status() = %q, want unknown", got.Status())
	}
}

// A site removed from the server: no WordPress at its path any more.
func TestScanOfASiteNoLongerThere(t *testing.T) {
	gone := sshx.Result{ExitCode: 1, Stderr: []byte("Error: This does not seem to be a WordPress installation.\nThe used path is: /home/u/sites/gone/")}
	got, _ := scanner(answering{gone}).Site(context.Background(), job("plain"), nil)
	if got.Outcome != model.ScanCannotLoad || got.Failure != scan.FailureNotWordPress {
		t.Fatalf("scan = %q, %q; want cannot-load, %s", got.Outcome, got.Failure, scan.FailureNotWordPress)
	}
}

func TestScanOfANetworkReadsItsSubsites(t *testing.T) {
	j := job("network")
	j.Site.Multisite = true
	got, subsites := scanner(replay(t, "network")).Site(context.Background(), j, nil)
	if got.Outcome != model.ScanCompleted || len(subsites) != 4 {
		t.Fatalf("scan = %q (%s), %d subsites; want completed with 4", got.Outcome, got.Error, len(subsites))
	}
}

func TestScanStopsOnConnectionFailure(t *testing.T) {
	refused := &sshx.ConnectionError{Failure: sshx.FailureRefused, Message: "Connection refused"}
	got, _ := scanner(failing{refused}).Site(context.Background(), job("plain"), nil)
	if got.Outcome != model.ScanFailed || got.Failure != string(sshx.FailureRefused) || got.Error == "" {
		t.Fatalf("scan = %+v, want failed, refused", got)
	}
}

func TestCancelledScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, _ := scanner(failing{context.Canceled}).Site(ctx, job("plain"), nil)
	if got.Outcome != model.ScanCancelled {
		t.Fatalf("outcome = %q, want cancelled", got.Outcome)
	}
}

func TestProgressCountsTheSteps(t *testing.T) {
	var steps []scan.Progress
	scanner(replay(t, "plain")).Site(context.Background(), job("plain"), func(p scan.Progress) { steps = append(steps, p) })
	if len(steps) == 0 {
		t.Fatal("no progress reported")
	}
	last := steps[len(steps)-1]
	// 14 commands for the versions and updates; for security, 4 to compare the files, then the
	// administrators and the one's two-factor, the constants' names and WP_DEBUG, the search
	// engines, WP-Cron, the options and transients, the database version: 27, then the end.
	if last.Total != 27 || len(steps) != 28 {
		t.Errorf("progress = %d reports, total %d; want 27 commands and the end", len(steps), last.Total)
	}
	if last.Done != last.Total || !slices.IsSortedFunc(steps, func(a, b scan.Progress) int { return a.Done - b.Done }) {
		t.Fatalf("progress = %+v, want increasing up to the total", steps)
	}
}

// failing answers every command with the same error.
// answering gives every command the same result.
type answering struct{ res sshx.Result }

func (a answering) Run(context.Context, sshx.Target, string) (sshx.Result, error) {
	return a.res, nil
}

type failing struct{ err error }

func (f failing) Run(context.Context, sshx.Target, string) (sshx.Result, error) {
	return sshx.Result{}, f.err
}

func TestUnreadableOutputIsClassified(t *testing.T) {
	fake := replay(t, "plain")
	remote, err := wpcli.PluginList().Remote("wp", "~/sites/plain")
	if err != nil {
		t.Fatal(err)
	}
	fake.Set(remote, sshx.Result{Stdout: []byte(`{"not":"a list"}`)})
	got, _ := scanner(fake).Site(context.Background(), job("plain"), nil)
	if got.Outcome != model.ScanFailed || got.Failure != scan.FailureUnreadable || !strings.Contains(got.Error, "plugin list") {
		t.Fatalf("scan = %q, failure %q, %q; want failed, unreadable, with the message", got.Outcome, got.Failure, got.Error)
	}
}

// A theme can live in a plugin's folder (register_theme_directory): read without the plugins,
// WordPress sees no theme. The scan reads the themes again with the plugins.
func TestScanOfAThemeInAPlugin(t *testing.T) {
	got, _ := scanner(replay(t, "plugin-theme")).Site(context.Background(), job("plugin-theme"), nil)
	if got.Outcome != model.ScanCompleted {
		t.Fatalf("scan = %q (%s), want completed", got.Outcome, got.Error)
	}
	theme, ok := item(got, model.ItemTheme, "fixture-theme")
	if !ok || theme.Status != "active" || theme.Version != "1.0.0" {
		t.Errorf("fixture-theme = %+v, %v; want the active theme read with the plugins", theme, ok)
	}
	if !got.ThemesChecked || !got.LanguagesChecked {
		t.Errorf("checks: themes %v, languages %v; want both proven", got.ThemesChecked, got.LanguagesChecked)
	}
}

// When loading the plugins fails, the active theme is listed with an unknown state and the theme
// check stays unproven; the rest of the scan completes.
func TestScanOfAThemeInABrokenPlugin(t *testing.T) {
	fake := replay(t, "plugin-theme")
	broken := sshx.Result{ExitCode: 255, Stderr: []byte("PHP Fatal error: Uncaught Error in fixture-themes.php")}
	for _, cmd := range []wpcli.Command{wpcli.ThemeList().WithPlugins(), wpcli.LanguageUpdates(wpcli.LanguagesOfThemes).WithPlugins()} {
		remote, err := cmd.Remote("wp", "~/sites/plugin-theme")
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(remote, broken)
	}
	got, _ := scanner(fake).Site(context.Background(), job("plugin-theme"), nil)
	if got.Outcome != model.ScanCompleted {
		t.Fatalf("scan = %q (%s), want completed", got.Outcome, got.Error)
	}
	theme, ok := item(got, model.ItemTheme, "fixture-theme")
	if !ok || theme.Status != "active" || theme.Update != model.UpdateUnknown {
		t.Errorf("fixture-theme = %+v, %v; want the active theme with an unknown state", theme, ok)
	}
	if got.ThemesChecked || !got.PluginsChecked {
		t.Errorf("checks: themes %v, plugins %v; want themes unproven only", got.ThemesChecked, got.PluginsChecked)
	}
}
