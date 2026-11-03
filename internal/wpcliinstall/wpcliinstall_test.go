package wpcliinstall

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
)

// The digest pinned is the one WP-CLI published with that release (its .sha512 file, kept in
// testdata when the version is pinned).
func TestThePinnedDigestIsWPCLIs(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "wp-cli-"+Version+".phar.sha512"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != SHA512 {
		t.Fatalf("SHA512 = %s, the release says %s", SHA512, got)
	}
	if want := "https://github.com/wp-cli/wp-cli/releases/download/v2.12.0/wp-cli-2.12.0.phar"; URL != want {
		t.Fatalf("URL = %s", URL)
	}
}

// pin makes Verify expect phar for the rest of the test.
func pin(t *testing.T, phar []byte) {
	t.Helper()
	saved := pinned
	sum := sha512.Sum512(phar)
	pinned.size, pinned.sha512 = int64(len(phar)), hex.EncodeToString(sum[:])
	t.Cleanup(func() { pinned = saved })
}

// The download is accepted only when it is the file pinned, and tells how far it went.
func TestDownload(t *testing.T) {
	phar := bytes.Repeat([]byte("<?php wp-cli "), 5000)
	pin(t, phar)
	served := phar
	var agent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.UserAgent()
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(served)
	}))
	defer server.Close()

	var steps []Progress
	got, err := Download(context.Background(), server.Client(), server.URL+"/wp-cli.phar", "Vigie/test", func(p Progress) { steps = append(steps, p) })
	if err != nil || !bytes.Equal(got, phar) {
		t.Fatalf("Download = %d bytes, %v", len(got), err)
	}
	if agent != "Vigie/test" {
		t.Errorf("User-Agent = %q", agent)
	}
	last := steps[len(steps)-1]
	before := steps[len(steps)-2]
	if before.Step != StepDownload || before.Done != int64(len(phar)) || last.Step != StepCheck {
		t.Errorf("progress ends with %+v, %+v", before, last)
	}

	for name, body := range map[string][]byte{
		"another file":   bytes.Repeat([]byte("<?php other  "), 5000),
		"a longer file":  append(append([]byte{}, phar...), 'x'),
		"a shorter file": phar[:len(phar)-1],
	} {
		served = body
		if _, err := Download(context.Background(), server.Client(), server.URL+"/wp-cli.phar", "", func(Progress) {}); !errors.Is(err, ErrDigest) {
			t.Errorf("%s: %v, want ErrDigest", name, err)
		}
	}
	if _, err := Download(context.Background(), server.Client(), server.URL+"/missing", "", func(Progress) {}); err == nil || errors.Is(err, ErrDigest) {
		t.Errorf("a missing file: %v", err)
	}
}

// fake answers each remote command in turn, recording them and their input.
type fake struct {
	results []sshx.Result
	remotes []string
	input   []byte
	reader  bool // whether it sends as it reads
}

func (f *fake) Run(ctx context.Context, target sshx.Target, remote string) (sshx.Result, error) {
	return f.RunInput(ctx, target, remote, nil)
}

func (f *fake) RunInput(_ context.Context, _ sshx.Target, remote string, input []byte) (sshx.Result, error) {
	f.remotes = append(f.remotes, remote)
	f.input = input
	res := f.results[0]
	f.results = f.results[1:]
	return res, nil
}

type readerFake struct{ *fake }

func (f readerFake) RunReader(ctx context.Context, target sshx.Target, remote string, input io.Reader) (sshx.Result, error) {
	b, err := io.ReadAll(input)
	if err != nil {
		return sshx.Result{}, err
	}
	f.reader = true
	return f.RunInput(ctx, target, remote, b)
}

