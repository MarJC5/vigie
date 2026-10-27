// Package sshxtest provides a fake sshx.Runner that replays recorded outputs, for tests that must
// never reach a server.
package sshxtest

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/MarJC5/vigie/internal/sshx"
)

// ErrUnexpected is returned for a remote command that has no recorded result.
var ErrUnexpected = errors.New("unexpected remote command")

// Fake answers each remote command with its recorded result and keeps the calls in order.
type Fake struct {
	mu      sync.Mutex
	results map[string]sshx.Result
	calls   []string
}

var _ sshx.Runner = (*Fake)(nil)

// NewFake returns a fake without recorded results.
func NewFake() *Fake {
	return &Fake{results: map[string]sshx.Result{}}
}

// Set records the result of a remote command.
func (f *Fake) Set(remote string, res sshx.Result) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[remote] = res
}

// Run returns the recorded result of remote.
func (f *Fake) Run(_ context.Context, _ sshx.Target, remote string) (sshx.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, remote)
	res, ok := f.results[remote]
	if !ok {
		return sshx.Result{}, fmt.Errorf("%w: %s", ErrUnexpected, remote)
	}
	return res, nil
}

// Calls returns the remote commands received, in order.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}
