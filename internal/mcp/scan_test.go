package mcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
	"github.com/MarJC5/vigie/locales"
)

// scanBench stores Clients/Bench/outdated.example.test, answered by the bench's recordings, at the
// level read; it returns the server, a client of it, and the database.
func scanBench(t *testing.T) (*Server, *sdk.ClientSession, *store.DB) {
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
	site := model.Site{ID: model.NewID(), HostID: bench.ID, Name: "outdated.example.test", Path: "~/sites/outdated"}
	for _, err := range []error{db.SaveFolder(ctx, clients), db.SaveHost(ctx, bench), db.SaveSite(ctx, site)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := sshxtest.NewFake()
	wpclitest.Replay(t, fake, "outdated", site.Path, "wp")
	ws := workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now})
	if err := ws.SetAssistantAccess(ctx, model.AssistantRead); err != nil {
		t.Fatal(err)
	}
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{Workspace: ws, Texts: texts, Lang: "en", Version: "0.3.0-test"})
	serverEnd, clientEnd := sdk.NewInMemoryTransports()
	if _, err := s.server.Connect(ctx, serverEnd, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "Claude", Version: "1.0"}, nil).Connect(ctx, clientEnd, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return s, cs, db
}

// A scan started by an assistant is the assistant's run, with its client's name, and answers the
// state of the sites it scanned (ADR-0043, points 4 and 7).
func TestAScanIsTheAssistantsRun(t *testing.T) {
	_, cs, db := scanBench(t)
	before := time.Now().Add(-time.Minute)
	raw, out, refused := answer(t, cs, "scan", map[string]any{"path": "Clients/Bench"})
	if refused {
		t.Fatalf("refused: %s", raw)
	}
	sites, _ := out["sites"].([]any)
	if len(sites) != 1 {
		t.Fatalf("sites = %v, want the bench's one", out["sites"])
	}
	scanned, _ := sites[0].(map[string]any)
	if scanned["path"] != "Clients/Bench/outdated.example.test" || scanned["outcome"] != "completed" || scanned["status"] == "" {
		t.Errorf("site = %v", scanned)
	}
	runs, err := db.Runs(context.Background(), before)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
	if runs[0].By != model.RunByAssistant || runs[0].ByClient != "Claude" || runs[0].Path != "Clients/Bench" || runs[0].Outcome != model.RunCompleted {
		t.Errorf("run = %+v, want the assistant Claude's, completed", runs[0])
	}
}

// A path Vigie does not keep is refused, and nothing is scanned.
func TestAScanOfAnUnknownPath(t *testing.T) {
	_, cs, db := scanBench(t)
	raw, _, refused := answer(t, cs, "scan", map[string]any{"path": "Clients/Nowhere"})
	if !refused || !strings.Contains(raw, "Clients/Nowhere") {
		t.Errorf("refused %v: %s", refused, raw)
	}
	if runs, _ := db.Runs(context.Background(), time.Now().Add(-time.Hour)); len(runs) != 0 {
		t.Errorf("runs = %+v, want none", runs)
	}
}

// One scan at a time from an assistant: a second call while one runs is refused, as the window
// refuses a scan while one runs.
func TestOneScanAtATime(t *testing.T) {
	s, cs, _ := scanBench(t)
	s.scanning.Lock()
	defer s.scanning.Unlock()
	raw, _, refused := answer(t, cs, "scan", map[string]any{"path": "Clients/Bench"})
	if !refused || !strings.Contains(raw, "already") {
		t.Errorf("refused %v: %s", refused, raw)
	}
}
