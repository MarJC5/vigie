package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// ScanRepository stores the scans of sites; every scan is kept.
type ScanRepository interface {
	SaveScan(ctx context.Context, s model.Scan) error
	LatestScans(ctx context.Context) (map[model.ID]model.Scan, error)
	LatestScan(ctx context.Context, siteID model.ID) (model.Scan, bool, error)
	ScanCount(ctx context.Context, siteID model.ID) (int, error)
	CompleteScanBefore(ctx context.Context, siteID, scanID model.ID) (model.Scan, bool, error)
	SaveSubsites(ctx context.Context, siteID model.ID, subsites []model.Subsite) error
}

var _ ScanRepository = (*DB)(nil)

// SaveScan stores a scan and its items.
func (db *DB) SaveScan(ctx context.Context, s model.Scan) error {
	return db.withTx(ctx, func(tx *sql.Tx) error { return saveScan(ctx, tx, s) })
}

func saveScan(ctx context.Context, tx *sql.Tx, s model.Scan) error {
	ok, err := exists(ctx, tx, "sites", s.SiteID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("site %s: %w", s.SiteID, model.ErrNotFound)
	}
	// The plugins active network-wide, or NULL when the scan read no subsites.
	var networkPlugins any
	if s.Network != nil {
		if networkPlugins, err = jsonList(s.Network.Plugins); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO scans (id, site_id, started_at, finished_at, outcome, failure, error,
			wordpress, php_cli, wp_cli, home, core_checked, plugins_checked, themes_checked, languages_checked,
			files_checked_at, run_id, site_url, php_web, php_web_seen_at, php_web_unknown, admin_email, admin_email_pending,
			debug_log_path, debug_log_size, debug_log_written, network_plugins)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID.String(), s.SiteID.String(), timeText(s.Started), timeText(s.Finished), string(s.Outcome), s.Failure, s.Error,
		s.WordPress, s.PHPCLI, s.WPCLI, s.Home, s.CoreChecked, s.PluginsChecked, s.ThemesChecked, s.LanguagesChecked,
		optionalTime(s.FilesChecked), optionalID(s.RunID), s.SiteURL, s.PHPWeb.Version, optionalTime(s.PHPWeb.Seen),
		string(s.PHPWeb.Unknown), s.AdminEmail, s.AdminEmailPending, s.DebugLog.Path, s.DebugLog.Size, s.DebugLog.Written,
		networkPlugins)
	if err != nil {
		return err
	}
	if s.Network != nil {
		for _, sub := range s.Network.Sites {
			plugins, err := jsonList(sub.Plugins)
			if err != nil {
				return err
			}
			admins, err := jsonList(sub.Admins)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO scan_subsites (scan_id, blog_id, theme, theme_name, theme_version,
					admin_email, plugins, admins) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				s.ID.String(), sub.BlogID, sub.Theme, sub.ThemeName, sub.ThemeVersion, sub.AdminEmail, plugins, admins)
			if err != nil {
				return err
			}
		}
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
	for i, f := range s.Findings {
		facts, err := json.Marshal(f.Facts)
		if err != nil {
			return err
		}
		if f.Facts == nil {
			facts = []byte("{}")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO scan_findings (scan_id, position, kind, subject, identifier, version,
				level, facts) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID.String(), i, string(f.Kind), f.Subject, f.Identifier, f.Version, string(f.Level), string(facts))
		if err != nil {
			return err
		}
	}
	return nil
}

// LatestScans returns, for each scanned site, its latest completed scan with its items and its
// findings, those an ignore of the site covers marked; a later scan that did not complete (cancelled,
// failed) is its Attempt. A site never scanned to the end gets its latest scan.
func (db *DB) LatestScans(ctx context.Context) (map[model.ID]model.Scan, error) {
	return db.latest(ctx, "")
}

// LatestScan is LatestScans for one site, as its page reads it; ok is false when it was never
// scanned.
func (db *DB) LatestScan(ctx context.Context, siteID model.ID) (model.Scan, bool, error) {
	scans, err := db.latest(ctx, siteID.String())
	scan, ok := scans[siteID]
	return scan, ok, err
}

// latest reads the latest scans of every site, or of one site when siteID is not empty. A site's
// scans are picked in one pass over the index scans_site_outcome, whatever the history kept: its
// latest completed one, else its latest; and its latest whatever its outcome.
func (db *DB) latest(ctx context.Context, siteID string) (map[model.ID]model.Scan, error) {
	filter, args, ignoresWhere := "", []any{}, ""
	if siteID != "" {
		filter, args, ignoresWhere = "WHERE site_id = ?", []any{siteID}, "WHERE site_id = ?"
	}
	shownScans := `s.id IN (SELECT coalesce(max(CASE WHEN outcome = 'completed' THEN id END), max(id)) FROM scans ` + filter + ` GROUP BY site_id)`
	latestScans := `s.id IN (SELECT max(id) FROM scans ` + filter + ` GROUP BY site_id)`
	shown, err := db.readScans(ctx, shownScans, args...)
	if err != nil {
		return nil, err
	}
	if err := db.loadScanParts(ctx, shownScans, shown, args...); err != nil {
		return nil, err
	}
	latest, err := db.readScans(ctx, latestScans, args...)
	if err != nil {
		return nil, err
	}
	ignores, err := db.ignores(ctx, ignoresWhere, args...)
	if err != nil {
		return nil, err
	}
	scans := map[model.ID]model.Scan{}
	for _, s := range shown {
		s.ApplyIgnores(ignores[s.SiteID])
		scans[s.SiteID] = s
	}
	for _, l := range latest {
		if s := scans[l.SiteID]; s.ID != l.ID {
			s.Attempt = &l
			scans[l.SiteID] = s
		}
	}
	return scans, nil
}

