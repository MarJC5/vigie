package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
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
	steps := []error{
		db.SaveScan(ctx, ok(shop, now.Add(-8*day), model.NoID)), // before the span: compared with
		db.SaveScan(ctx, unreachable(blog, now.Add(-8*day))),
		db.SaveScan(ctx, unreachable(blog, now.Add(-3*day))), // in the span, without a run
		db.StartRun(ctx, run),
		db.SaveScan(ctx, urgent(shop, now.Add(-2*day), run.ID)),
		db.SaveScan(ctx, ok(blog, now.Add(-2*day), run.ID)),
		db.EndRun(ctx, run.ID, now.Add(-2*day+time.Minute), model.RunCompleted),
		db.SaveNotice(ctx, model.Notice{ID: model.NewID(), RunID: run.ID, Kind: app.AlertUrgent, Names: []string{shop.Name}, Sent: now.Add(-2*day + time.Minute)}),
		db.StartOperation(ctx, model.Operation{ID: model.NewID(), SiteID: shop.ID, Kind: model.OperationUpdate, Started: now.Add(-day)}),
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
	kinds := []string{}
	for _, e := range journal.Entries {
		kinds = append(kinds, e.Kind)
	}
	if len(kinds) != 3 || kinds[0] != "update" || kinds[1] != "run" || kinds[2] != "scan" {
		t.Fatalf("entries %q, want the update, the run, the scan without a run, newest first", kinds)
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

	only, err := activity.Journal(ctx, 7, shop.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(only.Entries) != 2 || len(only.Entries[1].Changes) != 1 || only.Entries[1].Changes[0].Name != shop.Name {
		t.Fatalf("journal of shop = %+v, want its update and the run with its change only", only.Entries)
	}
}
