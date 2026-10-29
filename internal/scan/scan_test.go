package scan_test

import (
	"context"
	"slices"
	"strings"
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
	clock := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	return scan.Scanner{Runner: runner, Limiter: sshx.NewLimiter(), Now: func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}}
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
	if got.Status() != model.StatusAttention {
		t.Errorf("Status() = %q, want needs attention", got.Status())
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
	if got.Status() != model.StatusAttention {
		t.Errorf("Status() = %q, want needs attention", got.Status())
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
	if last.Total != 13 || len(steps) != 14 { // each command, then the end
		t.Errorf("progress = %d reports, total %d; want 13 commands and the end", len(steps), last.Total)
	}
	if last.Done != last.Total || !slices.IsSortedFunc(steps, func(a, b scan.Progress) int { return a.Done - b.Done }) {
		t.Fatalf("progress = %+v, want increasing up to the total", steps)
	}
}

// failing answers every command with the same error.
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
