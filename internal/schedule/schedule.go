// Package schedule starts the automatic scans of everything at fixed times of the day (ADR-0040):
// every interval from 06:05 local time. A time of the grid is skipped when a complete scan of
// everything ended less than half an interval before it, or after it, and when it came before
// automatic scans were turned on. It goes by the wall clock, checked at each tick: a computer
// asleep across several times runs one scan when it wakes, and a scan never starts while another
// runs.
package schedule

import (
	"context"
	"sync"
	"time"
)

// Tick is how often Run checks whether a scan is due.
const Tick = time.Minute

// The anchor of the grid: five past six, local time (ADR-0040).
const (
	anchorHour   = 6
	anchorMinute = 5
)

// Schedule decides when to scan everything; its functions are the rest of Vigie.
type Schedule struct {
	Now func() time.Time
	// Every is the interval chosen by the user, a number of hours that divides a day; 0 means
	// never.
	Every func(ctx context.Context) (time.Duration, error)
	// Last is when the latest complete scan of everything ended, if there was one.
	Last func(ctx context.Context) (time.Time, bool, error)
	// Since is when automatic scans were turned on, if known: the times before it are not missed.
	// Nil: every time counts.
	Since func(ctx context.Context) (time.Time, bool, error)
	// Busy tells whether a scan runs now, started by the user or by the schedule.
	Busy func() bool
	// Start starts a scan of everything and returns at once.
	Start func()
	// Ticks drives Run; nil is a ticker of Tick.
	Ticks <-chan time.Time
	// Failed, when set, hears the errors of Run's checks (reading the settings), each retried at
	// the next tick.
	Failed func(error)

	mu      sync.Mutex
	started time.Time // the latest scan the schedule started
}

// Next returns when the next scan is due: a time of the grid, at once when everything was never
// scanned, or a time already past when one was missed; false when automatic scans are off.
func (s *Schedule) Next(ctx context.Context) (time.Time, bool, error) {
	every, err := s.Every(ctx)
	if err != nil || every <= 0 {
		return time.Time{}, false, err
	}
	last, ok, err := s.Last(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if started.After(last) {
		last, ok = started, true
	}
	now := s.Now()
	if !ok {
		return now, true, nil // never scanned: at once
	}
	var since time.Time
	if s.Since != nil {
		if since, _, err = s.Since(ctx); err != nil {
			return time.Time{}, false, err
		}
	}
	// A time counts unless a scan ended less than half an interval before it, or after it.
	covered := func(t time.Time) bool { return !last.Before(t.Add(-every / 2)) }
	prev, next := Around(now, every)
	if !covered(prev) && !prev.Before(since) {
		return prev, true, nil // missed: as soon as possible
	}
	for covered(next) {
		_, next = Around(next, every)
	}
	return next, true, nil
}

// Check starts a scan of everything when one is due and none runs.
func (s *Schedule) Check(ctx context.Context) error {
	next, ok, err := s.Next(ctx)
	if err != nil || !ok || s.Busy() {
		return err
	}
	now := s.Now()
	if now.Before(next) {
		return nil
	}
	s.mu.Lock()
	s.started = now
	s.mu.Unlock()
	s.Start()
	return nil
}

// Run checks at once, then at each tick, until ctx ends.
func (s *Schedule) Run(ctx context.Context) {
	ticks := s.Ticks
	if ticks == nil {
		ticker := time.NewTicker(Tick)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		if err := s.Check(ctx); err != nil && s.Failed != nil {
			s.Failed(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
	}
}

// Around returns the time of the grid at or before t, and the one after it, in t's location: every
// interval from 06:05, the same times every day. A time the clock skips (the day it goes forward)
// falls on the next minute that exists; one it repeats counts once.
func Around(t time.Time, every time.Duration) (prev, next time.Time) {
	hours := int(every / time.Hour)
	if hours <= 0 || 24%hours != 0 {
		hours = 24
	}
	var times []time.Time
	for day := -1; day <= 1; day++ {
		d := t.AddDate(0, 0, day)
		for h := anchorHour % hours; h < 24; h += hours {
			at := time.Date(d.Year(), d.Month(), d.Day(), h, anchorMinute, 0, 0, t.Location())
			if len(times) == 0 || !at.Equal(times[len(times)-1]) {
				times = append(times, at)
			}
		}
	}
	for i, at := range times {
		if at.After(t) {
			return times[i-1], at
		}
	}
	return times[len(times)-1], times[len(times)-1].Add(every)
}
