// Package sshxtest provides a fake sshx.Runner that replays recorded outputs, for tests that must
// never reach a server.
package sshxtest

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	inputs  map[string][]byte // the standard input sent with a remote command, the last one
}

var (
	_ sshx.Runner       = (*Fake)(nil)
	_ sshx.StreamRunner = (*Fake)(nil)
	_ sshx.StreamRunner = sshx.ExecRunner{}
)

// NewFake returns a fake without recorded results.
func NewFake() *Fake {
	return &Fake{results: map[string]sshx.Result{}, inputs: map[string][]byte{}}
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

// RunInput keeps input, then returns the recorded result of remote.
func (f *Fake) RunInput(ctx context.Context, target sshx.Target, remote string, input []byte) (sshx.Result, error) {
	f.mu.Lock()
	f.inputs[remote] = append([]byte(nil), input...)
	f.mu.Unlock()
	return f.Run(ctx, target, remote)
}

// RunStream keeps what it reads on input, writes the recorded output of remote to out, and
// returns its result without it, as the real stream does.
func (f *Fake) RunStream(ctx context.Context, target sshx.Target, remote string, input io.Reader, out io.Writer) (sshx.Result, error) {
	var in []byte
	if input != nil {
		var err error
		if in, err = io.ReadAll(input); err != nil {
			return sshx.Result{}, err
		}
	}
	res, err := f.RunInput(ctx, target, remote, in)
	if err != nil {
		return res, err
	}
	if _, err := out.Write(res.Stdout); err != nil {
		return sshx.Result{}, err
	}
	res.Stdout = nil
	return res, nil
}

// Input returns the standard input last sent with remote; nil when none was.
func (f *Fake) Input(remote string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inputs[remote]
}

// Calls returns the remote commands received, in order.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}
