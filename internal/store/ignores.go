package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// SaveIgnore stores an ignore, replacing the one of the same finding of the site.
func (db *DB) SaveIgnore(ctx context.Context, i model.Ignore) error {
	if err := i.Validate(); err != nil {
		return err
	}
	return db.withTx(ctx, func(tx *sql.Tx) error { return saveIgnore(ctx, tx, i) })
}

func saveIgnore(ctx context.Context, tx *sql.Tx, i model.Ignore) error {
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
}

// DeleteIgnore puts a finding of a site back in its status.
func (db *DB) DeleteIgnore(ctx context.Context, siteID model.ID, key string) error {
	_, err := db.sql.ExecContext(ctx, "DELETE FROM ignores WHERE site_id = ? AND finding_key = ?", siteID.String(), key)
	return err
}

// Ignores returns the ignores of a site, sorted by finding key.
func (db *DB) Ignores(ctx context.Context, siteID model.ID) ([]model.Ignore, error) {
	bySite, err := db.ignores(ctx, "WHERE site_id = ?", siteID.String())
	return bySite[siteID], err
}

// ignores returns the ignores that match a condition, by site, each sorted by finding key.
func (db *DB) ignores(ctx context.Context, where string, args ...any) (map[model.ID][]model.Ignore, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT site_id, finding_key, until_version, reason, created_at
		FROM ignores `+where+` ORDER BY site_id, finding_key`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySite := map[model.ID][]model.Ignore{}
	for rows.Next() {
		var i model.Ignore
		var site, created string
		if err := rows.Scan(&site, &i.Key, &i.UntilVersion, &i.Reason, &created); err != nil {
			return nil, err
		}
		if i.SiteID, err = uuid.Parse(site); err != nil {
			return nil, err
		}
		if i.Created, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		bySite[i.SiteID] = append(bySite[i.SiteID], i)
	}
	return bySite, rows.Err()
}
