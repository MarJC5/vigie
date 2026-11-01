package app_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

// events records what the scanner sends to the interface.
type events struct {
	mu     sync.Mutex
	states []app.ScansState
	sent   chan struct{}
}

func newEvents() *events { return &events{sent: make(chan struct{}, 100)} }

func (e *events) emit(name string, data any) {
	if name != app.ScansEvent {
		return
	}
	e.mu.Lock()
	e.states = append(e.states, data.(app.ScansState))
	e.mu.Unlock()
	e.sent <- struct{}{}
}

// idle waits until a state says nothing runs any more.
func (e *events) idle(t *testing.T) app.ScansState {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-e.sent:
			e.mu.Lock()
			last := e.states[len(e.states)-1]
			e.mu.Unlock()
			if len(last.Running) == 0 {
				return last
			}
		case <-timeout:
			t.Fatal("the scan did not end")
		}
	}
}

// benchSites stores Bench with the outdated and network sites of the container bench.
func benchSites(t *testing.T, runner sshx.Runner) (*workspace.Workspace, *store.DB, model.Host) {
	t.Helper()
	ctx := context.Background()
	db := repository(t)
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	sites := []model.Site{
		{ID: model.NewID(), HostID: host.ID, Name: "outdated.example.test", Path: "~/sites/outdated"},
		{ID: model.NewID(), HostID: host.ID, Name: "network.example.test", Path: "~/sites/network", Multisite: true, Position: 1},
	}
	if err := db.AddHostWithSites(ctx, host, sites); err != nil {
		t.Fatal(err)
	}
	if fake, ok := runner.(*sshxtest.Fake); ok {
		wpclitest.Replay(t, fake, "outdated", "~/sites/outdated", "wp")
		wpclitest.Replay(t, fake, "network", "~/sites/network", "wp")
	}
	return workspace.New(db, workspace.Deps{Runner: runner, Now: time.Now}), db, host
}

func TestScanAHostInTheBackground(t *testing.T) {
	ws, db, host := benchSites(t, sshxtest.NewFake())
	ev := newEvents()
	s := app.NewScanner(ws, ev.emit)
	if p := s.Scan(host.ID.String()); p != nil {
		t.Fatal(p)
	}
	final := ev.idle(t)
	if final.Completed != 2 {
		t.Fatalf("final state = %+v, want two scans completed", final)
	}
	ev.mu.Lock()
	first := ev.states[0]
	ev.mu.Unlock()
	if len(first.Running) != 1 || first.Running[0].NodeID != host.ID.String() || first.Running[0].Sites != 2 {
		t.Fatalf("first state = %+v, want the host running with two sites", first)
	}
	latest, err := db.LatestScans(context.Background())
	if err != nil || len(latest) != 2 {
		t.Fatalf("stored scans = %d, %v", len(latest), err)
	}
}

// The end of a scan is told with the latest scans before it, for the notifications.
func TestTheEndOfAScanIsToldWithWhatCameBefore(t *testing.T) {
	ws, _, host := benchSites(t, sshxtest.NewFake())
	ev := newEvents()
	s := app.NewScanner(ws, ev.emit)
	var before map[model.ID]model.Scan
	var after []model.Scan
	s.OnScanned = func(_ *model.Tree, b map[model.ID]model.Scan, a []model.Scan, _ map[model.ID][]model.Change) {
		before, after = b, a
	}
	if p := s.Scan(host.ID.String()); p != nil {
		t.Fatal(p)
	}
	ev.idle(t)
	if before == nil || len(before) != 0 || len(after) != 2 {
		t.Fatalf("told %d scans before and %d after, want none before and the two made", len(before), len(after))
	}
}

// The schedule's scans are runs the schedule started.
func TestTheScheduledScanIsTheSchedules(t *testing.T) {
	ws, db, _ := benchSites(t, sshxtest.NewFake())
	ev := newEvents()
	s := app.NewScanner(ws, ev.emit)
	if p := s.ScanScheduled(); p != nil {
		t.Fatal(p)
	}
	ev.idle(t)
	runs, err := db.Runs(context.Background(), time.Now().Add(-time.Hour))
	if err != nil || len(runs) != 1 || runs[0].By != model.RunBySchedule || runs[0].Path != "" {
		t.Fatalf("Runs() = %+v, %v; want one of everything, by the schedule", runs, err)
	}
}

func TestScanUnknownNode(t *testing.T) {
	ws, _, _ := benchSites(t, sshxtest.NewFake())
	s := app.NewScanner(ws, newEvents().emit)
	if p := s.Scan(model.NewID().String()); code(p) != "problem.error" {
		t.Fatalf("Scan(unknown) = %+v", p)
	}
}

// blocking holds every command until the scan is cancelled, and tells when the first arrives.
type blocking struct {
	started chan struct{}
	once    *sync.Once
}

