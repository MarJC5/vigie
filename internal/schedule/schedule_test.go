package schedule_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/schedule"
)

// clock is a fake: the tests move it by hand.
type clock struct{ now time.Time }

// bench is a schedule over fakes: the interval, the latest complete scan of everything, when
// automatic scans were turned on, whether a scan runs, and the scans started.
type bench struct {
	clock   clock
	every   time.Duration
	last    time.Time // zero: none yet
	since   time.Time // zero: not known
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
		Since: func(context.Context) (time.Time, bool, error) {
			return b.since, !b.since.IsZero(), nil
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

func (b *bench) next(t *testing.T) time.Time {
	t.Helper()
	next, ok, err := b.s.Next(context.Background())
	if err != nil || !ok {
		t.Fatalf("Next() = %v, %v, %v", next, ok, err)
	}
	return next
}

var zurich = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		panic(err)
	}
	return loc
}()

// at is a local time in Zurich, on 3 November 2026 unless the day is given.
func at(hour, minute int, day ...int) time.Time {
	d := 3
	if len(day) > 0 {
		d = day[0]
	}
	return time.Date(2026, 11, d, hour, minute, 0, 0, zurich)
}

func clockOf(times []time.Time) []string {
	out := make([]string, len(times))
	for i, t := range times {
		out[i] = t.Format("15:04")
	}
	return out
}

// The times of a day for each interval: every interval from 06:05, the same every day.
func TestTheGridOfEachInterval(t *testing.T) {
	cases := map[time.Duration][]string{
		time.Hour:      {"00:05", "01:05", "02:05", "03:05", "04:05", "05:05", "06:05", "07:05", "08:05", "09:05", "10:05", "11:05", "12:05", "13:05", "14:05", "15:05", "16:05", "17:05", "18:05", "19:05", "20:05", "21:05", "22:05", "23:05"},
		3 * time.Hour:  {"00:05", "03:05", "06:05", "09:05", "12:05", "15:05", "18:05", "21:05"},
		6 * time.Hour:  {"00:05", "06:05", "12:05", "18:05"},
		12 * time.Hour: {"06:05", "18:05"},
		24 * time.Hour: {"06:05"},
	}
	for every, want := range cases {
		var got []time.Time
		for _, next := at(0, 0), at(0, 0); next.Before(at(0, 0, 4)); {
			_, next = schedule.Around(next, every)
			if next.Before(at(0, 0, 4)) {
				got = append(got, next)
			}
		}
		if !slices.Equal(clockOf(got), want) {
			t.Errorf("every %v: %v, want %v", every, clockOf(got), want)
		}
	}
	// At a time of the grid, that time is the one before.
	if prev, next := schedule.Around(at(12, 5), 6*time.Hour); !prev.Equal(at(12, 5)) || !next.Equal(at(18, 5)) {
		t.Errorf("Around(12:05) = %v, %v", prev, next)
	}
}

func TestOffMeansNever(t *testing.T) {
	b := newBench(0, time.Time{}, at(12, 0))
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

// The next scan is at the next time of the grid, and starts then, not before.
func TestDueAtTheNextTimeOfTheGrid(t *testing.T) {
	b := newBench(6*time.Hour, at(12, 20), at(13, 0))
	if next := b.next(t); !next.Equal(at(18, 5)) {
		t.Fatalf("Next() = %v, want 18:05", next)
	}
	b.clock.now = at(18, 4)
	b.check(t)
	if b.started != 0 {
		t.Fatal("started before its time")
	}
	b.clock.now = at(18, 5)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans at its time, want 1", b.started)
	}
	b.clock.now = at(18, 30)
	b.end(true)
	if next := b.next(t); !next.Equal(at(0, 5, 4)) {
		t.Fatalf("Next() after it = %v, want 00:05 the next day", next)
	}
}

// A scan of everything by hand less than half an interval before a time stands for it; one made
// earlier does not.
func TestARecentScanStandsForTheNextTime(t *testing.T) {
	b := newBench(6*time.Hour, at(16, 0), at(16, 30))
	if next := b.next(t); !next.Equal(at(0, 5, 4)) {
		t.Fatalf("Next() after a scan at 16:00 = %v, want 00:05: it stands for 18:05", next)
	}
	b = newBench(6*time.Hour, at(14, 0), at(14, 30))
	if next := b.next(t); !next.Equal(at(18, 5)) {
		t.Fatalf("Next() after a scan at 14:00 = %v, want 18:05", next)
	}
	// Every day, a scan by hand after 18:05 stands for the next morning's.
	b = newBench(24*time.Hour, at(19, 0), at(20, 0))
	if next := b.next(t); !next.Equal(at(6, 5, 5)) {
		t.Fatalf("Next() every day after 19:00 = %v, want 06:05 the day after next", next)
	}
	b = newBench(24*time.Hour, at(17, 0), at(20, 0))
	if next := b.next(t); !next.Equal(at(6, 5, 4)) {
		t.Fatalf("Next() every day after 17:00 = %v, want 06:05 (the next time)", next)
	}
}

