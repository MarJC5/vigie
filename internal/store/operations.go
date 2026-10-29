package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// ErrBusy is returned when an operation already runs on the site.
var ErrBusy = errors.New("an operation is already running on this site")

// OperationRepository stores the operations that write on servers, step by step.
type OperationRepository interface {
	StartOperation(ctx context.Context, op model.Operation) error
	SaveOperationStep(ctx context.Context, operationID model.ID, step model.OperationStep) error
	FinishOperation(ctx context.Context, id model.ID, outcome model.OperationOutcome, finished time.Time, summary string) error
	Operations(ctx context.Context, siteID model.ID) ([]model.Operation, error)
}

var _ OperationRepository = (*DB)(nil)

// StartOperation stores a new running operation, unless one already runs on the site. The window
// and the command line share the database, so the check and the insert are one transaction. A
// running operation whose process ended is marked interrupted, at the new operation's start.
func (db *DB) StartOperation(ctx context.Context, op model.Operation) error {
	return db.withTx(ctx, func(tx *sql.Tx) error {
		ok, err := exists(ctx, tx, "sites", op.SiteID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("site %s: %w", op.SiteID, model.ErrNotFound)
		}
		stale, err := runningOperations(ctx, tx, op.SiteID)
		if err != nil {
			return err
		}
		for id, pid := range stale {
			if processAlive(pid) {
				return ErrBusy
			}
			_, err := tx.ExecContext(ctx, "UPDATE operations SET outcome = ?, finished_at = ? WHERE id = ?",
				string(model.OperationInterrupted), timeText(op.Started), id)
			if err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO operations (id, site_id, kind, started_at, outcome, summary, backup_dir, pid)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			op.ID.String(), op.SiteID.String(), string(op.Kind), timeText(op.Started), string(model.OperationRunning),
			op.Summary, op.BackupDir, op.PID)
		return err
	})
}

// runningOperations returns the running operations of a site with their process.
func runningOperations(ctx context.Context, tx *sql.Tx, siteID model.ID) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, pid FROM operations WHERE site_id = ? AND outcome = ?",
		siteID.String(), string(model.OperationRunning))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	running := map[string]int{}
	for rows.Next() {
		var id string
		var pid int
		if err := rows.Scan(&id, &pid); err != nil {
			return nil, err
		}
		running[id] = pid
	}
	return running, rows.Err()
}

// processAlive tells whether a process of this user still runs: signal 0 checks without sending.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// SaveOperationStep stores a step, or updates it when one exists at its position.
func (db *DB) SaveOperationStep(ctx context.Context, operationID model.ID, st model.OperationStep) error {
	_, err := db.sql.ExecContext(ctx, `INSERT INTO operation_steps (operation_id, position, step, item_kind, slug,
			version_before, version_after, started_at, finished_at, outcome, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (operation_id, position) DO UPDATE SET step = excluded.step, item_kind = excluded.item_kind,
			slug = excluded.slug, version_before = excluded.version_before, version_after = excluded.version_after,
			started_at = excluded.started_at, finished_at = excluded.finished_at, outcome = excluded.outcome,
			message = excluded.message`,
		operationID.String(), st.Position, string(st.Step), string(st.ItemKind), st.Slug, st.VersionBefore,
		st.VersionAfter, timeText(st.Started), optionalTime(st.Finished), string(st.Outcome), st.Message)
	return err
}

// FinishOperation records the end of a running operation.
func (db *DB) FinishOperation(ctx context.Context, id model.ID, outcome model.OperationOutcome, finished time.Time, summary string) error {
	res, err := db.sql.ExecContext(ctx, "UPDATE operations SET outcome = ?, finished_at = ?, summary = ? WHERE id = ? AND outcome = ?",
		string(outcome), timeText(finished), summary, id.String(), string(model.OperationRunning))
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return fmt.Errorf("running operation %s: %w", id, model.ErrNotFound)
	}
	return nil
}

// Operations returns the operations of a site with their steps, the newest first.
func (db *DB) Operations(ctx context.Context, siteID model.ID) ([]model.Operation, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, kind, started_at, finished_at, outcome, summary, backup_dir, pid
		FROM operations WHERE site_id = ? ORDER BY id DESC`, siteID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ops []model.Operation
	index := map[string]int{}
	for rows.Next() {
		op := model.Operation{SiteID: siteID}
		var id, started, finished string
		if err := rows.Scan(&id, &op.Kind, &started, &finished, &op.Outcome, &op.Summary, &op.BackupDir, &op.PID); err != nil {
			return nil, err
		}
		if op.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		if op.Started, err = parseOptionalTime(started); err != nil {
			return nil, err
		}
		if op.Finished, err = parseOptionalTime(finished); err != nil {
			return nil, err
		}
		index[id] = len(ops)
		ops = append(ops, op)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ops, db.loadOperationSteps(ctx, siteID, ops, index)
}

func (db *DB) loadOperationSteps(ctx context.Context, siteID model.ID, ops []model.Operation, index map[string]int) error {
	rows, err := db.sql.QueryContext(ctx, `SELECT s.operation_id, s.position, s.step, s.item_kind, s.slug, s.version_before,
			s.version_after, s.started_at, s.finished_at, s.outcome, s.message
		FROM operation_steps s JOIN operations o ON o.id = s.operation_id
		WHERE o.site_id = ? ORDER BY s.operation_id, s.position`, siteID.String())
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var st model.OperationStep
		var opID, started, finished string
		if err := rows.Scan(&opID, &st.Position, &st.Step, &st.ItemKind, &st.Slug, &st.VersionBefore, &st.VersionAfter,
			&started, &finished, &st.Outcome, &st.Message); err != nil {
			return err
		}
		if st.Started, err = parseOptionalTime(started); err != nil {
			return err
		}
		if st.Finished, err = parseOptionalTime(finished); err != nil {
			return err
		}
		i := index[opID]
		ops[i].Steps = append(ops[i].Steps, st)
	}
	return rows.Err()
}

// optionalTime stores a zero time as an empty text.
func optionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return timeText(t)
}

func parseOptionalTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}
