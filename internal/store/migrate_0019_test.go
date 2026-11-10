package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// A database of Vigie 0.3 gains the journal of the assistants' reads of errors through migration
// 0019, empty, and keeps what it had.
func TestMigration0019AddsTheReadsOfErrors(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, upTo(t, 18), "0.3.1"); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	run := model.Run{ID: model.NewID(), By: model.RunByAssistant, ByClient: "Claude", Started: at, Outcome: model.RunCompleted, Sites: 1}
	if err := db.StartRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "mcp.access", "read"); err != nil {
		t.Fatal(err)
	}

	if err := db.Migrate(ctx, Migrations(), "0.4.0"); err != nil {
		t.Fatal(err)
	}
	if reads, err := db.ErrorReads(ctx, time.Time{}); err != nil || len(reads) != 0 {
		t.Errorf("ErrorReads() = %+v, %v; want none", reads, err)
	}
	if runs, err := db.Runs(ctx, at); err != nil || len(runs) != 1 || runs[0].ID != run.ID {
		t.Errorf("Runs() = %+v, %v; want the run kept", runs, err)
	}
	if value, _, err := db.Setting(ctx, "mcp.access"); err != nil || value != "read" {
		t.Errorf("mcp.access = %q, %v; want read kept", value, err)
	}
	read := model.ErrorRead{ID: model.NewID(), SiteID: model.NewID(), Path: "Shop/shop.example.test", Client: "Claude", At: at.Add(time.Hour)}
	if err := db.SaveErrorRead(ctx, read); err != nil {
		t.Errorf("SaveErrorRead() after the migration: %v", err)
	}
}
