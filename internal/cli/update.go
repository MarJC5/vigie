package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
)

type updateItemJSON struct {
	Kind     string `json:"kind"`
	Slug     string `json:"slug,omitempty"`
	Language string `json:"language,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

type updateJSON struct {
	Path       string           `json:"path"`
	Outcome    string           `json:"outcome"`
	Error      string           `json:"error,omitempty"`
	Items      []updateItemJSON `json:"items"`
	Operation  string           `json:"operation,omitempty"`
	BackupDir  string           `json:"backupDir,omitempty"`
	FailedStep string           `json:"failedStep,omitempty"`
}

// updateTarget previews the updates of the sites of the target, asks, then updates them.
func updateTarget(ctx context.Context, env Env, ws *workspace.Workspace, on target, asJSON, yes bool) int {
	if asJSON && !yes {
		return env.fail(env.t("cli.update.jsonNeedsYes", nil))
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	node, err := on.node(tree)
	if err != nil {
		return env.fail(on.explain(env, err))
	}
	previews, err := ws.PreviewNode(ctx, node)
	if err != nil {
		return env.fail(on.explain(env, err))
	}

	// The sites that can be updated; the others keep the reason they cannot.
	var planned []workspace.Planned
	results := map[model.ID]workspace.SiteUpdate{}
	for _, p := range previews {
		switch {
		case p.Err != nil:
			results[p.Site.ID] = workspace.SiteUpdate{Site: p.Site, Path: p.Path, Outcome: workspace.SiteRefused, Err: p.Err}
		case p.Preview.Pending() == 0:
			results[p.Site.ID] = workspace.SiteUpdate{Site: p.Site, Path: p.Path, Outcome: workspace.SiteNothing}
		case !p.Preview.DumpTool:
			results[p.Site.ID] = workspace.SiteUpdate{Site: p.Site, Path: p.Path, Outcome: workspace.SiteRefused, Err: wpcli.ErrNoDumpTool}
		default:
			planned = append(planned, workspace.Planned{SitePreview: p, Selection: update.All(p.Preview)})
		}
	}

	if !asJSON {
		for _, p := range previews {
			fmt.Fprintln(env.Out, p.Path)
			if r, ok := results[p.Site.ID]; ok {
				fmt.Fprintln(env.Out, "  "+outcomeText(env, r))
				continue
			}
			for _, line := range itemLines(env, p.Preview) {
				fmt.Fprintln(env.Out, "  "+line)
			}
		}
	}
	if len(planned) == 0 {
		if !asJSON {
			fmt.Fprintln(env.Out, env.t("cli.update.nothing", nil))
		} else {
			writeJSON(env.Out, updatesJSON(previews, results))
		}
		return exitForUpdates(results)
	}
	if !yes && !confirm(env, len(planned)) {
		fmt.Fprintln(env.Out, env.t("cli.update.declined", nil))
		return exitAttention
	}

	done := ws.UpdateSites(ctx, planned, workspace.Follow{OnSite: func(r workspace.SiteUpdate) {
		if !asJSON {
			fmt.Fprintf(env.Out, "%s  %s\n", r.Path, outcomeText(env, r))
		}
	}})
	for _, r := range done {
		if r.Outcome == workspace.SiteNotStarted && !asJSON {
			fmt.Fprintf(env.Out, "%s  %s\n", r.Path, outcomeText(env, r))
		}
		results[r.Site.ID] = r
	}
	if asJSON {
		writeJSON(env.Out, updatesJSON(previews, results))
	}
	return exitForUpdates(results)
}

// confirm asks before writing anything; y, yes, o and oui answer yes.
func confirm(env Env, sites int) bool {
	fmt.Fprint(env.Out, env.t("cli.update.prompt", map[string]any{"count": sites}))
	if env.In == nil {
		return false
	}
	answer, _ := bufio.NewReader(env.In).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes", "o", "oui":
		return true
	}
	return false
}

// itemLines describes what an update changes, one line per item, the translations on one line.
func itemLines(env Env, p update.Preview) []string {
	var lines []string
	if p.Core != nil {
		lines = append(lines, env.t("cli.update.core", map[string]any{"from": p.Core.From, "to": p.Core.To}))
	}
	for _, it := range p.Plugins {
		lines = append(lines, env.t("cli.update.plugin", map[string]any{"slug": it.Slug, "from": it.From, "to": it.To}))
	}
	for _, it := range p.Themes {
		lines = append(lines, env.t("cli.update.theme", map[string]any{"slug": it.Slug, "from": it.From, "to": it.To}))
	}
	if len(p.Languages) > 0 {
		var list []string
		for _, l := range p.Languages {
			of := l.Slug
			if of == "" {
				of = "WordPress"
			}
			list = append(list, l.Language+" ("+of+")")
		}
		lines = append(lines, env.t("cli.update.languages", map[string]any{"list": strings.Join(list, ", ")}))
	}
	return lines
}

// outcomeText says how a site's update ended, or why it did not start.
func outcomeText(env Env, r workspace.SiteUpdate) string {
	switch r.Outcome {
	case workspace.SiteUpdated:
		return env.t("cli.update.updated", nil)
	case workspace.SiteFailed:
		return env.t("cli.update.failed", map[string]any{"message": failedStep(r.Operation).Message}) + " " +
			env.t("cli.update.rollbackHint", nil)
	case workspace.SiteRefused:
		return env.t("cli.update.cannot", map[string]any{"reason": reason(env, r)})
	case workspace.SiteNothing:
		return env.t("cli.update.nothingHere", nil)
	case workspace.SiteCancelled:
		return env.t("cli.update.cancelled", nil)
	}
	return env.t("cli.update.notStarted", nil)
}

func reason(env Env, r workspace.SiteUpdate) string {
	if errors.Is(r.Err, wpcli.ErrNoDumpTool) {
		return env.t("cli.update.noDumpTool", nil)
	}
	return explain(env, r.Path, r.Err)
}

// failedStep is the last step that failed in an operation.
func failedStep(op model.Operation) model.OperationStep {
	for i := len(op.Steps) - 1; i >= 0; i-- {
		if op.Steps[i].Outcome == model.StepFailed {
			return op.Steps[i]
		}
	}
	return model.OperationStep{}
}

// exitForUpdates: an update that failed is an error; a site refused, not started or cancelled
// needs attention; else all good.
func exitForUpdates(results map[model.ID]workspace.SiteUpdate) int {
	code := exitOK
	for _, r := range results {
		switch r.Outcome {
		case workspace.SiteFailed:
			return exitError
		case workspace.SiteRefused, workspace.SiteNotStarted, workspace.SiteCancelled:
			code = exitAttention
		}
	}
	return code
}

func updatesJSON(previews []workspace.SitePreview, results map[model.ID]workspace.SiteUpdate) []updateJSON {
	out := make([]updateJSON, len(previews))
	for i, p := range previews {
		r := results[p.Site.ID]
		u := updateJSON{Path: p.Path, Outcome: string(r.Outcome), Items: []updateItemJSON{}}
		if r.Err != nil {
			u.Error = r.Err.Error()
		}
		if p.Preview.Core != nil {
			u.Items = append(u.Items, itemJSON(*p.Preview.Core))
		}
		for _, list := range [][]update.Item{p.Preview.Plugins, p.Preview.Themes, p.Preview.Languages} {
			for _, it := range list {
				u.Items = append(u.Items, itemJSON(it))
			}
		}
		if r.Operation.ID != model.NoID {
			u.Operation, u.BackupDir = r.Operation.ID.String(), r.Operation.BackupDir
			u.FailedStep = failedStep(r.Operation).Message
		}
		out[i] = u
	}
	return out
}

func itemJSON(it update.Item) updateItemJSON {
	return updateItemJSON{Kind: string(it.Kind), Slug: it.Slug, Language: it.Language, From: it.From, To: it.To}
}