func (b blocking) Run(ctx context.Context, _ sshx.Target, _ string) (sshx.Result, error) {
	b.once.Do(func() { close(b.started) })
	<-ctx.Done()
	return sshx.Result{}, ctx.Err()
}

func (b blocking) RunInput(ctx context.Context, target sshx.Target, remote string, _ []byte) (sshx.Result, error) {
	return b.Run(ctx, target, remote)
}

func TestCancelAScan(t *testing.T) {
	runner := blocking{started: make(chan struct{}), once: &sync.Once{}}
	ws, db, _ := benchSites(t, runner)
	ev := newEvents()
	s := app.NewScanner(ws, ev.emit)
	if p := s.Scan(""); p != nil { // everything
		t.Fatal(p)
	}
	if state := s.State(); len(state.Running) != 1 || state.Running[0].NodeID != "" {
		t.Fatalf("State() = %+v, want the whole tree running", state)
	}
	if p := s.Scan(""); p != nil {
		t.Fatalf("a second start of the same scan is ignored, got %+v", p)
	}
	<-runner.started // cancelled while the scans run, not before they start
	s.CancelAll()
	ev.idle(t)
	latest, err := db.LatestScans(context.Background())
	if err != nil || len(latest) != 2 {
		t.Fatalf("stored scans = %d, %v; want the cancelled scans kept", len(latest), err)
	}
	for _, sc := range latest {
		if sc.Outcome != model.ScanCancelled {
			t.Errorf("outcome = %q, want cancelled", sc.Outcome)
		}
	}
}

// "Check files now" compares the files again within the day; a plain scan carries them over.
func TestCheckFilesAgain(t *testing.T) {
	fake := sshxtest.NewFake()
	ws, _, host := benchSites(t, fake)
	ev := newEvents()
	s := app.NewScanner(ws, ev.emit)
	checksums := func() int {
		n := 0
		for _, c := range fake.Calls() {
			if strings.Contains(c, "'verify-checksums'") {
				n++
			}
		}
		return n
	}
	for _, step := range []struct {
		start func(string) *app.Problem
		want  int
	}{{s.Scan, 4}, {s.Scan, 4}, {s.CheckFiles, 8}} {
		if p := step.start(host.ID.String()); p != nil {
			t.Fatal(p)
		}
		ev.idle(t)
		if got := checksums(); got != step.want {
			t.Fatalf("%d checksum commands, want %d", got, step.want)
		}
	}
}

// A site being scanned is not updated nor rolled back, alone or with its host, until its scan ends.
func TestNoUpdateWhileScanning(t *testing.T) {
	runner := blocking{started: make(chan struct{}), once: &sync.Once{}}
	ws, _, host := benchSites(t, runner)
	ev := newEvents()
	s := app.NewScanner(ws, ev.emit)
	u := app.NewUpdates(ws, func(string, any) {}, s)
	if p := s.Scan(host.ID.String()); p != nil {
		t.Fatal(p)
	}
	<-runner.started
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	site := tree.SitesUnder(model.Node{Kind: model.KindHost, ID: host.ID})[0].ID.String()
	for name, p := range map[string]*app.Problem{
		"Update":    u.Update(site, app.SelectionView{Core: true}),
		"UpdateAll": u.UpdateAll(host.ID.String()),
		"Rollback":  u.Rollback(site, model.NewID().String()),
	} {
		if code(p) != "problem.scanning" {
			t.Errorf("%s during a scan = %+v, want problem.scanning", name, p)
		}
	}
	s.CancelAll()
	ev.idle(t)
	if p := u.Update(site, app.SelectionView{Core: true}); code(p) == "problem.scanning" {
		t.Errorf("Update after the scan = %+v, want it no longer refused for the scan", p)
	}
}

// After a run, the history is kept in check (ADR-0031): a scan of a site older than a year is gone,
// the scans of the run stay.
func TestARunPrunesTheHistory(t *testing.T) {
	ws, db, host := benchSites(t, sshxtest.NewFake())
	ctx := context.Background()
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	site := tree.SitesUnder(model.Node{Kind: model.KindHost, ID: host.ID})[0]
	old := model.Scan{ID: model.NewID(), SiteID: site.ID, Started: time.Now().Add(-400 * 24 * time.Hour), Finished: time.Now().Add(-400 * 24 * time.Hour),
		Outcome: model.ScanFailed, Failure: "refused", Recorded: true, RecordedStatus: model.StatusUnknown, RecordedChanges: []model.Change{}}
	if err := db.SaveScan(ctx, old); err != nil {
		t.Fatal(err)
	}
	ev := newEvents()
	if p := app.NewScanner(ws, ev.emit).Scan(host.ID.String()); p != nil {
		t.Fatal(p)
	}
	ev.idle(t)
	scans, err := db.Scans(ctx, site.ID)
	if err != nil || len(scans) != 1 || scans[0].ID == old.ID {
		t.Fatalf("scans of the site = %d, %v; want the run's only", len(scans), err)
	}
}
