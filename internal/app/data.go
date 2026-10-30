package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// The kinds of files the import asks for.
const (
	FileVigie = "vigie" // an export, JSON
	FileCSV   = "csv"   // a site list
	FileAny   = "any"   // a file an add-on reads
	// SourceAddOn marks an import of what an add-on wrote.
	SourceAddOn = "addon"
)

var (
	errNotExportable = errors.New("a site is exported with its host")
	errNoImport      = errors.New("no import is being previewed")
	errUnknownFile   = errors.New("unknown kind of file")
)

// FilePicker asks the user for a file with the system's dialogs; "" when they cancel.
type FilePicker interface {
	// SaveFile asks where to save a file, suggesting its name.
	SaveFile(name string) (string, error)
	// OpenFile asks for a file of a kind (FileVigie, FileCSV).
	OpenFile(kind string) (string, error)
}

// Data serves the Data tab of the settings: export the site manager, import an export, a CSV
// site list or the hosts of ~/.ssh/config, each import shown before anything is added.
type Data struct {
	ws   *workspace.Workspace
	pick FilePicker
	now  func() time.Time

	mu      sync.Mutex
	pending *pendingImport // the import being previewed
}

type pendingImport struct {
	source ImportPreview // what the file says, without the plan
	file   export.File
	plan   workspace.ImportPlan
}

// NewData returns the service.
func NewData(ws *workspace.Workspace, pick FilePicker, now func() time.Time) *Data {
	return &Data{ws: ws, pick: pick, now: now}
}

// ExportSummary is what an export of a node holds, for the export dialog.
type ExportSummary struct {
	Path    string `json:"path"` // "" for everything
	Folders int    `json:"folders"`
	Hosts   int    `json:"hosts"`
	Sites   int    `json:"sites"`
	Ignores int    `json:"ignores"`
	// HistoryBytes is how much larger the file is with the history.
	HistoryBytes int      `json:"historyBytes"`
	Problem      *Problem `json:"problem"`
}

// ExportSummary counts what an export of everything ("") or of a folder or host would hold.
func (d *Data) ExportSummary(ctx context.Context, nodeID string) ExportSummary {
	path, err := d.exportPath(ctx, nodeID)
	if err != nil {
		return ExportSummary{Problem: ProblemOf(err)}
	}
	f, err := d.ws.Export(ctx, path, true)
	if err != nil {
		return ExportSummary{Problem: ProblemOf(err)}
	}
	with, err := size(f)
	if err != nil {
		return ExportSummary{Problem: ProblemOf(err)}
	}
	f.History = nil
	without, err := size(f)
	if err != nil {
		return ExportSummary{Problem: ProblemOf(err)}
	}
	return ExportSummary{Path: path, Folders: len(f.Folders), Hosts: len(f.Hosts), Sites: len(f.Sites),
		Ignores: len(f.Ignores), HistoryBytes: with - without}
}

func size(f export.File) (int, error) {
	var buf bytes.Buffer
	err := export.Write(&buf, f)
	return buf.Len(), err
}

// exportPath is the path of a folder or host; "" is everything.
func (d *Data) exportPath(ctx context.Context, nodeID string) (string, error) {
	if nodeID == "" {
		return "", nil
	}
	n, err := findNode(ctx, d.ws, nodeID)
	if err != nil {
		return "", err
	}
	if n.Kind == model.KindSite {
		return "", errNotExportable
	}
	tree, err := d.ws.DB().LoadTree(ctx)
	if err != nil {
		return "", err
	}
	return tree.PathOf(n), nil
}

// ExportResult is the answer of Export: the file written, "" when the user cancelled.
type ExportResult struct {
	File    string   `json:"file"`
	Problem *Problem `json:"problem"`
}

