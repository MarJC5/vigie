package main

import (
	"sync"
	"sync/atomic"
	"testing"
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
