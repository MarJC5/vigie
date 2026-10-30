package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// SaveIgnore stores an ignore, replacing the one of the same finding of the site.
func (db *DB) SaveIgnore(ctx context.Context, i model.Ignore) error {
	if err := i.Validate(); err != nil {
		return err
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		ok, err := exists(ctx, tx, "sites", i.SiteID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("site %s: %w", i.SiteID, model.ErrNotFound)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO ignores (site_id, finding_key, until_version, reason, created_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (site_id, finding_key) DO UPDATE SET
				until_version = excluded.until_version, reason = excluded.reason, created_at = excluded.created_at`,
			i.SiteID.String(), i.Key, i.UntilVersion, i.Reason, timeText(i.Created))
		return err
	})
}

// DeleteIgnore puts a finding of a site back in its status.
func (db *DB) DeleteIgnore(ctx context.Context, siteID model.ID, key string) error {
	_, err := db.sql.ExecContext(ctx, "DELETE FROM ignores WHERE site_id = ? AND finding_key = ?", siteID.String(), key)
	return err
}

// Ignores returns the ignores of a site, sorted by finding key.
func (db *DB) Ignores(ctx context.Context, siteID model.ID) ([]model.Ignore, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT finding_key, until_version, reason, created_at
		FROM ignores WHERE site_id = ? ORDER BY finding_key`, siteID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Ignore
	for rows.Next() {
		i := model.Ignore{SiteID: siteID}
		var created string
		if err := rows.Scan(&i.Key, &i.UntilVersion, &i.Reason, &created); err != nil {
			return nil, err
		}
		if i.Created, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		list = append(list, i)
	}
	return list, rows.Err()
}
