package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"

	"modernc.org/sqlite"
)

// DB is Vigie's database.
type DB struct {
	sql  *sql.DB
	path string
}

func init() {
	// The database path can come from the command line (--data-dir): a _pragma value of the DSN
	// must never run a second statement (modernc.org/sqlite recommends this setting for any DSN
	// that is not a compile-time constant).
	sqlite.StrictPragmas(true)
}

// Open opens or creates the database file at path, readable by its owner only, with foreign keys
// enforced.
func Open(ctx context.Context, path string) (*DB, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, err
	}

	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"}).String()
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection: SQLite has a single writer, and scans of several hosts write concurrently.
	conn.SetMaxOpenConns(1)
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &DB{sql: conn, path: path}, nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.sql.Close()
}

// SchemaVersion returns the version of the schema, stored in PRAGMA user_version.
func (db *DB) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := db.sql.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v)
	return v, err
}
