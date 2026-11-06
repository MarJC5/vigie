package mcp

import (
	"bytes"
	"compress/gzip"
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/locales"
)

// fakeUpdates offers the plugins given and records the jobs it runs.
type fakeUpdates struct {
	mu      sync.Mutex
	plugins []update.Item
	runs    []update.Job
}

func (f *fakeUpdates) Preview(context.Context, update.Job) (update.Preview, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return update.Preview{Plugins: slices.Clone(f.plugins), DumpTool: true}, nil
}

func (f *fakeUpdates) Run(_ context.Context, job update.Job, _ update.Preview, _ update.Selection) (model.Operation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, job)
	return model.Operation{ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationUpdate, Outcome: model.OperationCompleted}, nil
}

func (f *fakeUpdates) Rollback(context.Context, update.Job, model.Operation) (model.Operation, error) {
	return model.Operation{}, nil
}

func (f *fakeUpdates) ran() []update.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.runs)
}

// actBench is the read fixture at the level act, its updates offered by a fake, its copies run on
// a fake server that answers nothing, and a clock the test moves.
type actBench struct {
	ws      *workspace.Workspace
	updates *fakeUpdates
	now     time.Time
}

func newActBench(t *testing.T) *actBench {
	t.Helper()
	b := &actBench{updates: &fakeUpdates{plugins: []update.Item{{Kind: model.ItemPlugin, Slug: "woocommerce", From: "9.8.1", To: "9.8.5"}}},
		now: at.Add(2 * time.Hour)}
	b.ws = fixture(t, func(d *workspace.Deps) {
		d.Updates = func(func(model.Operation, model.OperationStep)) workspace.Updates { return b.updates }
		d.Runner = copyAnswers(t)
		d.DiskFree = func(string) (uint64, error) { return 1 << 40, nil }
	})
	if err := b.ws.SetAssistantAccess(context.Background(), model.AssistantAct); err != nil {
		t.Fatal(err)
	}
	return b
}

// copyAnswers is the account of the fixture's site, answering what a copy of its database asks.
func copyAnswers(t *testing.T) *sshxtest.Fake {
	t.Helper()
	fake := sshxtest.NewFake()
	wp := func(c wpcli.Command) string {
		line, err := c.Remote("wp", "~/sites/boulangerie")
		if err != nil {
			t.Fatal(err)
		}
		return line
	}
	ok := func(out string) sshx.Result { return sshx.Result{Stdout: []byte(out)} }
	fake.Set(wp(wpcli.CopyPaths()), ok(`{"root":"/home/u/sites/boulangerie","content":"/home/u/sites/boulangerie/wp-content","plugins":"/home/u/sites/boulangerie/wp-content/plugins","muPlugins":"/home/u/sites/boulangerie/wp-content/mu-plugins","themes":"/home/u/sites/boulangerie/wp-content/themes","uploads":"/home/u/sites/boulangerie/wp-content/uploads","prefix":"wp_"}`))
	fake.Set(wp(wpcli.CoreVersion()), ok("7.1.3\n"))
	fake.Set(wp(wpcli.DatabaseSize()), ok("12400000\n"))
	fake.Set(wp(wpcli.DatabaseTables()), ok(`[{"Name":"wp_posts","Size":"12400000 B"}]`))
	fake.Set(remote.CopyTools(), ok("find (GNU findutils) 4.10.0\n---\ntar (GNU tar) 1.35\n"))
	for _, dir := range []string{"/home/u/sites/boulangerie/wp-content", "/home/u/sites/boulangerie"} {
		line, _ := remote.ContentSizes(dir)
		fake.Set(line, ok("4\tplugins/\n---\n8\t.\n"))
	}
	var dump bytes.Buffer
	zw := gzip.NewWriter(&dump)
	zw.Write([]byte("-- MySQL dump\n-- Dump completed on 2026-11-06 12:30:02\n"))
	zw.Close()
	fake.Set(remote.DumpCompressed(wp(wpcli.DatabaseDump(nil))), sshx.Result{Stdout: dump.Bytes(), Stderr: []byte("vigie-dump-exit 0\n")})
	return fake
}

