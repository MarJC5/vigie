package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/workspace"
)

// failureKeys maps the classified ssh failures to their explanation in the catalogs.
var failureKeys = map[string]string{
	string(sshx.FailureUnresolved):     "failure.unresolved",
	string(sshx.FailureRefused):        "failure.refused",
	string(sshx.FailureTimedOut):       "failure.timedOut",
	string(sshx.FailureHostKeyUnknown): "failure.hostKeyUnknown",
	string(sshx.FailureHostKeyChanged): "failure.hostKeyChanged",
	string(sshx.FailureAuthRefused):    "failure.authRefused",
	string(sshx.FailureKeyFileMissing): "failure.keyFileMissing",
}

// foundKeys maps the discovery states to their label in the catalogs.
var foundKeys = map[discover.State]string{
	discover.StateNew:          "found.new",
	discover.StateAlreadyAdded: "found.alreadyAdded",
	discover.StateNotInstalled: "found.notInstalled",
	discover.StateCannotLoad:   "found.cannotLoad",
}

// explain turns an error of the workspace into a sentence for the user.
func explain(env Env, path string, err error) string {
	var connErr *sshx.ConnectionError
	switch {
	case errors.Is(err, model.ErrNotFound):
		return env.t("cli.notFound", map[string]any{"path": path})
	case errors.Is(err, workspace.ErrNotAHost):
		return env.t("cli.notAHost", map[string]any{"path": path})
	case errors.As(err, &connErr):
		return failureText(env, string(connErr.Failure), connErr.Message)
	}
	return err.Error()
}

func failureText(env Env, failure, message string) string {
	if key, ok := failureKeys[failure]; ok {
		return env.t(key, nil)
	}
	return env.t("failure.other", map[string]any{"message": message})
}

// summary is one line about a scan: versions and pending updates, or why it did not complete.
func summary(env Env, s model.Scan) string {
	switch s.Outcome {
	case model.ScanCompleted:
	case model.ScanCannotLoad:
		return env.t("outcome.cannotLoad", map[string]any{"message": s.Error})
	case model.ScanCancelled:
		return env.t("outcome.cancelled", nil)
	default:
		if s.Failure != "" {
			return failureText(env, s.Failure, s.Error)
		}
		return env.t("outcome.failed", map[string]any{"message": s.Error})
	}
	parts := []string{env.t("cli.wordpress", map[string]any{"version": s.WordPress})}
	if newest := newestCoreUpdate(s); newest != "" {
		parts = append(parts, env.t("cli.coreUpdate", map[string]any{"version": newest}))
	}
	pending := s.PendingUpdates()
	for _, c := range []struct {
		key   string
		count int
	}{{"cli.pluginUpdates", pending.Plugins}, {"cli.themeUpdates", pending.Themes}, {"cli.languageUpdates", pending.Languages}} {
		if c.count > 0 {
			parts = append(parts, env.t(c.key, map[string]any{"count": c.count}))
		}
	}
	if !s.CoreChecked || !s.PluginsChecked || !s.ThemesChecked || !s.LanguagesChecked {
		parts = append(parts, env.t("cli.unchecked", nil))
	}
	return strings.Join(parts, ", ")
}

// newestCoreUpdate returns the highest WordPress release offered, or "".
func newestCoreUpdate(s model.Scan) string {
	newest := ""
	for _, item := range s.Items {
		if item.Kind == model.ItemCore && item.Update == model.UpdateAvailable &&
			(newest == "" || model.CompareVersions(item.UpdateVersion, newest) > 0) {
			newest = item.UpdateVersion
		}
	}
	return newest
}

func statusText(env Env, s model.Scan) string {
	return env.t("status."+string(s.Status()), nil)
}

// exitFor is the worst outcome: an error when a scan did not complete, attention when a site
// needs it.
func exitFor(scans []model.Scan) int {
	code := exitOK
	for _, s := range scans {
		switch {
		case s.Outcome != model.ScanCompleted:
			return exitError
		case s.Status() == model.StatusAttention:
			code = exitAttention
		}
	}
	return code
}

type scanJSON struct {
	Path      string      `json:"path"`
	Outcome   string      `json:"outcome"`
	Status    string      `json:"status"`
	Failure   string      `json:"failure,omitempty"`
	Error     string      `json:"error,omitempty"`
	WordPress string      `json:"wordpress,omitempty"`
	PHPCLI    string      `json:"phpCli,omitempty"`
	WPCLI     string      `json:"wpCli,omitempty"`
	Home      string      `json:"home,omitempty"`
	Pending   pendingJSON `json:"pending"`
	Checked   checkedJSON `json:"checked"`
	Subsites  int         `json:"subsites,omitempty"`
}

type pendingJSON struct {
	Core      int `json:"core"`
	Plugins   int `json:"plugins"`
	Themes    int `json:"themes"`
	Languages int `json:"languages"`
}

