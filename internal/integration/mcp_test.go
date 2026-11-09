//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// The built vigie mcp, started by an MCP client as a client starts it, on a Vigie whose one host is
// the bench account (ADR-0043): off refuses, read lists the sites and scans, act previews an
// update, refuses it with a confirmation it never gave, runs it with the right one, and the journal
// holds what the assistant started.
func TestAnAssistantThroughVigieMCP(t *testing.T) {
	ctx := context.Background()
	a := sshAccess(t)
	dir := t.TempDir()
	vigie := buildVigie(t)

	// The user's Vigie: the bench account, its host key trusted, the assistant's site.
	ws, err := workspace.Open(ctx, dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	db := ws.DB()
	host := model.Host{ID: model.NewID(), Name: "Bench", Hostname: a.host, Port: a.port, User: "vigie", KeyPath: a.key, WPCLI: "wp"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "assistant.example.test", Path: "~/sites/assistant",
		URL: "http://assistant.example.test"}
	for _, err := range []error{db.SaveHost(ctx, host), db.SaveSite(ctx, site)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	known, err := os.ReadFile(a.trusted(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "known_hosts"), known, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()

	// The checks of an update ask the site by its name, which only the bench's web server knows:
	// vigie reaches it through a proxy of this test.
	cmd := exec.Command(vigie, "--data-dir", dir, "mcp")
	cmd.Env = append(os.Environ(), "HTTP_PROXY="+benchProxy(t), "NO_PROXY=", "SSH_AUTH_SOCK=")
	cmd.Stderr = os.Stderr
	session, err := sdk.NewClient(&sdk.Implementation{Name: "bench-client", Version: "1.0"}, nil).
		Connect(ctx, &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	call := func(tool string, args map[string]any) (map[string]any, string, bool) {
		t.Helper()
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		raw, _ := json.Marshal(res)
		var out map[string]any
		if b, err := json.Marshal(res.StructuredContent); err == nil {
			_ = json.Unmarshal(b, &out)
		}
		return out, string(raw), res.IsError
	}
	tools := func() []string {
		t.Helper()
		list, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range list.Tools {
			names = append(names, tool.Name)
		}
		return names
	}
	sitePath := "Bench/assistant.example.test"

	// Off, the default: nothing listed, a call refused.
	if names := tools(); len(names) != 0 {
		t.Fatalf("off: tools %v, want none", names)
	}
	if _, raw, refused := call("sites", nil); !refused || !strings.Contains(raw, "Settings") {
		t.Fatalf("off: sites answered %s", raw)
	}

	// Read: the sites and a scan, no action.
	if err := ws.SetAssistantAccess(ctx, model.AssistantRead); err != nil {
		t.Fatal(err)
	}
	if names := tools(); !slices.Contains(names, "scan") || slices.Contains(names, "update") {
		t.Fatalf("read: tools %v", names)
	}
	if _, raw, refused := call("sites", nil); refused || !strings.Contains(raw, sitePath) {
		t.Fatalf("read: sites answered %s", raw)
	}
	scanned, raw, refused := call("scan", map[string]any{"path": "Bench"})
	sites, _ := scanned["sites"].([]any)
	if refused || len(sites) != 1 || !strings.Contains(raw, `"outcome":"completed"`) {
		t.Fatalf("read: scan answered %s", raw)
	}
	if _, raw, refused := call("update", map[string]any{"path": sitePath}); !refused {
		t.Fatalf("read: update answered %s", raw)
	}

	// Act: a preview first, nothing run; a confirmation Vigie never gave refused; the right one runs.
	if err := ws.SetAssistantAccess(ctx, model.AssistantAct); err != nil {
		t.Fatal(err)
	}
	preview, raw, refused := call("update", map[string]any{"path": sitePath})
	token, _ := preview["confirmation"].(string)
	if refused || preview["done"] != false || token == "" || !strings.Contains(raw, "akismet 5.3") {
		t.Fatalf("act: preview %s", raw)
	}
	if ops, _ := db.Operations(ctx, site.ID); len(ops) != 0 {
		t.Fatalf("a preview ran %d operations", len(ops))
	}
	if _, raw, refused := call("update", map[string]any{"path": sitePath, "confirmation": strings.Repeat("0", len(token))}); !refused {
		t.Fatalf("act: a confirmation never given ran: %s", raw)
	}
	done, raw, refused := call("update", map[string]any{"path": sitePath, "confirmation": token})
	if refused || done["done"] != true || !strings.Contains(raw, `"outcome":"updated"`) {
		t.Fatalf("act: update %s", raw)
	}
	if v := assistantPlugin(t, "akismet"); v == "5.3" {
		t.Errorf("akismet still %s after the update", v)
	}

	// The journal: the scan and the update, the assistant's, with its client's name.
	runs, err := db.Runs(ctx, started)
	if err != nil || len(runs) == 0 {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
	for _, r := range runs {
		if r.By != model.RunByAssistant || r.ByClient != "bench-client" {
			t.Errorf("run %+v, want the assistant bench-client's", r)
		}
	}
	ops, err := db.Operations(ctx, site.ID)
	if err != nil || len(ops) != 1 || ops[0].By != model.RunByAssistant || ops[0].ByClient != "bench-client" {
		t.Errorf("operations = %+v, %v; want the assistant's update", ops, err)
	}
}

// buildVigie builds the vigie command for the test, as the server build. On Linux, without cgo too:
// with it, Wails asks for GTK and WebKit, which the bench's machine has not and vigie mcp never uses.
func buildVigie(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "vigie")
	build := exec.Command("go", "build", "-tags", "server", "-o", out, "./cmd/vigie")
	if runtime.GOOS == "linux" {
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
	}
	build.Dir = filepath.Join("..", "..")
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, b)
	}
	return out
}

// benchProxy forwards every request it gets as an HTTP proxy to the account's web server, the
// name of the site kept.
func benchProxy(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	host, err := account.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := account.MappedPort(ctx, "80/tcp")
	if err != nil {
		t.Fatal(err)
	}
	var dialer net.Dialer
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL.Scheme, r.Out.URL.Host, r.Out.Host = "http", r.In.Host, r.In.Host
		},
		Transport: &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port.Port()))
		}},
	}
	server := httptest.NewServer(proxy)
	t.Cleanup(server.Close)
	return server.URL
}

// assistantPlugin reads a plugin's version on the assistant's site.
func assistantPlugin(t *testing.T, slug string) string {
	t.Helper()
	res, err := runAs(context.Background(), "wp", "plugin", "get", slug, "--field=version", "--skip-plugins", "--skip-themes", "--path=sites/assistant")
	if err != nil || res.exit != 0 {
		t.Fatalf("version of %s: %v %s", slug, err, res.stderr)
	}
	return strings.TrimSpace(res.stdout)
}
