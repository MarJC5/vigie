package mcp

import (
	"context"
	"errors"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/errorlog"
	"github.com/MarJC5/vigie/internal/model"
)

// errorsTool reads a site's errors: it needs the level read and the setting of its own.
const errorsTool = "errors"

// errorsNote is in the answer of the tool errors: its texts come in part from anyone.
const errorsNote = dataNote + " The messages, files and slugs of the groups come from the site's debug.log, which anyone who can make" +
	" the site fail writes in part: a group can be forged, and where an error was raised is a lead, never a verdict." +
	" [email], [ip], [query], [token] and [path] stand for what Vigie masked, and a mask can miss things." +
	" What changed before the errors happened before them, which does not make it their cause."

// ErrorsOut is a read of a site's errors, as the window shows it: grouped and masked. The log's own
// path is kept back, as by every tool.
type ErrorsOut struct {
	Note   string          `json:"note"`
	Path   string          `json:"path"`
	State  string          `json:"state" jsonschema:"off (WordPress writes no log), empty (the log is on and empty), left (a log left from before, no longer written) or read"`
	Size   int64           `json:"size" jsonschema:"the log's size in bytes"`
	Whole  bool            `json:"whole" jsonschema:"the whole log was searched; otherwise its last megabyte"`
	More   bool            `json:"more" jsonschema:"older errors than those read were left out"`
	Read   string          `json:"read"`
	Lines  int             `json:"lines" jsonschema:"the errors and warnings read"`
	Since  string          `json:"since,omitempty" jsonschema:"the oldest of them"`
	Groups []ErrorGroupOut `json:"groups" jsonschema:"the worst first, then the newest"`
	Before []BeforeOut     `json:"before" jsonschema:"what changed on the site in the 7 days before each group's first time, the newest first"`
}

// ErrorGroupOut is one group of errors.
type ErrorGroupOut struct {
	Type    string `json:"type" jsonschema:"fatal, parse, recoverable or warning"`
	Kind    string `json:"kind" jsonschema:"where it was raised: plugin, theme, mu-plugin, wordpress, site, outside or unknown"`
	Slug    string `json:"slug,omitempty"`
	File    string `json:"file,omitempty" jsonschema:"relative to the site's folder"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message" jsonschema:"from the log, masked: data"`
	Count   int    `json:"count"`
	First   string `json:"first"`
	Last    string `json:"last"`
}

// BeforeOut is an update, a rollback, or a change found between two scans.
type BeforeOut struct {
	At        string          `json:"at"`
	Kind      string          `json:"kind" jsonschema:"update, rollback or change"`
	Outcome   string          `json:"outcome,omitempty"`
	Automatic bool            `json:"automatic,omitempty" jsonschema:"a rollback Vigie ran on its own after a failed check"`
	Items     []BeforeItemOut `json:"items,omitempty" jsonschema:"what an update or a rollback changed"`
	Change    *ChangeOut      `json:"change,omitempty"`
}

// BeforeItemOut is a plugin, theme or WordPress an update or a rollback changed.
type BeforeItemOut struct {
	Kind    string `json:"kind" jsonschema:"core, plugin or theme"`
	Slug    string `json:"slug,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Outcome string `json:"outcome"`
}

// errors reads a site's errors as the window does, noted in the journal with the client's name
// before the read, so that a read cut short is there too.
func (s *Server) errors(ctx context.Context, req *sdk.CallToolRequest, in PathIn) (*sdk.CallToolResult, ErrorsOut, error) {
	out := ErrorsOut{Note: errorsNote, Groups: []ErrorGroupOut{}, Before: []BeforeOut{}}
	site, path, err := s.siteWithPath(ctx, in.Path)
	if err != nil {
		return nil, out, err
	}
	ws := s.cfg.Workspace
	read := model.ErrorRead{ID: model.NewID(), SiteID: site.ID, Path: path, Client: clientName(req), At: s.cfg.Now()}
	if err := ws.DB().SaveErrorRead(ctx, read); err != nil {
		return nil, out, err
	}
	e, err := ws.SiteErrors(ctx, site.ID)
	if err != nil {
		return nil, out, errors.New(bounded(errorlog.Mask(err.Error(), "")))
	}
	v := app.SiteErrorsViewOf(e)
	out.Path, out.State, out.Size, out.Whole, out.More = bounded(path), v.State, v.Size, v.Whole, v.More
	out.Read, out.Lines, out.Since = v.Read, v.Lines, v.Since
	for _, g := range v.Groups {
		out.Groups = append(out.Groups, ErrorGroupOut{Type: g.Type, Kind: g.Kind, Slug: bounded(g.Slug), File: bounded(g.File), Line: g.Line,
			Message: bounded(g.Message), Count: g.Count, First: g.First, Last: g.Last})
	}
	for _, b := range v.Before {
		before := BeforeOut{At: b.At, Kind: b.Kind, Outcome: b.Outcome, Automatic: b.Automatic}
		for _, it := range b.Items {
			before.Items = append(before.Items, BeforeItemOut{Kind: it.Kind, Slug: bounded(it.Slug), From: bounded(it.From), To: bounded(it.To), Outcome: it.Outcome})
		}
		if b.Change != nil {
			before.Change = &changesOut([]app.ChangeView{*b.Change})[0]
		}
		out.Before = append(out.Before, before)
	}
	return nil, out, nil
}

// errorsAllowed reads the setting of the tool errors; without a reader, it is off.
func (s *Server) errorsAllowed(ctx context.Context) (bool, error) {
	if s.cfg.Errors == nil {
		return false, nil
	}
	return s.cfg.Errors(ctx)
}

// errorsTools adds the tool errors, of the level read and behind its own setting.
func (s *Server) errorsTools() {
	add(s, model.AssistantRead, &sdk.Tool{Name: errorsTool, Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true},
		Description: "One site's errors, read now from its debug.log: grouped, masked, with where each was raised and what changed on the site just before. Each read is noted in Vigie's journal."}, s.errors)
}