// Export asks where to save, then writes the export of everything ("") or of a folder or host,
// readable by its owner only.
func (d *Data) Export(ctx context.Context, nodeID string, history bool) ExportResult {
	path, err := d.exportPath(ctx, nodeID)
	if err != nil {
		return ExportResult{Problem: ProblemOf(err)}
	}
	f, err := d.ws.Export(ctx, path, history)
	if err != nil {
		return ExportResult{Problem: ProblemOf(err)}
	}
	target, err := d.pick.SaveFile(exportName(path, d.now()))
	if err != nil || target == "" {
		return ExportResult{Problem: ProblemOf(err)}
	}
	if err := export.WriteFile(target, f); err != nil {
		return ExportResult{Problem: ProblemOf(err)}
	}
	return ExportResult{File: target}
}

// exportName suggests "vigie-2026-10-30.json", or with the name of the node exported
// ("vigie-infomaniak-2026-10-30.json").
func exportName(path string, at time.Time) string {
	name := "vigie"
	if path != "" {
		last := path[strings.LastIndex(path, "/")+1:]
		slug := strings.Join(strings.FieldsFunc(strings.ToLower(last), func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.')
		}), "-")
		if slug != "" {
			name += "-" + slug
		}
	}
	return name + "-" + at.Format("2006-01-02") + ".json"
}

// ImportPreview is what an import would do, shown before it does it.
type ImportPreview struct {
	Cancelled bool   `json:"cancelled"` // the user closed the file dialog
	Source    string `json:"source"`    // FileVigie, FileCSV or SourceAddOn
	File      string `json:"file"`      // the file's name
	// An export: when and by which Vigie it was written, with its history or not.
	Exported string `json:"exported"`
	App      string `json:"app"`
	History  bool   `json:"history"`
	// A CSV list: the lines read after the header, and their separator.
	Lines     int    `json:"lines"`
	Separator string `json:"separator"`
	// What an add-on wrote: its name and title, and its report word for word (also when it failed).
	AddOn      string `json:"addOn"`
	AddOnTitle string `json:"addOnTitle"`
	Report     string `json:"report"`

	Into    string          `json:"into"` // the folder it adds into; "" for the top
	Items   []ImportItem    `json:"items"`
	Keys    []ImportKeyView `json:"keys"`
	Added   int             `json:"added"`
	Problem *Problem        `json:"problem"`
}

// ImportItem is a folder, host or site of the file and what the import does with it, or a line of
// a CSV list that was refused.
type ImportItem struct {
	Kind   string `json:"kind"` // folder, host, site, line
	Path   string `json:"path"`
	Status string `json:"status"` // added, present, refused
	Reason string `json:"reason"` // workspace.ImportReason, or for a line exchange.CSVReason
	Detail string `json:"detail"` // why it is refused; for a line, its values

	Account    string `json:"account"`  // a host: its alias, or user@hostname:port
	SitePath   string `json:"sitePath"` // a site: its WordPress root
	Subsites   int    `json:"subsites"`
	Scans      int    `json:"scans"`
	Operations int    `json:"operations"`
	Line       int    `json:"line"`
	Value      string `json:"value"` // a line: the value at fault
}

// ImportKeyView is a key of the file for a host added, for the user to keep or leave out.
type ImportKeyView struct {
	HostID      string `json:"hostId"`
	Host        string `json:"host"`   // its path
	Server      string `json:"server"` // the server's name in known_hosts
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	Refused     string `json:"refused"` // workspace.KeyRefusal; "" when it can be kept
}