type checkedJSON struct {
	Core      bool `json:"core"`
	Plugins   bool `json:"plugins"`
	Themes    bool `json:"themes"`
	Languages bool `json:"languages"`
}

func scanPath(ctx context.Context, env Env, ws *workspace.Workspace, path string, asJSON bool) int {
	results, err := ws.Scan(ctx, path, nil)
	if err != nil {
		return env.fail(explain(env, path, err))
	}
	scans := make([]model.Scan, len(results))
	for i, r := range results {
		scans[i] = r.Scan
	}
	if asJSON {
		out := make([]scanJSON, len(results))
		for i, r := range results {
			s, p := r.Scan, r.Scan.PendingUpdates()
			out[i] = scanJSON{
				Path: r.Path, Outcome: string(s.Outcome), Status: string(s.Status()), Failure: s.Failure, Error: s.Error,
				WordPress: s.WordPress, PHPCLI: s.PHPCLI, WPCLI: s.WPCLI, Home: s.Home,
				Pending:  pendingJSON{Core: p.Core, Plugins: p.Plugins, Themes: p.Themes, Languages: p.Languages},
				Checked:  checkedJSON{Core: s.CoreChecked, Plugins: s.PluginsChecked, Themes: s.ThemesChecked, Languages: s.LanguagesChecked},
				Subsites: len(r.Subsites),
			}
		}
		writeJSON(env.Out, out)
		return exitFor(scans)
	}
	if len(results) == 0 {
		fmt.Fprintln(env.Out, env.t("cli.noSites", nil))
		return exitOK
	}
	w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Path, statusText(env, r.Scan), summary(env, r.Scan))
	}
	w.Flush()
	return exitFor(scans)
}

type treeJSON struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Status    string `json:"status,omitempty"`
	WordPress string `json:"wordpress,omitempty"`
}

// tree prints the site manager, each site with the status of its latest scan.
func tree(ctx context.Context, env Env, ws *workspace.Workspace, asJSON bool) int {
	t, err := ws.DB().LoadTree(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	latest, err := ws.DB().LatestScans(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	nodes := []treeJSON{}
	w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
	t.Walk(func(n model.Node, depth int) {
		indent := strings.Repeat("  ", depth)
		entry := treeJSON{Path: t.PathOf(n), Kind: n.Kind.String()}
		switch n.Kind {
		case model.KindFolder:
			f, _ := t.Folder(n.ID)
			fmt.Fprintf(w, "%s%s/\n", indent, f.Name)
		case model.KindHost:
			h, _ := t.Host(n.ID)
			fmt.Fprintf(w, "%s%s/\n", indent, h.Name)
		case model.KindSite:
			site, _ := t.Site(n.ID)
			scan, scanned := latest[n.ID]
			if !scanned {
				fmt.Fprintf(w, "%s%s\t%s\n", indent, site.Name, env.t("detail.notScanned", nil))
				break
			}
			entry.Status, entry.WordPress = string(scan.Status()), scan.WordPress
			fmt.Fprintf(w, "%s%s\t%s\t%s\n", indent, site.Name, statusText(env, scan), summary(env, scan))
		}
		nodes = append(nodes, entry)
	})
	if asJSON {
		writeJSON(env.Out, nodes)
		return exitOK
	}
	w.Flush()
	return exitOK
}

type foundJSON struct {
	Path      string `json:"path"`
	State     string `json:"state"`
	Name      string `json:"name"`
	URL       string `json:"url,omitempty"`
	Multisite bool   `json:"multisite,omitempty"`
	Subsites  int    `json:"subsites,omitempty"`
	Message   string `json:"message,omitempty"`
}

// discoverHost lists the WordPress installs of a host's account; it adds nothing.
func discoverHost(ctx context.Context, env Env, ws *workspace.Workspace, path string, asJSON bool) int {
	found, truncated, err := ws.Discover(ctx, path)
	if err != nil {
		return env.fail(explain(env, path, err))
	}
	if asJSON {
		out := make([]foundJSON, len(found))
		for i, f := range found {
			out[i] = foundJSON{Path: f.Path, State: string(f.State), Name: f.Name, URL: f.URL,
				Multisite: f.Multisite, Subsites: f.Subsites, Message: f.Message}
		}
		writeJSON(env.Out, out)
		return exitOK
	}
	w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
	for _, f := range found {
		detail := f.URL
		if f.State == discover.StateCannotLoad {
			detail = f.Message
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Path, env.t(foundKeys[f.State], nil), f.Name, detail)
	}
	w.Flush()
	if truncated {
		fmt.Fprintln(env.Out, env.t("cli.truncated", map[string]any{"count": discover.MaxCandidates}))
	}
	return exitOK
}

func writeJSON(out io.Writer, v any) {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
