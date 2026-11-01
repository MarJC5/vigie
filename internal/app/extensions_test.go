package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// plugin and theme items of a scan, for the tests of this file.
func plugin(slug, title, version string, update model.UpdateState, to string) model.ScanItem {
	return model.ScanItem{Kind: model.ItemPlugin, Slug: slug, Title: title, Status: "active", Version: version,
		Update: update, UpdateVersion: to, OnWordPressOrg: true}
}

// A plugin or theme carries the worst finding about it that needs the user; information does not.
func TestExtensionsCarryTheirWorstFinding(t *testing.T) {
	ctx := context.Background()
	m, db, _, outdated, _ := scannedIn(t)
	finished := time.Date(2026, 10, 30, 9, 0, 3, 0, time.UTC)
	err := db.SaveScan(ctx, model.Scan{
		ID: model.NewID(), SiteID: outdated.ID, Started: finished.Add(-3 * time.Second), Finished: finished,
		Outcome: model.ScanCompleted, WordPress: "7.1.4", Home: "https://outdated.example.test",
		Items: []model.ScanItem{
			plugin("kirki", "Kirki", "6.4.0", model.UpdateAvailable, "6.4.1"),
			plugin("akismet", "Akismet", "5.7.2", model.UpdateNone, ""),
		},
		Findings: []model.Finding{
			{Kind: model.KindVulnerability, Subject: "plugin:kirki", Identifier: "a", Version: "6.4.0", Level: model.LevelUrgent},
			{Kind: model.KindPluginUnverified, Subject: "plugin:akismet", Version: "5.7.2", Level: model.LevelInfo},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.SiteDetail(ctx, outdated.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	e := got.Scan.Extensions
	if len(e) != 2 || e[0].Worst == nil || e[0].Worst.Kind != "vulnerability" || e[1].Worst != nil {
		t.Fatalf("extensions = %+v", e)
	}
}

// Across a host's sites: each plugin with the sites that run it and their versions; a site never
// scanned is left out, and said so.
func TestExtensionsAcrossSites(t *testing.T) {
	ctx := context.Background()
	m, db, host, outdated, fresh := scannedIn(t)
	finished := time.Date(2026, 10, 30, 9, 0, 3, 0, time.UTC)
	for _, c := range []struct {
		site  model.Site
		kirki model.ScanItem
	}{
		{outdated, plugin("kirki", "Kirki", "6.4.0", model.UpdateAvailable, "6.4.1")},
		{fresh, plugin("kirki", "Kirki", "6.4.1", model.UpdateNone, "")},
	} {
		site, kirki := c.site, c.kirki
		scan := model.Scan{
			ID: model.NewID(), SiteID: site.ID, Started: finished.Add(-3 * time.Second), Finished: finished,
			Outcome: model.ScanCompleted, WordPress: "7.1.4", Home: "https://" + site.Name, Items: []model.ScanItem{kirki},
		}
		if site.ID == outdated.ID {
			scan.Findings = []model.Finding{{Kind: model.KindVulnerability, Subject: "plugin:kirki", Identifier: "a", Version: "6.4.0", Level: model.LevelUrgent}}
		}
		if err := db.SaveScan(ctx, scan); err != nil {
			t.Fatal(err)
		}
	}
	never := model.Site{ID: model.NewID(), HostID: host.ID, Name: "never.example.test", Path: "~/sites/never", Position: 2}
	if err := db.SaveSite(ctx, never); err != nil {
		t.Fatal(err)
	}

	got, err := m.Extensions(ctx, host.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Sites != 2 || len(got.LeftOut) != 1 || got.LeftOut[0].Name != "never.example.test" || got.LeftOut[0].Reason != "never" {
		t.Fatalf("sites read %d, left out %+v", got.Sites, got.LeftOut)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v, want Kirki alone", got.Items)
	}
	kirki := got.Items[0]
	if kirki.Kind != "plugin" || kirki.Slug != "kirki" || kirki.Updates != 1 || kirki.Worst == nil || kirki.Worst.Level != "urgent" || len(kirki.Sites) != 2 {
		t.Fatalf("kirki = %+v", kirki)
	}
	// Its sites, the oldest version first.
	if kirki.Sites[0].Name != "outdated.example.test" || kirki.Sites[0].Version != "6.4.0" || kirki.Sites[0].Worst == nil ||
		kirki.Sites[1].Name != "fresh.example.test" || kirki.Sites[1].Version != "6.4.1" || kirki.Sites[1].Worst != nil {
		t.Fatalf("kirki's sites = %+v", kirki.Sites)
	}
	// One site: that site's list only.
	one, err := m.Extensions(ctx, fresh.ID.String())
	if err != nil || one.Sites != 1 || len(one.Items) != 1 {
		t.Fatalf("one site: %+v, %v", one, err)
	}
}
