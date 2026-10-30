package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/MarJC5/vigie/internal/model"
)

// ImportBatch is what an import adds: new nodes, parents before children, the ignores and the
// history of the sites added.
type ImportBatch struct {
	Folders    []model.Folder
	Hosts      []model.Host
	Sites      []model.Site
	Ignores    []model.Ignore
	Scans      []model.Scan
	Operations []model.Operation
}

// ErrIDTaken is returned when an import would add a folder, host or site under the ID of one
// already stored: an import never changes what is there.
var ErrIDTaken = errors.New("ID already used")

// Import adds a batch in one transaction, with the checks of each kind (names unique among
// siblings, no folder inside itself): it all goes in, or nothing does.
func (db *DB) Import(ctx context.Context, b ImportBatch) error {
	for _, f := range b.Folders {
		if err := f.Validate(); err != nil {
			return err
		}
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		for _, table := range []struct {
			name string
			ids  []model.ID
		}{
			{"folders", ids(b.Folders, func(f model.Folder) model.ID { return f.ID })},
			{"hosts", ids(b.Hosts, func(h model.Host) model.ID { return h.ID })},
			{"sites", ids(b.Sites, func(s model.Site) model.ID { return s.ID })},
		} {
			for _, id := range table.ids {
				taken, err := exists(ctx, tx, table.name, id)
				if err != nil {
					return err
				}
				if taken {
					return fmt.Errorf("%s %s: %w", table.name, id, ErrIDTaken)
				}
			}
		}
		for _, f := range b.Folders {
			if err := saveFolder(ctx, tx, f); err != nil {
				return err
			}
		}
		for _, h := range b.Hosts {
			if err := saveHost(ctx, tx, h); err != nil {
				return err
			}
		}
		for _, s := range b.Sites {
			if err := saveSite(ctx, tx, s); err != nil {
				return err
			}
		}
		for _, i := range b.Ignores {
			if err := i.Validate(); err != nil {
				return err
			}
			if err := saveIgnore(ctx, tx, i); err != nil {
				return err
			}
		}
		for _, s := range b.Scans {
			if err := saveScan(ctx, tx, s); err != nil {
				return err
			}
		}
		for _, op := range b.Operations {
			if err := saveOperation(ctx, tx, op); err != nil {
				return err
			}
		}
		return nil
	})
}

// saveOperation stores an operation that ended elsewhere, with its steps; no process of this
// machine runs it.
func saveOperation(ctx context.Context, tx *sql.Tx, op model.Operation) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO operations (id, site_id, kind, started_at, finished_at, outcome, summary, backup_dir, pid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		op.ID.String(), op.SiteID.String(), string(op.Kind), timeText(op.Started), optionalTime(op.Finished),
		string(op.Outcome), op.Summary, op.BackupDir)
	if err != nil {
		return err
	}
	for _, st := range op.Steps {
		if err := saveStep(ctx, tx, op.ID, st); err != nil {
			return err
		}
	}
	return nil
}

func ids[T any](items []T, id func(T) model.ID) []model.ID {
	out := make([]model.ID, len(items))
	for i, item := range items {
		out[i] = id(item)
	}
	return out
}
