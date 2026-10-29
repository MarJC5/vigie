package sshx_test

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

var opts = sshx.Options{
	ControlDir:          "/tmp/vigie-ssh",
	ConnectTimeout:      10 * time.Second,
	ServerAliveInterval: 15 * time.Second,
	ControlPersist:      2 * time.Minute,
}

var common = []string{
	"-o", "BatchMode=yes",
	"-o", "StrictHostKeyChecking=yes",
	"-o", "ConnectTimeout=10",
	"-o", "ServerAliveInterval=15",
	"-o", "ControlMaster=auto",
	"-o", "ControlPath=/tmp/vigie-ssh/%C",
	"-o", "ControlPersist=120",
}

func TestArgsWithAlias(t *testing.T) {
	target := sshx.TargetFor(model.Host{Alias: "infomaniak"})
	got := sshx.Args(target, opts, "wp-cli 'core' 'version'")
	want := append(slices.Clone(common), "--", "infomaniak", "wp-cli 'core' 'version'")
	if !slices.Equal(got, want) {
		t.Fatalf("Args() =\n%q\nwant\n%q", got, want)
	}
}

func TestArgsWithHostnameUserPortAndKey(t *testing.T) {
	target := sshx.TargetFor(model.Host{Hostname: "example.org", User: "deploy", Port: 18765, KeyPath: "/Users/me/.ssh/id ed25519"})
	got := sshx.Args(target, opts, "true")
	want := append(slices.Clone(common),
		"-o", "IdentitiesOnly=yes", "-i", "/Users/me/.ssh/id ed25519",
		"-l", "deploy", "-p", "18765",
		"--", "example.org", "true")
	if !slices.Equal(got, want) {
		t.Fatalf("Args() =\n%q\nwant\n%q", got, want)
	}
}

func TestArgsDefaultPort(t *testing.T) {
	got := sshx.Args(sshx.TargetFor(model.Host{Hostname: "example.org", User: "u"}), opts, "true")
	if !slices.Contains(got, "22") {
		t.Fatalf("Args() = %q, want port 22", got)
	}
}

func TestDestinationCannotBecomeAnOption(t *testing.T) {
	got := sshx.Args(sshx.TargetFor(model.Host{Hostname: "-oProxyCommand=evil", User: "u"}), opts, "true")
	dash := slices.Index(got, "--")
	if dash < 0 || got[dash+1] != "-oProxyCommand=evil" {
		t.Fatalf("Args() = %q: the destination must come after --", got)
	}
}

func TestControlDirFitsSocketLimit(t *testing.T) {
	// A Unix socket path is limited to 104 bytes on macOS; %C expands to 40 hex characters.
	dir, err := sshx.ControlDir("/Users/someone-with-a-long-name/Library/Caches")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(dir) + 1 + 40; n > 104 {
		t.Fatalf("ControlDir() = %q: socket path would be %d bytes", dir, n)
	}
	long := "/" + strings.Repeat("x", 100)
	if _, err := sshx.ControlDir(long); !errors.Is(err, sshx.ErrPathTooLong) {
		t.Fatalf("ControlDir(very long) error = %v, want ErrPathTooLong", err)
	}
}

func TestQuote(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"core", "version"}, "'core' 'version'"},
		{[]string{"--path=~/sites/misits.ch"}, "'--path=~/sites/misits.ch'"},
		{[]string{"it's"}, `'it'\''s'`},
		{[]string{""}, "''"},
		{[]string{"$(rm -rf ~)", "`id`", "a;b", "a b"}, "'$(rm -rf ~)' '`id`' 'a;b' 'a b'"},
	}
	for _, tt := range tests {
		got, err := sshx.Quote(tt.args...)
		if err != nil || got != tt.want {
			t.Errorf("Quote(%q) = %q, %v; want %q", tt.args, got, err, tt.want)
		}
	}
	if _, err := sshx.Quote("a\x00b"); !errors.Is(err, sshx.ErrNUL) {
		t.Errorf("Quote with a NUL byte: %v, want ErrNUL", err)
	}
}

// FuzzQuoteThroughShell checks that a real POSIX shell reads back exactly the quoted argument.
func FuzzQuoteThroughShell(f *testing.F) {
	for _, seed := range []string{"", "plain", "it's", "'", "''", `\`, "$HOME", "$(id)", "`id`", "a b\tc\nd", "*?[]", "~", "--path=x", "é ü 日本"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		quoted, err := sshx.Quote(s)
		if strings.ContainsRune(s, 0) {
			if !errors.Is(err, sshx.ErrNUL) {
				t.Fatalf("Quote(%q) with NUL: %v", s, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("/bin/sh", "-c", "printf '%s' "+quoted).Output()
		if err != nil {
			t.Fatalf("sh with %s: %v", quoted, err)
		}
		if string(out) != s {
			t.Fatalf("shell read %q back as %q", s, out)
		}
	})
}

func TestArgsReadVigiesKnownHostsFirst(t *testing.T) {
	o := opts
	o.KnownHostsFile = "/Users/a/Library/Application Support/Vigie/known_hosts"
	args := sshx.Args(sshx.Target{Alias: "shared"}, o, "true")
	want := `UserKnownHostsFile="/Users/a/Library/Application Support/Vigie/known_hosts" ~/.ssh/known_hosts`
	if !slices.Contains(args, want) {
		t.Fatalf("Args() = %q, want %q", args, want)
	}
}
