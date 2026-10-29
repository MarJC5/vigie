//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/hostcheck"
	"github.com/MarJC5/vigie/internal/sshx"
)

// access is how the tests reach the account over SSH from this machine.
type access struct {
	host  string
	port  int
	key   string // authorized private key
	other string // a key the account does not accept
	dir   string // where both private keys are, removed after the run
}

var (
	accessOnce sync.Once
	accessVal  access
	accessErr  error
)

// sshAccess creates two key pairs in the account, authorizes one, and copies both private keys to
// this machine (test keys, generated per run, never committed).
func sshAccess(t *testing.T) access {
	t.Helper()
	accessOnce.Do(func() { accessVal, accessErr = newAccess() })
	if accessErr != nil {
		t.Fatal(accessErr)
	}
	// The user's own agent and keys stay out of the tests.
	t.Setenv("SSH_AUTH_SOCK", "")
	return accessVal
}

func newAccess() (access, error) {
	ctx := context.Background()
	var a access
	for _, name := range []string{"test-key", "other-key"} {
		if res, err := runAs(ctx, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", "/home/vigie/"+name); err != nil || res.exit != 0 {
			return a, errors.Join(err, errors.New(res.stderr))
		}
	}
	if res, err := runAs(ctx, "sh", "-c", "cat test-key.pub >> .ssh/authorized_keys && chmod 600 .ssh/authorized_keys"); err != nil || res.exit != 0 {
		return a, errors.Join(err, errors.New(res.stderr))
	}
	dir, err := os.MkdirTemp("", "vigie-integration-")
	if err != nil {
		return a, err
	}
	for _, name := range []string{"test-key", "other-key"} {
		res, err := runAs(ctx, "cat", "/home/vigie/"+name)
		if err != nil || res.exit != 0 {
			return a, errors.Join(err, errors.New(res.stderr))
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(res.stdout), 0o600); err != nil {
			return a, err
		}
	}
	a.dir, a.key, a.other = dir, filepath.Join(dir, "test-key"), filepath.Join(dir, "other-key")
	if a.host, err = account.Host(ctx); err != nil {
		return a, err
	}
	port, err := account.MappedPort(ctx, "22/tcp")
	if err != nil {
		return a, err
	}
	a.port, err = strconv.Atoi(port.Port())
	return a, err
}

func (a access) target(key string) sshx.Target {
	return sshx.Target{Hostname: a.host, User: "vigie", Port: a.port, KeyPath: key}
}

// runner gets its own directory of shared connections: ssh reuses an open master connection
// without checking host or client keys again, which would hide the failures tested here.
func (a access) runner(t *testing.T, knownHosts string) sshx.ExecRunner {
	t.Helper()
	// Short, for the 104 byte limit of Unix socket paths on macOS.
	controlDir, err := os.MkdirTemp("/tmp", "vg-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(controlDir) })
	return sshx.ExecRunner{Timeout: 30 * time.Second, Options: sshx.Options{
		ControlDir:          controlDir,
		ConnectTimeout:      10 * time.Second,
		ServerAliveInterval: 15 * time.Second,
		ControlPersist:      time.Second,
		KnownHostsFile:      knownHosts,
	}}
}

// trusted scans the account's keys and trusts its ed25519 key in a new known_hosts file.
func (a access) trusted(t *testing.T) string {
	t.Helper()
	keys, err := sshx.KeyScanner{}.Scan(context.Background(), a.host, a.port)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "known_hosts")
	for _, k := range keys {
		if k.Type == "ssh-ed25519" {
			if err := sshx.Trust(file, sshx.KnownHostsName(a.host, a.port), k); err != nil {
				t.Fatal(err)
			}
			return file
		}
	}
	t.Fatalf("no ed25519 key in %+v", keys)
	return ""
}

func TestKeyScanFingerprintsMatchTheServer(t *testing.T) {
	a := sshAccess(t)
	keys, err := sshx.KeyScanner{}.Scan(context.Background(), a.host, a.port)
	if err != nil {
		t.Fatal(err)
	}
	res, err := runAs(context.Background(), "sh", "-c", "for f in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -lf $f; done")
	if err != nil || res.exit != 0 {
		t.Fatal(err, res.stderr)
	}
	if len(keys) != 3 {
		t.Fatalf("Scan() = %d keys, want ed25519, ecdsa and rsa", len(keys))
	}
	for _, k := range keys {
		fp, err := k.Fingerprint()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.stdout, fp) {
			t.Errorf("%s fingerprint %s not printed by ssh-keygen on the server:\n%s", k.Type, fp, res.stdout)
		}
	}
}

func TestConnectionTestFindsPHPAndWPCLI(t *testing.T) {
	a := sshAccess(t)
	report, err := hostcheck.Check(context.Background(), a.runner(t, a.trusted(t)), a.target(a.key), "")
	if err != nil {
		t.Fatal(err)
	}
	if report.WPCLI != "wp" || report.WPCLIVersion != "2.12.0" || !strings.HasPrefix(report.PHPVersion, "8.4.") {
		t.Fatalf("Check() = %+v, want wp 2.12.0 on PHP 8.4", report)
	}
}

func TestConnectionFailuresAreClassified(t *testing.T) {
	a := sshAccess(t)
	empty := filepath.Join(t.TempDir(), "known_hosts")
	changed := a.trusted(t)
	pub, err := os.ReadFile(a.key + ".pub")
	if err != nil {
		// The public key stays in the account: read it from there.
		res, rerr := runAs(context.Background(), "cat", "/home/vigie/test-key.pub")
		if rerr != nil || res.exit != 0 {
			t.Fatal(err, rerr)
		}
		pub = []byte(res.stdout)
	}
	fields := strings.Fields(string(pub))
	// Another valid ed25519 key in place of the server's.
	if err := sshx.Trust(changed, sshx.KnownHostsName(a.host, a.port), sshx.HostKey{Type: fields[0], Key: fields[1]}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		knownHosts string
		key        string
		want       sshx.Failure
	}{
		{"host key not trusted yet", empty, a.key, sshx.FailureHostKeyUnknown},
		{"host key changed", changed, a.key, sshx.FailureHostKeyChanged},
		{"key refused", a.trusted(t), a.other, sshx.FailureAuthRefused},
	} {
		_, err := hostcheck.Check(context.Background(), a.runner(t, tc.knownHosts), a.target(tc.key), "")
		var connErr *sshx.ConnectionError
		if !errors.As(err, &connErr) || connErr.Failure != tc.want {
			t.Errorf("%s: %v, want %s", tc.name, err, tc.want)
		}
	}
}
