package store

import (
	"context"
	"time"

	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// StartRun records a run as it starts.
func (db *DB) StartRun(ctx context.Context, r model.Run) error {
	by, client := startedBy(r.By, r.ByClient)
	_, err := db.sql.ExecContext(ctx, `INSERT INTO runs (id, path, started_by, by_client, started_at, outcome, sites)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, r.ID.String(), r.Path, by, client, timeText(r.Started), string(r.Outcome), r.Sites)
	return err
}

// startedBy is the origin as stored: the window when none is given, and the client's name kept for
// an assistant only (ADR-0043, point 7).
func startedBy(by model.RunBy, client string) (string, string) {
	if by == "" {
		by = model.RunByUser
	}
	if by != model.RunByAssistant {
		client = ""
	}
	return string(by), client
}

// EndRun records how a run ended.
func (db *DB) EndRun(ctx context.Context, id model.ID, finished time.Time, outcome model.RunOutcome) error {
	_, err := db.sql.ExecContext(ctx, "UPDATE runs SET finished_at = ?, outcome = ? WHERE id = ?",
		timeText(finished), string(outcome), id.String())
	return err
}

// Runs returns the runs started since a time, oldest first.
func (db *DB) Runs(ctx context.Context, since time.Time) ([]model.Run, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, path, started_by, by_client, started_at, finished_at, outcome, sites
		FROM runs WHERE started_at >= ? ORDER BY started_at, id`, timeText(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []model.Run
	for rows.Next() {
		var r model.Run
		var id, by, started, finished, outcome string
		if err := rows.Scan(&id, &r.Path, &by, &r.ByClient, &started, &finished, &outcome, &r.Sites); err != nil {
			return nil, err
		}
		if r.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if r.Started, err = time.Parse(time.RFC3339Nano, started); err != nil {
			return nil, err
		}
		if finished != "" {
			if r.Finished, err = time.Parse(time.RFC3339Nano, finished); err != nil {
				return nil, err
			}
		}
		r.By, r.Outcome = model.RunBy(by), model.RunOutcome(outcome)
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
