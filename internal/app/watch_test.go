package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// The menu bar icon and the window read one state: the next automatic scan, the urgent sites.
func TestWatchTellsTheNextScanAndTheUrgentSites(t *testing.T) {
	ctx := context.Background()
	m, db, _, _, fresh := scannedIn(t)
	finished := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	err := db.SaveScan(ctx, model.Scan{
		ID: model.NewID(), SiteID: fresh.ID, Started: finished.Add(-time.Second), Finished: finished,
		Outcome: model.ScanCompleted, CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
		Findings: []model.Finding{{Kind: model.KindDebugLog, Subject: "site", Level: model.LevelUrgent}},
	})
	if err != nil {
		t.Fatal(err)
	}
	next := time.Date(2026, 10, 30, 18, 0, 0, 0, time.UTC)
	on := app.NewWatch(m, func(context.Context) (time.Time, bool, error) { return next, true, nil })
	if got, err := on.State(ctx); err != nil || got.Next != "2026-10-30T18:00:00Z" || got.Urgent != 1 {
		t.Fatalf("State() = %+v, %v; want the next scan at 18:00 and one urgent site", got, err)
	}
	off := app.NewWatch(m, func(context.Context) (time.Time, bool, error) { return time.Time{}, false, nil })
	if got, _ := off.State(ctx); got.Next != "" {
		t.Errorf("State().Next = %q while automatic scans are off, want none", got.Next)
	}
}
