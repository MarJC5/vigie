package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// kindsOf lists the changes of a view as "kind subject".
func kindsOf(changes []app.ChangeView) []string {
	var out []string
	for _, c := range changes {
		out = append(out, c.Kind+" "+c.Subject)
	}
	return out
}

// A site scanned before the span and in it, a failed scan in between, an update of Vigie: the
// journal's run and the site's page tell the same changes (ADR-0027).
func TestTheChangesOfASite(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	shop := model.Site{ID: model.NewID(), HostID: host.ID, Name: "shop.example.test", Path: "~/sites/shop"}
	if err := db.AddHostWithSites(ctx, host, []model.Site{shop}); err != nil {
		t.Fatal(err)
	}
	admin := func(login string) model.Finding {
		return model.Finding{Kind: model.KindAdministrator, Subject: "user:" + login, Level: model.LevelInfo, Facts: map[string]string{"two-factor": "on"}}
	}
	scan := func(at time.Time, run model.ID, akismet string, extra []model.ScanItem, admins ...model.Finding) model.Scan {
		items := append([]model.ScanItem{
			{Kind: model.ItemCore, Version: "7.1.4", Update: model.UpdateNone},
			{Kind: model.ItemPlugin, Slug: "akismet", Title: "Akismet Anti-spam", Status: "active", Version: akismet, Update: model.UpdateNone, OnWordPressOrg: true},
		}, extra...)
		return model.Scan{ID: model.NewID(), SiteID: shop.ID, RunID: run, Started: at, Finished: at.Add(time.Second), Outcome: model.ScanCompleted,
			CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true, Items: items, Findings: admins}
	}
	run := model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: now.Add(-day), Outcome: model.RunCompleted, Sites: 1}
	op := model.Operation{ID: model.NewID(), SiteID: shop.ID, Kind: model.OperationUpdate, Started: now.Add(-2 * day)}
	manager := []model.ScanItem{{Kind: model.ItemPlugin, Slug: "wp-file-manager", Title: "WP File Manager", Status: "active", Version: "7.2.1", Update: model.UpdateNone}}
	before := scan(now.Add(-8*day), model.NoID, "5.3", nil, admin("marie"))
	after := scan(now.Add(-day), run.ID, "5.7.2", manager, admin("marie"), admin("root2"))
	for _, err := range []error{
		db.SaveScan(ctx, before),
		db.SaveScan(ctx, model.Scan{ID: model.NewID(), SiteID: shop.ID, Started: now.Add(-3 * day), Finished: now.Add(-3 * day), Outcome: model.ScanFailed, Failure: "refused"}),
		db.StartOperation(ctx, op),
		db.SaveOperationStep(ctx, op.ID, model.OperationStep{Position: 1, Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: "akismet",
			VersionBefore: "5.3", VersionAfter: "5.7.2", Started: op.Started, Outcome: model.StepDone}),
		db.StartRun(ctx, run),
		db.SaveScan(ctx, after),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"admin-added user:root2", "installed plugin:wp-file-manager", "version plugin:akismet"}

	journal, err := app.NewActivity(db, func() time.Time { return now }).Journal(ctx, 7, "")
	if err != nil {
		t.Fatal(err)
	}
	sameOnceNoted(t, db, now, journal)
	i := slices.IndexFunc(journal.Entries, func(e app.ActivityEntry) bool { return e.Kind == "run" })
	if i < 0 || len(journal.Entries[i].SiteChanges) != 1 {
		t.Fatalf("journal = %+v, want the run with one site's changes", journal.Entries)
	}
	sc := journal.Entries[i].SiteChanges[0]
	if sc.SiteID != shop.ID.String() || sc.Name != shop.Name || !slices.Equal(kindsOf(sc.Changes), want) {
		t.Errorf("the run's changes = %+v, want %q", sc, want)
	}
	if v := sc.Changes[2]; !v.ByVigie || v.From != "5.3" || v.To != "5.7.2" || v.Name != "Akismet Anti-spam" {
		t.Errorf("the version change = %+v, want by Vigie", v)
	}

	site, err := app.NewSiteManager(db).SiteDetail(ctx, shop.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kindsOf(site.Changes), want) || site.ChangesSince != before.Finished.UTC().Format(time.RFC3339) {
		t.Errorf("site changes = %q since %q, want %q since the complete scan before", kindsOf(site.Changes), site.ChangesSince, want)
	}
	if site.Changes[0].Name != "root2" {
		t.Errorf("the administrator's name = %q, want its login", site.Changes[0].Name)
	}
}