// ChooseImport asks for a file of a kind (FileVigie, FileCSV), reads it, and previews its import
// at the top of the site manager.
func (d *Data) ChooseImport(ctx context.Context, kind string) ImportPreview {
	if kind != FileVigie && kind != FileCSV {
		return ImportPreview{Problem: ProblemOf(errUnknownFile)}
	}
	file, err := d.pick.OpenFile(kind)
	if err != nil {
		return ImportPreview{Problem: ProblemOf(err)}
	}
	if file == "" {
		return ImportPreview{Cancelled: true}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ImportPreview{Problem: ProblemOf(err)}
	}
	p := pendingImport{source: ImportPreview{Source: kind, File: filepath.Base(file)}}
	if kind == FileVigie {
		f, err := export.Read(bytes.NewReader(data))
		if err != nil {
			return ImportPreview{Problem: ProblemOf(err)}
		}
		p.file = f
		p.source.Exported, p.source.App, p.source.History = f.Exported.UTC().Format(time.RFC3339), f.App, f.History != nil
	} else {
		f, report, err := exchange.ReadCSV(bytes.NewReader(data))
		if err != nil {
			return ImportPreview{Problem: ProblemOf(err)}
		}
		p.file = f
		p.source.Lines, p.source.Separator = report.Lines, report.Separator
		for _, l := range report.Refused {
			p.source.Items = append(p.source.Items, ImportItem{Kind: "line", Status: string(workspace.ImportRefused),
				Reason: string(l.Reason), Detail: l.Text, Line: l.Line, Value: l.Value})
		}
	}
	d.mu.Lock()
	d.pending = &p
	d.mu.Unlock()
	return d.PreviewImport(ctx, "")
}

// previewAddOn previews the import of what an add-on wrote, at the top of the site manager.
func (d *Data) previewAddOn(ctx context.Context, f export.File, name, title, release, report string) ImportPreview {
	p := pendingImport{file: f, source: ImportPreview{Source: SourceAddOn, AddOn: name, AddOnTitle: title, App: release,
		Exported: f.Exported.UTC().Format(time.RFC3339), Report: report}}
	d.mu.Lock()
	d.pending = &p
	d.mu.Unlock()
	return d.PreviewImport(ctx, "")
}

// PreviewImport previews the import being chosen into a folder ("" for the top).
func (d *Data) PreviewImport(ctx context.Context, intoID string) ImportPreview {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending == nil {
		return ImportPreview{Problem: ProblemOf(errNoImport)}
	}
	into, err := d.intoPath(ctx, intoID)
	if err != nil {
		return ImportPreview{Problem: ProblemOf(err)}
	}
	plan, err := d.ws.PlanImport(ctx, d.pending.file, into)
	if err != nil {
		return ImportPreview{Problem: ProblemOf(err)}
	}
	d.pending.plan = plan
	view := d.pending.source
	view.Into = intoID
	view.Items = append(itemsOf(plan, d.pending.file), view.Items...)
	view.Keys = keysOf(plan)
	view.Added = plan.Added()
	return view
}

// intoPath is the path of the folder an import adds into; "" is the top.
func (d *Data) intoPath(ctx context.Context, intoID string) (string, error) {
	if intoID == "" {
		return "", nil
	}
	n, err := findNode(ctx, d.ws, intoID)
	if err != nil {
		return "", err
	}
	if n.Kind != model.KindFolder {
		return "", workspace.ErrNotAFolder
	}
	tree, err := d.ws.DB().LoadTree(ctx)
	if err != nil {
		return "", err
	}
	return tree.PathOf(n), nil
}

func itemsOf(plan workspace.ImportPlan, f export.File) []ImportItem {
	hosts, sites := map[string]export.Host{}, map[string]export.Site{}
	for _, h := range f.Hosts {
		hosts[h.ID] = h
	}
	for _, s := range f.Sites {
		sites[s.ID] = s
	}
	scans, operations := map[string]int{}, map[string]int{}
	if f.History != nil {
		for _, s := range f.History.Scans {
			scans[s.Site]++
		}
		for _, o := range f.History.Operations {
			operations[o.Site]++
		}
	}
	items := []ImportItem{}
	for _, e := range plan.Entries {
		item := ImportItem{Kind: e.Kind.String(), Path: e.Path, Status: string(e.Status), Reason: string(e.Reason), Detail: e.Detail}
		switch e.Kind {
		case model.KindHost:
			item.Account = account(hosts[e.FileID])
		case model.KindSite:
			s := sites[e.FileID]
			item.SitePath, item.Subsites = s.Path, len(s.Subsites)
			if e.Status == workspace.ImportAdded {
				item.Scans, item.Operations = scans[e.FileID], operations[e.FileID]
			}
		}
		items = append(items, item)
	}
	return items
}

