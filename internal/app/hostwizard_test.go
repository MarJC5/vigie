package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

func benchKeys(t *testing.T) []sshx.HostKey {
	t.Helper()
	out, err := os.ReadFile(filepath.Join("..", "sshx", "testdata", "keyscan.txt"))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := sshx.ParseKeyScan(out)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// keyScanner answers for ssh.example.test:2222 only.
type keyScanner struct{ keys []sshx.HostKey }

func (k keyScanner) Scan(_ context.Context, host string, port int) ([]sshx.HostKey, error) {
	if host != "ssh.example.test" || port != 2222 {
		return nil, sshx.ErrNoHostKey
	}
	return k.keys, nil
}

type noAliases struct{}

func (noAliases) Resolve(context.Context, string) (sshx.Resolved, error) {
	return sshx.Resolved{}, os.ErrNotExist
}

func wizard(t *testing.T) (*app.HostWizard, *sshxtest.Fake, string, *workspace.Workspace) {
	t.Helper()
	fake := sshxtest.NewFake()
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	ws := workspace.New(repository(t), workspace.Deps{
		Runner: fake, KeyScanner: keyScanner{benchKeys(t)}, Resolver: noAliases{}, KnownHosts: knownHosts, Now: time.Now,
	})
	return app.NewHostWizard(ws), fake, knownHosts, ws
}

var details = app.HostDetails{Name: "Bench", Hostname: "ssh.example.test", Port: 2222, User: "u12ab"}

func code(p *app.Problem) string {
	if p == nil {
		return ""
	}
	return p.Code
}

func TestCheckDetails(t *testing.T) {
	w, _, _, _ := wizard(t)
	for _, tc := range []struct {
		change func(*app.HostDetails)
		want   string
	}{
		{func(*app.HostDetails) {}, ""},
		{func(d *app.HostDetails) { d.Name = " " }, "problem.emptyName"},
		{func(d *app.HostDetails) { d.Name = "a/b" }, "problem.slashInName"},
		{func(d *app.HostDetails) { d.Hostname = "" }, "problem.noConnection"},
		{func(d *app.HostDetails) { d.User = "" }, "problem.noUser"},
		{func(d *app.HostDetails) { d.Port = 70000 }, "problem.invalidPort"},
		{func(d *app.HostDetails) { d.FolderID = model.NewID().String() }, "problem.folderNotFound"},
	} {
		d := details
		tc.change(&d)
		if got := code(w.Check(context.Background(), d)); got != tc.want {
			t.Errorf("Check(%+v) = %q, want %q", d, got, tc.want)
		}
	}
}

func TestOnlyAKeyReceivedFromTheServerCanBeTrusted(t *testing.T) {
	w, _, knownHosts, _ := wizard(t)
	keys := w.HostKeys(context.Background(), details)
	if keys.Problem != nil || keys.Name != "[ssh.example.test]:2222" || len(keys.Keys) != 3 {
		t.Fatalf("HostKeys() = %+v", keys)
	}
	if p := w.Trust(keys.Name, "SHA256:not-received"); code(p) != "problem.keyNotReceived" {
		t.Fatalf("Trust(unknown fingerprint) = %+v, want keyNotReceived", p)
	}
	if p := w.Trust(keys.Name, keys.Keys[0].Fingerprint); p != nil {
		t.Fatalf("Trust() = %+v", p)
	}
	data, err := os.ReadFile(knownHosts)
	if err != nil || !strings.HasPrefix(string(data), "[ssh.example.test]:2222 ") {
		t.Fatalf("known_hosts = %q, %v", data, err)
	}
	other := details
	other.Hostname = "elsewhere.example.test"
	if got := w.HostKeys(context.Background(), other); code(got.Problem) != "problem.noHostKey" {
		t.Fatalf("HostKeys(no key) = %+v", got)
	}
}

func TestConnectionResultAndProblems(t *testing.T) {
	w, fake, _, _ := wizard(t)
	php, _ := sshx.Quote("php", "-r", "echo PHP_VERSION;")
	fake.Set(php, sshx.Result{Stdout: []byte("8.4.27")})
	info, _ := wpcli.CLIInfo().Remote("wp", "")
	fake.Set(info, wpclitest.Recorded(t, "plain", "cli-info"))
	got := w.TestConnection(context.Background(), details)
	if got.Problem != nil || got.WPCLI != "wp" || got.WPCLIVersion != "2.12.0" || got.PHPVersion != "8.4.27" {
		t.Fatalf("TestConnection() = %+v", got)
	}

	fake.Set(info, sshx.Result{ExitCode: 127})
	for _, c := range []string{"wp-cli", "~/bin/wp", "php ~/wp-cli.phar"} {
		remote, _ := wpcli.CLIInfo().Remote(c, "")
		fake.Set(remote, sshx.Result{ExitCode: 127})
	}
	got = w.TestConnection(context.Background(), details)
	if code(got.Problem) != "problem.noWpCli" || !strings.Contains(got.Problem.Detail, "wp-cli") {
		t.Fatalf("TestConnection(no WP-CLI) = %+v", got)
	}
}

func TestConnectionFailureUsesTheFailureText(t *testing.T) {
	w, _, _, _ := wizard(t)
	// The fake knows no command: a real runner would have failed to connect; here an unexpected
	// command is reported as an error.
	got := w.TestConnection(context.Background(), details)
	if code(got.Problem) != "problem.error" {
		t.Fatalf("TestConnection() = %+v", got)
	}
	if p := app.ProblemOf(&sshx.ConnectionError{Failure: sshx.FailureTimedOut, Message: "timed out"}); p.Code != "failure.timedOut" {
		t.Fatalf("ProblemOf(timed out) = %+v", p)
	}
}

func TestDiscoverAndAdd(t *testing.T) {
	w, fake, _, ws := wizard(t)
	fake.Set(discover.FindCommand(), sshx.Result{Stdout: []byte("./sites/plain/wp-includes/version.php\n./sites/network/wp-includes/version.php\n")})
	wpclitest.Replay(t, fake, "plain", "~/sites/plain", "wp")
	wpclitest.Replay(t, fake, "network", "~/sites/network", "wp")
	found := w.Discover(context.Background(), details)
	if found.Problem != nil || len(found.Installs) != 2 {
		t.Fatalf("Discover() = %+v", found)
	}

	keys := w.HostKeys(context.Background(), details)
	fingerprint := keys.Keys[2].Fingerprint
	choices := []app.Choice{}
	for _, i := range found.Installs {
		choices = append(choices, app.Choice{Path: i.Path, Name: i.Name, URL: i.URL, Multisite: i.Multisite})
	}
	if got := w.Add(context.Background(), details, fingerprint, choices); code(got.Problem) != "problem.keyNotReceived" {
		t.Fatalf("Add() before trusting = %+v, want keyNotReceived", got)
	}
	if p := w.Trust(keys.Name, fingerprint); p != nil {
		t.Fatal(p)
	}
	added := w.Add(context.Background(), details, fingerprint, choices)
	if added.Problem != nil || added.HostID == "" {
		t.Fatalf("Add() = %+v", added)
	}
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	node, err := tree.Resolve("Bench")
	if err != nil {
		t.Fatal(err)
	}
	host, _ := tree.Host(node.ID)
	sites := tree.SitesUnder(node)
	if host.HostKeyFingerprint != fingerprint || len(sites) != 2 || !sites[0].Multisite && !sites[1].Multisite {
		t.Fatalf("stored host %+v, sites %+v", host, sites)
	}
	if again := w.Add(context.Background(), details, fingerprint, nil); code(again.Problem) != "problem.duplicateName" {
		t.Fatalf("Add() twice = %+v, want duplicateName", again)
	}
}
