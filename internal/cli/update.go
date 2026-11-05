package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

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
	Ignored  bool   `json:"ignored,omitempty"`  // left out: the site ignores this update
	SetAside bool   `json:"setAside,omitempty"` // left out: this version rolled back before (ADR-0042)
}

// rolledBackJSON is an item that broke the site and went back on its own (ADR-0042).
type rolledBackJSON struct {
	Kind    string `json:"kind"`
	Slug    string `json:"slug"`
	Version string `json:"version"` // the version that broke the site
	BackTo  string `json:"backTo"`
	Reason  string `json:"reason"` // the check that failed
}

type updateJSON struct {
	Path       string           `json:"path"`
	Outcome    string           `json:"outcome"`
	Error      string           `json:"error,omitempty"`
	Items      []updateItemJSON `json:"items"`
	Operation  string           `json:"operation,omitempty"`
	BackupDir  string           `json:"backupDir,omitempty"`
	FailedStep string           `json:"failedStep,omitempty"`
	RolledBack []rolledBackJSON `json:"rolledBack,omitempty"`
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
	// What each site will change, read on the servers: the progress line counts them (ADR-0037).
	total := len(tree.SitesUnder(node))
	p := startProgress(env, asJSON, env.t("cli.progress.preview", map[string]any{"done": 0, "count": total}))
	previews, err := ws.PreviewNodesFollowed(ctx, []model.Node{node}, func(done, total int) {
		p.set(env.t("cli.progress.preview", map[string]any{"done": done, "count": total}))
	})
	p.end()
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
		// Nothing offered, or only updates the site ignores or set aside.
		case p.Preview.Pending() == len(p.Ignored)+len(p.SetAside):
			results[p.Site.ID] = workspace.SiteUpdate{Site: p.Site, Path: p.Path, Outcome: workspace.SiteNothing}
		case !p.Preview.DumpTool:
			results[p.Site.ID] = workspace.SiteUpdate{Site: p.Site, Path: p.Path, Outcome: workspace.SiteRefused, Err: wpcli.ErrNoDumpTool}
		default:
			planned = append(planned, workspace.Planned{SitePreview: p, Selection: p.Bulk()})
		}
	}

	if !asJSON {
		for _, p := range previews {
			fmt.Fprintln(env.Out, p.Path)
			if r, ok := results[p.Site.ID]; ok {
				fmt.Fprintln(env.Out, "  "+outcomeText(env, r))
				continue
			}
			for _, line := range itemLines(env, p) {
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

	// A line per site being updated with its step (ADR-0037, 2b); each site's outcome printed above
	// the block as it ends.
	p = startProgress(env, asJSON, env.t("cli.progress.update", map[string]any{"done": 0, "count": len(planned), "running": 0}))
	names := map[model.ID]string{}
	for _, pl := range planned {
		names[pl.Site.ID] = pl.Site.Name
	}
	var mu sync.Mutex
	steps := map[model.ID]string{}
	var running []model.ID
	ended := 0
	show := func() {
		var items []item
		for _, id := range running {
			items = append(items, item{text: names[id] + " · " + steps[id]})
		}
		p.set(env.t("cli.progress.update", map[string]any{"done": ended, "count": len(planned), "running": len(running)}), items...)
	}
	done := ws.UpdateSites(ctx, planned, workspace.Follow{
		OnSite: func(r workspace.SiteUpdate) {
			mu.Lock()
			running = slices.DeleteFunc(running, func(id model.ID) bool { return id == r.Site.ID })
			ended++
			show()
			mu.Unlock()
			if !asJSON {
				p.above(func() { fmt.Fprintf(env.Out, "%s  %s\n", r.Path, outcomeText(env, r)) })
			}
		},
		OnStep: func(op model.Operation, st model.OperationStep) {
			mu.Lock()
			defer mu.Unlock()
			if !slices.Contains(running, op.SiteID) {
				running = append(running, op.SiteID)
			}
			steps[op.SiteID] = stepText(env, st)
			show()
		},
	})
	p.end()
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
func itemLines(env Env, sp workspace.SitePreview) []string {
	p := sp.Preview
	var lines []string
	// An update the site ignores is listed, said left out.
	add := func(it update.Item, line string) {
		switch {
		case sp.IsIgnored(it):
			line += " " + env.t("cli.update.ignored", nil)
		case sp.IsSetAside(it):
			line += " " + env.t("cli.update.setAside", nil)
		}
		lines = append(lines, line)
	}
	if p.Core != nil {
		add(*p.Core, env.t("cli.update.core", map[string]any{"from": p.Core.From, "to": p.Core.To}))
	}
	for _, it := range p.Plugins {
		add(it, env.t("cli.update.plugin", map[string]any{"slug": it.Slug, "from": it.From, "to": it.To}))
	}
	for _, it := range p.Themes {
		add(it, env.t("cli.update.theme", map[string]any{"slug": it.Slug, "from": it.From, "to": it.To}))
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
		parts := []string{env.t("cli.update.updated", nil)}
		for _, b := range rolledBack(r) {
			parts = append(parts, env.t("cli.update.rolledBack", map[string]any{"slug": b.Slug, "version": b.Version, "reason": b.Reason}))
		}
		return strings.Join(parts, ", ")
	case workspace.SiteFailed:
		text := env.t("cli.update.failed", map[string]any{"message": failedStep(r.Operation).Message})
		for _, back := range r.Rollbacks {
			if st := restoreStep(back); back.Outcome != model.OperationCompleted && st.Slug != "" {
				text += ", " + env.t("cli.update.rollbackFailed", map[string]any{"slug": st.Slug})
			}
		}
		return text + ". " + env.t("cli.update.rollbackHint", nil)
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

// restoreStep is the step of a rollback that put an item back; zero without one.
func restoreStep(back model.Operation) model.OperationStep {
	for _, st := range back.Steps {
		if st.Step == model.StepRestoreFiles {
			return st
		}
	}
	return model.OperationStep{}
}

// rolledBack lists the items of a site's update that went back on their own and passed, each with
// the check that failed after its update.
func rolledBack(r workspace.SiteUpdate) []rolledBackJSON {
	var out []rolledBackJSON
	for _, back := range r.Rollbacks {
		st := restoreStep(back)
		if back.Outcome != model.OperationCompleted || st.Slug == "" {
			continue
		}
		b := rolledBackJSON{Kind: string(st.ItemKind), Slug: st.Slug, Version: st.VersionBefore, BackTo: st.VersionAfter}
		for _, c := range r.Operation.Steps {
			if c.Step == model.StepCheck && c.Outcome == model.StepFailed && c.ItemKind == st.ItemKind && c.Slug == st.Slug {
				b.Reason = c.Message
			}
		}
		out = append(out, b)
	}
	return out
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
			u.Items = append(u.Items, itemJSON(p, *p.Preview.Core))
		}
		for _, list := range [][]update.Item{p.Preview.Plugins, p.Preview.Themes, p.Preview.Languages} {
			for _, it := range list {
				u.Items = append(u.Items, itemJSON(p, it))
			}
		}
		if r.Operation.ID != model.NoID {
			u.Operation, u.BackupDir = r.Operation.ID.String(), r.Operation.BackupDir
			if r.Outcome != workspace.SiteUpdated {
				u.FailedStep = failedStep(r.Operation).Message
			}
		}
		u.RolledBack = rolledBack(r)
		out[i] = u
	}
	return out
}

func itemJSON(p workspace.SitePreview, it update.Item) updateItemJSON {
	return updateItemJSON{Kind: string(it.Kind), Slug: it.Slug, Language: it.Language, From: it.From, To: it.To,
		Ignored: p.IsIgnored(it), SetAside: p.IsSetAside(it)}
}

// stepText is a step of an update as the window says it: "akismet 5.3 → 5.7.2", "Database backup".
func stepText(env Env, st model.OperationStep) string {
	item := st.Slug
	if st.ItemKind == model.ItemCore {
		item = "WordPress"
	}
	switch {
	case st.Step == model.StepUpdate && st.ItemKind == model.ItemLanguage:
		return env.t("update.step.languages", nil)
	case st.Step == model.StepUpdate:
		return env.t("update.step.update", map[string]any{"item": item, "from": st.VersionBefore, "to": st.VersionAfter})
	case st.Step == model.StepBackupFiles && st.ItemKind == model.ItemCore:
		return env.t("update.step.backup-core", nil)
	case st.Step == model.StepBackupFiles:
		return env.t("update.step.backup-files", map[string]any{"item": item})
	case st.Step == model.StepRestoreFiles:
		// The version it goes back to: VersionBefore is the one that broke the site.
		return env.t("update.step.restore-files", map[string]any{"item": item, "version": st.VersionAfter})
	}
	return env.t("update.step."+string(st.Step), nil)
}
