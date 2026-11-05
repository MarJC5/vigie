package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

// fakeUpdates stands for the updater: previews as given, and every run completed unless told.
type fakeUpdates struct {
	mu       sync.Mutex
	previews map[string]update.Preview
	failed   map[string]bool
	// rolls names the sites whose akismet breaks the site and rolls back on its own; cannotRoll
	// those whose rollback fails too (ADR-0042). The rollbacks go in db, as the updater stores them.
	rolls, cannotRoll map[string]bool
	db                *store.DB
	runs              []string
}

func (f *fakeUpdates) Preview(_ context.Context, job update.Job) (update.Preview, error) {
	return f.previews[job.Site.Name], nil
}

func (f *fakeUpdates) Run(_ context.Context, job update.Job, _ update.Preview, _ update.Selection) (model.Operation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, job.Site.Name)
	op := model.Operation{ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationUpdate, Outcome: model.OperationCompleted}
	if f.failed[job.Site.Name] {
		op.Outcome = model.OperationFailed
		op.Steps = []model.OperationStep{{Step: model.StepCheck, ItemKind: model.ItemPlugin, Slug: "fixture-breaks", Outcome: model.StepFailed,
			Message: "http://shop.example.test/ shows the critical error page of WordPress (status 500)"}}
	}
	if rolls, fails := f.rolls[job.Site.Name], f.cannotRoll[job.Site.Name]; (rolls || fails) && job.AutoRollback {
		op.Steps = []model.OperationStep{
			{Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: "akismet", VersionBefore: "5.3", VersionAfter: "5.7.2", Outcome: model.StepDone},
			{Step: model.StepCheck, ItemKind: model.ItemPlugin, Slug: "akismet", Outcome: model.StepFailed,
				Message: "http://shop.example.test/ shows the critical error page of WordPress (status 500)"},
		}
		back := model.Operation{ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationRollback, Started: time.Now(),
			OfOperation: op.ID, Automatic: true}
		ctx := context.Background()
		if err := f.db.StartOperation(ctx, back); err != nil {
			return op, err
		}
		restore := model.OperationStep{Step: model.StepRestoreFiles, ItemKind: model.ItemPlugin, Slug: "akismet",
			VersionBefore: "5.7.2", VersionAfter: "5.3", Started: time.Now(), Outcome: model.StepDone}
		if err := f.db.SaveOperationStep(ctx, back.ID, restore); err != nil {
			return op, err
		}
		outcome := model.OperationCompleted
		if fails {
			outcome, op.Outcome = model.OperationFailed, model.OperationFailed
		}
		if err := f.db.FinishOperation(ctx, back.ID, outcome, time.Now(), "plugin akismet back to 5.3"); err != nil {
			return op, err
		}
	}
	return op, nil
}

func (f *fakeUpdates) Rollback(context.Context, update.Job, model.Operation) (model.Operation, error) {
	return model.Operation{}, nil
}

var akismet = update.Preview{DumpTool: true,
	Core:    &update.Item{Kind: model.ItemCore, From: "7.1.3", To: "7.1.4"},
	Plugins: []update.Item{{Kind: model.ItemPlugin, Slug: "akismet", From: "5.3", To: "5.7.2"}}}

// updateEnv stores Shop/shop.example.test and Shop/blog.example.test, with their previews.
func updateEnv(t *testing.T, f *fakeUpdates) func(context.Context) (*workspace.Workspace, error) {
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
	host := model.Host{ID: model.NewID(), Name: "Shop", Alias: "shop"}
	if err := db.SaveHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	fake := sshxtest.NewFake()
	for i, name := range []string{"shop.example.test", "blog.example.test"} {
		site := model.Site{ID: model.NewID(), HostID: host.ID, Name: name, Path: "~/sites/" + name, Position: i}
		if err := db.SaveSite(ctx, site); err != nil {
			t.Fatal(err)
		}
		// The scan after an update answers as the plain site of the bench.
		wpclitest.Replay(t, fake, "plain", site.Path, "wp")
	}
	f.db = db
	ws := workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now, Updates: func(func(model.Operation, model.OperationStep)) workspace.Updates { return f }})
	return func(context.Context) (*workspace.Workspace, error) { return ws, nil }
}

func runUpdate(t *testing.T, f *fakeUpdates, lang, input string, args ...string) (int, string, string) {
	t.Helper()
	opts, err := cli.Parse(append([]string{"update"}, args...))
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), opts, cli.Env{
		In: strings.NewReader(input), Out: &out, Err: &errOut, Lang: lang, Texts: texts(t), Open: updateEnv(t, f),
	})
	return code, out.String(), errOut.String()
}

