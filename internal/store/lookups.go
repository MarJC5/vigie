package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LookupAnswer is the last answer of an outside lookup (ADR-0014): its HTTP status, its body, and
// when it was fetched.
type LookupAnswer struct {
	Status  int
	Body    []byte
	Fetched time.Time
}

// Lookup returns the cached answer of source for key, and whether there is one.
func (db *DB) Lookup(ctx context.Context, source, key string) (LookupAnswer, bool, error) {
	var a LookupAnswer
	var fetched string
	err := db.sql.QueryRowContext(ctx, "SELECT status, body, fetched_at FROM lookup_cache WHERE source = ? AND key = ?",
		source, key).Scan(&a.Status, &a.Body, &fetched)
	if errors.Is(err, sql.ErrNoRows) {
		return LookupAnswer{}, false, nil
	}
	if err != nil {
		return LookupAnswer{}, false, err
	}
	if a.Fetched, err = time.Parse(time.RFC3339Nano, fetched); err != nil {
		return LookupAnswer{}, false, err
	}
	return a, true, nil
}

// PutLookup stores the answer of source for key, replacing the older one.
func (db *DB) PutLookup(ctx context.Context, source, key string, a LookupAnswer) error {
	_, err := db.sql.ExecContext(ctx, `INSERT INTO lookup_cache (source, key, status, body, fetched_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (source, key) DO UPDATE SET status = excluded.status, body = excluded.body, fetched_at = excluded.fetched_at`,
		source, key, a.Status, a.Body, timeText(a.Fetched))
	return err
}
