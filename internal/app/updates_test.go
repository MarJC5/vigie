package app_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

// fakeUpdater records its operations in the database and reports two steps per site; a site
// named in fail breaks on its check.
type fakeUpdater struct {
	db     *store.DB
	onStep func(model.Operation, model.OperationStep)
	fail   map[string]bool
	gate   chan struct{} // when set, a run waits for it, so a test sees it running
}

func (f fakeUpdater) Preview(_ context.Context, job update.Job) (update.Preview, error) {
	if job.Site.Name == "broken" {
		return update.Preview{}, wpcli.ErrCannotLoad
	}
	return update.Preview{DumpTool: true, Plugins: []update.Item{{Kind: model.ItemPlugin, Slug: "akismet", From: "5.3", To: "5.7.2"}}}, nil
}

func (f fakeUpdater) Run(ctx context.Context, job update.Job, _ update.Preview, _ update.Selection) (model.Operation, error) {
	return f.operation(ctx, job, model.OperationUpdate, f.fail[job.Site.Name])
}

func (f fakeUpdater) Rollback(ctx context.Context, job update.Job, _ model.Operation) (model.Operation, error) {
	return f.operation(ctx, job, model.OperationRollback, false)
}

func (f fakeUpdater) operation(ctx context.Context, job update.Job, kind model.OperationKind, fails bool) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: model.NewID(), SiteID: job.Site.ID, Kind: kind, Started: now, Outcome: model.OperationRunning,
		BackupDir: ".vigie/backups/" + job.Site.ID.String() + "/20261029-131203", PID: 1}
	if err := f.db.StartOperation(ctx, op); err != nil {
		return op, err
	}
	steps := []model.OperationStep{
		{Position: 0, Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: "akismet", VersionBefore: "5.3", VersionAfter: "5.7.2", Started: now},
		{Position: 1, Step: model.StepCheck, ItemKind: model.ItemPlugin, Slug: "akismet", Started: now},
	}
	if kind == model.OperationRollback {
		steps[0].Step = model.StepRestoreFiles
	}
	op.Outcome = model.OperationCompleted
	for i, st := range steps {
		st.Outcome = model.StepRunning
		f.onStep(op, st)
		if f.gate != nil && i == 0 {
			<-f.gate
		}
		st.Outcome, st.Finished = model.StepDone, now
		if fails && st.Step == model.StepCheck {
			st.Outcome, st.Message, op.Outcome = model.StepFailed, "http://a1/ answers 500 (server-error)", model.OperationFailed
		}
		if err := f.db.SaveOperationStep(ctx, op.ID, st); err != nil {
			return op, err
		}
		op.Steps = append(op.Steps, st)
		f.onStep(op, st)
	}
	return op, f.db.FinishOperation(ctx, op.ID, op.Outcome, now, "1 of 1")
}

// updateEvents records what the service sends to the window.
type updateEvents struct {
	mu     sync.Mutex
	states []app.UpdatesState
	sent   chan struct{}
}

// count is how many states were sent so far.
func (e *updateEvents) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.states)
}

// at is the state sent in the i-th place, from 0.
func (e *updateEvents) at(i int) app.UpdatesState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.states[i]
}

func (e *updateEvents) emit(name string, data any) {
	if name != app.UpdatesEvent {
		return
	}
	e.mu.Lock()
	e.states = append(e.states, data.(app.UpdatesState))
	e.mu.Unlock()
	e.sent <- struct{}{}
}

// done waits for a state with no run going on and at least n operations ended.
func (e *updateEvents) done(t *testing.T, n int) app.UpdatesState {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-e.sent:
			e.mu.Lock()
			last := e.states[len(e.states)-1]
			e.mu.Unlock()
			if len(last.Running) == 0 && last.Active == 0 && last.Completed >= n {
				return last
			}
		case <-timeout:
			t.Fatal("the updates did not end")
		}
	}
}

// updatesService stores host A with sites a1, a2 and broken, behind the fake updater.
func updatesService(t *testing.T, fail map[string]bool, gate chan struct{}) (*app.Updates, *updateEvents, map[string]string) {
	t.Helper()
	ctx := context.Background()
	db := repository(t)
	host := model.Host{ID: model.NewID(), Name: "A", Alias: "a"}
	if err := db.SaveHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"host": host.ID.String()}
	fake := sshxtest.NewFake()
	for i, name := range []string{"a1", "a2", "broken"} {
		site := model.Site{ID: model.NewID(), HostID: host.ID, Name: name, Path: "~/sites/" + name, Position: i}
		if err := db.SaveSite(ctx, site); err != nil {
			t.Fatal(err)
		}
		wpclitest.Replay(t, fake, "plain", site.Path, "wp")
		ids[name] = site.ID.String()
	}
	ws := workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now,
		Updates: func(onStep func(model.Operation, model.OperationStep)) workspace.Updates {
			return fakeUpdater{db: db, onStep: onStep, fail: fail, gate: gate}
		}})
	ev := &updateEvents{sent: make(chan struct{}, 200)}
	return app.NewUpdates(ws, ev.emit, nil), ev, ids
}

