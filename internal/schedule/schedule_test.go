package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/schedule"
)

// clock is a fake: the tests move it by hand.
type clock struct{ now time.Time }

// bench is a schedule over fakes: the interval, the latest complete scan of everything, whether a
// scan runs, and the scans started.
type bench struct {
	clock   clock
	every   time.Duration
	last    time.Time // zero: none yet
	busy    bool
	started int
	s       *schedule.Schedule
}

func newBench(every time.Duration, last time.Time, now time.Time) *bench {
	b := &bench{clock: clock{now}, every: every, last: last}
	b.s = &schedule.Schedule{
		Now:   func() time.Time { return b.clock.now },
		Every: func(context.Context) (time.Duration, error) { return b.every, nil },
		Last: func(context.Context) (time.Time, bool, error) {
			return b.last, !b.last.IsZero(), nil
		},
		Busy:  func() bool { return b.busy },
		Start: func() { b.started++; b.busy = true },
	}
	return b
}

// end ends the running scan; complete records it as a scan of everything.
func (b *bench) end(complete bool) {
	b.busy = false
	if complete {
		b.last = b.clock.now
	}
}

func (b *bench) check(t *testing.T) {
	t.Helper()
	if err := b.s.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
}

var noon = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

func TestOffMeansNever(t *testing.T) {
	b := newBench(0, time.Time{}, noon)
	for range 3 {
		b.check(t)
		b.clock.now = b.clock.now.Add(48 * time.Hour)
	}
	if b.started != 0 {
		t.Fatalf("started %d scans while off", b.started)
	}
	if _, ok, _ := b.s.Next(context.Background()); ok {
		t.Error("Next() gives a time while off")
	}
}

// The next scan is due an interval after the latest complete scan of everything.
func TestDueAnIntervalAfterTheLatestCompleteScan(t *testing.T) {
	b := newBench(6*time.Hour, noon, noon.Add(time.Hour))
	if next, ok, _ := b.s.Next(context.Background()); !ok || !next.Equal(noon.Add(6*time.Hour)) {
		t.Fatalf("Next() = %v, %v; want 18:00", next, ok)
	}
	b.clock.now = noon.Add(6*time.Hour - time.Minute)
	b.check(t)
	if b.started != 0 {
		t.Fatal("started before its time")
	}
	b.clock.now = noon.Add(6 * time.Hour)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans at its time, want 1", b.started)
	}
}

// Without a complete scan yet, the first one is due at once.
func TestTheFirstScanIsDueAtOnce(t *testing.T) {
	b := newBench(time.Hour, time.Time{}, noon)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans, want the first at once", b.started)
	}
}

// A laptop asleep across several intervals runs one scan when it wakes.
func TestASleepAcrossIntervalsGivesOneScan(t *testing.T) {
	b := newBench(time.Hour, noon, noon)
	b.clock.now = noon.Add(7 * time.Hour) // the first check after waking
	b.check(t)
	b.clock.now = b.clock.now.Add(time.Minute)
	b.check(t)
	b.clock.now = b.clock.now.Add(9 * time.Minute)
	b.end(true)
	b.clock.now = b.clock.now.Add(time.Minute)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans after a sleep of seven intervals, want 1", b.started)
	}
}

// A scan due while another runs waits for it to end.
func TestAScanWaitsForTheOneRunning(t *testing.T) {
	b := newBench(time.Hour, noon, noon.Add(time.Hour))
	b.busy = true // a scan the user started
	b.check(t)
	if b.started != 0 {
		t.Fatal("started over a running scan")
	}
	b.busy = false
	b.clock.now = b.clock.now.Add(time.Minute)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans once the other ended, want 1", b.started)
	}
}

// A scan the schedule started and that did not complete (cancelled, failed) is tried again an
// interval later, not at the next check.
func TestAnIncompleteScanIsTriedAgainAnIntervalLater(t *testing.T) {
	b := newBench(3*time.Hour, noon, noon.Add(3*time.Hour))
	b.check(t)
	b.clock.now = b.clock.now.Add(5 * time.Minute)
	b.end(false)
	b.clock.now = b.clock.now.Add(time.Minute)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans after a cancelled one, want it tried again later", b.started)
	}
	if next, ok, _ := b.s.Next(context.Background()); !ok || !next.Equal(noon.Add(6*time.Hour)) {
		t.Fatalf("Next() = %v, %v; want an interval after the cancelled scan started", next, ok)
	}
}

// Run checks at once, then at each tick, until its context ends.
func TestRunChecksUntilStopped(t *testing.T) {
	b := newBench(time.Hour, time.Time{}, noon)
	ticks := make(chan time.Time)
	b.s.Ticks = ticks
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.s.Run(ctx); close(done) }()
	ticks <- noon // the first tick only returns once the first check is done
	cancel()
	<-done
	if b.started != 1 {
		t.Fatalf("Run started %d scans, want 1", b.started)
	}
}
