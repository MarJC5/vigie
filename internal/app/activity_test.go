package app_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/store"
)

// The journal of a span of days: each run with what it changed and what Vigie notified, updates
// and rollbacks, and older scans without a run; newest first; filtered by a site.
func TestTheJournalTellsWhatChanged(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	shop := model.Site{ID: model.NewID(), HostID: host.ID, Name: "shop.example.test", Path: "~/sites/shop"}
	blog := model.Site{ID: model.NewID(), HostID: host.ID, Name: "blog.example.test", Path: "~/sites/blog", Position: 1}
	if err := db.AddHostWithSites(ctx, host, []model.Site{shop, blog}); err != nil {
		t.Fatal(err)
	}
	checked := func(s model.Scan) model.Scan {
		s.CoreChecked, s.PluginsChecked, s.ThemesChecked, s.LanguagesChecked = true, true, true, true
		return s
	}
	ok := func(site model.Site, at time.Time, run model.ID) model.Scan {
		return checked(model.Scan{ID: model.NewID(), SiteID: site.ID, RunID: run, Started: at, Finished: at.Add(time.Second), Outcome: model.ScanCompleted})
	}
	urgent := func(site model.Site, at time.Time, run model.ID) model.Scan {
		s := ok(site, at, run)
		s.Findings = []model.Finding{{Kind: model.KindDebugLog, Subject: "site", Level: model.LevelUrgent}}
		return s
	}
	unreachable := func(site model.Site, at time.Time) model.Scan {
		return model.Scan{ID: model.NewID(), SiteID: site.ID, Started: at, Finished: at.Add(time.Second), Outcome: model.ScanFailed, Failure: string(sshx.FailureTimedOut)}
	}
	day := 24 * time.Hour
	run := model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: now.Add(-2 * day), Outcome: model.RunRunning, Sites: 2}
	op := model.Operation{ID: model.NewID(), SiteID: shop.ID, Kind: model.OperationUpdate, Started: now.Add(-day)}
	ended := "Update done, shop.example.test: 1 item updated."
	steps := []error{
		db.SaveScan(ctx, ok(shop, now.Add(-8*day), model.NoID)), // before the span: compared with
		db.SaveScan(ctx, unreachable(blog, now.Add(-8*day))),
		db.SaveScan(ctx, unreachable(blog, now.Add(-3*day))), // in the span, without a run
		db.StartRun(ctx, run),
		db.SaveScan(ctx, urgent(shop, now.Add(-2*day), run.ID)),
		db.SaveScan(ctx, ok(blog, now.Add(-2*day), run.ID)),
		db.EndRun(ctx, run.ID, now.Add(-2*day+time.Minute), model.RunCompleted),
		db.SaveNotice(ctx, model.Notice{ID: model.NewID(), RunID: run.ID, Kind: app.AlertUrgent, Names: []string{shop.Name}, Sent: now.Add(-2*day + time.Minute)}),
		db.StartOperation(ctx, op),
		// The end of the update notified (ADR-0035): shown under its operation.
		db.SaveNotice(ctx, model.Notice{ID: model.NewID(), RunID: op.ID, Kind: model.NoticeEnded, Text: ended, Sent: now.Add(-day + time.Minute)}),
	}
	for _, err := range steps {
		if err != nil {
			t.Fatal(err)
		}
	}
	activity := app.NewActivity(db, func() time.Time { return now })

	journal, err := activity.Journal(ctx, 7, "")
	if err != nil {
		t.Fatal(err)
	}
	sameOnceNoted(t, db, now, journal)
	kinds := []string{}
	for _, e := range journal.Entries {
		kinds = append(kinds, e.Kind)
	}
	if len(kinds) != 3 || kinds[0] != "update" || kinds[1] != "run" || kinds[2] != "scan" {
		t.Fatalf("entries %q, want the update, the run, the scan without a run, newest first", kinds)
	}
	if u := journal.Entries[0]; len(u.Notices) != 1 || u.Notices[0].Kind != model.NoticeEnded || u.Notices[0].Text != ended {
		t.Errorf("update entry = %+v, want its end notified", u)
	}
	r := journal.Entries[1]
	if r.By != "schedule" || r.Outcome != "completed" || r.Sites != 2 || len(r.Notices) != 1 || r.Notices[0].Names[0] != shop.Name {
		t.Errorf("run entry = %+v", r)
	}
	if len(r.Changes) != 2 || r.Changes[0] != (app.Change{Kind: app.AlertUrgent, NodeID: shop.ID.String(), Name: shop.Name}) ||
		r.Changes[1] != (app.Change{Kind: app.ChangeRecovered, NodeID: blog.ID.String(), Name: blog.Name}) {
		t.Errorf("changes = %+v, want shop urgent then blog back", r.Changes)
	}
	if s := journal.Entries[2]; s.SiteID != blog.ID.String() || len(s.Changes) != 0 {
		t.Errorf("scan entry = %+v, want blog's, unreachable as before: no change", s)
	}

	// What happened then (ADR-0030): ignoring the urgent finding now leaves the run's line as it was;
	// the site's page follows the ignore.
	if err := db.SaveIgnore(ctx, model.Ignore{SiteID: shop.ID, Key: "debug-log|site|", Reason: "known", Created: now}); err != nil {
		t.Fatal(err)
	}
	after, err := activity.Journal(ctx, 7, "")
	if err != nil || !reflect.DeepEqual(after, journal) {
		t.Errorf("journal after an ignore = %+v, %v; want it as it was", after, err)
	}
	if page, err := app.NewSiteManager(db).SiteDetail(ctx, shop.ID.String()); err != nil || page.Scan.Status == "danger" {
		t.Errorf("the site's page = %+v, %v; want the ignore applied", page.Scan, err)
	}

	only, err := activity.Journal(ctx, 7, shop.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(only.Entries) != 2 || len(only.Entries[1].Changes) != 1 || only.Entries[1].Changes[0].Name != shop.Name {
		t.Fatalf("journal of shop = %+v, want its update and the run with its change only", only.Entries)
	}
}

// sameOnceNoted checks that the journal read from what each scan noted of itself equals the journal
// computed from the scans' details (ADR-0030): the scans of these tests are stored without notes,
// then noted as the first start after the upgrade notes them.
func sameOnceNoted(t *testing.T, db *store.DB, now time.Time, computed app.Journal) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.RecordScans(ctx); err != nil {
		t.Fatal(err)
	}
	noted, err := app.NewActivity(db, func() time.Time { return now }).Journal(ctx, 7, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(noted, computed) {
		t.Fatalf("journal from the noted facts =\n%+v\nwant the computed one\n%+v", noted, computed)
	}
}