func TestPreviewAHost(t *testing.T) {
	u, _, ids := updatesService(t, nil, nil)
	previews, err := u.Preview(context.Background(), ids["host"])
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) != 3 || previews[0].Path != "A/a1" || len(previews[0].Plugins) != 1 || !previews[0].DumpTool {
		t.Fatalf("previews = %+v", previews)
	}
	if want := (app.UpdateItemView{Kind: "plugin", Slug: "akismet", From: "5.3", To: "5.7.2"}); previews[0].Plugins[0] != want {
		t.Errorf("item = %+v", previews[0].Plugins[0])
	}
	if previews[0].Backups != "~/.vigie/backups/"+ids["a1"]+"/" {
		t.Errorf("backups = %q", previews[0].Backups)
	}
	if code(previews[2].Problem) != "problem.cannotLoad" {
		t.Errorf("broken = %+v", previews[2].Problem)
	}
}

// A site's update runs in the background: the window sees its steps, then its end.
func TestUpdateASite(t *testing.T) {
	gate := make(chan struct{})
	u, ev, ids := updatesService(t, nil, gate)
	if p := u.Update(ids["a1"], app.SelectionView{Plugins: []string{"akismet"}}); code(p) != "problem.previewFirst" {
		t.Fatalf("Update() before a preview = %+v", p)
	}
	if _, err := u.Preview(context.Background(), ids["a1"]); err != nil {
		t.Fatal(err)
	}
	ended := make(chan app.RunEnd, 1)
	u.OnEnded = func(e app.RunEnd) { ended <- e }
	if p := u.Update(ids["a1"], app.SelectionView{Plugins: []string{"akismet"}}); p != nil {
		t.Fatal(p)
	}
	// Running, its first step under way.
	<-ev.sent
	for {
		state := u.State()
		if len(state.Running) == 1 && len(state.Running[0].Steps) == 1 {
			if state.Running[0].SiteID != ids["a1"] || state.Running[0].Steps[0].Outcome != "running" {
				t.Fatalf("running = %+v", state.Running)
			}
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(gate)
	state := ev.done(t, 1)
	if state.Completed != 1 || len(state.Bulks) != 0 {
		t.Errorf("state = %+v", state)
	}
	history, err := u.History(context.Background(), ids["a1"])
	if err != nil || len(history) != 1 || history[0].Outcome != "completed" || len(history[0].Steps) != 2 || history[0].Rollback != nil {
		t.Fatalf("History() = %+v, %v", history, err)
	}
	// Its end as the notifications read it (ADR-0035): the site, its item updated, its operation.
	end := <-ended
	if end.Kind != app.EndUpdate || end.By != model.RunByUser || end.Several || end.Name != "a1" || end.Items != 1 || end.Failed != 0 ||
		end.RunID.String() != history[0].ID || end.Cancelled || end.Finished.Before(end.Started) {
		t.Fatalf("end = %+v", end)
	}
}

// A site's update that failed ends stopped at its item, the items after it not started.
func TestTheEndOfAFailedUpdate(t *testing.T) {
	u, ev, ids := updatesService(t, map[string]bool{"a1": true}, nil)
	if _, err := u.Preview(context.Background(), ids["a1"]); err != nil {
		t.Fatal(err)
	}
	ended := make(chan app.RunEnd, 1)
	u.OnEnded = func(e app.RunEnd) { ended <- e }
	if p := u.Update(ids["a1"], app.SelectionView{Plugins: []string{"akismet"}}); p != nil {
		t.Fatal(p)
	}
	ev.done(t, 1)
	end := <-ended
	// Named by its title in the site's latest scan.
	if end.Failed != 1 || end.FailedKind != model.ItemPlugin || end.FailedItem != "Akismet Anti-spam: Spam Protection" || end.Items != 0 || end.NotStarted != 0 {
		t.Fatalf("end = %+v", end)
	}
}

// Updating a host keeps the result of each site for the summary; a failed one offers its rollback.
func TestUpdateAHostAndRollBack(t *testing.T) {
	u, ev, ids := updatesService(t, map[string]bool{"a1": true}, nil)
	if _, err := u.Preview(context.Background(), ids["host"]); err != nil {
		t.Fatal(err)
	}
	ended := make(chan app.RunEnd, 1)
	u.OnEnded = func(e app.RunEnd) { ended <- e }
	if p := u.UpdateAll(ids["host"]); p != nil {
		t.Fatal(p)
	}
	state := ev.done(t, 1)
	// The end of a run over several sites counts them; the host stopped at a1.
	if end := <-ended; !end.Several || end.Name != "A" || end.Updated != 0 || end.Failed != 1 || end.RunID == model.NoID {
		t.Fatalf("end = %+v", end)
	}
	if len(state.Bulks) != 1 || !slices.Equal(state.Bulks[0].NodeIDs, []string{ids["host"]}) {
		t.Fatalf("bulks = %+v, want the host's", state.Bulks)
	}
	outcomes := map[string]string{}
	for _, r := range state.Bulks[0].Sites {
		outcomes[r.Path] = r.Outcome
	}
	// The host stops at a1: the sites after it, broken included, are not started.
	if outcomes["A/a1"] != "failed" || outcomes["A/a2"] != "not-started" || outcomes["A/broken"] != "not-started" {
		t.Fatalf("bulk = %+v", state.Bulks[0].Sites)
	}
	history, err := u.History(context.Background(), ids["a1"])
	if err != nil || len(history) != 1 || history[0].Rollback == nil || history[0].Rollback.Slug != "akismet" {
		t.Fatalf("History() = %+v, %v", history, err)
	}
	if p := u.Rollback(ids["a1"], history[0].ID); p != nil {
		t.Fatal(p)
	}
	ev.done(t, 2)
	history, err = u.History(context.Background(), ids["a1"])
	if err != nil || len(history) != 2 || history[0].Kind != "rollback" || history[0].Outcome != "completed" {
		t.Fatalf("History() after the rollback = %+v, %v", history, err)
	}
}

// A site in a run is listed in the state until the run ends, and refused at once by another update.
func TestASiteInARunIsBusy(t *testing.T) {
	gate := make(chan struct{})
	u, ev, ids := updatesService(t, nil, gate)
	if _, err := u.Preview(context.Background(), ids["host"]); err != nil {
		t.Fatal(err)
	}
	if p := u.Update(ids["a1"], app.SelectionView{Plugins: []string{"akismet"}}); p != nil {
		t.Fatal(p)
	}
	if state := u.State(); len(state.Sites) != 1 || state.Sites[0] != ids["a1"] {
		t.Fatalf("sites = %v", state.Sites)
	}
	if p := u.Update(ids["a1"], app.SelectionView{Plugins: []string{"akismet"}}); code(p) != "problem.busy" {
		t.Errorf("second Update() = %+v", p)
	}
	if p := u.Rollback(ids["a1"], ids["a1"]); code(p) != "problem.busy" {
		t.Errorf("Rollback() during the update = %+v", p)
	}
	close(gate)
	if state := ev.done(t, 1); len(state.Sites) != 0 {
		t.Errorf("sites after the run = %v", state.Sites)
	}
}

// A site whose preview could not be read is refused before anything starts.
func TestUpdateOfAPreviewInError(t *testing.T) {
	u, _, ids := updatesService(t, nil, nil)
	if _, err := u.Preview(context.Background(), ids["broken"]); err != nil {
		t.Fatal(err)
	}
	if p := u.Update(ids["broken"], app.SelectionView{}); code(p) != "problem.cannotLoad" {
		t.Errorf("Update() of a preview in error = %+v", p)
	}
}

// Each update of several sites keeps its summary with the items it covered: updating a host shows
// no summary of another run while it goes, and updating the same items again drops their old one
// as it starts.
func TestEachBulkSummaryKeepsWhatItCovered(t *testing.T) {
	u, ev, ids := updatesService(t, nil, nil)
	if _, err := u.Preview(context.Background(), ids["host"]); err != nil {
		t.Fatal(err)
	}
	covers := func(state app.UpdatesState, nodes ...string) bool {
		return slices.ContainsFunc(state.Bulks, func(b app.BulkView) bool { return slices.Equal(b.NodeIDs, nodes) })
	}
	if p := u.UpdateMany([]string{ids["a2"]}); p != nil {
		t.Fatal(p)
	}
	state := ev.done(t, 1)
	if len(state.Bulks) != 1 || !covers(state, ids["a2"]) || len(state.Bulks[0].Sites) != 1 {
		t.Fatalf("after a2: %+v", state.Bulks)
	}

	// The state the start sends, before the run goes: nothing of the run can be in it yet.
	sent := ev.count()
	if p := u.UpdateAll(ids["host"]); p != nil {
		t.Fatal(p)
	}
	if started := ev.at(sent); covers(started, ids["host"]) || !covers(started, ids["a2"]) {
		t.Fatalf("while the host updates: %+v, want a2's summary alone", started.Bulks)
	}
	state = ev.done(t, state.Completed+1)
	if len(state.Bulks) != 2 || !covers(state, ids["a2"]) || !covers(state, ids["host"]) {
		t.Fatalf("after the host: %+v", state.Bulks)
	}

	sent = ev.count()
	if p := u.UpdateMany([]string{ids["a2"]}); p != nil {
		t.Fatal(p)
	}
	if started := ev.at(sent); covers(started, ids["a2"]) || !covers(started, ids["host"]) {
		t.Fatalf("while a2 updates again: %+v, want the host's summary alone", started.Bulks)
	}
	ev.done(t, state.Completed+1)
}
