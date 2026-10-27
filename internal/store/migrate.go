package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
)

//go:embed migrations/*.sql
var embedded embed.FS

// Migrations returns the schema migrations shipped with Vigie.
func Migrations() fs.FS {
	sub, err := fs.Sub(embedded, "migrations")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}

var migrationName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads NNNN_name.sql files, numbered from 1 without gaps.
func loadMigrations(migrations fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return nil, err
	}
	var list []migration
	for _, e := range entries {
		m := migrationName.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			return nil, fmt.Errorf("migration %q: name must be NNNN_name.sql", e.Name())
		}
		version, _ := strconv.Atoi(m[1])
		data, err := fs.ReadFile(migrations, e.Name())
		if err != nil {
			return nil, err
		}
		list = append(list, migration{version: version, name: e.Name(), sql: string(data)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })
	for i, m := range list {
		if m.version != i+1 {
			return nil, fmt.Errorf("migration %q: expected number %04d", m.name, i+1)
		}
	}
	return list, nil
}

// Migrate brings the schema to the latest version. Before changing an existing database it writes
// a consistent copy next to it, named after label (the Vigie version). All pending migrations run
// in one transaction: on failure nothing is applied.
func (db *DB) Migrate(ctx context.Context, migrations fs.FS, label string) error {
	list, err := loadMigrations(migrations)
	if err != nil {
		return err
	}
	current, err := db.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	if current > len(list) {
		return fmt.Errorf("database schema %d is newer than this Vigie (%d)", current, len(list))
	}
	pending := list[current:]
	if len(pending) == 0 {
		return nil
	}
	if current > 0 {
		if err := db.backup(ctx, label); err != nil {
			return fmt.Errorf("copy before migrating: %w", err)
		}
	}

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, m := range pending {
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}
	// PRAGMA does not accept a bound parameter; the value is an integer we computed.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", len(list))); err != nil {
		return err
	}
	return tx.Commit()
}

// backup writes a consistent copy of the database with VACUUM INTO, unless one already exists for
// this label (a previous attempt that failed left the database unchanged).
func (db *DB) backup(ctx context.Context, label string) error {
	target := db.path + ".before-" + label
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if _, err := db.sql.ExecContext(ctx, "VACUUM INTO ?", target); err != nil {
		return err
	}
	return os.Chmod(target, 0o600)
}
