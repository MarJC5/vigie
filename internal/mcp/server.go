// Package mcp is Vigie's local MCP server, started by an assistant's client as vigie mcp, over stdio
// (ADR-0043). It calls the workspace as the command does. What an assistant may do is the level set
// in Settings, read at every call: off lists no tool, read lists the read tools, act adds the
// actions, each confirmed.
package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
)

// The revisions answered: the current one, and the one before for the clients that have not moved
// yet (ADR-0043, point 1).
var revisions = []string{"2026-07-28", "2025-11-25"}

// Config is what the server needs from outside.
type Config struct {
	// Access reads what an assistant may do; called at every call.
	Access func(context.Context) (model.AssistantAccess, error)
	// Texts and Lang say the server's refusals in the user's language.
	Texts   *i18n.Catalogs
	Lang    string
	Version string
}

// Server is Vigie's MCP server: the SDK's, its tools each with the level they need.
type Server struct {
	server *sdk.Server
	cfg    Config
	levels map[string]model.AssistantAccess
}

// New makes the server with the tools of this version of Vigie.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, levels: map[string]model.AssistantAccess{}}
	s.server = sdk.NewServer(&sdk.Implementation{Name: "vigie", Title: "Vigie", Version: cfg.Version}, &sdk.ServerOptions{
		Instructions:              cfg.Texts.T(cfg.Lang, "mcp.instructions", nil),
		Capabilities:              &sdk.ServerCapabilities{Tools: &sdk.ToolCapabilities{}},
		SupportedProtocolVersions: revisions,
	})
	s.server.AddReceivingMiddleware(s.gate)
	return s
}

// Run serves one client over a transport until it closes.
func (s *Server) Run(ctx context.Context, t sdk.Transport) error {
	return s.server.Run(ctx, t)
}

// add adds a tool the level needs.
func (s *Server) add(level model.AssistantAccess, tool *sdk.Tool, handler sdk.ToolHandler) {
	s.levels[tool.Name] = level
	s.server.AddTool(tool, handler)
}

// gate reads the level at every call: the tools listed are those it allows, and a call to another
// is refused with where to change it. The list is never cached by the client (no time to live), as
// the level may change between two calls.
func (s *Server) gate(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		switch method {
		case "tools/list":
			access, err := s.cfg.Access(ctx)
			if err != nil {
				return nil, err
			}
			res, err := next(ctx, method, req)
			if list, ok := res.(*sdk.ListToolsResult); ok && err == nil {
				allowed := make([]*sdk.Tool, 0, len(list.Tools))
				for _, tool := range list.Tools {
					if access.Allows(s.levels[tool.Name]) {
						allowed = append(allowed, tool)
					}
				}
				list.Tools = allowed
			}
			return res, err
		case "tools/call":
			access, err := s.cfg.Access(ctx)
			if err != nil {
				return nil, err
			}
			call, ok := req.(*sdk.CallToolRequest)
			if !ok || call.Params == nil {
				return next(ctx, method, req)
			}
			if access == model.AssistantOff {
				return s.refusal("mcp.off"), nil
			}
			if level, known := s.levels[call.Params.Name]; known && !access.Allows(level) {
				return s.refusal("mcp.needsAct"), nil
			}
		}
		return next(ctx, method, req)
	}
}

// refusal is a tool's error the assistant reads and can tell the user.
func (s *Server) refusal(key string) *sdk.CallToolResult {
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: s.cfg.Texts.T(s.cfg.Lang, key, nil)}}}
}
