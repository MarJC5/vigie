package sshx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrConnection is returned when ssh itself fails (exit status 255): unreachable host, refused
	// key, unknown or changed host key.
	ErrConnection = errors.New("ssh connection failed")
	// ErrTimeout is returned when a command runs longer than the runner's timeout.
	ErrTimeout = errors.New("command timed out")
)

// Result is the outcome of a remote command. A non-zero exit code is a result, not an error.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Runner runs a quoted remote command on a target.
type Runner interface {
	Run(ctx context.Context, target Target, remote string) (Result, error)
	// RunInput runs remote with input on its standard input: a fixed text of Vigie's, such as the
	// read-only script of a network's subsites (ADR-0029).
	RunInput(ctx context.Context, target Target, remote string, input []byte) (Result, error)
}

// ExecRunner runs commands with the system ssh, so ~/.ssh/config and the user's SSH agent apply.
type ExecRunner struct {
	SSH     string // path of the ssh binary; "ssh" when empty
	Options Options
	Timeout time.Duration // per command
}

// Run runs remote on target and waits for it, at most Timeout; it reads nothing on its standard
// input.
func (r ExecRunner) Run(ctx context.Context, target Target, remote string) (Result, error) {
	return r.RunInput(ctx, target, remote, nil)
}

// RunInput runs remote on target with input on its standard input, at most Timeout.
func (r ExecRunner) RunInput(ctx context.Context, target Target, remote string, input []byte) (Result, error) {
	var in io.Reader
	if input != nil {
		in = bytes.NewReader(input)
	}
	return r.RunReader(ctx, target, remote, in)
}

// RunReader runs remote on target with what input gives on its standard input, at most Timeout:
// a large input is read as it is sent, so its reader can tell how far it went.
func (r ExecRunner) RunReader(ctx context.Context, target Target, remote string, input io.Reader) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	binary := r.SSH
	if binary == "" {
		binary = "ssh"
	}
	cmd := exec.CommandContext(ctx, binary, Args(target, r.Options, remote)...)
	cmd.WaitDelay = time.Second // do not wait for pipes held by a killed process
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if input != nil {
		cmd.Stdin = input
	}
	err := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return res, fmt.Errorf("after %s: %w", r.Timeout, ErrTimeout)
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		// ssh "exits with the exit status of the remote command or with 255 if an error occurred".
		if res.ExitCode == 255 {
			return res, &ConnectionError{Failure: Classify(res.Stderr), Message: strings.TrimSpace(stderr.String())}
		}
		return res, nil
	}
	return res, err
}

// RunStream runs remote on target with what input gives on its standard input (nil: nothing), and
// writes its output to out as it comes, never holding it: a dump or an archive of a copy
// (ADR-0041). Its timeout is on silence: it fails when the command writes nothing for Timeout, however
// long it runs while output keeps coming. The result holds the exit code and the error output, its
// first 64 KiB.
func (r ExecRunner) RunStream(ctx context.Context, target Target, remote string, input io.Reader, out io.Writer) (Result, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	binary := r.SSH
	if binary == "" {
		binary = "ssh"
	}
	cmd := exec.CommandContext(ctx, binary, Args(target, r.Options, remote)...)
	cmd.WaitDelay = time.Second
	written := &lastWrite{w: out}
	written.touch()
	stderr := &capped{max: 64 << 10}
	cmd.Stdout, cmd.Stderr = written, stderr
	if input != nil {
		cmd.Stdin = input
	}
	// The silence watch: the command is stopped once nothing came for Timeout.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		tick := time.NewTicker(silenceTick(r.Timeout))
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				if time.Since(written.at()) > r.Timeout {
					cancel(fmt.Errorf("nothing for %s: %w", r.Timeout, ErrTimeout))
					return
				}
			}
		}
	}()
	err := cmd.Run()
	res := Result{Stderr: stderr.Bytes()}
	if cause := context.Cause(ctx); cause != nil {
		return res, cause
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		if res.ExitCode == 255 {
			return res, &ConnectionError{Failure: Classify(res.Stderr), Message: strings.TrimSpace(string(res.Stderr))}
		}
		return res, nil
	}
	return res, err
}

// silenceTick is how often a stream checks its silence: a tenth of its timeout, between 10 ms and a
// second.
func silenceTick(timeout time.Duration) time.Duration {
	return min(max(timeout/10, 10*time.Millisecond), time.Second)
}

// lastWrite passes what it is given to w and keeps when it last was.
type lastWrite struct {
	w    io.Writer
	mu   sync.Mutex
	last time.Time
}

func (l *lastWrite) Write(p []byte) (int, error) {
	l.touch()
	return l.w.Write(p)
}

func (l *lastWrite) touch() {
	l.mu.Lock()
	l.last = time.Now()
	l.mu.Unlock()
}

func (l *lastWrite) at() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last
}

// capped keeps the first max bytes written to it and drops the rest.
type capped struct {
	max int
	buf bytes.Buffer
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (c *capped) Bytes() []byte { return c.buf.Bytes() }

// Resolved is the effective configuration ssh uses for an alias of ~/.ssh/config.
type Resolved struct {
	Hostname     string
	Port         int
	User         string
	HostKeyAlias string
}

// KnownHostsName is the name ssh looks the host key up by: the host key alias as is when set
// (OpenSSH adds no port to it), else the host name with its port.
func (r Resolved) KnownHostsName() string {
	if r.HostKeyAlias != "" {
		return r.HostKeyAlias
	}
	return KnownHostsName(r.Hostname, r.Port)
}

// Resolve asks ssh for the configuration of alias (ssh -G), so Include and Match blocks are read
// by OpenSSH itself.
func (r ExecRunner) Resolve(ctx context.Context, alias string) (Resolved, error) {
	if err := checkHostName(alias); err != nil {
		return Resolved{}, err
	}
	binary := r.SSH
	if binary == "" {
		binary = "ssh"
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-G", "--", alias)
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return Resolved{}, fmt.Errorf("ssh -G %s: %w: %s", alias, err, strings.TrimSpace(stderr.String()))
	}
	var res Resolved
	for line := range strings.Lines(stdout.String()) {
		key, value, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch key {
		case "hostname":
			res.Hostname = value
		case "user":
			res.User = value
		case "hostkeyalias":
			res.HostKeyAlias = value
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return Resolved{}, fmt.Errorf("ssh -G %s: port %q: %w", alias, value, err)
			}
			res.Port = port
		}
	}
	if res.Hostname == "" {
		return Resolved{}, fmt.Errorf("ssh -G %s: no host name", alias)
	}
	return res, nil
}
