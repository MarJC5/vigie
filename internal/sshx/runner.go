package sshx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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
}

// ExecRunner runs commands with the system ssh, so ~/.ssh/config and the user's SSH agent apply.
type ExecRunner struct {
	SSH     string // path of the ssh binary; "ssh" when empty
	Options Options
	Timeout time.Duration // per command
}

// Run runs remote on target and waits for it, at most Timeout.
func (r ExecRunner) Run(ctx context.Context, target Target, remote string) (Result, error) {
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
