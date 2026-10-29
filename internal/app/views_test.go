package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// scanned stores the bench host with two sites, one scanned (needs attention), one never scanned.
func scanned(t *testing.T) (*app.SiteManager, model.Host, model.Site, model.Site) {
	t.Helper()
	ctx := context.Background()
	db := repository(t)
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	outdated := model.Site{ID: model.NewID(), HostID: host.ID, Name: "outdated.example.test", Path: "~/sites/outdated"}
	fresh := model.Site{ID: model.NewID(), HostID: host.ID, Name: "fresh.example.test", Path: "~/sites/fresh", Position: 1}
	if err := db.AddHostWithSites(ctx, host, []model.Site{outdated, fresh}); err != nil {
		t.Fatal(err)
	}
	finished := time.Date(2026, 10, 29, 9, 0, 3, 0, time.UTC)
	err := db.SaveScan(ctx, model.Scan{
		ID: model.NewID(), SiteID: outdated.ID, Started: finished.Add(-3 * time.Second), Finished: finished,
		Outcome: model.ScanCompleted, WordPress: "7.1.3", PHPCLI: "8.4.27", WPCLI: "2.12.0", Home: "https://outdated.example.test",
		CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: false,
		Items: []model.ScanItem{
			{Kind: model.ItemCore, Status: "minor", Version: "7.1.3", Update: model.UpdateAvailable, UpdateVersion: "7.1.4"},
			{Kind: model.ItemPlugin, Slug: "akismet", Title: "Akismet", Status: "inactive", Version: "5.3",
				Update: model.UpdateAvailable, UpdateVersion: "5.7.2", OnWordPressOrg: true},
			{Kind: model.ItemPlugin, Slug: "fixture-premium", Title: "Fixture Premium", Status: "active", Version: "1.0.0", Update: model.UpdateNone},
			{Kind: model.ItemTheme, Slug: "twentytwentyfive", Title: "Twenty Twenty-Five", Status: "active", Version: "1.5", Update: model.UpdateNone},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return app.NewSiteManager(db), host, outdated, fresh
}

func TestTreeCarriesTheLatestStatus(t *testing.T) {
	m, _, outdated, fresh := scanned(t)
	tree, err := m.Tree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, n := range tree.Nodes {
		status[n.ID] = n.Status
	}
	if status[outdated.ID.String()] != "warn" || status[fresh.ID.String()] != "" {
		t.Fatalf("statuses = %v, want warn for the scanned site, none for the other", status)
	}
}

func TestSiteDetail(t *testing.T) {
	m, _, outdated, fresh := scanned(t)
	got, err := m.SiteDetail(context.Background(), outdated.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	s := got.Scan
	if s == nil || s.Status != "warn" || s.WordPress != "7.1.3" || s.Finished != "2026-10-29T09:00:03Z" || s.Problem != nil {
		t.Fatalf("scan = %+v", s)
	}
	if s.Checked.Languages || !s.Checked.Core || s.Pending.Core != 1 || s.Pending.Plugins != 1 {
		t.Fatalf("checked %+v, pending %+v", s.Checked, s.Pending)
	}
	if s.Plugins != 2 || s.Themes != 1 {
		t.Fatalf("installed: %d plugins, %d themes", s.Plugins, s.Themes)
	}
	if len(s.Updates) != 2 || s.Updates[0].Kind != "core" || s.Updates[1].Slug != "akismet" {
		t.Fatalf("updates = %+v", s.Updates)
	}
	if len(s.SourceUnknown) != 1 || s.SourceUnknown[0] != "Fixture Premium" {
		t.Fatalf("source unknown = %v", s.SourceUnknown)
	}

	never, err := m.SiteDetail(context.Background(), fresh.ID.String())
	if err != nil || never.Scan != nil || never.Name != "fresh.example.test" || never.Path != "~/sites/fresh" {
		t.Fatalf("never scanned = %+v, %v", never, err)
	}
}

func TestScanProblemIsTranslatable(t *testing.T) {
	for _, tc := range []struct {
		scan model.Scan
		want string
	}{
		{model.Scan{Outcome: model.ScanFailed, Failure: "timedOut", Error: "timed out"}, "failure.timedOut"},
		{model.Scan{Outcome: model.ScanFailed, Error: "unexpected output"}, "outcome.failed"},
		{model.Scan{Outcome: model.ScanCannotLoad, Error: "Error establishing a database connection."}, "outcome.cannotLoad"},
		{model.Scan{Outcome: model.ScanCancelled}, "outcome.cancelled"},
	} {
		if got := app.ScanProblem(tc.scan); got == nil || got.Code != tc.want || got.Detail != tc.scan.Error {
			t.Errorf("ScanProblem(%q) = %+v, want %s", tc.scan.Outcome, got, tc.want)
		}
	}
	if app.ScanProblem(model.Scan{Outcome: model.ScanCompleted}) != nil {
		t.Error("a completed scan has no problem")
	}
}

func TestSitesOfAHost(t *testing.T) {
	m, host, outdated, _ := scanned(t)
	rows, err := m.Sites(context.Background(), host.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != outdated.ID.String() || rows[0].Status != "warn" || rows[0].Pending != 2 ||
		rows[0].WordPress != "7.1.3" || rows[1].Status != "" {
		t.Fatalf("Sites() = %+v", rows)
	}
}

func TestHostDetail(t *testing.T) {
	m, host, outdated, _ := scanned(t)
	got, err := m.HostDetail(context.Background(), host.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Bench" || got.Alias != "bench" || got.WPCLI != "wp" || got.Port != 22 {
		t.Fatalf("HostDetail() = %+v", got)
	}
	if len(got.Sites) != 2 || got.Sites[0].ID != outdated.ID.String() {
		t.Fatalf("sites = %+v", got.Sites)
	}
	if _, err := m.HostDetail(context.Background(), outdated.ID.String()); err == nil {
		t.Fatal("HostDetail(a site) succeeded, want an error")
	}
}
