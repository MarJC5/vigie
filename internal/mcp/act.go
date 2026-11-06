package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
)

// The confirmation an action tool takes, described the same way everywhere.
const confirmationDoc = "leave empty first: the tool answers a preview and a confirmation; once the user agrees, call again with the same arguments and that confirmation (one use, five minutes)"

// ActionOut is the answer of an action: a preview waiting for the user's yes, or what was done.
type ActionOut struct {
	Note         string      `json:"note"`
	Done         bool        `json:"done" jsonschema:"true once the action ran; false for a preview"`
	Declined     bool        `json:"declined,omitempty" jsonschema:"the user said no: nothing was done"`
	Preview      []string    `json:"preview,omitempty" jsonschema:"what the action will do, to show the user as it is"`
	Confirmation string      `json:"confirmation,omitempty" jsonschema:"to send back once the user agrees, with the same arguments"`
	Expires      string      `json:"expires,omitempty"`
	Results      []ResultOut `json:"results,omitempty"`
}

// ResultOut is how an action ended on one site.
type ResultOut struct {
	Path    string `json:"path"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

// UpdateIn names what to update.
type UpdateIn struct {
	Path         string `json:"path" jsonschema:"a site, or a host or folder for all its sites, by its path"`
	Confirmation string `json:"confirmation,omitempty" jsonschema:"leave empty first: the tool answers a preview and a confirmation; once the user agrees, call again with the same arguments and that confirmation (one use, five minutes)"`
}

// CopyIn names what to copy to this computer.
type CopyIn struct {
	Path         string `json:"path" jsonschema:"a site, or a host or folder for all its sites, by its path"`
	Content      string `json:"content,omitempty" jsonschema:"database, content (the database and wp-content, the default) or site (the whole site folder)"`
	Confirmation string `json:"confirmation,omitempty" jsonschema:"leave empty first: the tool answers a preview and a confirmation; once the user agrees, call again with the same arguments and that confirmation (one use, five minutes)"`
}

// IgnoreIn names what to ignore and for how long.
type IgnoreIn struct {
	Path         string `json:"path" jsonschema:"the site by its path"`
	Key          string `json:"key" jsonschema:"the finding's or update's key, as the tool site gives it"`
	Permanently  bool   `json:"permanently,omitempty" jsonschema:"for good, which needs a reason; else until the next version"`
	Reason       string `json:"reason,omitempty" jsonschema:"why, in the user's words"`
	Confirmation string `json:"confirmation,omitempty" jsonschema:"leave empty first: the tool answers a preview and a confirmation; once the user agrees, call again with the same arguments and that confirmation (one use, five minutes)"`
}

// KeyIn names an ignore to remove.
type KeyIn struct {
	Path         string `json:"path" jsonschema:"the site by its path"`
	Key          string `json:"key" jsonschema:"the ignored finding's or update's key"`
	Confirmation string `json:"confirmation,omitempty" jsonschema:"leave empty first: the tool answers a preview and a confirmation; once the user agrees, call again with the same arguments and that confirmation (one use, five minutes)"`
}

// action is what an action tool does: its preview, the lines the user is shown, and its run of
// what that preview found.
type action struct {
	tool    string
	args    any // the call's arguments, the confirmation left out
	preview func(context.Context) ([]string, error)
	run     func(context.Context) ([]ResultOut, error)
}

var (
	errActing       = errors.New("mcp.acting")
	errNoToken      = errors.New("mcp.badToken")
	errAskInstead   = errors.New("mcp.askInstead")
	errBadState     = errors.New("mcp.badState")
	errUnknownKey   = errors.New("mcp.unknownKey")
	errNotIgnored   = errors.New("mcp.notIgnored")
	errReasonNeeded = errors.New("mcp.reasonNeeded")
	errNoContent    = errors.New("mcp.unknownContent")
)

// act asks the user's yes, then runs the action (ADR-0043, point 5). A client that declares it can
// ask the user is asked through it (MRTR), and a confirmation the assistant sends is then refused;
// otherwise the first call answers a preview and a confirmation, and only a second call with it
// runs. In both, what runs is what the user was shown: a preview that changed asks again.
func (s *Server) act(ctx context.Context, req *sdk.CallToolRequest, a action, confirmation string) (*sdk.CallToolResult, ActionOut, error) {
	out := ActionOut{Note: dataNote}
	if !s.acting.TryLock() {
		return nil, out, s.said(errActing, "")
	}
	defer s.acting.Unlock()
	raw, err := json.Marshal(a.args)
	if err != nil {
		return nil, out, err
	}
	args := string(raw)
	lines, err := a.preview(ctx)
	if err != nil {
		return nil, out, err
	}
	digest, now := digestOf(lines), s.cfg.Now()
	params := req.Params
	switch {
	case canAsk(req) && params.RequestState != "":
		token, ok := s.confirm.verify(params.RequestState)
		if !ok {
			return nil, out, s.said(errBadState, "")
		}
		shown, ok := s.confirm.take(now, token, a.tool, args)
		if !ok {
			return nil, out, s.said(errNoToken, "")
		}
		answer, _ := params.InputResponses["confirm"].(*sdk.ElicitResult)
		if answer == nil || answer.Action != "accept" {
			out.Declined, out.Preview = true, lines
			return nil, out, nil
		}
		if shown != digest {
			return s.ask(now, a, args, lines, digest), out, nil
		}
		return s.done(ctx, a, out)
	case canAsk(req):
		if confirmation != "" {
			return nil, out, s.said(errAskInstead, "")
		}
		return s.ask(now, a, args, lines, digest), out, nil
	case confirmation == "":
		token, expires := s.confirm.issue(now, a.tool, args, digest)
		out.Preview, out.Confirmation, out.Expires = lines, token, rfc3339(expires)
		return nil, out, nil
	}
	shown, ok := s.confirm.take(now, confirmation, a.tool, args)
	if !ok {
		return nil, out, s.said(errNoToken, "")
	}
	if shown != digest {
		token, expires := s.confirm.issue(now, a.tool, args, digest)
		out.Preview, out.Confirmation, out.Expires = lines, token, rfc3339(expires)
		return nil, out, nil
	}
	return s.done(ctx, a, out)
}

// ask is Vigie's question to the user, carried by the client, with what the action will do.
func (s *Server) ask(now time.Time, a action, args string, lines []string, digest string) *sdk.CallToolResult {
	token, _ := s.confirm.issue(now, a.tool, args, digest)
	message := s.cfg.Texts.T(s.cfg.Lang, "mcp.ask", map[string]any{"what": strings.Join(lines, "\n")})
	return &sdk.CallToolResult{
		InputRequests: sdk.InputRequestMap{"confirm": &sdk.ElicitParams{Message: message,
			RequestedSchema: map[string]any{"type": "object", "properties": map[string]any{}}}},
		RequestState: s.confirm.sign(token),
	}
}

func (s *Server) done(ctx context.Context, a action, out ActionOut) (*sdk.CallToolResult, ActionOut, error) {
	results, err := a.run(ctx)
	if err != nil {
		return nil, out, err
	}
	out.Done, out.Results = true, results
	return nil, out, nil
}

// canAsk tells whether the client declared that it can ask the user a question in a form.
func canAsk(req *sdk.CallToolRequest) bool {
	caps := req.ClientCapabilities()
	return caps != nil && caps.Elicitation != nil && (caps.Elicitation.Form != nil || caps.Elicitation.URL == nil)
}

// --- update ---

func (s *Server) update(ctx context.Context, req *sdk.CallToolRequest, in UpdateIn) (*sdk.CallToolResult, ActionOut, error) {
	var planned []workspace.Planned
	a := action{tool: "update", args: UpdateIn{Path: in.Path}}
	a.preview = func(ctx context.Context) ([]string, error) {
		node, err := s.node(ctx, in.Path)
		if err != nil {
			return nil, err
		}
		previews, err := s.cfg.Workspace.PreviewNodes(ctx, []model.Node{node})
		if err != nil {
			return nil, err
		}
		var not []workspace.SiteUpdate
		planned, not = workspace.PlanBulk(previews)
		var lines []string
		for _, p := range planned {
			for _, it := range s.planItems(p) {
				lines = append(lines, bounded(p.Path+": "+it))
			}
		}
		for _, r := range not {
			lines = append(lines, bounded(r.Path+": "+s.outcomeText(r)))
		}
		if len(planned) > 0 {
			lines = append(lines, s.cfg.Texts.T(s.cfg.Lang, "mcp.updateSafety", nil))
		}
		return lines, nil
	}
	a.run = func(ctx context.Context) ([]ResultOut, error) {
		for i := range planned {
			planned[i].Job.By, planned[i].Job.ByClient = model.RunByAssistant, clientName(req)
		}
		var results []ResultOut
		for _, r := range s.cfg.Workspace.UpdateSites(ctx, planned, workspace.Follow{}) {
			result := ResultOut{Path: bounded(r.Path), Outcome: string(r.Outcome)}
			if r.Err != nil {
				result.Detail = bounded(r.Err.Error())
			}
			for _, rb := range r.Rollbacks {
				result.Detail = bounded(strings.TrimSpace(result.Detail + " " + s.cfg.Texts.T(s.cfg.Lang, "mcp.rolledBack",
					map[string]any{"outcome": string(rb.Outcome)})))
			}
			results = append(results, result)
		}
		return results, nil
	}
	return s.act(ctx, req, a, in.Confirmation)
}

// planItems are the lines of what a site's update will change.
func (s *Server) planItems(p workspace.Planned) []string {
	var lines []string
	line := func(it update.Item, name string) string { return fmt.Sprintf("%s %s → %s", name, it.From, it.To) }
	if p.Selection.Core && p.Preview.Core != nil {
		lines = append(lines, line(*p.Preview.Core, "WordPress"))
	}
	for _, it := range p.Preview.Plugins {
		if slices.Contains(p.Selection.Plugins, it.Slug) {
			lines = append(lines, line(it, it.Slug))
		}
	}
	for _, it := range p.Preview.Themes {
		if slices.Contains(p.Selection.Themes, it.Slug) {
			lines = append(lines, line(it, it.Slug))
		}
	}
	if p.Selection.Languages && len(p.Preview.Languages) > 0 {
		lines = append(lines, s.cfg.Texts.T(s.cfg.Lang, "mcp.translations", map[string]any{"count": len(p.Preview.Languages)}))
	}
	return lines
}

// outcomeText says why a site is not updated.
func (s *Server) outcomeText(r workspace.SiteUpdate) string {
	if r.Outcome == workspace.SiteNothing {
		return s.cfg.Texts.T(s.cfg.Lang, "mcp.nothingToUpdate", nil)
	}
	detail := ""
	if r.Err != nil {
		detail = r.Err.Error()
	}
	return s.cfg.Texts.T(s.cfg.Lang, "mcp.notUpdated", map[string]any{"detail": detail})
}

// --- copy ---

func (s *Server) copy(ctx context.Context, req *sdk.CallToolRequest, in CopyIn) (*sdk.CallToolResult, ActionOut, error) {
	content := sitecopy.Content(in.Content)
	if content == "" {
		content = sitecopy.WithContent
	}
	if content != sitecopy.DatabaseOnly && content != sitecopy.WithContent && content != sitecopy.WholeSite {
		return nil, ActionOut{Note: dataNote}, s.said(errNoContent, in.Content)
	}
	var node model.Node
	a := action{tool: "copy", args: CopyIn{Path: in.Path, Content: string(content)}}
	a.preview = func(ctx context.Context) ([]string, error) {
		tree, err := s.cfg.Workspace.DB().LoadTree(ctx)
		if err != nil {
			return nil, err
		}
		if node, err = tree.Resolve(in.Path); err != nil {
			return nil, s.said(errUnknownPath, in.Path)
		}
		what := s.cfg.Texts.T(s.cfg.Lang, "mcp.content."+string(content), nil)
		var lines []string
		for _, site := range tree.SitesUnder(node) {
			lines = append(lines, bounded(tree.PathOf(model.Node{Kind: model.KindSite, ID: site.ID})+": "+what))
		}
		return append(lines, s.cfg.Texts.T(s.cfg.Lang, "mcp.copyWhere", nil)), nil
	}
	a.run = func(ctx context.Context) ([]ResultOut, error) {
		copies, err := s.cfg.Workspace.CopySites(ctx, []model.Node{node}, content,
			workspace.CopyFollow{By: model.RunByAssistant, ByClient: clientName(req)})
		if err != nil {
			return nil, err
		}
		var results []ResultOut
		for _, c := range copies {
			result := ResultOut{Path: bounded(c.Path), Outcome: string(c.Copy.Outcome)}
			if c.Err != nil {
				result.Outcome, result.Detail = string(model.CopyFailed), bounded(c.Err.Error())
			}
			results = append(results, result)
		}
		return results, nil
	}
	return s.act(ctx, req, a, in.Confirmation)
}

// --- ignore, stop ignoring ---

func (s *Server) ignore(ctx context.Context, req *sdk.CallToolRequest, in IgnoreIn) (*sdk.CallToolResult, ActionOut, error) {
	reason := strings.TrimSpace(in.Reason)
	if in.Permanently && reason == "" {
		return nil, ActionOut{Note: dataNote}, s.said(errReasonNeeded, "")
	}
	var ignore model.Ignore
	a := action{tool: "ignore", args: IgnoreIn{Path: in.Path, Key: in.Key, Permanently: in.Permanently, Reason: reason}}
	a.preview = func(ctx context.Context) ([]string, error) {
		site, err := s.siteAt(ctx, in.Path)
		if err != nil {
			return nil, err
		}
		what, version, err := s.ignorable(ctx, site, in.Key)
		if err != nil {
			return nil, err
		}
		ignore = model.Ignore{SiteID: site.ID, Key: in.Key, Reason: reason, Created: s.cfg.Now()}
		key := "mcp.ignoreUntil"
		if in.Permanently {
			key = "mcp.ignoreForGood"
		} else {
			ignore.UntilVersion = version
		}
		return []string{bounded(in.Path + ": " + s.cfg.Texts.T(s.cfg.Lang, key, map[string]any{"what": what, "version": version, "reason": reason}))}, nil
	}
	a.run = func(ctx context.Context) ([]ResultOut, error) {
		if err := s.cfg.Workspace.DB().SaveIgnore(ctx, ignore); err != nil {
			return nil, err
		}
		return []ResultOut{{Path: bounded(in.Path), Outcome: "ignored"}}, nil
	}
	return s.act(ctx, req, a, in.Confirmation)
}

func (s *Server) stopIgnoring(ctx context.Context, req *sdk.CallToolRequest, in KeyIn) (*sdk.CallToolResult, ActionOut, error) {
	var site model.Site
	a := action{tool: "stop_ignoring", args: KeyIn{Path: in.Path, Key: in.Key}}
	a.preview = func(ctx context.Context) ([]string, error) {
		var err error
		if site, err = s.siteAt(ctx, in.Path); err != nil {
			return nil, err
		}
		ignores, err := s.cfg.Workspace.DB().Ignores(ctx, site.ID)
		if err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(ignores, func(i model.Ignore) bool { return i.Key == in.Key }) {
			return nil, s.said(errNotIgnored, in.Path)
		}
		return []string{bounded(in.Path + ": " + s.cfg.Texts.T(s.cfg.Lang, "mcp.stopIgnoring", map[string]any{"what": in.Key}))}, nil
	}
	a.run = func(ctx context.Context) ([]ResultOut, error) {
		if err := s.cfg.Workspace.DB().DeleteIgnore(ctx, site.ID, in.Key); err != nil {
			return nil, err
		}
		return []ResultOut{{Path: bounded(in.Path), Outcome: "counts again"}}, nil
	}
	return s.act(ctx, req, a, in.Confirmation)
}

// ignorable finds a finding or an update offered of the site's latest scan by its key: how to
// name it, and the version an ignore until the next one holds for (the finding's installed one,
// the update's offered one).
func (s *Server) ignorable(ctx context.Context, site model.Site, key string) (string, string, error) {
	view, err := s.siteManager.SiteDetail(ctx, site.ID.String())
	if err != nil {
		return "", "", err
	}
	if view.Scan != nil {
		for _, f := range view.Scan.Findings {
			if f.Key == key {
				return f.Kind + " " + f.Subject, f.Version, nil
			}
		}
		for _, u := range view.Scan.Updates {
			if u.Key == key && u.Key != "" {
				return fmt.Sprintf("%s %s → %s", u.Slug, u.Version, u.UpdateVersion), u.UpdateVersion, nil
			}
		}
	}
	return "", "", s.said(errUnknownKey, site.Name)
}

// actTools adds the tools of the level act (ADR-0043, point 4).
func (s *Server) actTools() {
	destructive := true
	add(s, model.AssistantAct, &sdk.Tool{Name: "update", Annotations: &sdk.ToolAnnotations{DestructiveHint: &destructive},
		Description: "Update a site, or all the sites of a host or folder, as Vigie's Update all does: a backup of the database first, checks after each item, a plugin or theme that breaks the site rolled back on its own. Waits for the user's yes."}, s.update)
	add(s, model.AssistantAct, &sdk.Tool{Name: "copy", Annotations: &sdk.ToolAnnotations{DestructiveHint: new(bool)},
		Description: "Copy sites to this computer, read only on the servers. Waits for the user's yes."}, s.copy)
	add(s, model.AssistantAct, &sdk.Tool{Name: "ignore", Annotations: &sdk.ToolAnnotations{DestructiveHint: new(bool)},
		Description: "Ignore a finding or an update of a site, until its next version or for good with a reason. Waits for the user's yes."}, s.ignore)
	add(s, model.AssistantAct, &sdk.Tool{Name: "stop_ignoring", Annotations: &sdk.ToolAnnotations{DestructiveHint: new(bool)},
		Description: "Make an ignored finding or update of a site count again. Waits for the user's yes."}, s.stopIgnoring)
}
