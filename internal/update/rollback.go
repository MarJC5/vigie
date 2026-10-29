package update

import (
	"context"
	"errors"
	"fmt"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// ErrNothingToRollBack is returned for an operation with nothing to undo.
var ErrNothingToRollBack = errors.New("nothing to roll back")

// RollbackOf tells what a rollback of an update would undo: a plugin or theme that was installed
// and failed its check, or WordPress after any failure once its maintenance mode was on (its
// files may be half replaced). A plugin WordPress could not install kept its old version.
func RollbackOf(op model.Operation) (Item, bool) {
	if op.Kind != model.OperationUpdate || (op.Outcome != model.OperationFailed && op.Outcome != model.OperationInterrupted) {
		return Item{}, false
	}
	failed := -1
	for i, st := range op.Steps {
		if st.Outcome == model.StepFailed || st.Outcome == model.StepRunning {
			failed = i
		}
	}
	if failed < 0 {
		return Item{}, false
	}
	// The item being updated when it failed: the last update step started before.
	for i := failed; i >= 0; i-- {
		st := op.Steps[i]
		if st.Step != model.StepUpdate {
			continue
		}
		item := Item{Kind: st.ItemKind, Slug: st.Slug, From: st.VersionBefore, To: st.VersionAfter}
		switch {
		case st.ItemKind == model.ItemCore:
			return item, true
		case (st.ItemKind == model.ItemPlugin || st.ItemKind == model.ItemTheme) && st.Outcome == model.StepDone:
			return item, op.Steps[failed].Step == model.StepCheck
		}
		return Item{}, false
	}
	return Item{}, false
}

// Rollback undoes the item that failed in an update: its folder back from its archive, and for
// WordPress its files and its database, in maintenance mode; then the site is checked again.
// The rollback is an operation of its own, recorded step by step.
func (u Updater) Rollback(ctx context.Context, job Job, failed model.Operation) (model.Operation, error) {
	item, ok := RollbackOf(failed)
	if !ok {
		return model.Operation{}, ErrNothingToRollBack
	}
	folder := ""
	for _, st := range failed.Steps {
		if st.Step == model.StepBackupFiles && st.ItemKind == item.Kind && st.Slug == item.Slug && st.Outcome == model.StepDone {
			folder = st.Message
		}
	}
	if folder == "" {
		return model.Operation{}, fmt.Errorf("no archive of %s: %w", item.Slug, ErrNothingToRollBack)
	}
	op := model.Operation{
		ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationRollback, Started: u.Now(),
		Outcome: model.OperationRunning, BackupDir: failed.BackupDir, PID: u.PID,
	}
	if err := u.Store.StartOperation(ctx, op); err != nil {
		return op, err
	}
	r := &run{s: session{u: u, job: job}, op: &op}
	// A rollback is never left halfway.
	work := context.WithoutCancel(ctx)
	outcome := model.OperationFailed
	if r.restore(work, item, folder) {
		outcome = model.OperationCompleted
	}
	op.Outcome, op.Finished = outcome, u.Now()
	op.Summary = fmt.Sprintf("%s %s back to %s", item.Kind, item.Slug, item.From)
	if err := u.Store.FinishOperation(work, op.ID, outcome, op.Finished, op.Summary); err != nil {
		return op, err
	}
	return op, nil
}

// restore puts the item back, then checks the site; false when a step failed.
func (r *run) restore(ctx context.Context, item Item, folder string) bool {
	if err := r.prepare(ctx); err != nil {
		r.fail(ctx, model.OperationStep{Step: model.StepRestoreFiles, ItemKind: item.Kind, Slug: item.Slug}, err)
		return false
	}
	exec := r.shellIn(ctx)
	back := model.OperationStep{Step: model.StepRestoreFiles, ItemKind: item.Kind, Slug: item.Slug,
		VersionBefore: item.To, VersionAfter: item.From}
	if item.Kind != model.ItemCore {
		if !r.step(ctx, back, func() (string, error) {
			return folder, exec(remote.Restore(archiveOf(r.op.BackupDir, item), folder))
		}) {
			return false
		}
		return r.check(ctx, item, -1)
	}
	if !r.restoreCore(ctx, back, folder) {
		return false
	}
	return r.check(ctx, item, -1)
}

// restoreCore puts back the files and the database of WordPress in maintenance mode, always
// turned off at the end.
func (r *run) restoreCore(ctx context.Context, back model.OperationStep, root string) (restored bool) {
	if !r.maintenance(ctx, true) {
		return false
	}
	defer func() { restored = r.maintenance(ctx, false) && restored }()
	exec := r.shellIn(ctx)
	if !r.step(ctx, back, func() (string, error) {
		return root, exec(remote.RestoreCore(archiveOf(r.op.BackupDir, Item{Kind: model.ItemCore}), root))
	}) {
		return false
	}
	dump := r.op.BackupDir + "/database.sql"
	return r.step(ctx, model.OperationStep{Step: model.StepRestoreDatabase}, func() (string, error) {
		if err := exec(remote.Uncompress(dump + ".gz")); err != nil {
			return "", err
		}
		if err := r.wpDone(ctx, wpcli.DatabaseImport(dump)); err != nil {
			return "", err
		}
		// The dump stays compressed in the backups; its copy goes.
		return dump + ".gz", exec(remote.RemoveFile(dump))
	})
}
