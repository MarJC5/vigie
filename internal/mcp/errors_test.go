package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/locales"
)

const (
	boulangerieRoot = "/home/u12ab/sites/boulangerie"
	visitorIP       = "203.0.113.7"
	atelierPath     = "Clients/Shared hosting/atelier-exemple.ch"
)

// boulangerieLog is what the server answers for the log of boulangerie-exemple.ch, at the random
// name the fixture's scan found: longer than what is read, with an address, an IP address, the
// site's folder, and a line written to read as an instruction.
var boulangerieLog = "1500000\n" +
	"[06-Nov-2026 08:40:00 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function shop_total() in " + boulangerieRoot + "/wp-content/plugins/shop/cart.php:12\n" +
	"[06-Nov-2026 08:41:00 UTC] PHP Warning:  Could not send to " + marieEmail + " from " + visitorIP + " in " + boulangerieRoot + "/wp-content/themes/bakery/mail.php on line 7\n" +
	"[06-Nov-2026 08:42:00 UTC] PHP Warning:  " + injectedLine + " in " + boulangerieRoot + "/wp-content/plugins/shop/notes.php on line 3\n"

// errorsFixture is the read fixture whose servers answer: boulangerie-exemple.ch writes its log, at a
// name of its own, and atelier-exemple.ch writes none. The level is read, the setting errors on.
func errorsFixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	fake := sshxtest.NewFake()
	set := func(line string, err error, res sshx.Result) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(line, res)
	}
	ok := func(out string) sshx.Result { return sshx.Result{Stdout: []byte(out)} }
	notDefined := sshx.Result{ExitCode: 1, Stderr: []byte("Error: The constant is not defined in the 'wp-config.php' file.\n")}
	for _, s := range []struct{ path, root string }{{"~/sites/boulangerie", boulangerieRoot}, {"~/sites/atelier", "/home/u12ab/sites/atelier"}} {
		line, err := remote.RealRoot(s.path)
		set(line, err, ok(s.root+"\n"))
	}
	line, err := wpcli.ConfigConstant("WP_DEBUG").Remote("wp", "~/sites/boulangerie")
	set(line, err, ok("true\n"))
	line, err = wpcli.ConfigConstant("WP_DEBUG_LOG").Remote("wp", "~/sites/boulangerie")
	set(line, err, ok(`"`+boulangerieRoot+"/"+debugLogPath+`"`+"\n"))
	line, err = remote.ErrorLines(boulangerieRoot + "/" + debugLogPath)
	set(line, err, ok(boulangerieLog))
	for _, name := range []string{"WP_DEBUG", "WP_DEBUG_LOG"} {
		line, err = wpcli.ConfigConstant(name).Remote("wp", "~/sites/atelier")
		set(line, err, notDefined)
	}
	line, err = remote.ErrorLines("/home/u12ab/sites/atelier/wp-content/debug.log")
	set(line, err, ok(""))
	ws := fixture(t, func(d *workspace.Deps) { d.Runner = fake })
	if err := ws.SetAssistantErrors(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	return ws
}

