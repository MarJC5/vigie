package store

import (
	"context"
	"time"

	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// SaveCopy records a copy of a site to the computer, or its resume (ADR-0041).
func (db *DB) SaveCopy(ctx context.Context, c model.Copy) error {
	resumed := 0
	if c.Resumed {
		resumed = 1
	}
	_, err := db.sql.ExecContext(ctx, `INSERT INTO copies
		(id, site_id, site_name, host_name, folder, content, resumed, started_at, finished_at, outcome, archives, bytes, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID.String(), c.SiteID.String(), c.SiteName, c.HostName, c.Folder, c.Content, resumed,
		timeText(c.Started), timeText(c.Finished), string(c.Outcome), c.Archives, c.Bytes, c.Message)
	return err
}

// Copies returns the copies started since a time, oldest first.
func (db *DB) Copies(ctx context.Context, since time.Time) ([]model.Copy, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, site_id, site_name, host_name, folder, content, resumed,
		started_at, finished_at, outcome, archives, bytes, message
		FROM copies WHERE started_at >= ? ORDER BY started_at, id`, timeText(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var copies []model.Copy
	for rows.Next() {
		var c model.Copy
		var id, site, started, finished, outcome string
		var resumed int
		if err := rows.Scan(&id, &site, &c.SiteName, &c.HostName, &c.Folder, &c.Content, &resumed,
			&started, &finished, &outcome, &c.Archives, &c.Bytes, &c.Message); err != nil {
			return nil, err
		}
		if c.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if c.SiteID, err = uuid.Parse(site); err != nil {
			return nil, err
		}
		if c.Started, err = time.Parse(time.RFC3339Nano, started); err != nil {
			return nil, err
		}
		if c.Finished, err = time.Parse(time.RFC3339Nano, finished); err != nil {
			return nil, err
		}
		c.Resumed, c.Outcome = resumed == 1, model.CopyOutcome(outcome)
		copies = append(copies, c)
	}
	return copies, rows.Err()
}
