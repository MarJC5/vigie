package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// Asked many times while it runs, the function runs once more after, not once per ask.
func TestCoalescerRunsOnceMoreAfterTheAsksWhileRunning(t *testing.T) {
	var c coalescer
	var runs atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	var done sync.WaitGroup
	done.Add(2)
	c.run(func() {
		runs.Add(1)
		close(started)
		<-release
		done.Done()
	})
	<-started
	for range 50 {
		c.run(func() { runs.Add(1); done.Done() })
	}
	close(release)
	done.Wait()
	if got := runs.Load(); got != 2 {
		t.Errorf("ran %d times, want 2: the first, then once for the 50 asks", got)
	}
}

// A run's end is notified when it was long, the user's, not stopped, and the window not in front
// (ADR-0035).
func TestTheEndIsNotifiedWhenTheWindowIsNotInFront(t *testing.T) {
	start := time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC)
	long := app.RunEnd{Kind: app.EndScan, By: model.RunByUser, Started: start, Finished: start.Add(time.Minute)}
	short := long
	short.Finished = start.Add(10 * time.Second)
	cases := []struct {
		name    string
		end     app.RunEnd
		inFront bool
		want    bool
	}{
		{"long, the window elsewhere", long, false, true},
		{"long, the window in front", long, true, false},
		{"short, the window elsewhere", short, false, false},
	}
	for _, c := range cases {
		if got := endFirst(c.end, c.inFront); got != c.want {
			t.Errorf("%s: endFirst = %v, want %v", c.name, got, c.want)
		}
	}
}