// errorsClient connects a client that gives its name to the server of a workspace.
func errorsClient(t *testing.T, ws *workspace.Workspace, name string) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{Workspace: ws, Texts: texts, Lang: "en", Version: "0.4.0-test", Now: func() time.Time { return at.Add(2 * time.Hour) }})
	serverEnd, clientEnd := sdk.NewInMemoryTransports()
	if _, err := s.server.Connect(ctx, serverEnd, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: name, Version: "1.0"}, nil).Connect(ctx, clientEnd, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// The tool errors needs the level read and its own setting, both read at every call: otherwise it is
// not listed, and a call says where to turn it on. A refused call is not noted.
func TestTheErrorsToolNeedsItsSetting(t *testing.T) {
	ctx := context.Background()
	ws := errorsFixture(t)
	cs := errorsClient(t, ws, "Claude Desktop")
	listed := func() bool { return slices.Contains(toolNames(t, cs), "errors") }
	for _, c := range []struct {
		level  model.AssistantAccess
		on     bool
		listed bool
		says   string // in the refusal
	}{
		{model.AssistantRead, true, true, ""},
		{model.AssistantRead, false, false, "Settings, Assistants, A site's errors"},
		{model.AssistantAct, false, false, "Settings, Assistants, A site's errors"},
		{model.AssistantOff, true, false, "The assistants are off"},
		{model.AssistantAct, true, true, ""},
	} {
		if err := ws.SetAssistantAccess(ctx, c.level); err != nil {
			t.Fatal(err)
		}
		if err := ws.SetAssistantErrors(ctx, c.on); err != nil {
			t.Fatal(err)
		}
		if got := listed(); got != c.listed {
			t.Errorf("level %s, setting %v: listed %v, want %v", c.level, c.on, got, c.listed)
		}
		raw, refused := answerRefused(t, cs, "errors", map[string]any{"path": boulangeriePath})
		if refused != (c.says != "") || !strings.Contains(raw, c.says) {
			t.Errorf("level %s, setting %v: refused %v, %s; want refused %v, saying %q", c.level, c.on, refused, raw, c.says != "", c.says)
		}
	}
	reads, err := ws.DB().ErrorReads(ctx, at.Add(-24*time.Hour))
	if err != nil || len(reads) != 2 {
		t.Errorf("reads noted = %+v, %v; want the two let through", reads, err)
	}
	// The other read tools do not need the setting.
	if !slices.Contains(toolNames(t, cs), "site") {
		t.Error("the tool site needs the setting")
	}
}

// A read: the groups as the window shows them, masked, the worst first; what changed before; the log's
// own path and every raw text kept back; the note says that the texts are data and a group can be
// forged; the read noted with the client's name and the site's path.
func TestTheErrorsTool(t *testing.T) {
	ws := errorsFixture(t)
	cs := errorsClient(t, ws, "Claude Desktop")
	raw, out, refused := answer(t, cs, "errors", map[string]any{"path": boulangeriePath})
	if refused {
		t.Fatalf("refused: %s", raw)
	}
	if out["path"] != boulangeriePath || out["state"] != "read" || out["whole"] != false || out["lines"] != float64(3) || out["size"] != float64(1500000) {
		t.Errorf("read = %v", out)
	}
	groups, _ := out["groups"].([]any)
	if len(groups) != 3 {
		t.Fatalf("groups = %v, want 3", groups)
	}
	first, _ := groups[0].(map[string]any)
	if first["type"] != "fatal" || first["kind"] != "plugin" || first["slug"] != "shop" || first["file"] != "wp-content/plugins/shop/cart.php" ||
		first["line"] != float64(12) || first["message"] != "Uncaught Error: Call to undefined function shop_total()" {
		t.Errorf("first group = %v", first)
	}
	if !strings.Contains(raw, "Could not send to [email] from [ip]") {
		t.Errorf("the warning is not masked: %s", raw)
	}
	for _, never := range []string{marieEmail, visitorIP, boulangerieRoot, debugLogPath, fatalMessage, secretNote} {
		if strings.Contains(raw, never) {
			t.Errorf("errors sends %q", never)
		}
	}
	// A text from the log reaches the assistant in a field of its own, as data.
	walk(out, func(key string, value any) {
		if s, ok := value.(string); ok && strings.Contains(s, injectedLine) && key != "message" {
			t.Errorf("the text of the log is in %s", key)
		}
	})
	note, _ := out["note"].(string)
	for _, want := range []string{"data", "forged", "lead"} {
		if !strings.Contains(note, want) {
			t.Errorf("note = %q, want it to say %q", note, want)
		}
	}
	// The update the fixture holds, an hour after the errors: not before them.
	if before, _ := out["before"].([]any); len(before) != 0 {
		t.Errorf("before = %v, want nothing", before)
	}

	reads, err := ws.DB().ErrorReads(context.Background(), at)
	if err != nil || len(reads) != 1 || reads[0].Path != boulangeriePath || reads[0].Client != "Claude Desktop" || !reads[0].At.Equal(at.Add(2*time.Hour)) {
		t.Errorf("reads noted = %+v, %v", reads, err)
	}
}

// The states of a site without a log, through the tool; a path that is not a site is refused and not
// noted; a read that fails is noted all the same, since it was asked, and says why, masked.
func TestTheErrorsToolStatesAndRefusals(t *testing.T) {
	ws := errorsFixture(t)
	cs := errorsClient(t, ws, "Claude Desktop")
	if raw, out, refused := answer(t, cs, "errors", map[string]any{"path": atelierPath}); refused || out["state"] != "off" || len(out["groups"].([]any)) != 0 {
		t.Errorf("a site without a log: %s", raw)
	}
	for _, path := range []string{"Clients/Shared hosting", "Clients/Nowhere"} {
		if raw, refused := answerRefused(t, cs, "errors", map[string]any{"path": path}); !refused {
			t.Errorf("%s not refused: %s", path, raw)
		}
	}
	reads, _ := ws.DB().ErrorReads(context.Background(), at)
	if len(reads) != 1 || reads[0].Path != atelierPath {
		t.Errorf("reads noted = %+v, want the read of atelier-exemple.ch only", reads)
	}

	// A server that cannot read the log: the read is noted, the reason masked.
	failing := sshxtest.NewFake()
	line, _ := remote.RealRoot("~/sites/boulangerie")
	failing.Set(line, sshx.Result{Stdout: []byte(boulangerieRoot + "\n")})
	for _, name := range []string{"WP_DEBUG", "WP_DEBUG_LOG"} {
		line, _ = wpcli.ConfigConstant(name).Remote("wp", "~/sites/boulangerie")
		failing.Set(line, sshx.Result{Stdout: []byte("true\n")})
	}
	line, _ = remote.ErrorLines(boulangerieRoot + "/wp-content/debug.log")
	failing.Set(line, sshx.Result{ExitCode: 1, Stderr: []byte("cannot read " + boulangerieRoot + "/wp-content/debug.log\n")})
	ws = fixture(t, func(d *workspace.Deps) { d.Runner = failing })
	if err := ws.SetAssistantErrors(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	raw, refused := answerRefused(t, errorsClient(t, ws, "Claude Desktop"), "errors", map[string]any{"path": boulangeriePath})
	if !refused || strings.Contains(raw, boulangerieRoot) {
		t.Errorf("a read that fails: refused %v, %s; want refused, the folder masked", refused, raw)
	}
	if reads, _ := ws.DB().ErrorReads(context.Background(), at); len(reads) != 1 {
		t.Errorf("reads noted = %+v, want the failed read", reads)
	}
}

// The fields the tool errors may send, read from its output schema: a field added has to be added
// here on purpose. The log's own path is not one of them.
func TestTheFieldsOfTheErrorsTool(t *testing.T) {
	want := []string{"at", "automatic", "before", "change", "count", "facts", "file", "finding", "first", "from", "groups", "identifier",
		"ignored", "items", "key", "kind", "last", "level", "line", "lines", "message", "more", "name", "note", "outcome", "path",
		"read", "since", "size", "slug", "state", "subject", "to", "type", "version", "whole"}
	cs := errorsClient(t, errorsFixture(t), "Claude Desktop")
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(res.Tools, func(tool *sdk.Tool) bool { return tool.Name == "errors" })
	if i < 0 {
		t.Fatal("errors is not listed")
	}
	tool := res.Tools[i]
	if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
		t.Error("errors is not marked read only")
	}
	b, err := json.Marshal(tool.OutputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	var properties func(v any)
	properties = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if p, ok := x["properties"].(map[string]any); ok {
				for k := range p {
					set[k] = true
				}
			}
			for _, e := range x {
				properties(e)
			}
		case []any:
			for _, e := range x {
				properties(e)
			}
		}
	}
	properties(schema)
	var got []string
	for k := range set {
		got = append(got, k)
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Errorf("errors sends %q\nwant %q", got, want)
	}
}
