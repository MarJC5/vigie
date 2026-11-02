package sshx

import (
	"context"
	"sync"
	"time"
)

// Openings is how many connections a Limiter lets open at once, whatever their hosts: an SSH
// daemon drops connections beyond 10 still opening (MaxStartups), and several hosts can be one
// server (ADR-0032).
const Openings = 8

// Limiter caps the commands running at the same time on each host, to stay gentle with shared
// hosting accounts, and the connections opening at the same time. Different hosts run in parallel.
type Limiter struct {
	mu       sync.Mutex
	slots    map[string]chan struct{}
	hosts    map[string]*connection
	openings chan struct{}
	persist  time.Duration // how long a shared connection stays open after its last command
}

// connection is what the limiter knows of a host's shared connection.
type connection struct {
	running int       // commands running on it
	open    bool      // the last command that ended succeeded
	ended   time.Time // when it ended
}

// NewLimiter returns an empty limiter, which takes a connection as closed once its commands end;
// see KeepOpen.
func NewLimiter() *Limiter {
	return &Limiter{slots: map[string]chan struct{}{}, hosts: map[string]*connection{}, openings: make(chan struct{}, Openings)}
}

// KeepOpen tells how long a shared connection stays open after its last command (ControlPersist):
// until then, a command on it opens nothing.
func (l *Limiter) KeepOpen(persist time.Duration) *Limiter {
	l.persist = persist
	return l
}

// Do runs fn once fewer than max commands run for key (the host ID) and, unless the host's
// connection is open, once fewer than Openings connections are opening. The first call for a key
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
		l.hosts[key] = &connection{}
	}
	host := l.hosts[key]
	l.mu.Unlock()

	select {
	case slot <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-slot }()

	l.mu.Lock()
	// Open while a command runs on it, or until a while before ControlPersist ends: a command
	// could reach it as it closes.
	opening := !host.open || host.running == 0 && time.Since(host.ended) > l.persist*9/10
	host.running++
	l.mu.Unlock()
	if opening {
		select {
		case l.openings <- struct{}{}:
		case <-ctx.Done():
			l.end(host, false)
			return ctx.Err()
		}
		defer func() { <-l.openings }()
	}
	err := fn()
	l.end(host, err == nil)
	return err
}

// end notes the end of a command on its host's connection.
func (l *Limiter) end(host *connection, succeeded bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	host.running--
	host.open, host.ended = succeeded, time.Now()
}
