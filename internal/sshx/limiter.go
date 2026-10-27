package sshx

import (
	"context"
	"sync"
)

// Limiter caps the commands running at the same time on each host, to stay gentle with shared
// hosting accounts. Different hosts run in parallel.
type Limiter struct {
	mu    sync.Mutex
	slots map[string]chan struct{}
}

// NewLimiter returns an empty limiter.
func NewLimiter() *Limiter {
	return &Limiter{slots: map[string]chan struct{}{}}
}

// Do runs fn once fewer than max commands run for key (the host ID). The first call for a key
// sets its limit.
func (l *Limiter) Do(ctx context.Context, key string, max int, fn func() error) error {
	if max < 1 {
		max = 1
	}
	l.mu.Lock()
	slot, ok := l.slots[key]
	if !ok {
		slot = make(chan struct{}, max)
		l.slots[key] = slot
	}
	l.mu.Unlock()

	select {
	case slot <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-slot }()
	return fn()
}
