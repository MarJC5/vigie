// Package mcp is Vigie's local MCP server, started by an assistant's client as vigie mcp, over stdio
// (ADR-0043). It calls the workspace as the command does. What an assistant may do is the level set
// in Settings, read at every call: off lists no tool, read lists the read tools, act adds the
// actions, each confirmed. The tool errors needs, on top of the level read, a setting of its own.
package mcp

import (
	"context"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// The revisions answered: the current one, and the one before for the clients that have not moved
// yet (ADR-0043, point 1).
var revisions = []string{"2026-07-28", "2025-11-25"}

// Config is what the server needs from outside.
type Config struct {
	// Workspace is the user's data; without it the server has no tool (the tests of the levels).
	Workspace *workspace.Workspace
	// Access reads what an assistant may do, at every call; the workspace's setting when nil.
	Access func(context.Context) (model.AssistantAccess, error)
	// Errors reads whether an assistant may read a site's errors, at every call; the workspace's
	// setting when nil.
	Errors func(context.Context) (bool, error)
	// Now is the time the journal counts its days from; the clock when nil.
	Now func() time.Time
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

	siteManager *app.SiteManager
	activity    *app.Activity
	copyList    *app.Copies
	// scanning holds while a scan from this server runs: one at a time.
	scanning sync.Mutex
	// acting holds while an action from this server previews or runs: one at a time.
	acting sync.Mutex
	// confirm holds the previews waiting for the user's yes.
	confirm *confirmations
}

// New makes the server with the tools of this version of Vigie.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, levels: map[string]model.AssistantAccess{}, confirm: newConfirmations()}
	s.server = sdk.NewServer(&sdk.Implementation{Name: "vigie", Title: "Vigie", Version: cfg.Version}, &sdk.ServerOptions{
		Instructions:              cfg.Texts.T(cfg.Lang, "mcp.instructions", nil),
		Capabilities:              &sdk.ServerCapabilities{Tools: &sdk.ToolCapabilities{}},
		SupportedProtocolVersions: revisions,
	})
	s.server.AddReceivingMiddleware(s.gate)
	if cfg.Workspace != nil {
		if s.cfg.Access == nil {
			s.cfg.Access = cfg.Workspace.AssistantAccess
		}
		if s.cfg.Errors == nil {
			s.cfg.Errors = cfg.Workspace.AssistantErrors
		}
		if s.cfg.Now == nil {
			s.cfg.Now = time.Now
		}
		db := cfg.Workspace.DB()
		s.siteManager = app.NewSiteManager(db)
		s.activity = app.NewActivity(db, s.cfg.Now)
		s.copyList = app.NewCopies(cfg.Workspace, nil)
		s.readTools()
		s.errorsTools()
		s.actTools()
	}
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
// is refused with where to change it. The tool errors also needs its setting, read at every call as
// well. The list is never cached by the client (no time to live), as both may change between two
// calls.
func (s *Server) gate(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		switch method {
		case "tools/list":
			access, err := s.cfg.Access(ctx)
			if err != nil {
				return nil, err
			}
			errorsOn, err := s.errorsAllowed(ctx)
			if err != nil {
				return nil, err
			}
			res, err := next(ctx, method, req)
			if list, ok := res.(*sdk.ListToolsResult); ok && err == nil {
				allowed := make([]*sdk.Tool, 0, len(list.Tools))
				for _, tool := range list.Tools {
					if access.Allows(s.levels[tool.Name]) && (tool.Name != errorsTool || errorsOn) {
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
			if call.Params.Name == errorsTool {
				on, err := s.errorsAllowed(ctx)
				if err != nil {
					return nil, err
				}
				if !on {
					return s.refusal("mcp.errorsOff"), nil
				}
			}
		}
		return next(ctx, method, req)
	}
}

// refusal is a tool's error the assistant reads and can tell the user.
func (s *Server) refusal(key string) *sdk.CallToolResult {
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: s.cfg.Texts.T(s.cfg.Lang, key, nil)}}}
}
