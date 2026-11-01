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
	all, first, items, findings := scaleSites(t, db, sites)
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

// scaleSites adds sites sites, 20 a host, and returns them, the first host, and the items and
// findings of a scan of each: 34 items and 5 findings.
func scaleSites(t *testing.T, db *store.DB, sites int) ([]model.Site, model.Host, []model.ScanItem, []model.Finding) {
	t.Helper()
	ctx := context.Background()
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
	return all, first, items, findings
}

// TestScaleOfAYearKept is the benchmark of ADR-0031: SITES sites (500) scanned every day for DAYS
// days (365), each scan noted (ADR-0030), a plugin updated on each site one day in 7. Two ways:
//   - pruned after each day's run, as Vigie does: the size of the database along the year (the
//     details level off at day 90, the scans kept without them at day 365), and each pruning's time;
//   - kept whole, as before ADR-0031, then pruned once, as at the first start after the upgrade
//     (workspace.TidyHistory): the time of that first pruning, then of compacting.
//
// Run with -scale -run TestScaleOfAYearKept; WAY=daily or WAY=upgrade runs one of them.
func TestScaleOfAYearKept(t *testing.T) {
	if !*scale {
		t.Skip("run with -scale: it stores a year of scans")
	}
	for _, way := range []string{"daily", "upgrade"} {
		if w := os.Getenv("WAY"); w != "" && w != way {
			continue
		}
		t.Run(way, func(t *testing.T) { scaleOfAYear(t, way == "daily") })
	}
}

func scaleOfAYear(t *testing.T, daily bool) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	sites, days := env("SITES", 500), env("DAYS", 365)
	all, _, items, findings := scaleSites(t, db, sites)
	size := func() string {
		s, err := db.Size(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("file %4d MB, %4d MB of it free", s.Bytes/1e6, s.Free/1e6)
	}
	start := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	insert := time.Now()
	var pruning, longest time.Duration
	for d := 1; d <= days; d++ {
		run := model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: start.Add(time.Duration(d) * 24 * time.Hour),
			Outcome: model.RunCompleted, Sites: sites}
		if err := db.StartRun(ctx, run); err != nil {
			t.Fatal(err)
		}
		for i, s := range all {
			at := run.Started.Add(time.Minute)
			changes := []model.Change{}
			if (d+i)%7 == 0 {
				changes = []model.Change{{Kind: model.ChangeVersion, Subject: "plugin:plugin-1", From: "1.0.1", To: "1.0.2"}}
			}
			scan := model.Scan{ID: model.NewID(), SiteID: s.ID, RunID: run.ID, Started: at, Finished: at.Add(20 * time.Second), Outcome: model.ScanCompleted,
				WordPress: "7.1.3", PHPCLI: "8.4.27", Home: "https://" + s.Name, CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
				Items: items, Findings: findings, FilesChecked: at,
				Recorded: true, RecordedStatus: model.StatusAttention, RecordedChanges: changes}
			if err := db.SaveScan(ctx, scan); err != nil {
				t.Fatal(err)
			}
		}
		end := run.Started.Add(time.Hour)
		if err := db.EndRun(ctx, run.ID, end, model.RunCompleted); err != nil {
			t.Fatal(err)
		}
		if !daily {
			continue
		}
		began := time.Now()
		report, err := db.Prune(ctx, end)
		if err != nil {
			t.Fatal(err)
		}
		took := time.Since(began)
		pruning += took
		longest = max(longest, took)
		switch d {
		case 30, 60, 89, 90, 91, 92, 120, 180, 270, days:
			t.Logf("day %3d: %s; pruned in %s (%d scans removed, %d left bare)", d, size(), took.Round(time.Millisecond), report.Scans, report.Details)
		}
	}
	t.Logf("%d sites x %d days = %d scans, stored in %s", sites, days, sites*days, time.Since(insert).Round(time.Second))
	if daily {
		t.Logf("pruning after each run: %s in all, %s at most", pruning.Round(time.Millisecond), longest.Round(time.Millisecond))
	} else {
		t.Logf("kept whole: %s", size())
		began := time.Now()
		report, err := db.Prune(ctx, start.Add(time.Duration(days)*24*time.Hour+time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("first pruning: %s (%d scans removed, %d left bare); %s", time.Since(began).Round(time.Millisecond), report.Scans, report.Details, size())
	}
	began := time.Now()
	if err := db.Compact(ctx); err != nil {
		t.Fatal(err)
	}
	t.Logf("compacted in %s: %s", time.Since(began).Round(time.Millisecond), size())
	a := app.NewActivity(db, time.Now)
	began = time.Now()
	if _, err := a.Journal(ctx, 30, ""); err != nil {
		t.Fatal(err)
	}
	t.Logf("journal of 30 days: %s", time.Since(began).Round(time.Millisecond))
}
