package update_test

import (
	"context"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/httpcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/wpcli"
)

var broken = httpcheck.Result{URL: "http://updates.example.test/", Status: 500, Problem: httpcheck.ProblemCriticalError}

// rollbacks returns the rollbacks stored for the bench's site, the newest first.
func (b *runBench) rollbacks() []model.Operation {
	b.t.Helper()
	ops, err := b.db.Operations(context.Background(), b.job.Site.ID)
	if err != nil {
		b.t.Fatal(err)
	}
	return slices.DeleteFunc(ops, func(op model.Operation) bool { return op.Kind != model.OperationRollback })
}

// restoreAkismet answers the restore of the plugin's folder from its archive, and returns its line.
func (b *runBench) restoreAkismet() string {
	line := b.line(remote.Restore(b.dir+"/plugin-akismet.tar.gz", root+"/wp-content/plugins/akismet"))
	b.fake.Set(line, sshx.Result{})
	return line
}

func kinds(steps []model.OperationStep) []model.StepKind {
	out := []model.StepKind{}
	for _, st := range steps {
		out = append(out, st.Step)
	}
	return out
}

// A plugin that breaks the site goes back on its own, as an operation of its own, and the run goes
// on with the next items (ADR-0042, points 1 and 3).
func TestBrokenPluginRollsBackOnItsOwn(t *testing.T) {
	b := newRun(t)
	b.job.AutoRollback = true
	b.checks.results = []httpcheck.Result{broken} // the plugin's check; the rollback's and the theme's pass
	restore := b.restoreAkismet()
	u := b.updater()
	var heard []model.OperationKind
	u.OnStep = func(op model.Operation, _ model.OperationStep) { heard = append(heard, op.Kind) }

	op, err := u.Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}, Themes: []string{"twentytwentyfour"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationCompleted || op.Summary != "1 of 2, 1 rolled back" {
		t.Errorf("update = %q, %q; want completed, 1 of 2, 1 rolled back", op.Outcome, op.Summary)
	}
	calls := b.fake.Calls()
	theme := index(calls, wp(t, wpcli.UpdateOne(wpcli.ExtensionThemes, "twentytwentyfour")))
	if index(calls, restore) < 0 || theme < index(calls, restore) {
		t.Errorf("restore at %d, theme updated at %d: the plugin goes back before the next item", index(calls, restore), theme)
	}
	backs := b.rollbacks()
	if len(backs) != 1 {
		t.Fatalf("rollbacks = %d, want 1", len(backs))
	}
	back := backs[0]
	if back.OfOperation != op.ID || !back.Automatic || back.Outcome != model.OperationCompleted || back.BackupDir != op.BackupDir {
		t.Errorf("rollback = %+v", back)
	}
	if want := []model.StepKind{model.StepRestoreFiles, model.StepCheck}; !slices.Equal(kinds(back.Steps), want) {
		t.Errorf("rollback steps %v, want %v", kinds(back.Steps), want)
	}
	if !slices.Contains(heard, model.OperationRollback) {
		t.Error("the rollback's steps were not followed")
	}
	if _, ok := update.RollbackOf(op); ok {
		t.Error("a rollback is still offered for an update whose item went back on its own")
	}
}

// A rollback that fails stops the run there, the next items left (ADR-0042, point 3).
func TestRollbackOnItsOwnThatFailsStops(t *testing.T) {
	b := newRun(t)
	b.job.AutoRollback = true
	b.checks.results = []httpcheck.Result{broken, broken} // the plugin's check, then the rollback's
	b.restoreAkismet()
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}, Themes: []string{"twentytwentyfour"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationFailed || op.Summary != "0 of 2" {
		t.Errorf("update = %q, %q; want failed, 0 of 2", op.Outcome, op.Summary)
	}
	if index(b.fake.Calls(), wp(t, wpcli.UpdateOne(wpcli.ExtensionThemes, "twentytwentyfour"))) >= 0 {
		t.Error("the theme was updated after a rollback that failed")
	}
	if backs := b.rollbacks(); len(backs) != 1 || backs[0].Outcome != model.OperationFailed || !backs[0].Automatic {
		t.Errorf("rollbacks = %+v", backs)
	}
	// The user can still try the rollback again.
	if item, ok := update.RollbackOf(op); !ok || item.Slug != "akismet" {
		t.Errorf("RollbackOf() = %+v, %v", item, ok)
	}
}

// WordPress never goes back on its own: its rollback restores the database (ADR-0042, point 2).
func TestWordPressNeverRollsBackOnItsOwn(t *testing.T) {
	b := newRun(t)
	b.job.AutoRollback = true
	b.checks.results = []httpcheck.Result{broken}
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Core: true, Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationFailed || len(b.rollbacks()) != 0 {
		t.Errorf("update %q, rollbacks %d; want failed, none", op.Outcome, len(b.rollbacks()))
	}
	if item, ok := update.RollbackOf(op); !ok || item.Kind != model.ItemCore {
		t.Errorf("RollbackOf() = %+v, %v; want WordPress offered", item, ok)
	}
}

// A plugin WordPress could not install kept its old version: nothing to roll back, the run stops.
func TestPluginNotInstalledStopsWithoutRollback(t *testing.T) {
	b := newRun(t)
	b.job.AutoRollback = true
	b.fake.Set(wp(t, wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet")), sshx.Result{ExitCode: 1, Stderr: []byte("Error: Download failed.\n")})
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}, Themes: []string{"twentytwentyfour"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationFailed || len(b.rollbacks()) != 0 {
		t.Errorf("update %q, rollbacks %d; want failed, none", op.Outcome, len(b.rollbacks()))
	}
}

// The setting off, a broken plugin stops the run and waits for the user, as before ADR-0042.
func TestWithoutRollbackOnItsOwnTheRunStops(t *testing.T) {
	b := newRun(t)
	b.checks.results = []httpcheck.Result{broken}
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}, Themes: []string{"twentytwentyfour"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationFailed || len(b.rollbacks()) != 0 {
		t.Errorf("update %q, rollbacks %d; want failed, none", op.Outcome, len(b.rollbacks()))
	}
	if _, ok := update.RollbackOf(op); !ok {
		t.Error("no rollback offered")
	}
}

// A cancel during the rollback lets it end, then the run stops before the next item.
func TestCancelDuringTheRollbackLetsItEnd(t *testing.T) {
	b := newRun(t)
	b.job.AutoRollback = true
	b.checks.results = []httpcheck.Result{broken}
	restore := b.restoreAkismet()
	ctx, cancel := context.WithCancel(context.Background())
	u := b.updater()
	u.Runner = cancelling{Runner: b.fake, at: restore, cancel: cancel}
	op, err := u.Run(ctx, b.job, b.preview, update.Selection{Plugins: []string{"akismet"}, Themes: []string{"twentytwentyfour"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationCancelled {
		t.Errorf("update %q, want cancelled", op.Outcome)
	}
	if backs := b.rollbacks(); len(backs) != 1 || backs[0].Outcome != model.OperationCompleted {
		t.Errorf("rollbacks = %+v; want one completed", backs)
	}
	if index(b.fake.Calls(), wp(t, wpcli.UpdateOne(wpcli.ExtensionThemes, "twentytwentyfour"))) >= 0 {
		t.Error("the theme was updated after the cancel")
	}
}
