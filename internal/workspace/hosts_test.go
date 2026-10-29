package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

// recordedKeys are the keys of the bench server, as ssh-keyscan printed them.
func recordedKeys(t *testing.T) []sshx.HostKey {
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

// scanner answers for one server address.
type scanner struct {
	host string
	port int
	keys []sshx.HostKey
}

func (s scanner) Scan(_ context.Context, host string, port int) ([]sshx.HostKey, error) {
	if host != s.host || port != s.port {
		return nil, sshx.ErrNoHostKey
	}
	return s.keys, nil
}

// resolver knows one alias of ~/.ssh/config.
type resolver struct {
	alias    string
	resolved sshx.Resolved
}

func (r resolver) Resolve(_ context.Context, alias string) (sshx.Resolved, error) {
	if alias != r.alias {
		return sshx.Resolved{}, errors.New("unknown alias")
	}
	return r.resolved, nil
}

func newHosts(t *testing.T) (*workspace.Workspace, *store.DB, *sshxtest.Fake, string) {
	t.Helper()
	db := emptyDB(t)
	fake := sshxtest.NewFake()
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	ws := workspace.New(db, workspace.Deps{
		Runner:     fake,
		KeyScanner: scanner{host: "ssh.example.test", port: 2222, keys: recordedKeys(t)},
		Resolver:   resolver{alias: "shared", resolved: sshx.Resolved{Hostname: "ssh.example.test", Port: 2222, User: "u12ab"}},
		KnownHosts: knownHosts,
		Now:        time.Now,
	})
	return ws, db, fake, knownHosts
}

func TestHostKeysOfAHostname(t *testing.T) {
	ws, _, _, _ := newHosts(t)
	name, keys, err := ws.HostKeys(context.Background(), model.Host{Hostname: "ssh.example.test", Port: 2222, User: "u12ab"})
	if err != nil || name != "[ssh.example.test]:2222" || len(keys) != 3 {
		t.Fatalf("HostKeys() = %q, %d keys, %v", name, len(keys), err)
	}
}

func TestHostKeysOfAnAlias(t *testing.T) {
	ws, _, _, _ := newHosts(t)
	name, keys, err := ws.HostKeys(context.Background(), model.Host{Alias: "shared"})
	if err != nil || name != "[ssh.example.test]:2222" || len(keys) != 3 {
		t.Fatalf("HostKeys() = %q, %d keys, %v", name, len(keys), err)
	}
}

func TestTrustWritesVigiesKnownHosts(t *testing.T) {
	ws, _, _, knownHosts := newHosts(t)
	key := recordedKeys(t)[2]
	if err := ws.Trust("[ssh.example.test]:2222", key); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(knownHosts)
	if err != nil || !strings.Contains(string(data), "[ssh.example.test]:2222 "+key.Type+" "+key.Key) {
		t.Fatalf("known_hosts = %q, %v", data, err)
	}
}

func TestTestConnectionOfANewHost(t *testing.T) {
	ws, _, fake, _ := newHosts(t)
	php, err := sshx.Quote("php", "-r", "echo PHP_VERSION;")
	if err != nil {
		t.Fatal(err)
	}
	fake.Set(php, sshx.Result{Stdout: []byte("8.4.27")})
	info, err := wpcli.CLIInfo().Remote("wp-cli", "")
	if err != nil {
		t.Fatal(err)
	}
	fake.Set(info, wpclitest.Recorded(t, "plain", "cli-info"))
	report, err := ws.TestConnection(context.Background(), model.Host{Alias: "shared", WPCLI: "wp-cli"})
	if err != nil || report.WPCLI != "wp-cli" || report.WPCLIVersion != "2.12.0" {
		t.Fatalf("TestConnection() = %+v, %v", report, err)
	}
}

func TestDiscoverAnUnsavedHost(t *testing.T) {
	ws, _, fake, _ := newHosts(t)
	fake.Set(discover.FindCommand(), sshx.Result{Stdout: []byte("./sites/plain/wp-includes/version.php\n")})
	wpclitest.Replay(t, fake, "plain", "~/sites/plain", "wp")
	found, _, err := ws.DiscoverAccount(context.Background(), model.Host{Alias: "shared"})
	if err != nil || len(found) != 1 || found[0].State != discover.StateNew || found[0].Name != "plain.example.test" {
		t.Fatalf("DiscoverAccount() = %+v, %v", found, err)
	}
}

func TestAddHostWithTheChosenSites(t *testing.T) {
	ws, db, _, _ := newHosts(t)
	host := model.Host{ID: model.NewID(), Name: "Shared", Alias: "shared", HostKeyFingerprint: "SHA256:x"}
	sites := []model.Site{{ID: model.NewID(), HostID: host.ID, Name: "plain.example.test", Path: "~/sites/plain"}}
	if err := ws.AddHost(context.Background(), host, sites); err != nil {
		t.Fatal(err)
	}
	tree, err := db.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.SitesUnder(model.Node{Kind: model.KindHost, ID: host.ID}); len(got) != 1 {
		t.Fatalf("stored sites = %+v", got)
	}
	other := []model.Site{{ID: model.NewID(), HostID: model.NewID(), Name: "x", Path: "~/x"}}
	if err := ws.AddHost(context.Background(), model.Host{ID: model.NewID(), Name: "Other", Alias: "o"}, other); !errors.Is(err, workspace.ErrSiteOfAnotherHost) {
		t.Fatalf("site of another host: %v, want ErrSiteOfAnotherHost", err)
	}
}
