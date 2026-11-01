package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// The space the history frees is told, and compacting gives it back (ADR-0031).
func TestCompactGivesTheFreeSpaceBack(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "a.example.test")
	start := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	for i := range 200 {
		s := scanOf(site, start.Add(time.Duration(i)*time.Hour), "7.1.3")
		s.RecordedChanges = []model.Change{}
		must(t, db.SaveScan(ctx, s))
	}
	full, err := db.Size(ctx)
	if err != nil || full.Bytes == 0 || full.Free != 0 {
		t.Fatalf("Size() = %+v, %v; want a size, nothing free", full, err)
	}
	if _, err := db.Prune(ctx, start.Add(800*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	pruned, err := db.Size(ctx)
	if err != nil || pruned.Bytes != full.Bytes || pruned.Free == 0 {
		t.Fatalf("after pruning = %+v, %v; want the same file, space free in it", pruned, err)
	}
	must(t, db.Compact(ctx))
	compacted, err := db.Size(ctx)
	if err != nil || compacted.Bytes >= pruned.Bytes || compacted.Free != 0 {
		t.Fatalf("after compacting = %+v, %v; want a smaller file, nothing free (was %+v)", compacted, err, pruned)
	}
}
