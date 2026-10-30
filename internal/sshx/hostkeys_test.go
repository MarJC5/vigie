package sshx_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
)

func scanned(t *testing.T) []sshx.HostKey {
	t.Helper()
	out, err := os.ReadFile("testdata/keyscan.txt")
	if err != nil {
		t.Fatal(err)
	}
	keys, err := sshx.ParseKeyScan(out)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// Expected values printed by ssh-keygen -lf on the server for the same keys.
func TestFingerprintsMatchSSHKeygen(t *testing.T) {
	want := map[string]string{
		"ssh-rsa":             "SHA256:C7A6jy0J8YO6e3wDBgqIOPDXyuyIe3qo7twDzAIDY2c",
		"ecdsa-sha2-nistp256": "SHA256:+91qk9uB/U6iayJDcd19LyfUoBfNK///ByvoJTlXrbQ",
		"ssh-ed25519":         "SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg",
	}
	keys := scanned(t)
	if len(keys) != len(want) {
		t.Fatalf("ParseKeyScan() = %d keys, want %d", len(keys), len(want))
	}
	for _, k := range keys {
		got, err := k.Fingerprint()
		if err != nil {
			t.Fatal(err)
		}
		if got != want[k.Type] {
			t.Errorf("%s fingerprint = %s, want %s", k.Type, got, want[k.Type])
		}
	}
}

func TestParseKeyScanRejectsMalformedLines(t *testing.T) {
	for _, out := range []string{
		"ssh.example.test ssh-ed25519\n",                                                 // no key
		"ssh.example.test ssh-ed25519 not-base64!\n",                                     // bad encoding
		"ssh.example.test ssh-dss AAAAB3NzaC1kc3MAAAA=\n",                                // unsupported type
		"ssh.example.test ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAINIno5axTixjU51Gfx+sgPSAp5I\n", // type differs from the key
	} {
		if _, err := sshx.ParseKeyScan([]byte(out)); !errors.Is(err, sshx.ErrBadHostKey) {
			t.Errorf("ParseKeyScan(%q) error = %v, want ErrBadHostKey", out, err)
		}
	}
}

func TestKnownHostsName(t *testing.T) {
	for _, tc := range []struct {
		host string
		port int
		want string
	}{
		{"ssh.example.test", 22, "ssh.example.test"},
		{"ssh.example.test", 0, "ssh.example.test"},
		{"ssh.example.test", 2222, "[ssh.example.test]:2222"},
	} {
		if got := sshx.KnownHostsName(tc.host, tc.port); got != tc.want {
			t.Errorf("KnownHostsName(%q, %d) = %q, want %q", tc.host, tc.port, got, tc.want)
		}
	}
}

func TestTrustWritesOnlyVigiesFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "known_hosts")
	keys := scanned(t)
	if err := sshx.Trust(file, "[other.example.test]:22", keys[0]); err != nil {
		t.Fatal(err)
	}
	if err := sshx.Trust(file, "[ssh.example.test]:2222", keys[1]); err != nil {
		t.Fatal(err)
	}
	// Trusting again replaces the key of that name, for example after a key change.
	if err := sshx.Trust(file, "[ssh.example.test]:2222", keys[2]); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want := "[other.example.test]:22 " + keys[0].Type + " " + keys[0].Key + "\n" +
		"[ssh.example.test]:2222 " + keys[2].Type + " " + keys[2].Key + "\n"
	if string(data) != want {
		t.Fatalf("known_hosts =\n%s\nwant\n%s", data, want)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("known_hosts mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestTrustedKeysReadBackWhatTrustWrote(t *testing.T) {
	file := filepath.Join(t.TempDir(), "known_hosts")
	if keys, err := sshx.TrustedKeys(file, "ssh.example.test"); err != nil || keys != nil {
		t.Fatalf("TrustedKeys() without a file = %v, %v", keys, err)
	}
	keys := scanned(t)
	for _, name := range []string{"ssh.example.test", "[ssh.example.test]:2222"} {
		if err := sshx.Trust(file, name, keys[0]); err != nil {
			t.Fatal(err)
		}
	}
	// A line Vigie would not write itself is skipped.
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("ssh.example.test ssh-ed25519 not-a-key\nssh.example.test,other ssh-ed25519 " + keys[1].Key + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got, err := sshx.TrustedKeys(file, "ssh.example.test")
	if err != nil || !reflect.DeepEqual(got, []sshx.HostKey{keys[0]}) {
		t.Errorf("TrustedKeys() = %v, %v; want the key trusted under that very name", got, err)
	}
}

func TestTrustRejectsNamesThatChangeTheLine(t *testing.T) {
	file := filepath.Join(t.TempDir(), "known_hosts")
	for _, name := range []string{"", "a b", "a,b", "!a", "|1|hashed", "@cert-authority", "a\nb"} {
		if err := sshx.Trust(file, name, scanned(t)[0]); !errors.Is(err, sshx.ErrBadHostName) {
			t.Errorf("Trust(%q) error = %v, want ErrBadHostName", name, err)
		}
	}
}

func TestKnownHostsFileRejectsQuotes(t *testing.T) {
	if _, err := sshx.KnownHostsFile(`/data/with"quote`); !errors.Is(err, sshx.ErrBadPath) {
		t.Fatalf("KnownHostsFile with a quote: %v, want ErrBadPath", err)
	}
	got, err := sshx.KnownHostsFile("/Users/a/Library/Application Support/Vigie")
	if err != nil || got != "/Users/a/Library/Application Support/Vigie/known_hosts" {
		t.Fatalf("KnownHostsFile() = %q, %v", got, err)
	}
}

func TestKeyScannerArguments(t *testing.T) {
	scanner := sshx.KeyScanner{Binary: fakeTool(t, "fake-keyscan")}
	keys, err := scanner.Scan(t.Context(), "ssh.example.test", 2222)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Fatalf("Scan() = %d keys, want 3", len(keys))
	}
	if _, err := scanner.Scan(t.Context(), "-oProxyCommand=x", 22); !errors.Is(err, sshx.ErrBadHostName) {
		t.Fatalf("host name starting with '-': %v, want ErrBadHostName", err)
	}
}

func TestKeyScannerReportsNoKey(t *testing.T) {
	scanner := sshx.KeyScanner{Binary: fakeTool(t, "fake-keyscan")}
	if _, err := scanner.Scan(t.Context(), "silent.example.test", 22); !errors.Is(err, sshx.ErrNoHostKey) {
		t.Fatalf("no key printed: %v, want ErrNoHostKey", err)
	}
}

func TestResolveReadsTheEffectiveConfiguration(t *testing.T) {
	r := runner(t, 0)
	got, err := r.Resolve(t.Context(), "shared")
	if err != nil {
		t.Fatal(err)
	}
	want := sshx.Resolved{Hostname: "ssh.example.test", Port: 2222, User: "u12ab", HostKeyAlias: "shared-key"}
	if got != want {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
	// OpenSSH looks a host key alias up as is, without the port (sshconnect.c).
	if name := got.KnownHostsName(); name != "shared-key" {
		t.Fatalf("KnownHostsName() = %q, want the host key alias alone", name)
	}
	got.HostKeyAlias = ""
	if name := got.KnownHostsName(); name != "[ssh.example.test]:2222" {
		t.Fatalf("KnownHostsName() = %q, want the host name with the port", name)
	}
	if _, err := r.Resolve(t.Context(), "-oProxyCommand=x"); !errors.Is(err, sshx.ErrBadHostName) {
		t.Fatalf("alias starting with '-': %v, want ErrBadHostName", err)
	}
}

func TestFakeKeyscanPrintsArguments(t *testing.T) {
	// The fake prints its arguments on stderr, so the expected command line is checked here.
	scanner := sshx.KeyScanner{Binary: fakeTool(t, "fake-keyscan"), Timeout: 0}
	_, err := scanner.Scan(t.Context(), "args.example.test", 2222)
	if err == nil || !strings.Contains(err.Error(), "-T 5 -p 2222 -t ed25519,ecdsa,rsa -- args.example.test") {
		t.Fatalf("Scan() error = %v, want the arguments echoed", err)
	}
}

// Whatever a server sends, parsing returns keys that fingerprint, or an error; it never panics.
func FuzzParseKeyScan(f *testing.F) {
	recorded, err := os.ReadFile("testdata/keyscan.txt")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(recorded)
	f.Add([]byte("h ssh-ed25519 AAAAC3NzaC1lZDI1NTE5\n"))
	f.Add([]byte("h ssh-rsa AAAA/////w==\n"))
	f.Fuzz(func(t *testing.T, out []byte) {
		keys, err := sshx.ParseKeyScan(out)
		if err != nil {
			return
		}
		for _, k := range keys {
			if _, err := k.Fingerprint(); err != nil {
				t.Fatalf("parsed key does not fingerprint: %v", err)
			}
		}
	})
}

func TestKeyScannerClassifiesItsFailures(t *testing.T) {
	scanner := sshx.KeyScanner{Binary: fakeTool(t, "fake-keyscan")}
	_, err := scanner.Scan(t.Context(), "refused.example.test", 22)
	var connErr *sshx.ConnectionError
	if !errors.As(err, &connErr) || connErr.Failure != sshx.FailureRefused {
		t.Fatalf("Scan() = %v, want a ConnectionError classified as refused", err)
	}
}
