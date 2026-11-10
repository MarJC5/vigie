package store

import (
	"context"
	"time"

	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// SaveErrorRead notes an assistant's read of a site's errors in the journal.
func (db *DB) SaveErrorRead(ctx context.Context, r model.ErrorRead) error {
	_, err := db.sql.ExecContext(ctx, "INSERT INTO error_reads (id, site_id, path, by_client, read_at) VALUES (?, ?, ?, ?, ?)",
		r.ID.String(), r.SiteID.String(), r.Path, r.Client, timeText(r.At))
	return err
}

// ErrorReads returns the assistants' reads of errors since a time, oldest first.
func (db *DB) ErrorReads(ctx context.Context, since time.Time) ([]model.ErrorRead, error) {
	rows, err := db.sql.QueryContext(ctx, "SELECT id, site_id, path, by_client, read_at FROM error_reads WHERE read_at >= ? ORDER BY read_at, id",
		timeText(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reads []model.ErrorRead
	for rows.Next() {
		var r model.ErrorRead
		var id, site, at string
		if err := rows.Scan(&id, &site, &r.Path, &r.Client, &at); err != nil {
			return nil, err
		}
		if r.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if r.SiteID, err = uuid.Parse(site); err != nil {
			return nil, err
		}
		if r.At, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return nil, err
		}
		reads = append(reads, r)
	}
	return reads, rows.Err()
}
