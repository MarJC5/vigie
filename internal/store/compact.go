package store

import "context"

// DatabaseSize is the size of the database file, and the space in it free for reuse: what the
// history pruned (ADR-0031), which SQLite reuses without giving it back to the disk.
type DatabaseSize struct {
	Bytes int64
	Free  int64
}

// Size returns the database's size and its free space, from SQLite's pages.
func (db *DB) Size(ctx context.Context) (DatabaseSize, error) {
	var pages, size, free int64
	for _, q := range []struct {
		pragma string
		into   *int64
	}{{"PRAGMA page_count", &pages}, {"PRAGMA page_size", &size}, {"PRAGMA freelist_count", &free}} {
		if err := db.sql.QueryRowContext(ctx, q.pragma).Scan(q.into); err != nil {
			return DatabaseSize{}, err
		}
	}
	return DatabaseSize{Bytes: pages * size, Free: free * size}, nil
}

// Compact rebuilds the database file without its free space (VACUUM): a few seconds, the database
// locked meanwhile.
func (db *DB) Compact(ctx context.Context) error {
	_, err := db.sql.ExecContext(ctx, "VACUUM")
	return err
}