// Scans returns every scan kept of a site, the oldest first, with its items and findings.
func (db *DB) Scans(ctx context.Context, siteID model.ID) ([]model.Scan, error) {
	scans, err := db.readScans(ctx, "s.site_id = ?", siteID.String())
	if err != nil {
		return nil, err
	}
	return scans, db.loadScanParts(ctx, "s.site_id = ?", scans, siteID.String())
}

// readScans reads the scans that match a condition on scans s, without their items and findings,
// in the order they were made.
func (db *DB) readScans(ctx context.Context, where string, args ...any) ([]model.Scan, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, site_id, started_at, finished_at, outcome, failure, error,
			wordpress, php_cli, wp_cli, home, core_checked, plugins_checked, themes_checked, languages_checked,
			files_checked_at, run_id, site_url, php_web, php_web_seen_at, php_web_unknown, admin_email, admin_email_pending,
			debug_log_path, debug_log_size, debug_log_written, network_plugins
		FROM scans s WHERE `+where+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scans []model.Scan
	for rows.Next() {
		var s model.Scan
		var id, site, started, finished, outcome, filesChecked, run, webSeen, webUnknown string
		var networkPlugins sql.NullString
		if err := rows.Scan(&id, &site, &started, &finished, &outcome, &s.Failure, &s.Error,
			&s.WordPress, &s.PHPCLI, &s.WPCLI, &s.Home,
			&s.CoreChecked, &s.PluginsChecked, &s.ThemesChecked, &s.LanguagesChecked, &filesChecked, &run, &s.SiteURL,
			&s.PHPWeb.Version, &webSeen, &webUnknown, &s.AdminEmail, &s.AdminEmailPending,
			&s.DebugLog.Path, &s.DebugLog.Size, &s.DebugLog.Written, &networkPlugins); err != nil {
			return nil, err
		}
		if networkPlugins.Valid {
			s.Network = &model.NetworkRead{Sites: []model.SubsiteRead{}}
			if err := json.Unmarshal([]byte(networkPlugins.String), &s.Network.Plugins); err != nil {
				return nil, err
			}
		}
		s.PHPWeb.Unknown = model.PHPWebUnknown(webUnknown)
		if s.PHPWeb.Seen, err = parseOptionalTime(webSeen); err != nil {
			return nil, err
		}
		if s.RunID, err = parseOptionalID(run); err != nil {
			return nil, err
		}
		if filesChecked != "" {
			if s.FilesChecked, err = time.Parse(time.RFC3339Nano, filesChecked); err != nil {
				return nil, err
			}
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
		scans = append(scans, s)
	}
	return scans, rows.Err()
}

// loadScanParts adds their items and findings, in their order, to scans read with the same
// condition.
func (db *DB) loadScanParts(ctx context.Context, where string, scans []model.Scan, args ...any) error {
	index := map[string]int{}
	for i, s := range scans {
		index[s.ID.String()] = i
	}
	rows, err := db.sql.QueryContext(ctx, `SELECT i.scan_id, i.kind, i.slug, i.title, i.status, i.version, i.language,
			i.update_state, i.update_version, i.auto_update, i.on_wordpress_org
		FROM scan_items i JOIN scans s ON s.id = i.scan_id
		WHERE `+where+`
		ORDER BY i.scan_id, i.position`, args...)
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
		if i, ok := index[scanID]; ok {
			scans[i].Items = append(scans[i].Items, item)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := db.loadSubsiteReads(ctx, where, scans, index, args...); err != nil {
		return err
	}
	return db.loadFindings(ctx, where, scans, index, args...)
}

// loadSubsiteReads adds to the networks' scans what each subsite had (ADR-0029).
func (db *DB) loadSubsiteReads(ctx context.Context, where string, scans []model.Scan, index map[string]int, args ...any) error {
	rows, err := db.sql.QueryContext(ctx, `SELECT u.scan_id, u.blog_id, u.theme, u.theme_name, u.theme_version,
			u.admin_email, u.plugins, u.admins
		FROM scan_subsites u JOIN scans s ON s.id = u.scan_id
		WHERE `+where+`
		ORDER BY u.scan_id, u.blog_id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var scanID, plugins, admins string
		var sub model.SubsiteRead
		if err := rows.Scan(&scanID, &sub.BlogID, &sub.Theme, &sub.ThemeName, &sub.ThemeVersion, &sub.AdminEmail,
			&plugins, &admins); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(plugins), &sub.Plugins); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(admins), &sub.Admins); err != nil {
			return err
		}
		if i, ok := index[scanID]; ok && scans[i].Network != nil {
			scans[i].Network.Sites = append(scans[i].Network.Sites, sub)
		}
	}
	return rows.Err()
}

