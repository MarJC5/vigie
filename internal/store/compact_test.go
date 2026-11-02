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

// A scan keeps the site's backups it read; one that did not read them says so (ADR-0033).
func TestScanKeepsTheBackupsItRead(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site := siteOnHost(t, db, "a.example.test")
	at := time.Date(2026, 10, 29, 13, 12, 3, 0, time.UTC)
	read := scanOf(site, at, "7.1.3")
	read.Backups = []model.Backup{{At: at.Add(-24 * time.Hour), Bytes: 45_100_000}, {At: at, Bytes: 48_200_000}}
	none := scanOf(site, at.Add(time.Hour), "7.1.3")
	none.Backups = []model.Backup{}
	unread := scanOf(site, at.Add(2*time.Hour), "7.1.3")
	for _, s := range []model.Scan{read, none, unread} {
		must(t, db.SaveScan(ctx, s))
	}
	scans, err := db.Scans(ctx, site.ID)
	if err != nil || len(scans) != 3 {
		t.Fatalf("Scans() = %d, %v", len(scans), err)
	}
	if got := scans[0].Backups; len(got) != 2 || !got[1].At.Equal(at) || got[1].Bytes != 48_200_000 {
		t.Errorf("backups read = %+v", got)
	}
	if scans[1].Backups == nil || len(scans[1].Backups) != 0 {
		t.Errorf("none there = %#v, want an empty list", scans[1].Backups)
	}
	if scans[2].Backups != nil {
		t.Errorf("not read = %#v, want nil", scans[2].Backups)
	}
}
