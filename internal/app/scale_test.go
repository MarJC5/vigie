package app_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

var scale = flag.Bool("scale", false, "store months of daily scans of hundreds of sites and time the reads (slow)")

func env(name string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil {
		return n
	}
	return def
}

// TestScaleOfHistory is the benchmark ADR-0006 asked for: SITES sites (500 by default), a scan of
// each every day for DAYS days (90), each with 34 items and 5 findings, then the time of the reads
// the window makes most. Run with -scale; JOURNAL=0 leaves the journal out.
func TestScaleOfHistory(t *testing.T) {
	if !*scale {
		t.Skip("run with -scale: it stores tens of thousands of scans")
	}
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vigie.db")
	db, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	sites, days := env("SITES", 500), env("DAYS", 90)
	perHost := 20
	var all []model.Site
	var first model.Host
	for h := 0; h*perHost < sites; h++ {
		host := model.Host{ID: model.NewID(), Name: fmt.Sprintf("Host %d", h), Alias: fmt.Sprintf("h%d", h)}
		if h == 0 {
			first = host
		}
		var list []model.Site
		for i := 0; i < perHost && h*perHost+i < sites; i++ {
			list = append(list, model.Site{ID: model.NewID(), HostID: host.ID, Name: fmt.Sprintf("site%d-%d.example.test", h, i), Position: i})
		}
		if err := db.AddHostWithSites(ctx, host, list); err != nil {
			t.Fatal(err)
		}
		all = append(all, list...)
	}
	items := []model.ScanItem{{Kind: model.ItemCore, Version: "7.1.3", Update: model.UpdateNone}}
	for p := 0; p < 25; p++ {
		items = append(items, model.ScanItem{Kind: model.ItemPlugin, Slug: fmt.Sprintf("plugin-%d", p), Title: fmt.Sprintf("Plugin %d", p),
			Status: "active", Version: "1.0." + strconv.Itoa(p), Update: model.UpdateNone, OnWordPressOrg: true})
	}
	for p := 0; p < 3; p++ {
		items = append(items, model.ScanItem{Kind: model.ItemTheme, Slug: fmt.Sprintf("theme-%d", p), Title: "Theme", Status: "inactive", Version: "1.0", Update: model.UpdateNone})
	}
	for p := 0; p < 5; p++ {
		items = append(items, model.ScanItem{Kind: model.ItemLanguage, Slug: fmt.Sprintf("plugin-%d", p), Language: "fr_FR", Update: model.UpdateNone})
	}
	findings := []model.Finding{
		{Kind: model.KindAdministrator, Subject: "user:admin", Level: model.LevelAttention, Facts: map[string]string{"two-factor": "off", "email": "a@example.test"}},
		{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention},
		{Kind: model.KindCertificate, Subject: "site", Version: "2027-01-01", Level: model.LevelInfo, Facts: map[string]string{"expires": "2027-01-01T00:00:00Z"}},
		{Kind: model.KindDatabaseVersion, Subject: "site", Level: model.LevelInfo, Facts: map[string]string{"version": "11.8.9"}},
		{Kind: model.KindVulnerability, Subject: "plugin:plugin-3", Identifier: "v1", Version: "1.0.3", Level: model.LevelAttention, Facts: map[string]string{"name": "XSS"}},
	}
	now := time.Now()
	insert := time.Now()
	for d := days; d >= 1; d-- {
		run := model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: now.Add(-time.Duration(d) * 24 * time.Hour), Outcome: model.RunCompleted, Sites: sites}
		if err := db.StartRun(ctx, run); err != nil {
			t.Fatal(err)
		}
		for _, s := range all {
			at := run.Started.Add(time.Minute)
			scan := model.Scan{ID: model.NewID(), SiteID: s.ID, RunID: run.ID, Started: at, Finished: at.Add(20 * time.Second), Outcome: model.ScanCompleted,
				WordPress: "7.1.3", PHPCLI: "8.4.27", Home: "https://" + s.Name, CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
				Items: items, Findings: findings, FilesChecked: at}
			if err := db.SaveScan(ctx, scan); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.EndRun(ctx, run.ID, run.Started.Add(time.Hour), model.RunCompleted); err != nil {
			t.Fatal(err)
		}
	}
	info, _ := os.Stat(path)
	t.Logf("%d sites x %d days = %d scans, stored in %s; database %d MB", sites, days, sites*days, time.Since(insert).Round(time.Second), info.Size()/1e6)

	m := app.NewSiteManager(db)
	measure := func(label string, fn func() error) {
		start := time.Now()
		if err := fn(); err != nil {
			t.Fatal(err)
		}
		t.Logf("  %-34s %s", label, time.Since(start).Round(time.Millisecond))
	}
	measure("LatestScans", func() error { _, err := db.LatestScans(ctx); return err })
	measure("Tree", func() error { _, err := m.Tree(ctx); return err })
	measure("SiteDetail", func() error { _, err := m.SiteDetail(ctx, all[0].ID.String()); return err })
	measure("Sites (all)", func() error { _, err := m.Sites(ctx, ""); return err })
	measure("HostDetail", func() error { _, err := m.HostDetail(ctx, first.ID.String()); return err })
	measure("Extensions (all)", func() error { _, err := m.Extensions(ctx, ""); return err })
	if os.Getenv("JOURNAL") == "0" {
		return
	}
	a := app.NewActivity(db, time.Now)
	measure("Journal 30 days, computed", func() error { _, err := a.Journal(ctx, 30, ""); return err })
	// The scans stored here noted nothing, as those stored before ADR-0030: noted once at the first
	// start, then read by the journal.
	measure("RecordScans (first start)", func() error { _, err := db.RecordScans(ctx); return err })
	measure("Journal 30 days", func() error { _, err := a.Journal(ctx, 30, ""); return err })
	measure("Journal 30 days, one site", func() error { _, err := a.Journal(ctx, 30, all[0].ID.String()); return err })
}