// jsonList stores a list of strings as JSON, an empty one as [].
func jsonList(list []string) (string, error) {
	if list == nil {
		list = []string{}
	}
	data, err := json.Marshal(list)
	return string(data), err
}

// loadFindings adds their findings to scans read by readScans with the same condition; index maps
// a scan's id to its place in scans.
func (db *DB) loadFindings(ctx context.Context, where string, scans []model.Scan, index map[string]int, args ...any) error {
	rows, err := db.sql.QueryContext(ctx, `SELECT f.scan_id, f.kind, f.subject, f.identifier, f.version, f.level, f.facts
		FROM scan_findings f JOIN scans s ON s.id = f.scan_id
		WHERE `+where+`
		ORDER BY f.scan_id, f.position`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var scanID, kind, level, facts string
		var f model.Finding
		if err := rows.Scan(&scanID, &kind, &f.Subject, &f.Identifier, &f.Version, &level, &facts); err != nil {
			return err
		}
		f.Kind, f.Level = model.FindingKind(kind), model.FindingLevel(level)
		if facts != "{}" {
			if err := json.Unmarshal([]byte(facts), &f.Facts); err != nil {
				return fmt.Errorf("facts of a finding of scan %s: %w", scanID, err)
			}
		}
		if i, ok := index[scanID]; ok {
			scans[i].Findings = append(scans[i].Findings, f)
		}
	}
	return rows.Err()
}

// ScanStatuses returns, for the Activity page, the scans started since a time and, for each of
// their sites, the scan just before and the complete scan before, oldest first: with their items,
// their findings and the site's ignores, so each tells its status and what changed (ADR-0027).
// sites keeps those of some sites only; nil keeps every site's.
func (db *DB) ScanStatuses(ctx context.Context, since time.Time, sites []model.ID) ([]model.Scan, error) {
	// Each subquery runs once, by the index of the start (scans_started), not once per scan kept.
	where := `s.started_at >= ?1 OR s.id IN (
		SELECT max(p.id) FROM scans p WHERE p.started_at < ?1 AND p.site_id IN (SELECT site_id FROM scans WHERE started_at >= ?1)
		GROUP BY p.site_id
		UNION
		SELECT max(p.id) FROM scans p WHERE p.started_at < ?1 AND p.outcome = 'completed'
			AND p.site_id IN (SELECT site_id FROM scans WHERE started_at >= ?1)
		GROUP BY p.site_id)`
	args := []any{timeText(since)}
	// Only the sites asked for, when there are some: a journal filtered on a host reads its sites'.
	if sites != nil {
		marks := make([]string, len(sites))
		for i, id := range sites {
			marks[i] = "?" + strconv.Itoa(i+2)
			args = append(args, id.String())
		}
		where = "(" + where + ") AND s.site_id IN (" + strings.Join(marks, ", ") + ")"
	}
	scans, err := db.readScans(ctx, where, args...)
	if err != nil {
		return nil, err
	}
	if err := db.loadScanParts(ctx, where, scans, args...); err != nil {
		return nil, err
	}
	ignores, err := db.ignores(ctx, "")
	if err != nil {
		return nil, err
	}
	for i := range scans {
		scans[i].ApplyIgnores(ignores[scans[i].SiteID])
	}
	return scans, nil
}

// CompleteScanBefore returns the complete scan of a site made before the scan given, with its items,
// findings and the site's ignores; ok is false when there is none (ADR-0027).
func (db *DB) CompleteScanBefore(ctx context.Context, siteID, scanID model.ID) (scan model.Scan, ok bool, err error) {
	where := `s.id = (SELECT max(p.id) FROM scans p WHERE p.site_id = ?1 AND p.id < ?2 AND p.outcome = 'completed')`
	scans, err := db.readScans(ctx, where, siteID.String(), scanID.String())
	if err != nil || len(scans) == 0 {
		return model.Scan{}, false, err
	}
	if err := db.loadScanParts(ctx, where, scans, siteID.String(), scanID.String()); err != nil {
		return model.Scan{}, false, err
	}
	ignores, err := db.ignores(ctx, "")
	if err != nil {
		return model.Scan{}, false, err
	}
	scans[0].ApplyIgnores(ignores[siteID])
	return scans[0], true, nil
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

// optionalID stores an absent ID as an empty text.
func optionalID(id model.ID) string {
	if id == model.NoID {
		return ""
	}
	return id.String()
}

func parseOptionalID(s string) (model.ID, error) {
	if s == "" {
		return model.NoID, nil
	}
	return uuid.Parse(s)
}