func newFake() *fakeUpdates {
	return &fakeUpdates{previews: map[string]update.Preview{"shop.example.test": akismet, "blog.example.test": akismet}, failed: map[string]bool{},
		rolls: map[string]bool{}, cannotRoll: map[string]bool{}}
}

// The preview first; nothing changes without a yes.
func TestUpdateAsksFirst(t *testing.T) {
	f := newFake()
	code, out, _ := runUpdate(t, f, "en", "n\n", "Shop")
	for _, want := range []string{"Shop/shop.example.test", "WordPress 7.1.3 → 7.1.4", "akismet 5.3 → 5.7.2", "Update 2 sites? [y/N]", "Nothing was updated."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if code != 1 || len(f.runs) != 0 {
		t.Fatalf("exit %d, runs %v; want 1 and nothing run", code, f.runs)
	}
}

func TestUpdateConfirmedInFrench(t *testing.T) {
	f := newFake()
	code, out, _ := runUpdate(t, f, "fr", "oui\n", "Shop")
	if code != 0 || len(f.runs) != 2 {
		t.Fatalf("exit %d, runs %v\n%s", code, f.runs, out)
	}
	for _, want := range []string{"Mettre à jour 2 sites ? [o/N]", "Shop/blog.example.test", "mis à jour"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// --yes answers for scripts; a failure says which check failed, points to the rollback, and
// leaves the next site of the host not started.
func TestUpdateWithYesAndAFailure(t *testing.T) {
	f := newFake()
	f.failed["shop.example.test"] = true
	code, out, _ := runUpdate(t, f, "en", "", "Shop", "--yes")
	if code != 3 {
		t.Fatalf("exit %d, want 3 (an update failed)\n%s", code, out)
	}
	for _, want := range []string{"critical error page", "History tab", "Shop/blog.example.test", "not started"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[y/N]") {
		t.Error("--yes still asked")
	}
}

// A site whose server cannot dump the database is listed with the reason, and not counted.
func TestUpdateWithoutADumpTool(t *testing.T) {
	f := newFake()
	nodump := akismet
	nodump.DumpTool = false
	f.previews["blog.example.test"] = nodump
	code, out, _ := runUpdate(t, f, "en", "y\n", "Shop")
	if code != 1 || !strings.Contains(out, "Update 1 site? [y/N]") || !strings.Contains(out, "no database dump tool") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if len(f.runs) != 1 || f.runs[0] != "shop.example.test" {
		t.Errorf("runs %v, want the shop only", f.runs)
	}
}

func TestNothingToUpdate(t *testing.T) {
	f := newFake()
	f.previews = map[string]update.Preview{"shop.example.test": {DumpTool: true}, "blog.example.test": {DumpTool: true}}
	code, out, _ := runUpdate(t, f, "en", "", "Shop")
	if code != 0 || !strings.Contains(out, "Nothing to update.") || strings.Contains(out, "[y/N]") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestUpdateJSON(t *testing.T) {
	f := newFake()
	code, _, errOut := runUpdate(t, f, "en", "", "Shop", "--json")
	if code != 3 || !strings.Contains(errOut, "--json needs --yes") {
		t.Fatalf("--json without --yes: exit %d, %q", code, errOut)
	}
	code, out, _ := runUpdate(t, f, "en", "", "Shop", "--json", "--yes")
	var got []struct {
		Path    string `json:"path"`
		Outcome string `json:"outcome"`
		Items   []struct {
			Kind, Slug, From, To string
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != 0 || len(got) != 2 || got[0].Outcome != "updated" || len(got[0].Items) != 2 || got[0].Items[1].Slug != "akismet" {
		t.Fatalf("exit %d, %+v", code, got)
	}
}

// An update the site ignores is listed, said left out, and not installed; a site with only
// ignored updates has nothing to update (ADR-0026).
func TestUpdateLeavesTheIgnoredUpdatesOut(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	f.previews["blog.example.test"] = update.Preview{DumpTool: true, Plugins: akismet.Plugins}
	open := updateEnv(t, f)
	ws, err := open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	key := model.UpdateKey(model.ScanItem{Kind: model.ItemPlugin, Slug: "akismet"})
	for _, path := range []string{"Shop/shop.example.test", "Shop/blog.example.test"} {
		n, err := tree.Resolve(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.DB().SaveIgnore(ctx, model.Ignore{SiteID: n.ID, Key: key, UntilVersion: "5.7.2", Created: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	opts, err := cli.Parse([]string{"update", "Shop", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := cli.Run(ctx, opts, cli.Env{In: strings.NewReader(""), Out: &out, Err: &out, Lang: "en", Texts: texts(t), Open: open})
	for _, want := range []string{"plugin akismet 5.3 → 5.7.2 (ignored, left out)", "Shop/blog.example.test\n  nothing to update"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if code != 0 || len(f.runs) != 1 || f.runs[0] != "shop.example.test" {
		t.Errorf("exit %d, runs %v; want shop.example.test only", code, f.runs)
	}
}

// A plugin that broke a site and went back on its own: the site is updated, its line says which
// version went back and why, and so does --json (ADR-0042).
func TestUpdateWithAPluginRolledBackOnItsOwn(t *testing.T) {
	f := newFake()
	f.rolls["shop.example.test"] = true
	code, out, _ := runUpdate(t, f, "fr", "", "Shop", "--yes")
	want := "mis à jour, akismet 5.7.2 revenu en arrière tout seul\u00a0: http://shop.example.test/ shows the critical error page of WordPress (status 500)"
	if code != 0 || !strings.Contains(out, "Shop/shop.example.test  "+want) {
		t.Fatalf("exit %d, want 0 and %q\n%s", code, want, out)
	}

	f = newFake()
	f.rolls["shop.example.test"] = true
	_, out, _ = runUpdate(t, f, "en", "", "Shop", "--json", "--yes")
	var got []struct {
		Path       string `json:"path"`
		Outcome    string `json:"outcome"`
		FailedStep string `json:"failedStep"`
		RolledBack []struct {
			Kind, Slug, Version, BackTo, Reason string
		} `json:"rolledBack"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	shop := got[0]
	if shop.Outcome != "updated" || shop.FailedStep != "" || len(shop.RolledBack) != 1 ||
		shop.RolledBack[0].Slug != "akismet" || shop.RolledBack[0].Version != "5.7.2" || shop.RolledBack[0].BackTo != "5.3" ||
		!strings.Contains(shop.RolledBack[0].Reason, "status 500") {
		t.Errorf("shop = %+v", shop)
	}
}

// A rollback on its own that fails stops the site, and says so.
func TestUpdateWhoseRollbackOnItsOwnFailed(t *testing.T) {
	f := newFake()
	f.cannotRoll["shop.example.test"] = true
	code, out, _ := runUpdate(t, f, "en", "", "Shop", "--yes")
	if code != 3 || !strings.Contains(out, "rollback of akismet failed") || !strings.Contains(out, "History tab") {
		t.Fatalf("exit %d, want 3\n%s", code, out)
	}
}

// The setting off, nothing rolls back on its own: the site fails as before.
func TestUpdateWithoutTheRollbackOnItsOwn(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	f.rolls["shop.example.test"] = true
	open := updateEnv(t, f)
	ws, err := open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.SetAutoRollback(ctx, false); err != nil {
		t.Fatal(err)
	}
	opts, err := cli.Parse([]string{"update", "Shop", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := cli.Run(ctx, opts, cli.Env{In: strings.NewReader(""), Out: &out, Err: &out, Lang: "en", Texts: texts(t), Open: open})
	if code != 0 || strings.Contains(out.String(), "rolled back") {
		t.Errorf("exit %d\n%s", code, out.String())
	}
}

// A version set aside on a site is listed, said left out, and not installed (ADR-0042, point 5).
func TestUpdateLeavesTheVersionsSetAsideOut(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	f.previews["blog.example.test"] = update.Preview{DumpTool: true, Plugins: akismet.Plugins}
	open := updateEnv(t, f)
	ws, err := open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"Shop/shop.example.test", "Shop/blog.example.test"} {
		n, err := tree.Resolve(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.DB().SaveSetAside(ctx, model.SetAside{SiteID: n.ID, Kind: model.ItemPlugin, Slug: "akismet", Version: "5.7.2",
			OperationID: model.NewID(), Created: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	opts, err := cli.Parse([]string{"update", "Shop", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := cli.Run(ctx, opts, cli.Env{In: strings.NewReader(""), Out: &out, Err: &out, Lang: "en", Texts: texts(t), Open: open})
	for _, want := range []string{"plugin akismet 5.3 → 5.7.2 (rolled back before, left out)", "Shop/blog.example.test\n  nothing to update"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if code != 0 || len(f.runs) != 1 || f.runs[0] != "shop.example.test" {
		t.Errorf("exit %d, runs %v; want shop.example.test only", code, f.runs)
	}
}
