package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/errorlog"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// errorsJSON is a read of a site's errors with --json: the site's path, then the window's view.
type errorsJSON struct {
	Path string `json:"path"`
	app.SiteErrorsView
}

// siteErrors reads the errors of one site and prints them. A read is no verdict on the site, which
// a log, sometimes an old one, cannot give: 0 when the read was done, errors found or not.
func siteErrors(ctx context.Context, env Env, ws *workspace.Workspace, path string, asJSON bool) int {
	if path == "" {
		return env.fail(env.t("cli.errors.needsSite", nil))
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	node, err := tree.Resolve(path)
	if err != nil {
		return env.fail(explain(env, path, err))
	}
	if node.Kind != model.KindSite {
		return env.fail(env.t("cli.errors.needsSite", nil))
	}
	read, err := ws.SiteErrors(ctx, node.ID)
	if err != nil {
		return env.fail(explain(env, path, err))
	}
	if asJSON {
		enc := json.NewEncoder(env.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(errorsJSON{Path: tree.PathOf(node), SiteErrorsView: app.SiteErrorsViewOf(read)}); err != nil {
			return env.fail(err.Error())
		}
		return exitOK
	}
	printErrors(env, tree.PathOf(node), read)
	return exitOK
}

func printErrors(env Env, path string, e workspace.SiteErrors) {
	t := func(key string, vars map[string]any) string { return env.t("cli.errors."+key, vars) }
	fmt.Fprintf(env.Out, "%s: %s\n", path, e.File)
	switch {
	case e.State == workspace.ErrorsOff:
		fmt.Fprintln(env.Out, t("off", nil))
		return
	case e.State == workspace.ErrorsEmpty:
		fmt.Fprintln(env.Out, t("empty", nil))
		return
	case e.State == workspace.ErrorsLeft:
		fmt.Fprintln(env.Out, t("left", nil))
	}
	scope := t("scopeWhole", map[string]any{"size": sizeText(e.Size, env.Lang)})
	if !e.Whole {
		scope = t("scopeTail", map[string]any{"size": sizeText(e.Size, env.Lang)})
	}
	if e.More {
		scope += t("scopeMore", nil)
	}
	if e.Lines == 0 {
		fmt.Fprintln(env.Out, t("readAt", map[string]any{"time": clockText(e.Read), "scope": scope}))
		fmt.Fprintln(env.Out, t("nothing", nil))
		return
	}
	fmt.Fprintln(env.Out, t("readSince", map[string]any{"time": clockText(e.Read), "since": dateText(e.Since), "scope": scope}))
	errs, warnings := 0, 0
	for _, g := range e.Groups {
		if g.Type.Error() {
			errs++
		} else {
			warnings++
		}
	}
	fmt.Fprintln(env.Out, t("summary", map[string]any{"errors": t("errorCount", map[string]any{"count": errs}),
		"warnings": t("warningCount", map[string]any{"count": warnings}), "lines": t("lineCount", map[string]any{"count": e.Lines})}))

	heading := ""
	for _, g := range e.Groups {
		if h := map[bool]string{true: t("errorsHead", nil), false: t("warningsHead", nil)}[g.Type.Error()]; h != heading {
			heading = h
			fmt.Fprintf(env.Out, "\n%s\n", heading)
		}
		fmt.Fprintf(env.Out, "  %s  %s\n", groupTitle(env, g), t("count", map[string]any{"count": g.Count}))
		var where []string
		if g.File != "" {
			where = append(where, t("place", map[string]any{"file": g.File, "line": g.Line}))
		}
		if g.Count == 1 || g.First.Equal(g.Last) {
			where = append(where, dateText(g.Last))
		} else {
			where = append(where, t("between", map[string]any{"first": dateText(g.First), "last": dateText(g.Last)}))
		}
		fmt.Fprintf(env.Out, "    %s\n    | %s\n", strings.Join(where, " · "), g.Message)
	}

	fmt.Fprintf(env.Out, "\n%s\n", t("beforeTitle", nil))
	if len(e.Before) == 0 {
		fmt.Fprintf(env.Out, "  %s\n", t("beforeNone", nil))
	}
	for _, b := range e.Before {
		fmt.Fprintf(env.Out, "  %s  %s\n", dateText(b.At), beforeText(env, b))
	}
	fmt.Fprintf(env.Out, "\n%s\n", t("lead", nil))
}

// groupTitle names a group by its type and where it was raised: "Fatal error in the theme astra-child".
func groupTitle(env Env, g errorlog.Group) string {
	return env.t("cli.errors.kind."+string(g.Kind), map[string]any{"type": env.t("cli.errors.type."+string(g.Type), nil), "slug": g.Slug})
}

// beforeText says in one line an update or a rollback of Vigie, or a change seen between two scans.
func beforeText(env Env, b workspace.Before) string {
	if b.Change != nil {
		return changeText(env, *b.Change)
	}
	op := b.Operation
	var items []string
	for _, st := range op.Steps {
		if st.ItemKind != "" && (st.Step == model.StepUpdate || st.Step == model.StepRestoreFiles) {
			name := st.Slug
			if st.ItemKind == model.ItemCore {
				name = "WordPress"
			}
			items = append(items, fmt.Sprintf("%s %s → %s", name, st.VersionBefore, st.VersionAfter))
		}
	}
	key := "cli.errors.update"
	if op.Kind == model.OperationRollback {
		key = "cli.errors.rollback"
		if op.Automatic {
			key = "cli.errors.rollbackOnItsOwn"
		}
	}
	return env.t(key, map[string]any{"items": strings.Join(items, ", ")})
}

// dateText and clockText give a time in the computer's zone, as a user reads it.
func dateText(at time.Time) string { return at.Local().Format("02.01.2006 15:04") }

func clockText(at time.Time) string { return at.Local().Format("15:04") }
