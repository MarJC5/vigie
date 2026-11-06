package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/workspace"
)

// The level of the assistants is off at first, set in Settings and read back; a level Vigie does
// not know is refused (ADR-0043, point 3).
func TestTheLevelOfTheAssistants(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New(repository(t), workspace.Deps{Runner: sshxtest.NewFake(), Now: time.Now})
	a := app.NewAssistants(ws, func() app.CommandView { return app.CommandView{} })
	if v, err := a.View(ctx); err != nil || v.Access != "off" {
		t.Fatalf("View() = %+v, %v; want off", v, err)
	}
	for _, level := range []string{"read", "act", "off"} {
		if p := a.SetAccess(ctx, level); p != nil {
			t.Fatalf("SetAccess(%q) = %+v", level, p)
		}
		if v, _ := a.View(ctx); v.Access != level {
			t.Errorf("after SetAccess(%q): %q", level, v.Access)
		}
	}
	if p := a.SetAccess(ctx, "everything"); p == nil {
		t.Error("SetAccess(everything) accepted")
	}
}

// The lines a client adds start the linked vigie command with mcp; none while it is not linked,
// the settings then saying to link it first.
func TestTheLinesOfAClient(t *testing.T) {
	ctx := context.Background()
	ws := workspace.New(repository(t), workspace.Deps{Runner: sshxtest.NewFake(), Now: time.Now})
	command := app.CommandView{Path: "/usr/local/bin/vigie", Command: "sudo ln -sf '/Applications/Vigie.app/Contents/MacOS/Vigie' '/usr/local/bin/vigie'"}
	a := app.NewAssistants(ws, func() app.CommandView { return command })
	if v, _ := a.View(ctx); v.Config != "" {
		t.Fatalf("not linked: config %q, want none", v.Config)
	}
	command = app.CommandView{Linked: true, Path: "/opt/homebrew/bin/vigie"}
	v, err := a.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(v.Config), &config); err != nil {
		t.Fatalf("config %q: %v", v.Config, err)
	}
	server := config.MCPServers["vigie"]
	if server.Command != "/opt/homebrew/bin/vigie" || len(server.Args) != 1 || server.Args[0] != "mcp" {
		t.Errorf("config = %s", v.Config)
	}
}
