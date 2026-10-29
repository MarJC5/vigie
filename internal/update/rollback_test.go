package update_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/httpcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// A plugin that broke the site goes back to its archived folder, then the site is checked again.
func TestRollbackOfAPlugin(t *testing.T) {
	b := newRun(t)
	b.checks.results = []httpcheck.Result{{URL: "http://updates.example.test/", Status: 500, Problem: httpcheck.ProblemCriticalError}}
	failed, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	item, ok := update.RollbackOf(failed)
	if !ok || item != (update.Item{Kind: model.ItemPlugin, Slug: "akismet", From: "5.3", To: "5.7.2"}) {
		t.Fatalf("RollbackOf() = %+v, %v", item, ok)
	}
	restore := b.line(remote.Restore(b.dir+"/plugin-akismet.tar.gz", root+"/wp-content/plugins/akismet"))
	b.fake.Set(restore, sshx.Result{})

	op, err := b.updater().Rollback(context.Background(), b.job, failed)
	if err != nil {
		t.Fatal(err)
	}
	if op.Kind != model.OperationRollback || op.Outcome != model.OperationCompleted || op.BackupDir != b.dir {
		t.Fatalf("rollback = %+v", op)
	}
	steps := []model.StepKind{}
	for _, st := range op.Steps {
		steps = append(steps, st.Step)
	}
	if want := []model.StepKind{model.StepRestoreFiles, model.StepCheck}; !slices.Equal(steps, want) {
		t.Errorf("steps %v, want %v", steps, want)
	}
	if index(b.fake.Calls(), restore) < 0 {
		t.Error("the folder was not restored")
	}
}

// WordPress goes back with its files and its database, in maintenance mode, always turned off.
func TestRollbackOfWordPress(t *testing.T) {
	b := newRun(t)
	b.fake.Set(wp(t, wpcli.UpdateDatabase(false)), sshx.Result{ExitCode: 1, Stderr: []byte("Error: Database upgrade failed.\n")})
	failed, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Core: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := update.RollbackOf(failed); !ok {
		t.Fatal("no rollback offered after a failed WordPress update")
	}
	dump := b.dir + "/database.sql"
	lines := []string{
		wp(t, wpcli.Maintenance(true)),
		b.line(remote.RestoreCore(b.dir+"/core.tar.gz", root)),
		b.line(remote.Uncompress(dump + ".gz")),
		wp(t, wpcli.DatabaseImport(dump)),
		b.line(remote.RemoveFile(dump)),
		wp(t, wpcli.Maintenance(false)),
	}
	for _, l := range lines[1:5] {
		b.fake.Set(l, sshx.Result{})
	}
	before := len(b.fake.Calls())
	op, err := b.updater().Rollback(context.Background(), b.job, failed)
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationCompleted {
		t.Fatalf("rollback = %+v", op)
	}
	calls := b.fake.Calls()[before:]
	last := -1
	for _, l := range lines {
		i := index(calls, l)
		if i <= last {
			t.Fatalf("%s at %d, after %d", l, i, last)
		}
		last = i
	}
}

// When the database cannot be restored, maintenance mode is still turned off.
func TestRollbackFailureTurnsMaintenanceOff(t *testing.T) {
	b := newRun(t)
	b.fake.Set(wp(t, wpcli.UpdateCore("7.1.4")), sshx.Result{ExitCode: 1, Stderr: []byte("Error: Download failed.\n")})
	failed, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Core: true})
	if err != nil {
		t.Fatal(err)
	}
	b.fake.Set(b.line(remote.RestoreCore(b.dir+"/core.tar.gz", root)), sshx.Result{})
	b.fake.Set(b.line(remote.Uncompress(b.dir+"/database.sql.gz")), sshx.Result{ExitCode: 1, Stderr: []byte("gzip: unexpected end of file\n")})
	before := len(b.fake.Calls())
	op, err := b.updater().Rollback(context.Background(), b.job, failed)
	if err != nil {
		t.Fatal(err)
	}
	calls := b.fake.Calls()[before:]
	if op.Outcome != model.OperationFailed || index(calls, wp(t, wpcli.Maintenance(false))) < 0 {
		t.Fatalf("outcome %q, calls %v", op.Outcome, calls)
	}
}

// Nothing to undo: a run that completed, or a plugin WordPress could not install (it kept the
// old version).
func TestNothingToRollBack(t *testing.T) {
	b := newRun(t)
	done, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.updater().Rollback(context.Background(), b.job, done); !errors.Is(err, update.ErrNothingToRollBack) {
		t.Errorf("rollback of a completed run: %v, want ErrNothingToRollBack", err)
	}
	b.fake.Set(wp(t, wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet")), sshx.Result{ExitCode: 1,
		Stdout: []byte(`[{"name":"akismet","old_version":"5.3","new_version":"5.7.2","status":"Error"}]`),
		Stderr: []byte("Error: No plugins updated (1 failed).\n")})
	notInstalled, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := update.RollbackOf(notInstalled); ok {
		t.Error("a rollback offered for a plugin that was not installed")
	}
}
