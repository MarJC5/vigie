package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// A run is kept with who started it and how it ended; its scans point to it.
func TestARunIsKeptWithItsScans(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "shop.example.test")
	start := time.Date(2026, 10, 31, 3, 0, 0, 0, time.UTC)
	run := model.Run{ID: model.NewID(), Path: "", By: model.RunBySchedule, Started: start, Outcome: model.RunRunning, Sites: 1}
	must(t, db.StartRun(ctx, run))
	scan := scanOf(site, start, "7.1.3")
	scan.RunID = run.ID
	must(t, db.SaveScan(ctx, scan))
	must(t, db.EndRun(ctx, run.ID, start.Add(time.Minute), model.RunCompleted))

	runs, err := db.Runs(ctx, start.Add(-time.Hour))
	if err != nil || len(runs) != 1 {
		t.Fatalf("Runs() = %+v, %v; want the run", runs, err)
	}
	got := runs[0]
	if got.ID != run.ID || got.By != model.RunBySchedule || got.Outcome != model.RunCompleted || got.Sites != 1 ||
		!got.Finished.Equal(start.Add(time.Minute)) {
		t.Errorf("Runs()[0] = %+v", got)
	}
	if later, _ := db.Runs(ctx, start.Add(time.Hour)); len(later) != 0 {
		t.Errorf("Runs() since after it = %+v, want none", later)
	}
	history, err := db.Scans(ctx, site.ID)
	if err != nil || len(history) != 1 || history[0].RunID != run.ID {
		t.Fatalf("Scans() = %+v, %v; want the scan of the run", history, err)
	}
}
