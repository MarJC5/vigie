// Package schedule starts the automatic scans of everything: the next one is due an interval after
// the latest complete scan of everything, or after the latest one the schedule started, whichever
// is later. It goes by the wall clock, checked at each tick: a computer asleep across several
// intervals runs one scan when it wakes, and a scan never starts while another runs.
package schedule

import (
	"context"
	"sync"
	"time"
)

// Tick is how often Run checks whether a scan is due.
const Tick = time.Minute

// Schedule decides when to scan everything; its functions are the rest of Vigie.
type Schedule struct {
	Now func() time.Time
	// Every is the interval chosen by the user; 0 means never.
	Every func(ctx context.Context) (time.Duration, error)
	// Last is when the latest complete scan of everything ended, if there was one.
	Last func(ctx context.Context) (time.Time, bool, error)
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

// Next returns when the next scan is due; false when automatic scans are off.
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
	if !ok {
		return s.Now(), true, nil // never scanned: at once
	}
	return last.Add(every), true, nil
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
