package cli

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/workspace"
)

// copyTarget copies the sites of a path or a tag to this computer (ADR-0041, point 8): read-only
// on the servers, so nothing is asked first. A line per site being copied with its archive
// (ADR-0037); each site's outcome printed above the block as it ends.
func copyTarget(ctx context.Context, env Env, ws *workspace.Workspace, on target, content string, asJSON bool) int {
	switch sitecopy.Content(content) {
	case sitecopy.DatabaseOnly, sitecopy.WithContent, sitecopy.WholeSite:
	default:
		return env.fail(env.t("cli.copy.badContent", map[string]any{"content": content}))
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	node, err := on.node(tree)
	if err != nil {
		return env.fail(on.explain(env, err))
	}
	total := len(tree.SitesUnder(node))
	if total == 0 {
		return env.fail(env.t("cli.noSites", nil))
	}
	p := startProgress(env, asJSON, env.t("cli.progress.copy", map[string]any{"done": 0, "count": total, "running": 0}))
	var mu sync.Mutex
	steps := map[model.ID]string{}
	names := map[model.ID]string{}
	var running []model.ID
	ended := 0
	show := func() {
		var items []item
		for _, id := range running {
			items = append(items, item{text: names[id] + " · " + steps[id]})
		}
		p.set(env.t("cli.progress.copy", map[string]any{"done": ended, "count": total, "running": len(running)}), items...)
	}
	results, err := ws.CopySites(ctx, []model.Node{node}, sitecopy.Content(content), workspace.CopyFollow{
		By: model.RunByCLI,
		OnProgress: func(s model.Site, pr sitecopy.Progress) {
			mu.Lock()
			defer mu.Unlock()
			if !slices.Contains(running, s.ID) {
				running = append(running, s.ID)
			}
			names[s.ID] = s.Name
			steps[s.ID] = env.t("cli.copy.step", map[string]any{"archive": pr.Archive, "done": pr.Done, "count": pr.Total})
			show()
		},
		OnSite: func(r workspace.SiteCopy) {
			mu.Lock()
			running = slices.DeleteFunc(running, func(id model.ID) bool { return id == r.Site.ID })
			ended++
			show()
			mu.Unlock()
			if !asJSON {
				p.above(func() { fmt.Fprintf(env.Out, "%s  %s\n", r.Path, copyOutcome(env, r)) })
			}
		},
	})
	p.end()
	if err != nil {
		return env.fail(err.Error())
	}
	code := exitOK
	out := make([]copyJSON, 0, len(results))
	for _, r := range results {
		j := copyJSON{Path: r.Path, Folder: r.Copy.Folder, Archives: r.Copy.Archives, Bytes: r.Copy.Bytes, Outcome: string(r.Copy.Outcome)}
		if r.Err != nil {
			code = exitError
			j.Error = r.Err.Error()
			if j.Outcome == "" {
				j.Outcome = string(model.CopyFailed)
			}
		}
		out = append(out, j)
	}
	if asJSON {
		writeJSON(env.Out, out)
	}
	return code
}

type copyJSON struct {
	Path     string `json:"path"`
	Folder   string `json:"folder,omitempty"`
	Archives int    `json:"archives"`
	Bytes    int64  `json:"bytes"`
	Outcome  string `json:"outcome"`
	Error    string `json:"error,omitempty"`
}

// copyOutcome is a site's copy as it ended: where it went, or why it did not.
func copyOutcome(env Env, r workspace.SiteCopy) string {
	if r.Err != nil {
		return env.t("cli.copy.failed", map[string]any{"message": r.Err.Error()})
	}
	return env.t("cli.copy.done", map[string]any{
		"archives": env.t("cli.copy.archives", map[string]any{"count": r.Copy.Archives}),
		"size":     sizeText(r.Copy.Bytes, env.Lang),
		"folder":   r.Copy.Folder,
	})
}

// sizeText is a size in megabytes, or gigabytes from one, a decimal comma in French.
func sizeText(bytes int64, lang string) string {
	value, unit, digits := float64(bytes)/1e6, "MB", 1
	if bytes >= 1e9 {
		value, unit, digits = float64(bytes)/1e9, "GB", 2
	}
	s := strconv.FormatFloat(value, 'f', digits, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if lang == "fr" {
		s = strings.Replace(s, ".", ",", 1)
		unit = map[string]string{"MB": "Mo", "GB": "Go"}[unit]
	}
	return s + " " + unit
}
