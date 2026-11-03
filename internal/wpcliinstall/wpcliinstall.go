// Package wpcliinstall installs WP-CLI in the home of an account that has none (ADR-0039): the
// version pinned here is downloaded on the user's machine, refused unless its SHA-512 is the one
// below, then sent over SSH to ~/bin/wp, and run once to check it answers. Nothing else is
// written on the server.
package wpcliinstall

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// The WP-CLI installed, pinned with its digest. When it is bumped, the release's GPG signature is
// checked against WP-CLI's release key first (docs/releasing.md), and testdata gets the release's
// .sha512 file.
const (
	Version = "2.12.0"
	SHA512  = "be928f6b8ca1e8dfb9d2f4b75a13aa4aee0896f8a9a0a1c45cd5d2c98605e6172e6d014dda2e27f88c98befc16c040cbb2bd1bfa121510ea5cdf5f6a30fe8832"
	Size    = 7142777
	// MinPHP is the oldest PHP WP-CLI runs on ("WP-CLI requires PHP 7.2.24 or later").
	MinPHP = "7.2.24"
	// Path is where it goes, in the account's home; Command is how the host then calls it.
	Path    = "~/bin/wp"
	Command = "~/bin/wp"
	// PHPCommand is tried when the home does not allow running programs.
	PHPCommand = "php ~/bin/wp"
)

// URL is the phar of the pinned version, on WP-CLI's release.
const URL = "https://github.com/wp-cli/wp-cli/releases/download/v" + Version + "/wp-cli-" + Version + ".phar"

// pinned is what Verify expects: the version above, or a small file in tests.
var pinned = struct {
	size   int64
	sha512 string
}{Size, SHA512}

var (
	// ErrDigest is returned when the file downloaded is not the WP-CLI pinned; nothing is sent.
	ErrDigest = errors.New("the file downloaded is not WP-CLI " + Version)
	// ErrExists is returned when ~/bin/wp is already there; it is left as it is.
	ErrExists = errors.New(Path + " already exists")
	// ErrNoAnswer is returned when neither command answers once installed; the file is removed.
	ErrNoAnswer = errors.New("WP-CLI does not answer on this account")
)

// Step is a step of an install, as the window and the journal name them.
type Step string

const (
	StepDownload Step = "download"
	StepCheck    Step = "check"
	StepSend     Step = "send"
	StepTest     Step = "test"
)

// Progress is where an install stands: its step, and for a download or a send the bytes so far.
type Progress struct {
	Step  Step
	Done  int64
	Total int64
}

// Download reads the pinned phar from url with client, at most Size bytes, telling progress as it
// reads, and refuses it unless it is the one pinned (ErrDigest).
func Download(ctx context.Context, client *http.Client, url, agent string, progress func(Progress)) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", agent)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, res.Status)
	}
	var b bytes.Buffer
	r := &counting{r: io.LimitReader(res.Body, pinned.size+1), on: func(n int64) { progress(Progress{Step: StepDownload, Done: n, Total: pinned.size}) }}
	if _, err := io.Copy(&b, r); err != nil {
		return nil, err
	}
	progress(Progress{Step: StepCheck})
	if err := Verify(b.Bytes()); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Verify tells whether phar is the WP-CLI pinned: its size and its SHA-512.
func Verify(phar []byte) error {
	sum := sha512.Sum512(phar)
	if int64(len(phar)) != pinned.size || hex.EncodeToString(sum[:]) != pinned.sha512 {
		return ErrDigest
	}
	return nil
}

// ReaderRunner is a runner that sends a standard input as it reads it (sshx.ExecRunner); without
// it, the send is told only when it ends.
type ReaderRunner interface {
	RunReader(ctx context.Context, target sshx.Target, remote string, input io.Reader) (sshx.Result, error)
}

// exitExists is the status of the send when ~/bin/wp is there already.
const exitExists = 3

// sendScript writes the standard input to a temporary name in ~/bin, makes it executable, and
// renames it ~/bin/wp unless that exists; a failure leaves nothing behind.
const sendScript = `set -e
mkdir -p "$HOME/bin"
tmp="$HOME/bin/.wp.vigie.$$"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp"
chmod 755 "$tmp"
if [ -e "$HOME/bin/wp" ]; then exit 3; fi
mv "$tmp" "$HOME/bin/wp"`

// SendCommand is the remote command of the send, its input the phar.
func SendCommand() (string, error) { return sshx.Quote("sh", "-c", sendScript) }

// RemoveCommand is the remote command that removes ~/bin/wp after a test that failed.
func RemoveCommand() (string, error) { return sshx.Quote("sh", "-c", `rm -f "$HOME/bin/wp"`) }

// Send writes phar to ~/bin/wp on target, telling progress as it is sent; ErrExists when ~/bin/wp
// is already there.
func Send(ctx context.Context, runner sshx.Runner, target sshx.Target, phar []byte, progress func(Progress)) error {
	remote, err := SendCommand()
	if err != nil {
		return err
	}
	total := int64(len(phar))
	progress(Progress{Step: StepSend, Total: total})
	var res sshx.Result
	if rr, ok := runner.(ReaderRunner); ok {
		in := &counting{r: bytes.NewReader(phar), on: func(n int64) { progress(Progress{Step: StepSend, Done: n, Total: total}) }}
		res, err = rr.RunReader(ctx, target, remote, in)
	} else {
		res, err = runner.RunInput(ctx, target, remote, phar)
	}
	if err != nil {
		return err
	}
	switch res.ExitCode {
	case 0:
		progress(Progress{Step: StepSend, Done: total, Total: total})
		return nil
	case exitExists:
		return ErrExists
	}
	return fmt.Errorf("%s: %s", Path, strings.TrimSpace(string(res.Stderr)))
}

// Answer is the command that answered once installed, and the version it told.
type Answer struct {
	Command string
	Version string
}

// Test runs the installed WP-CLI as ~/bin/wp, then as php ~/bin/wp; when neither answers, it
// removes ~/bin/wp and returns ErrNoAnswer with what the server said.
func Test(ctx context.Context, runner sshx.Runner, target sshx.Target, progress func(Progress)) (Answer, error) {
	progress(Progress{Step: StepTest})
	said := ""
	for _, command := range []string{Command, PHPCommand} {
		remote, err := wpcli.CLIInfo().Remote(command, "")
		if err != nil {
			return Answer{}, err
		}
		res, err := runner.Run(ctx, target, remote)
		if err != nil {
			return Answer{}, err
		}
		if res.ExitCode == 0 {
			if info, err := wpcli.ParseCLIInfo(res.Stdout); err == nil {
				return Answer{Command: command, Version: info.WPCLIVersion}, nil
			}
		}
		if s := strings.TrimSpace(string(res.Stderr)); s != "" {
			said = s
		}
	}
	remove, err := RemoveCommand()
	if err != nil {
		return Answer{}, err
	}
	if _, err := runner.Run(ctx, target, remove); err != nil {
		return Answer{}, err
	}
	if said == "" {
		return Answer{}, ErrNoAnswer
	}
	return Answer{}, fmt.Errorf("%w: %s", ErrNoAnswer, said)
}

// counting reports the bytes read so far through it.
type counting struct {
	r  io.Reader
	n  int64
	on func(int64)
}

func (c *counting) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.n += int64(n)
		c.on(c.n)
	}
	return n, err
}
