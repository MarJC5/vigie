package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// dataOptions are the options of export and import; each command takes its own.
type dataOptions struct {
	args               []string
	asJSON, yes        bool
	history, trustKeys bool
	sshConfig          bool
	output, into       string
}

// parseData reads the options a command allows; any other is refused.
func parseData(args []string, allowed ...string) (dataOptions, error) {
	var o dataOptions
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, hasValue := strings.Cut(a, "=")
		if !strings.HasPrefix(a, "-") {
			o.args = append(o.args, a)
			continue
		}
		if !slices.Contains(allowed, name) || (hasValue && name != "-o" && name != "--into") {
			return o, errors.New(a)
		}
		switch name {
		case "-o", "--into":
			if !hasValue {
				if i+1 >= len(args) {
					return o, errNeedsValue{name}
				}
				i++
				value = args[i]
			}
			if value == "" {
				return o, errNeedsValue{name}
			}
			if name == "-o" {
				o.output = value
			} else {
				o.into = value
			}
		case "--json":
			o.asJSON = true
		case "--yes":
			o.yes = true
		case "--history":
			o.history = true
		case "--trust-keys":
			o.trustKeys = true
		case "--ssh-config":
			o.sshConfig = true
		}
	}
	return o, nil
}

type errNeedsValue struct{ option string }

func (e errNeedsValue) Error() string { return e.option }

// optionError explains an option refused or without its value.
func optionError(env Env, err error) int {
	if needs, ok := errors.AsType[errNeedsValue](err); ok {
		return env.fail(env.t("cli.needsValue", map[string]any{"option": needs.option}))
	}
	return env.fail(env.t("cli.unknownOption", map[string]any{"option": err.Error()}))
}

type exportJSON struct {
	File       string `json:"file"`
	Folders    int    `json:"folders"`
	Hosts      int    `json:"hosts"`
	Sites      int    `json:"sites"`
	Scans      int    `json:"scans"`
	Operations int    `json:"operations"`
}

// exportPath writes the export of path (everything without one): to the file given with -o,
// readable by its owner only, or else to the standard output, for a script or an add-on.
func exportPath(ctx context.Context, env Env, ws *workspace.Workspace, o dataOptions) int {
	path := strings.Join(o.args, " ")
	f, err := ws.Export(ctx, path, o.history)
	switch {
	case errors.Is(err, workspace.ErrNotExportable):
		return env.fail(env.t("cli.export.notExportable", nil))
	case err != nil:
		return env.fail(explain(env, path, err))
	}
	if o.output == "" {
		if err := export.Write(env.Out, f); err != nil {
			return env.fail(err.Error())
		}
		return exitOK
	}
	if err := export.WriteFile(o.output, f); err != nil {
		return env.fail(err.Error())
	}
	summary := exportJSON{File: o.output, Folders: len(f.Folders), Hosts: len(f.Hosts), Sites: len(f.Sites)}
	if f.History != nil {
		summary.Scans, summary.Operations = len(f.History.Scans), len(f.History.Operations)
	}
	if o.asJSON {
		writeJSON(env.Out, summary)
		return exitOK
	}
	fmt.Fprintln(env.Out, env.t("cli.export.done", map[string]any{"file": o.output, "folders": summary.Folders,
		"hosts": summary.Hosts, "sites": summary.Sites}))
	fmt.Fprintln(env.Out, env.t("cli.export.sensitive", nil))
	return exitOK
}

type importEntryJSON struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type importKeyJSON struct {
	Host        string `json:"host"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Refused     string `json:"refused,omitempty"`
	Trusted     bool   `json:"trusted"`
}

type importLineJSON struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	Value  string `json:"value,omitempty"`
}

type sshConfigJSON struct {
	Alias    string `json:"alias"`
	Hostname string `json:"hostname,omitempty"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
	Present  string `json:"present,omitempty"`
	Error    string `json:"error,omitempty"`
}

type importTagJSON struct {
	Site   string `json:"site"`
	Tag    string `json:"tag"`
	Reason string `json:"reason"`
}

type importJSON struct {
	Into       string            `json:"into"`
	Entries    []importEntryJSON `json:"entries"`
	Tags       []importTagJSON   `json:"tagsLeftOut,omitempty"`
	Lines      []importLineJSON  `json:"lines,omitempty"`
	SSHConfig  []sshConfigJSON   `json:"sshConfig,omitempty"`
	Keys       []importKeyJSON   `json:"keys"`
	Ignores    int               `json:"ignores"`
	Scans      int               `json:"scans"`
	Operations int               `json:"operations"`
	Imported   bool              `json:"imported"`
}

