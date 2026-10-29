package app_test

import (
	"context"
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
