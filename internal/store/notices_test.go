package store_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// A notification sent is kept with its run, for the Activity page.
func TestANoticeIsKeptWithItsRun(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	sent := time.Date(2026, 10, 31, 3, 5, 0, 0, time.UTC)
	run := model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: sent.Add(-5 * time.Minute), Outcome: model.RunCompleted, Sites: 2}
	must(t, db.StartRun(ctx, run))
	notice := model.Notice{ID: model.NewID(), RunID: run.ID, Kind: "urgent", Names: []string{"shop.example.test", "blog.example.test"}, Sent: sent}
	must(t, db.SaveNotice(ctx, notice))

	notices, err := db.Notices(ctx, sent.Add(-time.Hour))
	if err != nil || len(notices) != 1 {
		t.Fatalf("Notices() = %+v, %v; want one", notices, err)
	}
	got := notices[0]
	if got.RunID != run.ID || got.Kind != "urgent" || !slices.Equal(got.Names, notice.Names) || !got.Sent.Equal(sent) {
		t.Errorf("Notices()[0] = %+v, want %+v", got, notice)
	}
}