// client connects a client named Claude; options decide whether it declares elicitation.
func (b *actBench) client(t *testing.T, options *sdk.ClientOptions) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{Workspace: b.ws, Texts: texts, Lang: "en", Version: "0.3.0-test", Now: func() time.Time { return b.now }})
	serverEnd, clientEnd := sdk.NewInMemoryTransports()
	if _, err := s.server.Connect(ctx, serverEnd, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "Claude", Version: "1.0"}, options).Connect(ctx, clientEnd, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

var site = map[string]any{"path": boulangeriePath}

func with(args map[string]any, key string, value any) map[string]any {
	out := map[string]any{}
	for k, v := range args {
		out[k] = v
	}
	out[key] = value
	return out
}

// At the level act: the read tools and the four actions, nothing else (ADR-0043, point 4).
func TestTheActTools(t *testing.T) {
	cs := newActBench(t).client(t, nil)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Name == "update" && (tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint) {
			t.Error("update is not marked destructive")
		}
	}
	slices.Sort(names)
	want := []string{"copies", "copy", "history", "ignore", "journal", "scan", "site", "sites", "stop_ignoring", "update"}
	if !slices.Equal(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
}

// Without a confirmation an action only previews, with a token; the token runs it once, as the
// assistant's run with its client's name (ADR-0043, point 5).
func TestATokenRunsAnActionOnce(t *testing.T) {
	b := newActBench(t)
	cs := b.client(t, nil)
	raw, out, refused := answer(t, cs, "update", site)
	if refused || out["done"] != false || !strings.Contains(raw, "woocommerce 9.8.1 → 9.8.5") {
		t.Fatalf("preview: refused %v, %s", refused, raw)
	}
	token, _ := out["confirmation"].(string)
	if token == "" || len(b.updates.ran()) != 0 {
		t.Fatalf("token %q, runs %d; want a token and nothing run", token, len(b.updates.ran()))
	}
	raw, out, refused = answer(t, cs, "update", with(site, "confirmation", token))
	if refused || out["done"] != true {
		t.Fatalf("confirmed: refused %v, %s", refused, raw)
	}
	runs := b.updates.ran()
	if len(runs) != 1 || runs[0].By != model.RunByAssistant || runs[0].ByClient != "Claude" {
		t.Errorf("runs = %+v, want one, by the assistant Claude", runs)
	}
	if _, _, refused := answer(t, cs, "update", with(site, "confirmation", token)); !refused {
		t.Error("a token ran twice")
	}
}

// A token holds only for its tool and its arguments, five minutes, at the level act.
func TestATokenHoldsForItsCallOnly(t *testing.T) {
	b := newActBench(t)
	cs := b.client(t, nil)
	tokenOf := func() string {
		_, out, _ := answer(t, cs, "update", site)
		return out["confirmation"].(string)
	}
	if _, _, refused := answer(t, cs, "update", map[string]any{"path": "Clients/Shared hosting", "confirmation": tokenOf()}); !refused {
		t.Error("a token held for other arguments")
	}
	if _, _, refused := answer(t, cs, "stop_ignoring", with(with(site, "key", "x"), "confirmation", tokenOf())); !refused {
		t.Error("a token held for another tool")
	}
	token := tokenOf()
	b.now = b.now.Add(5*time.Minute + time.Second)
	if _, _, refused := answer(t, cs, "update", with(site, "confirmation", token)); !refused {
		t.Error("a token held after five minutes")
	}
	b.now = b.now.Add(-time.Hour)
	token = tokenOf()
	if err := b.ws.SetAssistantAccess(context.Background(), model.AssistantRead); err != nil {
		t.Fatal(err)
	}
	if _, _, refused := answer(t, cs, "update", with(site, "confirmation", token)); !refused {
		t.Error("a token held at the level read")
	}
	if len(b.updates.ran()) != 0 {
		t.Errorf("%d runs, want none", len(b.updates.ran()))
	}
}

// What the user agreed to is what runs: an update offered since the preview asks again.
func TestAPreviewThatChangedIsAskedAgain(t *testing.T) {
	b := newActBench(t)
	cs := b.client(t, nil)
	_, out, _ := answer(t, cs, "update", site)
	b.updates.mu.Lock()
	b.updates.plugins = append(b.updates.plugins, update.Item{Kind: model.ItemPlugin, Slug: "akismet", From: "5.3", To: "5.7.2"})
	b.updates.mu.Unlock()
	raw, again, refused := answer(t, cs, "update", with(site, "confirmation", out["confirmation"]))
	if refused || again["done"] != false || !strings.Contains(raw, "akismet 5.3 → 5.7.2") || again["confirmation"] == "" {
		t.Errorf("refused %v: %s; want a new preview and token", refused, raw)
	}
	if len(b.updates.ran()) != 0 {
		t.Error("ran what the user had not seen")
	}
}

// A client that can ask the user: Vigie asks, the action runs on accept only, and the token path
// is refused so that the assistant cannot answer in the user's place.
func TestAClientThatAsksTheUser(t *testing.T) {
	for _, c := range []struct {
		action string
		runs   int
	}{{"accept", 1}, {"decline", 0}, {"cancel", 0}} {
		b := newActBench(t)
		var asked string
		cs := b.client(t, &sdk.ClientOptions{ElicitationHandler: func(_ context.Context, req *sdk.ElicitRequest) (*sdk.ElicitResult, error) {
			asked = req.Params.Message
			return &sdk.ElicitResult{Action: c.action}, nil
		}})
		raw, out, refused := answer(t, cs, "update", site)
		if refused || !strings.Contains(asked, "woocommerce 9.8.1 → 9.8.5") {
			t.Errorf("%s: refused %v, asked %q: %s", c.action, refused, asked, raw)
		}
		if got := len(b.updates.ran()); got != c.runs {
			t.Errorf("%s: %d runs, want %d", c.action, got, c.runs)
		}
		if c.runs == 0 && (out["done"] != false || out["declined"] != true) {
			t.Errorf("%s: %s; want declined, nothing done", c.action, raw)
		}
	}
	b := newActBench(t)
	cs := b.client(t, &sdk.ClientOptions{ElicitationHandler: func(context.Context, *sdk.ElicitRequest) (*sdk.ElicitResult, error) {
		return &sdk.ElicitResult{Action: "decline"}, nil
	}})
	if _, _, refused := answer(t, cs, "update", with(site, "confirmation", "0123456789abcdef")); !refused {
		t.Error("a token was taken from a client that can ask the user")
	}
}

// A state sent back that Vigie did not sign is refused.
func TestAForgedStateIsRefused(t *testing.T) {
	b := newActBench(t)
	cs := b.client(t, &sdk.ClientOptions{
		ElicitationHandler: func(context.Context, *sdk.ElicitRequest) (*sdk.ElicitResult, error) { return nil, nil },
		MultiRoundTrip:     &sdk.MultiRoundTripOptions{Disabled: true},
	})
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "update", Arguments: site,
		InputResponses: sdk.InputResponseMap{"confirm": &sdk.ElicitResult{Action: "accept"}}, RequestState: "forged"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || len(b.updates.ran()) != 0 {
		t.Errorf("result %+v, runs %d; want refused, nothing run", res, len(b.updates.ran()))
	}
}

// Ignore and stop ignoring, each confirmed; ignoring for good asks a reason, as the window does.
func TestIgnoreAndStopIgnoring(t *testing.T) {
	b := newActBench(t)
	cs := b.client(t, nil)
	ctx := context.Background()
	key := "update|plugin:woocommerce|"
	_, out, refused := answer(t, cs, "ignore", with(site, "key", key))
	if refused || out["done"] != false {
		t.Fatalf("preview of ignore: %v", out)
	}
	if _, out, refused = answer(t, cs, "ignore", with(with(site, "key", key), "confirmation", out["confirmation"])); refused || out["done"] != true {
		t.Fatalf("ignore: %v", out)
	}
	ignores, err := b.ws.DB().Ignores(ctx, siteID(t, b.ws))
	if err != nil || len(ignores) != 1 || ignores[0].Key != key || ignores[0].UntilVersion != "9.8.5" {
		t.Errorf("ignores = %+v, %v; want WooCommerce's update ignored until 9.8.5", ignores, err)
	}
	if _, _, refused := answer(t, cs, "ignore", with(with(site, "key", key), "permanently", true)); !refused {
		t.Error("ignoring for good without a reason was not refused")
	}
	if _, _, refused := answer(t, cs, "ignore", with(site, "key", "nowhere:x")); !refused {
		t.Error("a key the site does not have was not refused")
	}
	_, out, _ = answer(t, cs, "stop_ignoring", with(site, "key", key))
	if _, out, refused = answer(t, cs, "stop_ignoring", with(with(site, "key", key), "confirmation", out["confirmation"])); refused || out["done"] != true {
		t.Fatalf("stop ignoring: %v", out)
	}
	if ignores, _ := b.ws.DB().Ignores(ctx, siteID(t, b.ws)); len(ignores) != 0 {
		t.Errorf("ignores = %+v, want none", ignores)
	}
}

// A copy, confirmed, is the assistant's, with its client's name.
func TestACopyIsTheAssistants(t *testing.T) {
	b := newActBench(t)
	cs := b.client(t, nil)
	raw, out, refused := answer(t, cs, "copy", with(site, "content", "database"))
	if refused || out["done"] != false || !strings.Contains(raw, "boulangerie-exemple.ch") || strings.Contains(raw, copiesRootName) {
		t.Fatalf("preview of copy: refused %v, %s", refused, raw)
	}
	raw, out, refused = answer(t, cs, "copy", with(with(site, "content", "database"), "confirmation", out["confirmation"]))
	if refused || out["done"] != true || !strings.Contains(raw, `"outcome":"completed"`) {
		t.Fatalf("copy: refused %v, %s", refused, raw)
	}
	copies, err := b.ws.DB().Copies(context.Background(), at)
	if err != nil || len(copies) != 1 || copies[0].By != model.RunByAssistant || copies[0].ByClient != "Claude" {
		t.Errorf("copies = %+v, %v; want one, the assistant Claude's", copies, err)
	}
}

func siteID(t *testing.T, ws *workspace.Workspace) model.ID {
	t.Helper()
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n, err := tree.Resolve(boulangeriePath)
	if err != nil {
		t.Fatal(err)
	}
	return n.ID
}
