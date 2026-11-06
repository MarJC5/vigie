package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// A copy is kept for the journal, done, cut or resumed (ADR-0041).
func TestACopyIsKept(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	at := time.Date(2026, 11, 3, 20, 41, 0, 0, time.UTC)
	site := model.NewID()
	cut := model.Copy{ID: model.NewID(), SiteID: site, SiteName: "boulangerie-exemple.ch", HostName: "Shared hosting",
		Folder: "/Users/u/Documents/Vigie/Shared hosting/boulangerie-exemple.ch/2026-11-03 20.41", Content: "content",
		Started: at, Finished: at.Add(4 * time.Minute), Outcome: model.CopyFailed, Archives: 7, Bytes: 1_250_000_000, Message: "uploads-3.tar.gz: connection lost",
		By: model.RunByUser}
	resumed := cut
	resumed.ID, resumed.Resumed, resumed.Started, resumed.Finished = model.NewID(), true, at.Add(time.Hour), at.Add(time.Hour+time.Minute)
	resumed.Outcome, resumed.Archives, resumed.Bytes, resumed.Message = model.CopyCompleted, 2, 930_000_000, ""
	must(t, db.SaveCopy(ctx, cut))
	must(t, db.SaveCopy(ctx, resumed))

	got, err := db.Copies(ctx, at.Add(-time.Minute))
	if err != nil || len(got) != 2 {
		t.Fatalf("Copies() = %+v, %v", got, err)
	}
	if got[0] != cut {
		t.Errorf("first = %+v\nwant %+v", got[0], cut)
	}
	if !got[1].Resumed || got[1].Outcome != model.CopyCompleted || got[1].Archives != 2 {
		t.Errorf("second = %+v", got[1])
	}
	if later, _ := db.Copies(ctx, at.Add(30*time.Minute)); len(later) != 1 {
		t.Errorf("since half an hour after: %d copies, want 1", len(later))
	}
	bad := cut
	bad.ID, bad.Content = model.NewID(), "everything"
	if err := db.SaveCopy(ctx, bad); err == nil {
		t.Error("an unknown content was stored")
	}
}

// A copy keeps who started it, an assistant with the client's name (ADR-0043, point 7).
func TestACopyKeepsWhoStartedIt(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	at := time.Date(2026, 11, 6, 10, 30, 0, 0, time.UTC)
	c := model.Copy{ID: model.NewID(), SiteID: model.NewID(), SiteName: "boulangerie-exemple.ch", HostName: "Shared hosting",
		Folder: "/Users/u/Documents/Vigie/copies", Content: "database", Started: at, Finished: at.Add(time.Minute),
		Outcome: model.CopyCompleted, Archives: 1, Bytes: 38_000_000, By: model.RunByAssistant, ByClient: "Claude"}
	must(t, db.SaveCopy(ctx, c))
	got, err := db.Copies(ctx, at)
	if err != nil || len(got) != 1 || got[0] != c {
		t.Errorf("Copies() = %+v, %v\nwant %+v", got, err, c)
	}
}
