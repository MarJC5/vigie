package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

// siteOnHost stores a host and one site, and returns the site.
func siteOnHost(t *testing.T, db *store.DB, name string) model.Site {
	t.Helper()
	ctx := context.Background()
	host := model.Host{ID: model.NewID(), Name: "Host " + name, Alias: "h-" + name}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: name, Path: "~/sites/" + name}
	must(t, db.SaveHost(ctx, host))
	must(t, db.SaveSite(ctx, site))
	return site
}

func scanOf(site model.Site, started time.Time, wp string) model.Scan {
	return model.Scan{
		ID: model.NewID(), SiteID: site.ID,
		Started: started, Finished: started.Add(3 * time.Second),
		Outcome:   model.ScanCompleted,
		WordPress: wp, PHPCLI: "8.4.27", WPCLI: "2.12.0", Home: "https://" + site.Name,
		SiteURL:     "https://" + site.Name + "/wp",
		CoreChecked: true, PluginsChecked: true, ThemesChecked: false, LanguagesChecked: true,
		Items: []model.ScanItem{
			{Kind: model.ItemCore, Status: "minor", Version: wp, Update: model.UpdateAvailable, UpdateVersion: "7.1.4"},
			{Kind: model.ItemPlugin, Slug: "akismet", Title: "Akismet", Status: "active", Version: "5.3",
				Update: model.UpdateAvailable, UpdateVersion: "5.7.2", AutoUpdate: true, OnWordPressOrg: true},
			{Kind: model.ItemLanguage, Slug: "akismet", Language: "fr_FR", Update: model.UpdateAvailable},
		},
	}
}

func TestScansAreKeptAndTheLatestIsRead(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	a, b := siteOnHost(t, db, "a.example.test"), siteOnHost(t, db, "b.example.test")
	start := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	first, second := scanOf(a, start, "7.1.2"), scanOf(a, start.Add(time.Hour), "7.1.3")
	other := scanOf(b, start, "7.0")
	other.Outcome, other.Failure, other.Error, other.Items = model.ScanFailed, "timedOut", "ssh: connect to host: Operation timed out", nil
	for _, s := range []model.Scan{first, second, other} {
		must(t, db.SaveScan(ctx, s))
	}

	latest, err := db.LatestScans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 || !reflect.DeepEqual(latest[a.ID], second) || !reflect.DeepEqual(latest[b.ID], other) {
		t.Fatalf("LatestScans() =\n%+v\nwant the second scan of a and the failed scan of b", latest)
	}
	count, err := db.ScanCount(ctx, a.ID)
	if err != nil || count != 2 {
		t.Fatalf("ScanCount() = %d, %v; want both scans kept", count, err)
	}
}

func TestScanOfUnknownSiteIsRefused(t *testing.T) {
	db := migrated(t)
	s := scanOf(model.Site{ID: model.NewID(), Name: "ghost"}, time.Now(), "7.1.3")
	if err := db.SaveScan(context.Background(), s); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("SaveScan() for a missing site = %v, want ErrNotFound", err)
	}
}

func TestDeletingASiteDeletesItsScans(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "gone.example.test")
	must(t, db.SaveScan(ctx, scanOf(site, time.Now(), "7.1.3")))
	must(t, db.DeleteSite(ctx, site.ID))
	if latest, err := db.LatestScans(ctx); err != nil || len(latest) != 0 {
		t.Fatalf("LatestScans() after deleting the site = %v, %v", latest, err)
	}
}

func TestSubsitesReadByAScanKeepTheirPins(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "network.example.test")
	site.Multisite = true
	site.Subsites = []model.Subsite{{BlogID: 1, URL: "https://network.example.test/", Public: true, Pinned: true}}
	must(t, db.SaveSite(ctx, site))

	read := []model.Subsite{
		{BlogID: 1, URL: "https://network.example.test/", Public: true},
		{BlogID: 4, URL: "https://network.example.test/gamma/", Public: true, Archived: true},
	}
	must(t, db.SaveSubsites(ctx, site.ID, read))

	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := tree.Site(site.ID)
	got := stored.Subsites
	want := []model.Subsite{read[0], read[1]}
	want[0].Pinned = true
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subsites = %+v, want %+v (pins kept, new subsites added)", got, want)
	}
}

