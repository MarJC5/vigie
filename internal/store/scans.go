package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// ScanRepository stores the scans of sites; every scan is kept.
type ScanRepository interface {
	SaveScan(ctx context.Context, s model.Scan) error
	LatestScans(ctx context.Context) (map[model.ID]model.Scan, error)
	ScanCount(ctx context.Context, siteID model.ID) (int, error)
	SaveSubsites(ctx context.Context, siteID model.ID, subsites []model.Subsite) error
}

var _ ScanRepository = (*DB)(nil)

// SaveScan stores a scan and its items.
func (db *DB) SaveScan(ctx context.Context, s model.Scan) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		ok, err := exists(ctx, tx, "sites", s.SiteID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("site %s: %w", s.SiteID, model.ErrNotFound)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO scans (id, site_id, started_at, finished_at, outcome, failure, error,
				wordpress, php_cli, wp_cli, home, core_checked, plugins_checked, themes_checked, languages_checked)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID.String(), s.SiteID.String(), timeText(s.Started), timeText(s.Finished), string(s.Outcome), s.Failure, s.Error,
			s.WordPress, s.PHPCLI, s.WPCLI, s.Home, s.CoreChecked, s.PluginsChecked, s.ThemesChecked, s.LanguagesChecked)
		if err != nil {
			return err
		}
		for i, item := range s.Items {
			_, err := tx.ExecContext(ctx, `INSERT INTO scan_items (scan_id, position, kind, slug, title, status, version,
					language, update_state, update_version, auto_update, on_wordpress_org)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				s.ID.String(), i, string(item.Kind), item.Slug, item.Title, item.Status, item.Version,
				item.Language, string(item.Update), item.UpdateVersion, item.AutoUpdate, item.OnWordPressOrg)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// LatestScans returns the latest scan of each scanned site, with its items.
func (db *DB) LatestScans(ctx context.Context) (map[model.ID]model.Scan, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, site_id, started_at, finished_at, outcome, failure, error,
			wordpress, php_cli, wp_cli, home, core_checked, plugins_checked, themes_checked, languages_checked
		FROM scans s WHERE id = (SELECT max(id) FROM scans WHERE site_id = s.site_id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scans := map[model.ID]model.Scan{}
	byScan := map[string]model.ID{}
	for rows.Next() {
		var s model.Scan
		var id, site, started, finished, outcome string
		if err := rows.Scan(&id, &site, &started, &finished, &outcome, &s.Failure, &s.Error,
			&s.WordPress, &s.PHPCLI, &s.WPCLI, &s.Home,
			&s.CoreChecked, &s.PluginsChecked, &s.ThemesChecked, &s.LanguagesChecked); err != nil {
			return nil, err
		}
		if s.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if s.SiteID, err = uuid.Parse(site); err != nil {
			return nil, err
		}
		if s.Started, err = time.Parse(time.RFC3339Nano, started); err != nil {
			return nil, err
		}
		if s.Finished, err = time.Parse(time.RFC3339Nano, finished); err != nil {
			return nil, err
		}
		s.Outcome = model.ScanOutcome(outcome)
		scans[s.SiteID] = s
		byScan[id] = s.SiteID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return scans, db.loadScanItems(ctx, scans, byScan)
}

// loadScanItems adds the items of the given scans, in their order.
func (db *DB) loadScanItems(ctx context.Context, scans map[model.ID]model.Scan, byScan map[string]model.ID) error {
	rows, err := db.sql.QueryContext(ctx, `SELECT i.scan_id, i.kind, i.slug, i.title, i.status, i.version, i.language,
			i.update_state, i.update_version, i.auto_update, i.on_wordpress_org
		FROM scan_items i JOIN scans s ON s.id = i.scan_id
		WHERE s.id = (SELECT max(id) FROM scans WHERE site_id = s.site_id)
		ORDER BY i.scan_id, i.position`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var scanID, kind, state string
		var item model.ScanItem
		if err := rows.Scan(&scanID, &kind, &item.Slug, &item.Title, &item.Status, &item.Version, &item.Language,
			&state, &item.UpdateVersion, &item.AutoUpdate, &item.OnWordPressOrg); err != nil {
			return err
		}
		item.Kind, item.Update = model.ItemKind(kind), model.UpdateState(state)
		site, ok := byScan[scanID]
		if !ok {
			continue
		}
		s := scans[site]
		s.Items = append(s.Items, item)
		scans[site] = s
	}
	return rows.Err()
}

// ScanCount returns how many scans of a site are kept.
func (db *DB) ScanCount(ctx context.Context, siteID model.ID) (int, error) {
	var n int
	err := db.sql.QueryRowContext(ctx, "SELECT count(*) FROM scans WHERE site_id = ?", siteID.String()).Scan(&n)
	return n, err
}

// SaveSubsites replaces the subsites of a network with those a scan read, keeping the user's pins.
func (db *DB) SaveSubsites(ctx context.Context, siteID model.ID, subsites []model.Subsite) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		id := siteID.String()
		pinned := map[int]bool{}
		rows, err := tx.QueryContext(ctx, "SELECT blog_id FROM subsites WHERE site_id = ? AND pinned = 1", id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var blog int
			if err := rows.Scan(&blog); err != nil {
				rows.Close()
				return err
			}
			pinned[blog] = true
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM subsites WHERE site_id = ?", id); err != nil {
			return err
		}
		for _, sub := range subsites {
			_, err := tx.ExecContext(ctx, `INSERT INTO subsites (site_id, blog_id, url, public, archived, spam, deleted, pinned)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				id, sub.BlogID, sub.URL, sub.Public, sub.Archived, sub.Spam, sub.Deleted, pinned[sub.BlogID])
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func timeText(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
