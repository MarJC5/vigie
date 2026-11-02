package sshx_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
)

// fakeSSH is a stand-in for the ssh binary (testdata/fake-ssh): it prints its arguments, one per
// line, and behaves according to the remote command.
func fakeSSH(t *testing.T) string {
	return fakeTool(t, "fake-ssh")
}

// fakeTool returns the absolute path of a stand-in program in testdata.
func fakeTool(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func runner(t *testing.T, timeout time.Duration) sshx.ExecRunner {
	return sshx.ExecRunner{SSH: fakeSSH(t), Options: opts, Timeout: timeout}
}

func TestExecRunnerPassesArgumentsAndOutput(t *testing.T) {
	target := sshx.TargetFor(model.Host{Alias: "infomaniak"})
	res, err := runner(t, 5*time.Second).Run(context.Background(), target, "'wp' 'core' 'version'")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join(sshx.Args(target, opts, "'wp' 'core' 'version'"), "\n") + "\n"
	if string(res.Stdout) != want || res.ExitCode != 0 {
		t.Fatalf("Run() stdout =\n%s\nexit %d; want\n%s", res.Stdout, res.ExitCode, want)
	}
}

// A fixed text goes on the command's standard input; without one, the command reads nothing.
func TestExecRunnerSendsStandardInput(t *testing.T) {
	r := runner(t, 5*time.Second)
	res, err := r.RunInput(context.Background(), sshx.Target{Alias: "a"}, "input", []byte("<?php echo 1;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, got, _ := strings.Cut(string(res.Stdout), "input:\n"); got != "<?php echo 1;\n" {
		t.Fatalf("RunInput() stdout = %q", res.Stdout)
	}
	res, err = r.Run(context.Background(), sshx.Target{Alias: "a"}, "input")
	if _, got, _ := strings.Cut(string(res.Stdout), "input:\n"); err != nil || got != "" {
		t.Fatalf("Run() read %q, %v; want nothing", got, err)
	}
}

func TestExecRunnerReportsRemoteExitCode(t *testing.T) {
	res, err := runner(t, 5*time.Second).Run(context.Background(), sshx.Target{Alias: "a"}, "exit-3")
	if err != nil {
		t.Fatalf("a failing remote command is a result, not an error: %v", err)
	}
	if res.ExitCode != 3 || !strings.Contains(string(res.Stderr), "remote failed") {
		t.Fatalf("Run() = exit %d, stderr %q", res.ExitCode, res.Stderr)
	}
}

func TestExecRunnerConnectionError(t *testing.T) {
	_, err := runner(t, 5*time.Second).Run(context.Background(), sshx.Target{Alias: "a"}, "exit-255")
	if !errors.Is(err, sshx.ErrConnection) {
		t.Fatalf("ssh exit status 255: %v, want ErrConnection", err)
	}
	var connErr *sshx.ConnectionError
	if !errors.As(err, &connErr) || connErr.Failure != sshx.FailureRefused {
		t.Fatalf("ssh exit status 255: %#v, want a ConnectionError classified as refused", err)
	}
}

func TestExecRunnerTimeout(t *testing.T) {
	start := time.Now()
	_, err := runner(t, 300*time.Millisecond).Run(context.Background(), sshx.Target{Alias: "a"}, "sleep")
	if !errors.Is(err, sshx.ErrTimeout) {
		t.Fatalf("slow command: %v, want ErrTimeout", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout not enforced: took %s", time.Since(start))
	}
}

func TestFakeRunnerKeepsTheInputSent(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set("'wp' 'eval-file' '-'", sshx.Result{Stdout: []byte("{}")})
	res, err := fake.RunInput(context.Background(), sshx.Target{}, "'wp' 'eval-file' '-'", []byte("<?php echo 1;"))
	if err != nil || string(res.Stdout) != "{}" || string(fake.Input("'wp' 'eval-file' '-'")) != "<?php echo 1;" {
		t.Fatalf("RunInput() = %q, %v; input kept %q", res.Stdout, err, fake.Input("'wp' 'eval-file' '-'"))
	}
}

func TestFakeRunnerReplaysRecordedOutputs(t *testing.T) {
	fake := sshxtest.NewFake()
	fake.Set("'wp' 'core' 'version'", sshx.Result{Stdout: []byte("7.1.3\n")})
	res, err := fake.Run(context.Background(), sshx.Target{Alias: "a"}, "'wp' 'core' 'version'")
	if err != nil || string(res.Stdout) != "7.1.3\n" {
		t.Fatalf("Fake.Run() = %q, %v", res.Stdout, err)
	}
	if _, err := fake.Run(context.Background(), sshx.Target{Alias: "a"}, "'unknown'"); !errors.Is(err, sshxtest.ErrUnexpected) {
		t.Fatalf("unknown command: %v, want ErrUnexpected", err)
	}
	if calls := fake.Calls(); len(calls) != 2 || calls[0] != "'wp' 'core' 'version'" {
		t.Fatalf("Calls() = %q", calls)
	}
}

func TestLimiterCapsCommandsPerHost(t *testing.T) {
	limiter := sshx.NewLimiter()
	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := limiter.Do(context.Background(), "host-a", 2, func() error {
				n := running.Add(1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				running.Add(-1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if peak.Load() != 2 {
		t.Fatalf("peak concurrency on one host = %d, want 2", peak.Load())
	}
}

func TestLimiterHostsAreIndependent(t *testing.T) {
	limiter := sshx.NewLimiter()
	release := make(chan struct{})
	go limiter.Do(context.Background(), "host-a", 1, func() error { <-release; return nil })
	time.Sleep(20 * time.Millisecond) // host-a is now busy
	done := make(chan error, 1)
	go func() { done <- limiter.Do(context.Background(), "host-b", 1, func() error { return nil }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("host-b waited for host-a")
	}
	close(release)
}

func TestLimiterRespectsCancellation(t *testing.T) {
	limiter := sshx.NewLimiter()
	release := make(chan struct{})
	defer close(release)
	go limiter.Do(context.Background(), "h", 1, func() error { <-release; return nil })
	time.Sleep(20 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := limiter.Do(ctx, "h", 1, func() error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting with a cancelled context: %v", err)
	}
}

// peakOf runs fn for each key at once, each command 30 ms, and returns the most that ran together.
func peakOf(t *testing.T, limiter *sshx.Limiter, keys []string) int32 {
	t.Helper()
	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Go(func() {
			err := limiter.Do(context.Background(), key, 2, func() error {
				n := running.Add(1)
				for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
				}
				time.Sleep(30 * time.Millisecond)
				running.Add(-1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	return peak.Load()
}

// An SSH daemon drops connections beyond 10 still opening, and several accounts can be on one
// server: at most sshx.Openings connections open at once, whatever the hosts (ADR-0032).
func TestLimiterOpensFewConnectionsAtOnce(t *testing.T) {
	keys := make([]string, 20)
	for i := range keys {
		keys[i] = fmt.Sprintf("host-%d", i)
	}
	if peak := peakOf(t, sshx.NewLimiter().KeepOpen(time.Minute), keys); peak != sshx.Openings {
		t.Fatalf("connections opening at once = %d, want %d", peak, sshx.Openings)
	}
}

// A host whose connection is open runs its commands without waiting for an opening; once the
// connection has closed for lack of use, the next command opens it again.
func TestLimiterOpenConnectionsWaitForNoOpening(t *testing.T) {
	limiter := sshx.NewLimiter().KeepOpen(200 * time.Millisecond)
	ctx := context.Background()
	if err := limiter.Do(ctx, "open", 1, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	// Every opening taken by other hosts.
	release := make(chan struct{})
	for i := range sshx.Openings {
		go limiter.Do(ctx, fmt.Sprintf("busy-%d", i), 1, func() error { <-release; return nil })
	}
	defer close(release)
	time.Sleep(20 * time.Millisecond)
	if err := limiter.Do(ctx, "open", 1, func() error { return nil }); err != nil {
		t.Fatalf("a command on an open connection waited for an opening: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	waiting, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := limiter.Do(waiting, "open", 1, func() error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a connection closed for lack of use: %v, want to wait for an opening", err)
	}
}

// A command that failed leaves the connection taken as closed: the next one opens it again.
func TestLimiterFailedCommandLeavesTheConnectionClosed(t *testing.T) {
	limiter := sshx.NewLimiter().KeepOpen(time.Minute)
	ctx := context.Background()
	if err := limiter.Do(ctx, "h", 1, func() error { return errors.New("connection refused") }); err == nil {
		t.Fatal("the failure was not returned")
	}
	release := make(chan struct{})
	for i := range sshx.Openings {
		go limiter.Do(ctx, fmt.Sprintf("busy-%d", i), 1, func() error { <-release; return nil })
	}
	defer close(release)
	time.Sleep(20 * time.Millisecond)
	waiting, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := limiter.Do(waiting, "h", 1, func() error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("after a failure: %v, want to wait for an opening", err)
	}
}
