package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// SaveSetAside sets a version aside on its site, replacing the one of the same item (ADR-0042).
func (db *DB) SaveSetAside(ctx context.Context, s model.SetAside) error {
	if err := s.Validate(); err != nil {
		return err
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		ok, err := exists(ctx, tx, "sites", s.SiteID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("site %s: %w", s.SiteID, model.ErrNotFound)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO set_asides (site_id, item_kind, slug, version, operation_id, reason, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (site_id, item_kind, slug) DO UPDATE SET version = excluded.version,
				operation_id = excluded.operation_id, reason = excluded.reason, created_at = excluded.created_at`,
			s.SiteID.String(), string(s.Kind), s.Slug, s.Version, optionalID(s.OperationID), s.Reason, timeText(s.Created))
		return err
	})
}

// DeleteSetAside ends the set-aside of an item of a site: a newer version is offered, or the user
// updated it alone.
func (db *DB) DeleteSetAside(ctx context.Context, siteID model.ID, kind model.ItemKind, slug string) error {
	_, err := db.sql.ExecContext(ctx, "DELETE FROM set_asides WHERE site_id = ? AND item_kind = ? AND slug = ?",
		siteID.String(), string(kind), slug)
	return err
}

// SetAsides returns the versions set aside on a site, sorted by kind (plugins first) and slug.
func (db *DB) SetAsides(ctx context.Context, siteID model.ID) ([]model.SetAside, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT item_kind, slug, version, operation_id, reason, created_at
		FROM set_asides WHERE site_id = ? ORDER BY item_kind, slug`, siteID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.SetAside
	for rows.Next() {
		s := model.SetAside{SiteID: siteID}
		var op, created string
		if err := rows.Scan(&s.Kind, &s.Slug, &s.Version, &op, &s.Reason, &created); err != nil {
			return nil, err
		}
		if s.OperationID, err = parseOptionalID(op); err != nil {
			return nil, err
		}
		if s.Created, err = time.Parse(time.RFC3339Nano, created); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}
