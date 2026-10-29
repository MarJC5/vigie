package store_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

// withSites stores a host and two sites.
func withSites(t *testing.T, db *store.DB) (model.Site, model.Site) {
	t.Helper()
	ctx := context.Background()
	host := model.Host{ID: model.NewID(), Name: "h", Alias: "h"}
	a := model.Site{ID: model.NewID(), HostID: host.ID, Name: "a"}
	b := model.Site{ID: model.NewID(), HostID: host.ID, Name: "b"}
	must(t, db.SaveHost(ctx, host))
	must(t, db.SaveSite(ctx, a))
	must(t, db.SaveSite(ctx, b))
	return a, b
}

var t0 = time.Date(2026, 10, 29, 14, 12, 0, 0, time.UTC)

func running(site model.Site, started time.Time, pid int) model.Operation {
	return model.Operation{ID: model.NewID(), SiteID: site.ID, Kind: model.OperationUpdate, Started: started,
		BackupDir: "~/.vigie/backups/a/2026-10-29-1412", PID: pid}
}

func TestOperationIsRecordedStepByStep(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site, _ := withSites(t, db)
	op := running(site, t0, os.Getpid())
	must(t, db.StartOperation(ctx, op))

	backup := model.OperationStep{Position: 0, Step: model.StepBackupDatabase, Started: t0, Outcome: model.StepRunning}
	must(t, db.SaveOperationStep(ctx, op.ID, backup))
	backup.Finished, backup.Outcome, backup.Message = t0.Add(6*time.Second), model.StepDone, "38 MB"
	must(t, db.SaveOperationStep(ctx, op.ID, backup))
	update := model.OperationStep{Position: 1, Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: "acf-pro",
		VersionBefore: "6.8.10", VersionAfter: "6.9.0", Started: t0.Add(7 * time.Second),
		Finished: t0.Add(14 * time.Second), Outcome: model.StepFailed, Message: "home 500"}
	must(t, db.SaveOperationStep(ctx, op.ID, update))
	must(t, db.FinishOperation(ctx, op.ID, model.OperationFailed, t0.Add(15*time.Second), "1 of 2 items"))

	ops, err := db.Operations(ctx, site.ID)
	must(t, err)
	if len(ops) != 1 {
		t.Fatalf("Operations() = %d, want 1", len(ops))
	}
	want := op
	want.Outcome, want.Finished, want.Summary = model.OperationFailed, t0.Add(15*time.Second), "1 of 2 items"
	want.Steps = []model.OperationStep{backup, update}
	if !reflect.DeepEqual(ops[0], want) {
		t.Errorf("operation =\n%+v\nwant\n%+v", ops[0], want)
	}
}

func TestOneRunningOperationPerSite(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	a, b := withSites(t, db)
	first := running(a, t0, os.Getpid())
	must(t, db.StartOperation(ctx, first))
	if err := db.StartOperation(ctx, running(a, t0.Add(time.Second), os.Getpid())); !errors.Is(err, store.ErrBusy) {
		t.Fatalf("second operation on the same site: %v, want ErrBusy", err)
	}
	must(t, db.StartOperation(ctx, running(b, t0, os.Getpid())))
	must(t, db.FinishOperation(ctx, first.ID, model.OperationCompleted, t0.Add(time.Minute), ""))
	must(t, db.StartOperation(ctx, running(a, t0.Add(2*time.Minute), os.Getpid())))
}

// An operation whose process ended (the app quit or crashed) does not block the site: it is
// marked interrupted when the next one starts.
func TestOperationOfAnEndedProcessIsInterrupted(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site, _ := withSites(t, db)
	ended := exec.Command("true")
	must(t, ended.Run())
	stale := running(site, t0, ended.Process.Pid)
	must(t, db.StartOperation(ctx, stale))

	next := running(site, t0.Add(time.Hour), os.Getpid())
	must(t, db.StartOperation(ctx, next))
	ops, err := db.Operations(ctx, site.ID)
	must(t, err)
	if len(ops) != 2 || ops[0].ID != next.ID || ops[1].Outcome != model.OperationInterrupted || !ops[1].Finished.Equal(next.Started) {
		t.Fatalf("operations = %+v, want the new one first and the stale one interrupted", ops)
	}
}

func TestOperationsGoWithTheirSite(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	site, _ := withSites(t, db)
	op := running(site, t0, os.Getpid())
	must(t, db.StartOperation(ctx, op))
	must(t, db.SaveOperationStep(ctx, op.ID, model.OperationStep{Step: model.StepBackupDatabase, Started: t0, Outcome: model.StepRunning}))
	must(t, db.DeleteSite(ctx, site.ID))
	if err := db.StartOperation(ctx, running(site, t0, os.Getpid())); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("operation on a deleted site: %v, want ErrNotFound", err)
	}
	if ops, err := db.Operations(ctx, site.ID); err != nil || len(ops) != 0 {
		t.Errorf("operations of a deleted site = %v, %v", ops, err)
	}
}
