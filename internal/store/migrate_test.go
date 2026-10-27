package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/MarJC5/vigie/internal/store"
)

var (
	v1 = &fstest.MapFile{Data: []byte("CREATE TABLE notes (id TEXT PRIMARY KEY, body TEXT NOT NULL);")}
	v2 = &fstest.MapFile{Data: []byte("ALTER TABLE notes ADD COLUMN color INTEGER NOT NULL DEFAULT 0;")}
)

func openTemp(t *testing.T) (*store.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Application Support", "vigie.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func userVersion(t *testing.T, db *store.DB) int {
	t.Helper()
	v, err := db.SchemaVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOpenCreatesPrivateFile(t *testing.T) {
	_, path := openTemp(t)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("database permissions = %o, want 600", perm)
	}
}

func TestOpenEnablesForeignKeys(t *testing.T) {
	db, _ := openTemp(t)
	var on int
	if err := db.SQL().QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&on); err != nil {
		t.Fatal(err)
	}
	if on != 1 {
		t.Fatalf("foreign_keys = %d, want 1", on)
	}
}

func TestMigrateEmptyDatabase(t *testing.T) {
	db, path := openTemp(t)
	migrations := fstest.MapFS{"0001_notes.sql": v1, "0002_color.sql": v2}
	if err := db.Migrate(context.Background(), migrations, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if v := userVersion(t, db); v != 2 {
		t.Fatalf("schema version = %d, want 2", v)
	}
	if _, err := os.Stat(path + ".before-1.0.0"); !os.IsNotExist(err) {
		t.Fatalf("a new database needs no backup copy, got err=%v", err)
	}
}

func TestMigrateCopiesExistingDatabaseFirst(t *testing.T) {
	ctx := context.Background()
	db, path := openTemp(t)
	if err := db.Migrate(ctx, fstest.MapFS{"0001_notes.sql": v1}, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL().ExecContext(ctx, "INSERT INTO notes (id, body) VALUES ('a', 'kept')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, fstest.MapFS{"0001_notes.sql": v1, "0002_color.sql": v2}, "1.1.0"); err != nil {
		t.Fatal(err)
	}
	if v := userVersion(t, db); v != 2 {
		t.Fatalf("schema version = %d, want 2", v)
	}
	backup, err := store.Open(ctx, path+".before-1.1.0")
	if err != nil {
		t.Fatalf("backup copy: %v", err)
	}
	defer backup.Close()
	if v := userVersion(t, backup); v != 1 {
		t.Fatalf("backup schema version = %d, want 1", v)
	}
	var body string
	if err := backup.SQL().QueryRowContext(ctx, "SELECT body FROM notes WHERE id = 'a'").Scan(&body); err != nil || body != "kept" {
		t.Fatalf("backup content = %q, %v", body, err)
	}
}

func TestMigrateFailureLeavesDatabaseUnchanged(t *testing.T) {
	ctx := context.Background()
	db, _ := openTemp(t)
	if err := db.Migrate(ctx, fstest.MapFS{"0001_notes.sql": v1}, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	broken := fstest.MapFS{
		"0001_notes.sql": v1,
		"0002_good.sql":  v2,
		"0003_bad.sql":   &fstest.MapFile{Data: []byte("CREATE TABLE broken (;")},
	}
	if err := db.Migrate(ctx, broken, "1.1.0"); err == nil {
		t.Fatal("Migrate with a broken migration: want an error")
	}
	if v := userVersion(t, db); v != 1 {
		t.Fatalf("schema version after failure = %d, want 1", v)
	}
	if _, err := db.SQL().ExecContext(ctx, "SELECT color FROM notes"); err == nil {
		t.Fatal("migration 0002 applied although 0003 failed: want all or nothing")
	}
}

func TestMigrateUpToDateDoesNothing(t *testing.T) {
	ctx := context.Background()
	db, path := openTemp(t)
	migrations := fstest.MapFS{"0001_notes.sql": v1}
	for _, label := range []string{"1.0.0", "1.0.1"} {
		if err := db.Migrate(ctx, migrations, label); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path + ".before-1.0.1"); !os.IsNotExist(err) {
		t.Fatalf("no migration to apply, no copy expected, got err=%v", err)
	}
}

func TestMigrateRejectsBadFileNames(t *testing.T) {
	db, _ := openTemp(t)
	for _, name := range []string{"init.sql", "0001.sql", "0001_notes.txt", "0002_gap.sql"} {
		err := db.Migrate(context.Background(), fstest.MapFS{name: v1}, "1.0.0")
		if err == nil {
			t.Errorf("Migrate with %q: want an error", name)
		}
	}
}

func TestEmbeddedMigrationsApply(t *testing.T) {
	db, _ := openTemp(t)
	if err := db.Migrate(context.Background(), store.Migrations(), "dev"); err != nil {
		t.Fatalf("embedded migrations: %v", err)
	}
	if userVersion(t, db) < 1 {
		t.Fatal("embedded migrations: schema version not set")
	}
}
