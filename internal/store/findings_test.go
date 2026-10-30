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

func TestScanFindingsAreKeptWithTheScan(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "a.example.test")
	start := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	s := scanOf(site, start, "7.1.3")
	s.FilesChecked = start.Add(-2 * time.Hour)
	s.Findings = []model.Finding{
		{Kind: "vulnerability", Subject: "plugin:woocommerce", Identifier: "uuid-1", Version: "9.8.1", Level: model.LevelUrgent,
			Facts: map[string]string{"cvss": "9.8", "severity": "critical", "fixed": "9.8.3"}},
		{Kind: "domain", Subject: "site", Level: model.LevelNotChecked, Facts: map[string]string{"reason": "not-published"}},
		{Kind: "database", Subject: "site", Level: model.LevelInfo},
	}
	must(t, db.SaveScan(ctx, s))

	latest, err := db.LatestScans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(latest[site.ID], s) {
		t.Fatalf("LatestScans() =\n%+v\nwant\n%+v", latest[site.ID], s)
	}
}

func TestIgnoresOfASite(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "a.example.test")
	at := time.Date(2026, 10, 30, 14, 12, 0, 0, time.UTC)
	until := model.Ignore{SiteID: site.ID, Key: "vulnerability|plugin:contact-form-7|uuid-3", UntilVersion: "6.0.4", Created: at}
	forever := model.Ignore{SiteID: site.ID, Key: "setting|site|blog_public", Reason: "staging copy", Created: at}
	must(t, db.SaveIgnore(ctx, until))
	must(t, db.SaveIgnore(ctx, forever))
	if err := db.SaveIgnore(ctx, model.Ignore{SiteID: site.ID, Key: "setting|site|x", Created: at}); !errors.Is(err, model.ErrReasonRequired) {
		t.Fatalf("SaveIgnore() permanent without a reason = %v, want ErrReasonRequired", err)
	}
	if err := db.SaveIgnore(ctx, model.Ignore{SiteID: model.NewID(), Key: "k", Reason: "r", Created: at}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("SaveIgnore() for a missing site = %v, want ErrNotFound", err)
	}
	// Ignoring the same finding again replaces the ignore.
	forever.Reason = "staging copy, hidden on purpose"
	must(t, db.SaveIgnore(ctx, forever))

	got, err := db.Ignores(ctx, site.ID)
	if err != nil || !reflect.DeepEqual(got, []model.Ignore{forever, until}) {
		t.Fatalf("Ignores() = %+v, %v; want both, sorted by key", got, err)
	}
	must(t, db.DeleteIgnore(ctx, site.ID, until.Key))
	if got, _ := db.Ignores(ctx, site.ID); len(got) != 1 {
		t.Fatalf("Ignores() after DeleteIgnore = %+v", got)
	}
}

func TestLookupCache(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	at := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	if _, ok, err := db.Lookup(ctx, "wpvulnerability", "plugin/polylang"); ok || err != nil {
		t.Fatalf("Lookup() of nothing = %v, %v", ok, err)
	}
	first := store.LookupAnswer{Status: 200, Body: []byte(`{"error":0}`), Fetched: at}
	must(t, db.PutLookup(ctx, "wpvulnerability", "plugin/polylang", first))
	second := store.LookupAnswer{Status: 404, Body: []byte(`{"error":"closed"}`), Fetched: at.Add(24 * time.Hour)}
	must(t, db.PutLookup(ctx, "wordpress.org", "plugin/display-widgets", second))
	got, ok, err := db.Lookup(ctx, "wpvulnerability", "plugin/polylang")
	if err != nil || !ok || !reflect.DeepEqual(got, first) {
		t.Fatalf("Lookup() = %+v, %v, %v", got, ok, err)
	}
	// A newer answer replaces the older one.
	first.Fetched, first.Body = at.Add(time.Hour), []byte(`{"error":0,"data":{}}`)
	must(t, db.PutLookup(ctx, "wpvulnerability", "plugin/polylang", first))
	if got, _, _ := db.Lookup(ctx, "wpvulnerability", "plugin/polylang"); !reflect.DeepEqual(got, first) {
		t.Fatalf("Lookup() after a newer answer = %+v", got)
	}
}
