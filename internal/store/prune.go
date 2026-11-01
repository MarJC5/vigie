package store

import (
	"context"
	"database/sql"
	"time"
)

// How long the history is kept (ADR-0031).
const (
	// DetailsKept is how long a scan keeps its items, findings and subsites.
	DetailsKept = 90 * 24 * time.Hour
	// ChangesKept is how long a scan that changed something is kept, without its details; runs and
	// notices too.
	ChangesKept = 365 * 24 * time.Hour
)

// PruneReport tells what a pruning removed.
type PruneReport struct {
	Scans   int // scans removed
	Details int // scans left without their details
	Runs    int
	Notices int
}

// Prune keeps the history in check (ADR-0031): each site's latest scan, latest complete scan and the
// complete scan before it, whatever their age, with their details; the details of other scans for
// DetailsKept; after that, only the scans that changed something (their noted status, outcome,
// failure or changes differ from the scan before them, ADR-0030), until ChangesKept; runs and
// notices until ChangesKept. A scan not noted yet is never removed. Site by site, each in a
// transaction of its own: a failure leaves the rest for the next time.
func (db *DB) Prune(ctx context.Context, now time.Time) (PruneReport, error) {
	details, changes := timeText(now.Add(-DetailsKept)), timeText(now.Add(-ChangesKept))
	var report PruneReport
	rows, err := db.sql.QueryContext(ctx, "SELECT DISTINCT site_id FROM scans WHERE started_at < ?", details)
	if err != nil {
		return report, err
	}
	var sites []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return report, err
		}
		sites = append(sites, id)
	}
	if err := rows.Close(); err != nil {
		return report, err
	}
	for _, site := range sites {
		err := db.withTx(ctx, func(tx *sql.Tx) error {
			return pruneSite(ctx, tx, site, details, changes, &report)
		})
		if err != nil {
			return report, err
		}
	}
	for _, step := range []struct {
		query string
		count *int
	}{
		{"DELETE FROM runs WHERE started_at < ?", &report.Runs},
		{"DELETE FROM notices WHERE sent_at < ?", &report.Notices},
	} {
		res, err := db.sql.ExecContext(ctx, step.query, changes)
		if err != nil {
			return report, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return report, err
		}
		*step.count = int(n)
	}
	return report, nil
}

// pruneSite prunes one site's scans.
func pruneSite(ctx context.Context, tx *sql.Tx, site, details, changes string, report *PruneReport) error {
	// Kept whatever their age: the latest scan, the latest complete scan, the complete one before it.
	var latest, complete, before sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT max(id), max(CASE WHEN outcome = 'completed' THEN id END) FROM scans WHERE site_id = ?`,
		site).Scan(&latest, &complete)
	if err != nil {
		return err
	}
	if complete.Valid {
		err := tx.QueryRowContext(ctx, `SELECT max(id) FROM scans WHERE site_id = ? AND outcome = 'completed' AND id < ?`,
			site, complete.String).Scan(&before)
		if err != nil {
			return err
		}
	}
	keep := []any{latest.String, complete.String, before.String}
	removed := func(query string, args ...any) error {
		res, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		report.Scans += int(n)
		return err
	}
	// Older than a year: removed.
	if err := removed(`DELETE FROM scans WHERE site_id = ? AND started_at < ? AND recorded_changes IS NOT NULL
		AND id NOT IN (?, ?, ?)`, append([]any{site, changes}, keep...)...); err != nil {
		return err
	}
	// Older than DetailsKept and changing nothing from the scan before: removed. The scans before
	// are compared as they are now: a run of scans the same keeps its first.
	if err := removed(`DELETE FROM scans WHERE id IN (
		SELECT id FROM (
			SELECT id, started_at, recorded_changes, recorded_status, outcome, failure,
				lag(id) OVER w AS previous, lag(recorded_status) OVER w AS previous_status,
				lag(outcome) OVER w AS previous_outcome, lag(failure) OVER w AS previous_failure
			FROM scans WHERE site_id = ? WINDOW w AS (ORDER BY id))
		WHERE started_at < ? AND recorded_changes = '[]' AND previous IS NOT NULL
			AND recorded_status = previous_status AND outcome = previous_outcome AND failure = previous_failure
			AND id NOT IN (?, ?, ?))`, append([]any{site, details}, keep...)...); err != nil {
		return err
	}
	// Older than DetailsKept and kept: without their details.
	old := `SELECT id FROM scans WHERE site_id = ? AND started_at < ? AND id NOT IN (?, ?, ?)`
	args := append([]any{site, details}, keep...)
	var bare int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM scans s WHERE s.id IN (`+old+`) AND (
		EXISTS (SELECT 1 FROM scan_items WHERE scan_id = s.id) OR EXISTS (SELECT 1 FROM scan_findings WHERE scan_id = s.id)
		OR EXISTS (SELECT 1 FROM scan_subsites WHERE scan_id = s.id))`, args...).Scan(&bare)
	if err != nil {
		return err
	}
	report.Details += bare
	for _, table := range []string{"scan_items", "scan_findings", "scan_subsites"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE scan_id IN ("+old+")", args...); err != nil {
			return err
		}
	}
	return nil
}