// The send runs the script with the phar on its input; it tells the bytes sent when the runner
// streams them, and ErrExists when ~/bin/wp is there.
func TestSend(t *testing.T) {
	phar := bytes.Repeat([]byte("p"), 100_000)
	f := &fake{results: []sshx.Result{{}}}
	var sent []Progress
	if err := Send(context.Background(), readerFake{f}, sshx.Target{}, phar, func(p Progress) { sent = append(sent, p) }); err != nil {
		t.Fatal(err)
	}
	if !f.reader || !bytes.Equal(f.input, phar) || !strings.Contains(f.remotes[0], "mv ") {
		t.Fatalf("sent %d bytes with %q (reader %v)", len(f.input), f.remotes[0], f.reader)
	}
	if len(sent) < 3 || sent[len(sent)-1].Done != int64(len(phar)) {
		t.Errorf("progress %+v", sent)
	}

	if err := Send(context.Background(), &fake{results: []sshx.Result{{ExitCode: 3}}}, sshx.Target{}, phar, func(Progress) {}); !errors.Is(err, ErrExists) {
		t.Errorf("exit 3: %v", err)
	}
	err := Send(context.Background(), &fake{results: []sshx.Result{{ExitCode: 1, Stderr: []byte("mkdir: cannot create directory: Disk quota exceeded\n")}}}, sshx.Target{}, phar, func(Progress) {})
	if err == nil || !strings.Contains(err.Error(), "Disk quota exceeded") {
		t.Errorf("exit 1: %v", err)
	}
}

// The script itself, run by sh in a home of its own: the file lands executable at ~/bin/wp, no
// temporary file is left, and a second send leaves the first file as it was.
func TestTheSendScript(t *testing.T) {
	home := t.TempDir()
	run := func(input string) int {
		cmd := exec.Command("sh", "-c", sendScript)
		cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
		cmd.Stdin = strings.NewReader(input)
		err := cmd.Run()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		if err != nil {
			t.Fatal(err)
		}
		return 0
	}
	if code := run("first"); code != 0 {
		t.Fatalf("first send: exit %d", code)
	}
	wp := filepath.Join(home, "bin", "wp")
	info, err := os.Stat(wp)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("~/bin/wp: %v, %v", info, err)
	}
	if code := run("second"); code != exitExists {
		t.Fatalf("second send: exit %d, want %d", code, exitExists)
	}
	if b, _ := os.ReadFile(wp); string(b) != "first" {
		t.Errorf("~/bin/wp = %q after the second send", b)
	}
	entries, _ := os.ReadDir(filepath.Join(home, "bin"))
	if len(entries) != 1 {
		t.Errorf("~/bin holds %d files, want ~/bin/wp alone", len(entries))
	}
}

const cliInfo = `{"php_version":"8.4.27","wp_cli_version":"2.12.0"}`

// The test keeps the first command that answers; when none does, the file is removed and the
// server's words returned.
func TestTest(t *testing.T) {
	f := &fake{results: []sshx.Result{{ExitCode: 126, Stderr: []byte("bash: /home/u12ab/bin/wp: Permission denied")}, {Stdout: []byte(cliInfo)}}}
	got, err := Test(context.Background(), f, sshx.Target{}, func(Progress) {})
	if err != nil || got != (Answer{Command: PHPCommand, Version: "2.12.0"}) {
		t.Fatalf("Test = %+v, %v", got, err)
	}
	if len(f.remotes) != 2 {
		t.Errorf("ran %q", f.remotes)
	}

	f = &fake{results: []sshx.Result{{ExitCode: 126}, {ExitCode: 255 - 1, Stderr: []byte("PHP Warning: open_basedir restriction in effect")}, {}}}
	_, err = Test(context.Background(), f, sshx.Target{}, func(Progress) {})
	if !errors.Is(err, ErrNoAnswer) || !strings.Contains(err.Error(), "open_basedir") {
		t.Fatalf("Test = %v", err)
	}
	if last := f.remotes[len(f.remotes)-1]; !strings.Contains(last, `rm -f "$HOME/bin/wp"`) {
		t.Errorf("last command %q, want the removal", last)
	}
}
