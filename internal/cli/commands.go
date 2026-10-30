package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/workspace"
)

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

// failureText explains a classified ssh failure; "other" shows ssh's own message.
func failureText(env Env, failure, message string) string {
	return env.t("failure."+failure, map[string]any{"message": message})
}

// summary is one line about a scan: versions and pending updates, or why it did not complete.
func summary(env Env, s model.Scan) string {
	switch s.Outcome {
	case model.ScanCompleted:
	case model.ScanCannotLoad:
		if s.Failure == scan.FailureNotWordPress {
			return env.t("outcome.notWordPress", nil)
		}
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
	if s.Attempt != nil {
		parts = append(parts, env.t("cli.attempt", map[string]any{"reason": summary(env, *s.Attempt)}))
	}
	found := s.CountFindings()
	for _, c := range []struct {
		key   string
		count int
	}{{"cli.urgentFindings", found.Urgent}, {"cli.attentionFindings", found.Attention}, {"cli.notChecked", found.NotChecked}} {
		if c.count > 0 {
			parts = append(parts, env.t(c.key, map[string]any{"count": c.count}))
		}
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

// exitFor is the worst outcome: an error when a scan did not complete, urgent when a site is,
// attention when a site needs it or was not fully checked.
func exitFor(scans []model.Scan) int {
	code := exitOK
	for _, s := range scans {
		switch s.Status() {
		case model.StatusUnknown:
			return exitError
		case model.StatusUrgent:
			code = exitUrgent
		case model.StatusAttention, model.StatusPartial:
			code = max(code, exitAttention)
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
	// FilesChecked is when the files were last compared with wordpress.org's.
	FilesChecked string        `json:"filesChecked,omitempty"`
	Findings     []findingJSON `json:"findings"`
}

// findingJSON is a finding, its facts as Vigie stores them; an ignored one says until when and why.
type findingJSON struct {
	Kind       string            `json:"kind"`
	Subject    string            `json:"subject"`
	Identifier string            `json:"identifier,omitempty"`
	Version    string            `json:"version,omitempty"`
	Level      string            `json:"level"`
	Facts      map[string]string `json:"facts,omitempty"`
	Ignored    bool              `json:"ignored"`
	Ignore     *ignoreJSON       `json:"ignore,omitempty"`
}

type ignoreJSON struct {
	UntilVersion string `json:"untilVersion,omitempty"` // empty: permanently
	Reason       string `json:"reason,omitempty"`
	Created      string `json:"created"`
}

func findingsJSON(findings []model.Finding) []findingJSON {
	out := make([]findingJSON, len(findings))
	for i, f := range findings {
		out[i] = findingJSON{Kind: string(f.Kind), Subject: f.Subject, Identifier: f.Identifier, Version: f.Version,
			Level: string(f.Level), Facts: f.Facts, Ignored: f.Ignore != nil}
		if f.Ignore != nil {
			out[i].Ignore = &ignoreJSON{UntilVersion: f.Ignore.UntilVersion, Reason: f.Ignore.Reason,
				Created: f.Ignore.Created.UTC().Format(time.RFC3339)}
		}
	}
	return out
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

func scanPath(ctx context.Context, env Env, ws *workspace.Workspace, path string, opts workspace.ScanOptions, asJSON bool) int {
	results, err := ws.Scan(ctx, path, opts, nil)
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
				Subsites: len(r.Subsites), Findings: findingsJSON(s.Findings),
			}
			if !s.FilesChecked.IsZero() {
				out[i].FilesChecked = s.FilesChecked.UTC().Format(time.RFC3339)
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