// importSource is what an import reads, before its preview.
type importSource struct {
	file      export.File
	lines     []exchange.CSVRefusal
	sshConfig []workspace.SSHConfigHost
}

// importFile previews what a file (an export or a CSV site list) or the aliases named of
// ~/.ssh/config would add, asks, then adds it; each host key is trusted only once confirmed.
// --ssh-config without aliases lists them and adds nothing: the file names any server, not only
// the ones of WordPress sites.
func importFile(ctx context.Context, env Env, ws *workspace.Workspace, o dataOptions) int {
	if o.sshConfig && len(o.args) == 0 {
		return listSSHConfig(ctx, env, ws, o.asJSON)
	}
	if o.asJSON && !o.yes {
		return env.fail(env.t("cli.import.jsonNeedsYes", nil))
	}
	if !o.sshConfig && len(o.args) != 1 {
		return env.fail(env.t("cli.import.needsFile", nil))
	}
	var src importSource
	var err error
	if o.sshConfig {
		src, err = readSSHConfig(ctx, ws, o.args)
	} else {
		src, err = readImport(o.args[0])
	}
	if err != nil {
		return env.fail(importError(env, o, err))
	}
	plan, err := ws.PlanImport(ctx, src.file, o.into)
	switch {
	case errors.Is(err, workspace.ErrNotAFolder):
		return env.fail(env.t("cli.import.notAFolder", map[string]any{"path": o.into}))
	case err != nil:
		return env.fail(explain(env, o.into, err))
	}
	result := importJSONOf(plan, src)
	refused := len(src.lines) > 0 || len(plan.TagsLeftOut) > 0
	for _, e := range plan.Entries {
		refused = refused || e.Status == workspace.ImportRefused
	}
	for _, h := range src.sshConfig {
		refused = refused || h.Error != ""
	}
	done := func(code int) int {
		if o.asJSON {
			writeJSON(env.Out, result)
		}
		if code == exitOK && refused {
			return exitAttention
		}
		return code
	}
	if !o.asJSON {
		printImport(env, plan, src)
	}
	if plan.Added() == 0 {
		if !o.asJSON {
			fmt.Fprintln(env.Out, env.t("cli.import.nothing", nil))
		}
		return done(exitOK)
	}
	answers := bufio.NewReader(env.In)
	if env.In == nil {
		answers = bufio.NewReader(strings.NewReader(""))
	}
	if !o.yes && !ask(env, answers, env.t("cli.import.prompt", map[string]any{"count": plan.Added()})) {
		fmt.Fprintln(env.Out, env.t("cli.import.declined", nil))
		return exitAttention
	}
	var trusted []model.ID
	left := false
	for _, k := range plan.Keys {
		if k.Refused != "" {
			continue
		}
		if o.trustKeys || (!o.yes && ask(env, answers, env.t("cli.import.trustPrompt",
			map[string]any{"host": k.Host, "type": k.Key.Type, "fingerprint": k.Fingerprint}))) {
			trusted = append(trusted, k.HostID)
		} else {
			left = true
		}
	}
	if err := ws.Import(ctx, plan, trusted); err != nil {
		return env.fail(err.Error())
	}
	result.Imported = true
	for i, k := range plan.Keys {
		result.Keys[i].Trusted = k.Refused == "" && slices.Contains(trusted, k.HostID)
	}
	if !o.asJSON {
		fmt.Fprintln(env.Out, env.t("cli.import.done", map[string]any{"count": plan.Added()}))
		if left {
			fmt.Fprintln(env.Out, env.t("cli.import.keysLeft", nil))
		}
	}
	return done(exitOK)
}

// readImport reads an export, or a CSV site list: by its extension, or when it is not an export.
func readImport(file string) (importSource, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return importSource{}, err
	}
	if !strings.EqualFold(filepath.Ext(file), ".csv") {
		f, err := export.Read(bytes.NewReader(data))
		if !errors.Is(err, export.ErrNotAnExport) {
			return importSource{file: f}, err
		}
		if f, report, err := exchange.ReadCSV(bytes.NewReader(data)); err == nil {
			return importSource{file: f, lines: report.Refused}, nil
		}
		return importSource{}, export.ErrNotAnExport
	}
	f, report, err := exchange.ReadCSV(bytes.NewReader(data))
	return importSource{file: f, lines: report.Refused}, err
}

