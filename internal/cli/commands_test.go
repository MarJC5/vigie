package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
	"github.com/MarJC5/vigie/locales"
)

func texts(t *testing.T) *i18n.Catalogs {
	t.Helper()
	c, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// benchWorkspace stores Clients/Bench with the outdated, network and broken sites of the
// container bench, and answers their commands with the recordings. "Clients/Down" is a host whose
// connection is refused.
func benchWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	clients := model.Folder{ID: model.NewID(), Name: "Clients"}
	bench := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Bench", Alias: "bench"}
	down := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Down", Alias: "down", Position: 1}
	sites := []model.Site{
		{ID: model.NewID(), HostID: bench.ID, Name: "outdated.example.test", Path: "~/sites/outdated"},
		{ID: model.NewID(), HostID: bench.ID, Name: "network.example.test", Path: "~/sites/network", Multisite: true, Position: 1},
		{ID: model.NewID(), HostID: bench.ID, Name: "broken", Path: "~/sites/broken", Position: 2},
		{ID: model.NewID(), HostID: down.ID, Name: "unreachable.example.test", Path: "~/www"},
	}
	for _, err := range []error{db.SaveFolder(ctx, clients), db.SaveHost(ctx, bench), db.SaveHost(ctx, down)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := sshxtest.NewFake()
	for _, s := range sites {
		if err := db.SaveSite(ctx, s); err != nil {
			t.Fatal(err)
		}
		if s.HostID == bench.ID {
			wpclitest.Replay(t, fake, filepath.Base(s.Path), s.Path, "wp")
		}
	}
	return workspace.New(db, workspace.Deps{Runner: byHost{bench: fake}, Now: time.Now})
}

// byHost sends the bench's commands to the fake; every other host refuses the connection.
type byHost struct{ bench *sshxtest.Fake }

func (b byHost) Run(ctx context.Context, target sshx.Target, remote string) (sshx.Result, error) {
	if target.Alias == "bench" {
		return b.bench.Run(ctx, target, remote)
	}
	return sshx.Result{}, &sshx.ConnectionError{Failure: sshx.FailureRefused, Message: "ssh: connect to host down port 22: Connection refused"}
}

func TestScanASiteThatNeedsAttention(t *testing.T) {
	code, out, _ := run(t, "en", "scan", "Clients/Bench/outdated.example.test")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (needs attention)\n%s", code, out)
	}
	for _, want := range []string{"Clients/Bench/outdated.example.test", "Needs attention", "WordPress 7.1.3",
		"WordPress 7.1.4 available", "1 plugin update", "1 theme update", "2 translation updates"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestScanInFrench(t *testing.T) {
	_, out, _ := run(t, "fr", "scan", "Clients/Bench/outdated.example.test")
	for _, want := range []string{"À surveiller", "1 mise à jour d'extension", "2 mises à jour de traductions"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestScanErrorsExplainThemselves(t *testing.T) {
	code, out, _ := run(t, "en", "scan", "Clients")
	if code != 3 {
		t.Fatalf("exit %d, want 3 (a site could not be scanned)\n%s", code, out)
	}
	for _, want := range []string{
		"WordPress cannot be loaded: Error: Error establishing a database connection.",
		"The server refused the connection on this port.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestScanJSON(t *testing.T) {
	code, out, _ := run(t, "en", "scan", "Clients/Bench", "--json")
	var results []struct {
		Path      string `json:"path"`
		Outcome   string `json:"outcome"`
		Status    string `json:"status"`
		WordPress string `json:"wordpress"`
		Pending   struct {
			Core, Plugins, Themes, Languages int
		} `json:"pending"`
		Checked struct {
			Core, Plugins, Themes, Languages bool
		} `json:"checked"`
		Subsites int    `json:"subsites"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if code != 3 || len(results) != 3 {
		t.Fatalf("exit %d, %d results", code, len(results))
	}
	first := results[0]
	if first.Status != "warn" || first.WordPress != "7.1.3" || first.Pending.Plugins != 1 || !first.Checked.Core {
		t.Errorf("first = %+v", first)
	}
	if results[1].Subsites != 4 || results[2].Outcome != "cannot-load" || results[2].Error == "" {
		t.Errorf("network = %+v, broken = %+v", results[1], results[2])
	}
}

func TestScanUnknownPath(t *testing.T) {
	code, _, errOut := run(t, "en", "scan", "Clients/Nope")
	if code != 3 || !strings.Contains(errOut, `nothing at "Clients/Nope"`) {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestTreeShowsTheLatestStatus(t *testing.T) {
	ws := benchWorkspace(t)
	if _, err := ws.Scan(context.Background(), "Clients/Bench/outdated.example.test", nil); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	code := cli.Run(context.Background(), cli.Options{Command: []string{"tree"}}, cli.Env{
		Out: &out, Err: &errOut, Lang: "en", Texts: texts(t),
		Open: func(context.Context) (*workspace.Workspace, error) { return ws, nil },
	})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	for _, want := range []string{"Clients/", "  Bench/", "    outdated.example.test", "Needs attention", "WordPress 7.1.3",
		"    broken", "Not scanned yet"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("tree lacks %q:\n%s", want, out.String())
		}
	}
}

func TestDiscoverNeedsAHost(t *testing.T) {
	code, _, errOut := run(t, "en", "discover", "Clients")
	if code != 3 || !strings.Contains(errOut, `"Clients" is not a host`) {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = run(t, "en", "discover")
	if code != 3 || !strings.Contains(errOut, "discover needs the path of a host") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestDiscoverExplainsAConnectionFailure(t *testing.T) {
	code, _, errOut := run(t, "en", "discover", "Clients/Down")
	if code != 3 || !strings.Contains(errOut, "The server refused the connection") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}
