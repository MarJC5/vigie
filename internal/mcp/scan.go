package mcp

import (
	"context"
	"errors"
	"slices"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// ScanIn chooses what to scan.
type ScanIn struct {
	Path string `json:"path,omitempty" jsonschema:"a folder, host or site by its path; every site when empty and no tag is given"`
	Tag  string `json:"tag,omitempty" jsonschema:"the sites with this tag, wherever they are, in place of a path"`
}

// ScanResultOut is the state of the sites a scan read.
type ScanResultOut struct {
	Note  string       `json:"note"`
	Sites []ScannedOut `json:"sites"`
}

// ScannedOut is a site as its scan left it.
type ScannedOut struct {
	Path      string      `json:"path"`
	Outcome   string      `json:"outcome" jsonschema:"completed, cannot-load, failed or cancelled"`
	Problem   string      `json:"problem,omitempty" jsonschema:"why the scan did not complete"`
	Status    string      `json:"status,omitempty" jsonschema:"ok, warn, danger, partial or unknown"`
	WordPress string      `json:"wordpress,omitempty"`
	Updates   int         `json:"updates,omitempty"`
	Worst     *FindingOut `json:"worst,omitempty"`
	Changes   int         `json:"changes,omitempty" jsonschema:"what changed since the site's scan before; the tool site tells them"`
}

var errScanRunning = errors.New("mcp.scanRunning")

// scan scans as the command does, as the assistant's run with its client's name; one at a time
// from this server. It only reads on the servers (ADR-0043, point 4).
func (s *Server) scan(ctx context.Context, req *sdk.CallToolRequest, in ScanIn) (*sdk.CallToolResult, ScanResultOut, error) {
	out := ScanResultOut{Note: dataNote, Sites: []ScannedOut{}}
	if !s.scanning.TryLock() {
		return nil, out, s.said(errScanRunning, "")
	}
	defer s.scanning.Unlock()
	ws := s.cfg.Workspace
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		return nil, out, err
	}
	var nodes []model.Node
	switch {
	case in.Tag != "":
		for _, site := range tree.SitesUnder(model.Root) {
			if slices.ContainsFunc(site.Tags, func(t string) bool { return strings.EqualFold(t, in.Tag) }) {
				nodes = append(nodes, model.Node{Kind: model.KindSite, ID: site.ID})
			}
		}
		if len(nodes) == 0 {
			return nil, out, s.said(errNoSiteTagged, in.Tag)
		}
	case in.Path != "":
		n, err := tree.Resolve(in.Path)
		if err != nil {
			return nil, out, s.said(errUnknownPath, in.Path)
		}
		nodes = []model.Node{n}
	default:
		nodes = []model.Node{model.Root}
	}
	results, err := ws.ScanNodes(ctx, nodes, workspace.ScanOptions{By: model.RunByAssistant, ByClient: clientName(req)}, nil)
	if err != nil {
		return nil, out, err
	}
	// The history kept in check after the run, as after vigie scan (ADR-0031).
	_, _ = ws.Prune(context.WithoutCancel(ctx))
	rows, err := s.siteManager.Sites(ctx, "")
	if err != nil {
		return nil, out, err
	}
	byPath := map[string]app.SiteRow{}
	for _, r := range rows {
		byPath[r.Path] = r
	}
	for _, r := range results {
		site := ScannedOut{Path: bounded(r.Path), Outcome: string(r.Scan.Outcome), Changes: len(r.Changes)}
		if r.Scan.Outcome != model.ScanCompleted {
			site.Problem = bounded(strings.TrimSpace(r.Scan.Failure + " " + r.Scan.Error))
		}
		if row, ok := byPath[r.Path]; ok {
			site.Status, site.WordPress, site.Updates = row.Status, bounded(row.WordPress), row.Pending
			if row.Worst != nil {
				w := findingOut(*row.Worst)
				site.Worst = &w
			}
		}
		out.Sites = append(out.Sites, site)
	}
	return nil, out, nil
}

// clientName is the name the assistant's client gives, "" when it gives none.
func clientName(req *sdk.CallToolRequest) string {
	if req == nil {
		return ""
	}
	if info := req.ClientInfo(); info != nil {
		return bounded(info.Name)
	}
	return ""
}