// errNotInSSHConfig is returned for an alias that ~/.ssh/config does not name.
type errNotInSSHConfig struct{ alias string }

func (e errNotInSSHConfig) Error() string { return e.alias }

// readSSHConfig takes the aliases chosen that no host reaches yet and ssh can read; the others
// are shown with why they are left out.
func readSSHConfig(ctx context.Context, ws *workspace.Workspace, chosen []string) (importSource, error) {
	config, err := ws.SSHConfigHosts(ctx)
	if err != nil {
		return importSource{}, err
	}
	var aliases []string
	var shown []workspace.SSHConfigHost
	for _, alias := range chosen {
		i := slices.IndexFunc(config.Hosts, func(h workspace.SSHConfigHost) bool { return h.Alias == alias })
		if i < 0 {
			return importSource{}, errNotInSSHConfig{alias}
		}
		h := config.Hosts[i]
		shown = append(shown, h)
		if h.Present == "" && h.Error == "" && !slices.Contains(aliases, alias) {
			aliases = append(aliases, alias)
		}
	}
	return importSource{file: workspace.SSHConfigFile(aliases), sshConfig: shown}, nil
}

// listSSHConfig lists the names of ~/.ssh/config with their account, and adds nothing.
func listSSHConfig(ctx context.Context, env Env, ws *workspace.Workspace, asJSON bool) int {
	config, err := ws.SSHConfigHosts(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	if asJSON {
		list := []sshConfigJSON{}
		for _, h := range config.Hosts {
			list = append(list, sshConfigJSON{Alias: h.Alias, Hostname: h.Hostname, Port: h.Port, User: h.User,
				Present: h.Present, Error: h.Error})
		}
		writeJSON(env.Out, list)
		return exitOK
	}
	if len(config.Hosts) == 0 {
		fmt.Fprintln(env.Out, env.t("cli.import.sshNone", nil))
		return exitOK
	}
	w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
	for _, h := range config.Hosts {
		switch {
		case h.Error != "":
			fmt.Fprintf(w, "  %s\t\t%s\n", h.Alias, env.t("cli.import.sshError", map[string]any{"message": h.Error}))
		case h.Present != "":
			fmt.Fprintf(w, "  %s\t%s@%s:%d\t%s\n", h.Alias, h.User, h.Hostname, h.Port,
				env.t("cli.import.sshPresent", map[string]any{"path": h.Present}))
		default:
			fmt.Fprintf(w, "  %s\t%s@%s:%d\n", h.Alias, h.User, h.Hostname, h.Port)
		}
	}
	w.Flush()
	fmt.Fprintln(env.Out, env.t("cli.import.sshChoose", map[string]any{"alias": config.Hosts[0].Alias}))
	return exitOK
}

func importError(env Env, o dataOptions, err error) string {
	if missing, ok := errors.AsType[errNotInSSHConfig](err); ok {
		return env.t("cli.import.sshUnknown", map[string]any{"alias": missing.alias})
	}
	switch {
	case errors.Is(err, export.ErrNewerVersion):
		return env.t("cli.import.newer", nil)
	case errors.Is(err, export.ErrNotAnExport):
		return env.t("cli.import.notAnExport", map[string]any{"file": o.args[0]})
	}
	return err.Error()
}

// printImport shows the preview: each node of the file and what the import does with it, the
// lines and names left out, the keys to confirm.
func printImport(env Env, plan workspace.ImportPlan, src importSource) {
	into := plan.Into
	if into == "" {
		into = env.t("cli.import.top", nil)
	}
	fmt.Fprintln(env.Out, env.t("cli.import.into", map[string]any{"path": into}))
	w := tabwriter.NewWriter(env.Out, 0, 0, 2, ' ', 0)
	for _, h := range src.sshConfig {
		switch {
		case h.Present != "":
			fmt.Fprintf(w, "  %s\t%s\t%s\n", env.t("cli.import.status.present", nil), h.Alias,
				env.t("cli.import.sshPresent", map[string]any{"path": h.Present}))
		case h.Error != "":
			fmt.Fprintf(w, "  %s\t%s\t%s\n", env.t("cli.import.status.refused", nil), h.Alias,
				env.t("cli.import.sshError", map[string]any{"message": h.Error}))
		}
	}
	for _, e := range plan.Entries {
		note := ""
		switch {
		case e.Reason == workspace.ReasonInvalid:
			note = e.Detail
		case e.Reason != "":
			note = env.t("cli.import.reason."+string(e.Reason), nil)
		}
		status := env.t("cli.import.status."+string(e.Status), nil)
		if note == "" {
			fmt.Fprintf(w, "  %s\t%s\n", status, e.Path)
			continue
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\n", status, e.Path, note)
	}
	w.Flush()
	for _, l := range plan.TagsLeftOut {
		fmt.Fprintln(env.Out, env.t("cli.import.tagLeftOut", map[string]any{"tag": l.Tag, "site": l.Site,
			"reason": env.t("cli.import.tag."+string(l.Reason), nil)}))
	}
	for _, l := range src.lines {
		reason := env.t("cli.import.line."+string(l.Reason), nil)
		if l.Value != "" {
			reason += " (" + l.Value + ")"
		}
		fmt.Fprintln(env.Out, env.t("cli.import.lineRefused", map[string]any{"line": l.Line, "reason": reason}))
	}
	if plan.Ignores+plan.Scans+plan.Operations > 0 {
		fmt.Fprintln(env.Out, env.t("cli.import.history", map[string]any{"ignores": plan.Ignores, "scans": plan.Scans,
			"operations": plan.Operations}))
	}
	for _, k := range plan.Keys {
		if k.Refused == "" {
			fmt.Fprintln(env.Out, env.t("cli.import.keyOffered", map[string]any{"host": k.Host, "type": k.Key.Type,
				"fingerprint": k.Fingerprint}))
			continue
		}
		fmt.Fprintln(env.Out, env.t("cli.import.keyRefused", map[string]any{"host": k.Host,
			"reason": env.t("cli.import.key."+string(k.Refused), nil)}))
	}
}

// ask asks a yes or no question; y, yes, o and oui answer yes.
func ask(env Env, answers *bufio.Reader, question string) bool {
	fmt.Fprint(env.Out, question)
	answer, _ := answers.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes", "o", "oui":
		return true
	}
	return false
}

func importJSONOf(plan workspace.ImportPlan, src importSource) importJSON {
	out := importJSON{Into: plan.Into, Entries: []importEntryJSON{}, Keys: []importKeyJSON{},
		Ignores: plan.Ignores, Scans: plan.Scans, Operations: plan.Operations}
	for _, e := range plan.Entries {
		out.Entries = append(out.Entries, importEntryJSON{Kind: e.Kind.String(), Name: e.Name, Path: e.Path,
			Status: string(e.Status), Reason: string(e.Reason), Detail: e.Detail})
	}
	for _, l := range plan.TagsLeftOut {
		out.Tags = append(out.Tags, importTagJSON{Site: l.Site, Tag: l.Tag, Reason: string(l.Reason)})
	}
	for _, k := range plan.Keys {
		out.Keys = append(out.Keys, importKeyJSON{Host: k.Host, Name: k.Name, Type: k.Key.Type, Fingerprint: k.Fingerprint,
			Refused: string(k.Refused)})
	}
	for _, l := range src.lines {
		out.Lines = append(out.Lines, importLineJSON{Line: l.Line, Reason: string(l.Reason), Value: l.Value})
	}
	for _, h := range src.sshConfig {
		out.SSHConfig = append(out.SSHConfig, sshConfigJSON{Alias: h.Alias, Hostname: h.Hostname, Port: h.Port, User: h.User,
			Present: h.Present, Error: h.Error})
	}
	return out
}

// runData runs export or import, each with its own options.
func runData(ctx context.Context, env Env, name string, args []string) int {
	allowed := []string{"--history", "-o", "--json"}
	if name == "import" {
		allowed = []string{"--into", "--yes", "--trust-keys", "--ssh-config", "--json"}
	}
	o, err := parseData(args, allowed...)
	if err != nil {
		return optionError(env, err)
	}
	ws, err := env.Open(ctx)
	if err != nil {
		return env.fail(err.Error())
	}
	defer ws.Close()
	if name == "export" {
		return exportPath(ctx, env, ws, o)
	}
	return importFile(ctx, env, ws, o)
}