// Without a complete scan yet, the first one is due at once.
func TestTheFirstScanIsDueAtOnce(t *testing.T) {
	b := newBench(time.Hour, time.Time{}, at(12, 0))
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans, want the first at once", b.started)
	}
}

// A laptop asleep across several times runs one scan when it wakes, then follows the grid.
func TestASleepAcrossTimesGivesOneScan(t *testing.T) {
	b := newBench(time.Hour, at(12, 10), at(12, 10))
	b.clock.now = at(19, 30) // the first check after waking
	b.check(t)
	b.clock.now = at(19, 31)
	b.check(t)
	b.clock.now = at(19, 34)
	b.end(true)
	b.clock.now = at(19, 41)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans after a sleep across seven times, want 1", b.started)
	}
	if next := b.next(t); !next.Equal(at(20, 5)) {
		t.Fatalf("Next() = %v, want 20:05", next)
	}
}

// Turned on at 10:00, automatic scans do not catch up the time of 06:05: the first comes at 12:05.
func TestNoCatchUpBeforeTheyWereTurnedOn(t *testing.T) {
	b := newBench(6*time.Hour, at(18, 0, 1), at(10, 0))
	b.since = at(10, 0)
	b.check(t)
	if b.started != 0 {
		t.Fatal("caught up a time before automatic scans were turned on")
	}
	if next := b.next(t); !next.Equal(at(12, 5)) {
		t.Fatalf("Next() = %v, want 12:05", next)
	}
	// Turned on before it, the time of 06:05 was missed: one scan at once.
	b.since = at(5, 0)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans, want the missed 06:05 at once", b.started)
	}
}

// A scan due while another runs waits for it to end.
func TestAScanWaitsForTheOneRunning(t *testing.T) {
	b := newBench(time.Hour, at(11, 10), at(12, 5))
	b.busy = true // a scan the user started
	b.check(t)
	if b.started != 0 {
		t.Fatal("started over a running scan")
	}
	b.busy = false
	b.clock.now = at(12, 6)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans once the other ended, want 1", b.started)
	}
}

// A scan the schedule started and that did not complete (cancelled, failed) is tried again at the
// next time, not at the next check.
func TestAnIncompleteScanIsTriedAgainAtTheNextTime(t *testing.T) {
	b := newBench(3*time.Hour, at(9, 10), at(12, 5))
	b.check(t)
	b.clock.now = at(12, 10)
	b.end(false)
	b.clock.now = at(12, 11)
	b.check(t)
	if b.started != 1 {
		t.Fatalf("started %d scans after a cancelled one, want it tried again later", b.started)
	}
	if next := b.next(t); !next.Equal(at(15, 5)) {
		t.Fatalf("Next() = %v, want 15:05", next)
	}
}

// The days the clock changes: a time that does not exist falls on the next minute that does; a
// time that comes twice counts once.
func TestTheDaysTheClockChanges(t *testing.T) {
	forward := time.Date(2026, 3, 29, 1, 30, 0, 0, zurich)
	if _, next := schedule.Around(forward, time.Hour); next.Format("15:04 MST") != "03:05 CEST" {
		t.Errorf("after 01:30 the day the clock goes forward: %v, want 03:05 CEST", next)
	}
	_, after := schedule.Around(time.Date(2026, 3, 29, 3, 5, 0, 0, zurich), time.Hour)
	if after.Format("15:04") != "04:05" {
		t.Errorf("after 03:05 that day: %v, want 04:05", after)
	}
	back := time.Date(2026, 10, 25, 1, 30, 0, 0, zurich)
	var times []string
	for next := back; len(times) < 3; {
		_, next = schedule.Around(next, time.Hour)
		times = append(times, next.Format("15:04 MST"))
	}
	if times[0] == times[1] {
		t.Errorf("the day the clock goes back, a time counted twice: %v", times)
	}
}

// Run checks at once, then at each tick, until its context ends.
func TestRunChecksUntilStopped(t *testing.T) {
	b := newBench(time.Hour, time.Time{}, at(12, 0))
	ticks := make(chan time.Time)
	b.s.Ticks = ticks
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.s.Run(ctx); close(done) }()
	ticks <- at(12, 0) // the first tick only returns once the first check is done
	cancel()
	<-done
	if b.started != 1 {
		t.Fatalf("Run started %d scans, want 1", b.started)
	}
}