// account is how a host reaches its server: its alias, or user@hostname, with the port when not 22.
func account(h export.Host) string {
	if h.Alias != "" {
		return h.Alias
	}
	a := h.User + "@" + h.Hostname
	if h.Port != 0 && h.Port != 22 {
		a += ":" + strconv.Itoa(h.Port)
	}
	return a
}

func keysOf(plan workspace.ImportPlan) []ImportKeyView {
	keys := []ImportKeyView{}
	for _, k := range plan.Keys {
		keys = append(keys, ImportKeyView{HostID: k.HostID.String(), Host: k.Host, Server: k.Name, Type: k.Key.Type,
			Fingerprint: k.Fingerprint, Refused: string(k.Refused)})
	}
	return keys
}

// ImportResult is the answer of an import: how many folders, hosts and sites it added.
type ImportResult struct {
	Added   int      `json:"added"`
	Problem *Problem `json:"problem"`
}

// ApplyImport adds what the last preview showed, keeping the keys of the hosts given.
func (d *Data) ApplyImport(ctx context.Context, trustHostIDs []string) ImportResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending == nil {
		return ImportResult{Problem: ProblemOf(errNoImport)}
	}
	var trusted []model.ID
	for _, id := range trustHostIDs {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return ImportResult{Problem: ProblemOf(errNodeNotFound)}
		}
		trusted = append(trusted, parsed)
	}
	if err := d.ws.Import(ctx, d.pending.plan, trusted); err != nil {
		return ImportResult{Problem: ProblemOf(err)}
	}
	added := d.pending.plan.Added()
	d.pending = nil
	return ImportResult{Added: added}
}

// CancelImport forgets the import being previewed.
func (d *Data) CancelImport() {
	d.mu.Lock()
	d.pending = nil
	d.mu.Unlock()
}

// SSHConfigView lists the names of ~/.ssh/config, and how many patterns it left out.
type SSHConfigView struct {
	Hosts    []SSHAlias `json:"hosts"`
	Patterns int        `json:"patterns"`
	Problem  *Problem   `json:"problem"`
}

// SSHAlias is a name of ~/.ssh/config, the account ssh reaches through it, and the host already
// reaching it or why ssh cannot read it.
type SSHAlias struct {
	Alias   string `json:"alias"`
	Account string `json:"account"` // user@hostname:port
	Present string `json:"present"` // the path of the host already reaching it
	Error   string `json:"error"`
}

// SSHConfig reads ~/.ssh/config (ssh -G for each name); it connects to nothing.
func (d *Data) SSHConfig(ctx context.Context) SSHConfigView {
	config, err := d.ws.SSHConfigHosts(ctx)
	if err != nil {
		return SSHConfigView{Hosts: []SSHAlias{}, Problem: ProblemOf(err)}
	}
	view := SSHConfigView{Hosts: []SSHAlias{}, Patterns: config.Patterns}
	for _, h := range config.Hosts {
		a := SSHAlias{Alias: h.Alias, Present: h.Present, Error: h.Error}
		if h.Error == "" {
			a.Account = h.User + "@" + h.Hostname + ":" + strconv.Itoa(h.Port)
		}
		view.Hosts = append(view.Hosts, a)
	}
	return view
}

// ImportSSHConfig adds a host for each alias chosen, in a folder ("" for the top).
func (d *Data) ImportSSHConfig(ctx context.Context, aliases []string, intoID string) ImportResult {
	into, err := d.intoPath(ctx, intoID)
	if err != nil {
		return ImportResult{Problem: ProblemOf(err)}
	}
	plan, err := d.ws.PlanImport(ctx, workspace.SSHConfigFile(aliases), into)
	if err != nil {
		return ImportResult{Problem: ProblemOf(err)}
	}
	if err := d.ws.Import(ctx, plan, nil); err != nil {
		return ImportResult{Problem: ProblemOf(err)}
	}
	return ImportResult{Added: plan.Added()}
}
