package store_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

// The history kept (ADR-0031): each site's latest scans whatever their age, details for 90 days,
// then only the scans that changed something, for a year; runs, notices and the reads of errors for a
// year.
func TestPruneKeepsWhatThePagesAndTheJournalNeed(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	site := siteOnHost(t, db, "a.example.test")
	scan := func(age time.Duration, status model.SiteStatus, changes []model.Change) model.Scan {
		s := scanOf(site, now.Add(-age), "7.1.3")
		s.Recorded, s.RecordedStatus, s.RecordedChanges = true, status, changes
		return s
	}
	changed := []model.Change{{Kind: model.ChangeVersion, Subject: "core", From: "7.1.2", To: "7.1.3"}}
	scans := []model.Scan{
		scan(400*day, model.StatusAttention, nil),  // 0: older than a year, gone
		scan(200*day, model.StatusAttention, nil),  // 1: the first within the year: changed from nothing, kept bare
		scan(199*day, model.StatusAttention, nil),  // 2: the same as before: gone
		scan(150*day, model.StatusUrgent, nil),     // 3: turned urgent: kept bare
		scan(149*day, model.StatusUrgent, changed), // 4: a change: kept bare
		scan(120*day, model.StatusUrgent, nil),     // 5: the same as before: gone
		scan(100*day, model.StatusAttention, nil),  // 6: back to attention: kept bare
		scan(95*day, model.StatusAttention, nil),   // 7: the complete scan before the latest one: kept whole
	}
	failed := model.Scan{ID: model.NewID(), SiteID: site.ID, Started: now.Add(-94 * day), Finished: now.Add(-94 * day),
		Outcome: model.ScanFailed, Failure: "refused", Recorded: true, RecordedStatus: model.StatusUnknown, RecordedChanges: []model.Change{}}
	recent := scan(10*day, model.StatusAttention, nil) // the latest, and the latest complete: kept whole
	for _, s := range append(scans, failed, recent) {
		must(t, db.SaveScan(ctx, s))
	}
	// A scan stored before ADR-0030, not noted yet: never removed by what it did not note.
	unnoted := scanOf(siteOnHost(t, db, "b.example.test"), now.Add(-200*day), "7.1.3")
	unnoted.Recorded, unnoted.RecordedStatus, unnoted.RecordedChanges = false, "", nil
	must(t, db.SaveScan(ctx, unnoted))
	old, fresh := model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: now.Add(-400 * day), Outcome: model.RunCompleted},
		model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: now.Add(-10 * day), Outcome: model.RunCompleted}
	for _, err := range []error{
		db.StartRun(ctx, old), db.StartRun(ctx, fresh),
		db.SaveNotice(ctx, model.Notice{ID: model.NewID(), RunID: old.ID, Kind: "urgent", Names: []string{"a"}, Sent: now.Add(-400 * day)}),
		db.SaveNotice(ctx, model.Notice{ID: model.NewID(), RunID: fresh.ID, Kind: "urgent", Names: []string{"a"}, Sent: now.Add(-10 * day)}),
		db.SaveErrorRead(ctx, model.ErrorRead{ID: model.NewID(), SiteID: site.ID, Path: "a", At: now.Add(-400 * day)}),
		db.SaveErrorRead(ctx, model.ErrorRead{ID: model.NewID(), SiteID: site.ID, Path: "a", At: now.Add(-10 * day)}),
	} {
		must(t, err)
	}

	if _, err := db.Prune(ctx, now); err != nil {
		t.Fatal(err)
	}
	got, err := db.Scans(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	kept := map[model.ID]int{} // scan ID: its items left
	for _, s := range got {
		kept[s.ID] = len(s.Items)
	}
	want := map[model.ID]int{
		scans[1].ID: 0, scans[3].ID: 0, scans[4].ID: 0, // changed something: bare
		scans[6].ID: 0, // older than 90 days, not one of the latest: bare
		scans[7].ID: len(scans[7].Items),
		failed.ID:   0,                 // failed after a complete scan: changed something, bare
		recent.ID:   len(recent.Items), // within 90 days: whole
	}
	if len(kept) != len(want) {
		t.Errorf("kept %d scans, want %d", len(kept), len(want))
	}
	for id, items := range want {
		if n, ok := kept[id]; !ok || n != items {
			t.Errorf("scan %s: kept %v with %d items, want kept with %d", id, ok, n, items)
		}
	}
	// The site's latest scans, whatever their age: the latest, the latest complete, the one before.
	latest, ok, err := db.LatestScan(ctx, site.ID)
	if err != nil || !ok || latest.ID != recent.ID || len(latest.Items) == 0 {
		t.Errorf("latest scan = %s with %d items, want the recent one whole", latest.ID, len(latest.Items))
	}
	if before, ok, err := db.CompleteScanBefore(ctx, site.ID, recent.ID); err != nil || !ok || before.ID != scans[7].ID {
		t.Errorf("complete scan before the latest = %s, want kept", before.ID)
	}
	if _, ok, _ := db.LatestScan(ctx, unnoted.SiteID); !ok {
		t.Error("a scan not noted yet was removed")
	}
	runs, _ := db.Runs(ctx, now.Add(-1000*day))
	notices, _ := db.Notices(ctx, now.Add(-1000*day))
	reads, _ := db.ErrorReads(ctx, now.Add(-1000*day))
	if len(runs) != 1 || runs[0].ID != fresh.ID || len(notices) != 1 || len(reads) != 1 || !reads[0].At.Equal(now.Add(-10*day)) {
		t.Errorf("runs %+v, %d notices, reads %+v; want those of the year only", runs, len(notices), reads)
	}
	// Again: nothing more to remove.
	report, err := db.Prune(ctx, now)
	if err != nil || report != (store.PruneReport{}) {
		t.Errorf("Prune() again = %+v, %v; want nothing", report, err)
	}
}

// A site whose only complete scans are old keeps its latest complete one whole, and the one before.
func TestPruneKeepsTheLatestScansWhateverTheirAge(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	site := siteOnHost(t, db, "a.example.test")
	var ids []model.ID
	for _, age := range []int{500, 450, 400} {
		s := scanOf(site, now.Add(-time.Duration(age)*24*time.Hour), "7.1.3")
		s.RecordedChanges = []model.Change{}
		must(t, db.SaveScan(ctx, s))
		ids = append(ids, s.ID)
	}
	if _, err := db.Prune(ctx, now); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Scans(ctx, site.ID)
	var left []model.ID
	for _, s := range got {
		left = append(left, s.ID)
		if len(s.Items) == 0 {
			t.Errorf("scan %s kept without its details", s.ID)
		}
	}
	if !slices.Equal(left, ids[1:]) {
		t.Errorf("kept %v, want the latest two complete scans %v", left, ids[1:])
	}
}
