package app

import (
	"context"
	"encoding/json"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// AssistantsView is the tab Assistants of the settings (ADR-0043): the level, and the lines a
// client adds to its MCP servers.
type AssistantsView struct {
	Access string `json:"access"` // off, read or act
	// Config starts the linked vigie command with mcp, as a client's configuration; empty while the
	// command is not linked, a client having nothing to start then.
	Config string `json:"config"`
}

// Assistants serves the tab Assistants.
type Assistants struct {
	ws      *workspace.Workspace
	command func() CommandView
}

// NewAssistants returns the service; command reads the vigie command's state, as the settings do.
func NewAssistants(ws *workspace.Workspace, command func() CommandView) *Assistants {
	return &Assistants{ws: ws, command: command}
}

// View reads the level and the lines for a client, each time it is asked.
func (a *Assistants) View(ctx context.Context) (AssistantsView, error) {
	access, err := a.ws.AssistantAccess(ctx)
	if err != nil {
		return AssistantsView{}, err
	}
	v := AssistantsView{Access: string(access)}
	if c := a.command(); c.Linked {
		v.Config = clientConfig(c.Path)
	}
	return v, nil
}

// SetAccess sets what an assistant may do; vigie mcp reads it at its next call.
func (a *Assistants) SetAccess(ctx context.Context, access string) *Problem {
	return ProblemOf(a.ws.SetAssistantAccess(ctx, model.AssistantAccess(access)))
}

// clientConfig is the entry of Vigie in a client's MCP servers, as most clients read it, laid out
// as the clients' documentation shows it.
func clientConfig(path string) string {
	quoted, _ := json.Marshal(path)
	return "{\n  \"mcpServers\": {\n    \"vigie\": {\n      \"command\": " + string(quoted) + ",\n      \"args\": [\"mcp\"]\n    }\n  }\n}"
}