// A scan cancelled or failed after a complete one does not hide it: the complete one is shown, the
// later one is its attempt. A site never scanned to the end shows its latest scan.
func TestAnIncompleteScanKeepsTheLatestCompleteOne(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "a.example.test")
	never := siteOnHost(t, db, "b.example.test")
	start := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	complete := scanOf(site, start, "7.1.3")
	complete.Findings = []model.Finding{{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention}}
	must(t, db.SaveScan(ctx, complete))
	cancelled := model.Scan{ID: model.NewID(), SiteID: site.ID, Started: start.Add(time.Hour), Finished: start.Add(time.Hour + time.Second),
		Outcome: model.ScanCancelled, Error: "cancelled"}
	must(t, db.SaveScan(ctx, cancelled))
	failed := model.Scan{ID: model.NewID(), SiteID: never.ID, Started: start, Finished: start.Add(time.Second),
		Outcome: model.ScanFailed, Failure: "refused", Error: "Connection refused"}
	must(t, db.SaveScan(ctx, failed))

	latest, err := db.LatestScans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := latest[site.ID]
	if got.ID != complete.ID || len(got.Items) != len(complete.Items) || len(got.Findings) != 1 || got.Status() != model.StatusAttention {
		t.Fatalf("shown scan = %+v, want the complete one with its items and findings", got)
	}
	if got.Attempt == nil || got.Attempt.ID != cancelled.ID || got.Attempt.Outcome != model.ScanCancelled {
		t.Errorf("attempt = %+v, want the cancelled scan", got.Attempt)
	}
	if n := latest[never.ID]; n.ID != failed.ID || n.Attempt != nil || n.Status() != model.StatusUnknown {
		t.Errorf("never complete = %+v, want its failed scan, unknown", n)
	}
	// Scanned again to the end: no attempt any more.
	again := scanOf(site, start.Add(2*time.Hour), "7.1.4")
	must(t, db.SaveScan(ctx, again))
	if latest, _ = db.LatestScans(ctx); latest[site.ID].ID != again.ID || latest[site.ID].Attempt != nil {
		t.Errorf("after a complete scan: %+v", latest[site.ID])
	}
}

func TestEveryScanOfASite(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "a.example.test")
	other := siteOnHost(t, db, "b.example.test")
	start := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	first, second := scanOf(site, start, "7.1.3"), scanOf(site, start.Add(time.Hour), "7.1.4")
	second.Findings = []model.Finding{{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention}}
	for _, s := range []model.Scan{first, scanOf(other, start, "7.1.3"), second} {
		must(t, db.SaveScan(ctx, s))
	}
	got, err := db.Scans(ctx, site.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("Scans() = %d scans, %v; want the site's two", len(got), err)
	}
	if got[0].ID != first.ID || got[1].ID != second.ID || len(got[0].Items) != len(first.Items) || len(got[1].Findings) != 1 {
		t.Errorf("Scans() = %+v, want the oldest first with their items and findings", got)
	}
}

// The journal compares a scan with the one just before it for the status, and with the complete
// one before it for the changes (ADR-0027): both are read, with their items.
func TestScanStatusesReadTheCompleteScanBefore(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "a.example.test")
	start := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	complete := scanOf(site, start, "7.1.3")
	failed := model.Scan{ID: model.NewID(), SiteID: site.ID, Started: start.Add(time.Hour), Finished: start.Add(time.Hour + time.Second),
		Outcome: model.ScanFailed, Failure: "refused"}
	recent := scanOf(site, start.Add(3*time.Hour), "7.1.4")
	for _, s := range []model.Scan{complete, failed, recent} {
		must(t, db.SaveScan(ctx, s))
	}
	got, err := db.ScanStatuses(ctx, start.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != complete.ID || got[1].ID != failed.ID || got[2].ID != recent.ID || len(got[2].Items) != len(recent.Items) {
		t.Fatalf("ScanStatuses() = %d scans %+v; want the complete one, the failed one, the recent one, with their items", len(got), got)
	}

	before, ok, err := db.CompleteScanBefore(ctx, site.ID, recent.ID)
	if err != nil || !ok || before.ID != complete.ID || len(before.Items) != len(complete.Items) {
		t.Fatalf("CompleteScanBefore() = %+v, %v, %v; want the complete scan, the failed one skipped", before, ok, err)
	}
	if _, ok, err := db.CompleteScanBefore(ctx, site.ID, complete.ID); ok || err != nil {
		t.Errorf("CompleteScanBefore(the first scan) = %v, %v; want none", ok, err)
	}
}
