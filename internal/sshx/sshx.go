// Package sshx builds the arguments of the system ssh command and quotes remote commands. It is the
// only package that starts processes.
package sshx

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

var (
	// ErrNUL is returned when an argument contains a NUL byte, which no command line can carry.
	ErrNUL = errors.New("argument contains a NUL byte")
	// ErrPathTooLong is returned when the connection sharing socket would not fit a Unix socket path.
	ErrPathTooLong = errors.New("control socket path too long")
)

// Target is where ssh connects: an alias from ~/.ssh/config, or hostname, user and port.
type Target struct {
	Alias    string
	Hostname string
	User     string
	Port     int
	KeyPath  string
}

// TargetFor returns the target of a host.
func TargetFor(h model.Host) Target {
	return Target{Alias: h.Alias, Hostname: h.Hostname, User: h.User, Port: h.EffectivePort(), KeyPath: h.KeyPath}
}

// Options are the connection settings shared by every command.
type Options struct {
	ControlDir          string // directory of the connection sharing sockets
	ConnectTimeout      time.Duration
	ServerAliveInterval time.Duration
	ControlPersist      time.Duration
}

// Args returns the arguments for the system ssh: no prompt (a missing key or an unknown host key
// fails instead of waiting), one shared connection per account, and the destination after "--"
// so it can never be read as an option.
func Args(t Target, o Options, remote string) []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=yes",
		"-o", "ConnectTimeout=" + seconds(o.ConnectTimeout),
		"-o", "ServerAliveInterval=" + seconds(o.ServerAliveInterval),
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + filepath.Join(o.ControlDir, "%C"),
		"-o", "ControlPersist=" + seconds(o.ControlPersist),
	}
	if t.KeyPath != "" {
		// Only this key: some hosts close the connection after a few refused keys.
		args = append(args, "-o", "IdentitiesOnly=yes", "-i", t.KeyPath)
	}
	destination := t.Alias
	if destination == "" {
		port := t.Port
		if port == 0 {
			port = 22
		}
		args = append(args, "-l", t.User, "-p", strconv.Itoa(port))
		destination = t.Hostname
	}
	return append(args, "--", destination, remote)
}

func seconds(d time.Duration) string {
	return strconv.Itoa(int(d / time.Second))
}

// %C expands to a 40 character hash, and a Unix socket path holds at most 103 characters plus the
// terminating NUL on macOS (sun_path[104]).
const (
	controlHashLen = 40
	maxSocketPath  = 103
)

// ControlDir returns the directory for connection sharing sockets under base (the user cache
// directory), or ErrPathTooLong when a socket path would not fit.
func ControlDir(base string) (string, error) {
	dir := filepath.Join(base, "vigie-ssh")
	if len(dir)+1+controlHashLen > maxSocketPath {
		return "", fmt.Errorf("%s: %w", dir, ErrPathTooLong)
	}
	return dir, nil
}

// Quote joins arguments into one string for the remote shell, each one in POSIX single quotes:
// ssh sends the command as a single string that the server's shell splits again.
func Quote(args ...string) (string, error) {
	quoted := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsRune(a, 0) {
			return "", ErrNUL
		}
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " "), nil
}
