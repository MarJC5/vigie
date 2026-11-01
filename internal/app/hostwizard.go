package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"uuid"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/addon/info"
	"github.com/MarJC5/vigie/internal/addons"
	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/hostcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// Problem is why a step failed. Code is the key of its text in the catalogs ("failure.timedOut",
// "problem.noWpCli"); Detail fills its {{message}}.
type Problem struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

var (
	errKeyNotReceived = errors.New("key not received from the server")
	errNodeNotFound   = errors.New("no folder, host or site with this ID")
	errTagNotFound    = errors.New("no site has this tag")
)

// problemCodes maps known errors to their text; ProblemOf checks them in this order.
var problemCodes = []struct {
	err  error
	code string
}{
	{model.ErrEmptyName, "problem.emptyName"},
	{model.ErrSlashInName, "problem.slashInName"},
	{model.ErrNoConnection, "problem.noConnection"},
	{model.ErrNoUser, "problem.noUser"},
	{model.ErrInvalidPort, "problem.invalidPort"},
	{model.ErrDuplicateName, "problem.duplicateName"},
	{model.ErrSiteMoved, "problem.siteMoved"},
	{model.ErrUnknownColor, "problem.unknownColor"},
	{model.ErrNotesTooLong, "problem.notesTooLong"},
	{model.ErrInvalidAddress, "problem.invalidAddress"},
	{model.ErrEmptyTag, "problem.emptyTag"},
	{model.ErrTagTooLong, "problem.tagTooLong"},
	{model.ErrTagCharacter, "problem.tagCharacter"},
	{model.ErrTooManyTags, "problem.tooManyTags"},
	{errTagNotFound, "problem.tagNotFound"},
	{store.ErrNotEmpty, "problem.folderNotEmpty"},
	{store.ErrBusy, "problem.busy"},
	{wpcli.ErrNoDumpTool, "problem.noDumpTool"},
	{wpcli.ErrCannotLoad, "problem.cannotLoad"},
	{errPreviewFirst, "problem.previewFirst"},
	{update.ErrNotOffered, "problem.notOffered"},
	{update.ErrNothingToRollBack, "problem.nothingToRollBack"},
	{store.ErrCycle, "problem.moveIntoItself"},
	{errNodeNotFound, "problem.notFound"},
	{model.ErrReasonRequired, "problem.reasonRequired"},
	{errScanning, "problem.scanning"},
	{workspace.ErrUnknownDestination, "problem.unknownDestination"},
	{model.ErrNotFound, "problem.folderNotFound"},
	{sshx.ErrNoHostKey, "problem.noHostKey"},
	{errKeyNotReceived, "problem.keyNotReceived"},
	{hostcheck.ErrNoPHP, "problem.noPhp"},
	{discover.ErrFind, "problem.findFailed"},
	{export.ErrNotAnExport, "problem.notAnExport"},
	{export.ErrNewerVersion, "problem.newerExport"},
	{export.ErrInvalid, "problem.invalidExport"},
	{exchange.ErrNotACSV, "problem.notACSV"},
	{errNotExportable, "problem.notExportable"},
	{errNoImport, "problem.noImport"},
	{workspace.ErrNotAFolder, "problem.notAFolder"},
	{workspace.ErrKeyChanged, "problem.keyChanged"},
	{store.ErrIDTaken, "problem.importChanged"},
	{workspace.ErrNoSSHConfig, "problem.noSSHConfig"},
	{sshx.ErrConfigDepth, "problem.sshConfigDepth"},
	{errNoAddOn, "problem.addonMissing"},
	{addons.ErrNotASource, "problem.addonNotASource"},
	{addons.ErrTimedOut, "problem.addonTimedOut"},
	{addons.ErrNoExport, "problem.addonNoExport"},
	{info.ErrNotAnInfo, "problem.addonInvalid"},
	{info.ErrNewerVersion, "problem.addonNewer"},
	{info.ErrInvalid, "problem.addonInvalid"},
}

// ProblemOf turns an error into a Problem the interface can translate; nil gives nil.
func ProblemOf(err error) *Problem {
	if err == nil {
		return nil
	}
	var connErr *sshx.ConnectionError
	if errors.As(err, &connErr) {
		return &Problem{Code: "failure." + string(connErr.Failure), Detail: connErr.Message}
	}
	for _, p := range problemCodes {
		if errors.Is(err, p.err) {
			return &Problem{Code: p.code, Detail: err.Error()}
		}
	}
	if _, ok := errors.AsType[*addons.Failed](err); ok {
		return &Problem{Code: "problem.addonFailed", Detail: err.Error()}
	}
	return &Problem{Code: "problem.error", Detail: err.Error()}
}

// HostDetails is what the user enters in the first step of the wizard.
type HostDetails struct {
	Name     string `json:"name"`
	Alias    string `json:"alias"` // an entry of ~/.ssh/config, instead of hostname, port and user
	Hostname string `json:"hostname"`
	Port     int    `json:"port"` // 22 when zero
	User     string `json:"user"`
	KeyPath  string `json:"keyPath"` // optional: only this key is offered
	WPCLI    string `json:"wpCli"`   // WP-CLI command; "wp" when empty
	FolderID string `json:"folderId"`
	// MaxConcurrent is the number of commands at a time on the account; 2 when zero.
	MaxConcurrent int `json:"maxConcurrent"`
}

// host is the model host the details describe, with a new ID.
func (d HostDetails) host() (model.Host, error) {
	h := model.Host{
		ID: model.NewID(), Name: strings.TrimSpace(d.Name), Alias: strings.TrimSpace(d.Alias),
		Hostname: strings.TrimSpace(d.Hostname), Port: d.Port, User: strings.TrimSpace(d.User),
		KeyPath: strings.TrimSpace(d.KeyPath), WPCLI: strings.TrimSpace(d.WPCLI), MaxConcurrent: d.MaxConcurrent,
	}
	if d.FolderID != "" {
		id, err := uuid.Parse(d.FolderID)
		if err != nil {
			return h, model.ErrNotFound
		}
		h.FolderID = id
	}
	return h, nil
}

// HostKey is a key the server presented, shown by its fingerprint.
type HostKey struct {
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
}

// KeysResult is the answer of HostKeys.
type KeysResult struct {
	Name    string    `json:"name"` // how the keys are recorded in known_hosts
	Keys    []HostKey `json:"keys"`
	Problem *Problem  `json:"problem"`
}

// TestResult is the answer of TestConnection.
type TestResult struct {
	PHPVersion   string   `json:"phpVersion"`
	WPCLI        string   `json:"wpCli"`
	WPCLIVersion string   `json:"wpCliVersion"`
	Problem      *Problem `json:"problem"`
}

// Install is a WordPress install found on the account.
type Install struct {
	Path      string `json:"path"`
	State     string `json:"state"` // new, already-added, not-installed, cannot-load
	Name      string `json:"name"`
	URL       string `json:"url"`
	Multisite bool   `json:"multisite"`
	Subsites  int    `json:"subsites"`
	Message   string `json:"message"`
}

// DiscoverResult is the answer of Discover.
type DiscoverResult struct {
	Installs  []Install `json:"installs"`
	Truncated bool      `json:"truncated"`
	Problem   *Problem  `json:"problem"`
}

// Choice is an install the user chose to add, with the name they gave it.
type Choice struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Multisite bool   `json:"multisite"`
}

// DiscoveryEvent carries a DiscoveryProgress to the window while Discover checks the installs found.
const DiscoveryEvent = "vigie:discovery"

// DiscoveryProgress tells how many installs were checked out of those found.
type DiscoveryProgress struct {
	Checked int `json:"checked"`
	Total   int `json:"total"`
}

// AddResult is the answer of Add.
type AddResult struct {
	HostID  string   `json:"hostId"`
	Problem *Problem `json:"problem"`
}

// HostWizard serves the "add a host" wizard. A key can only be trusted, and a host only added
// with it, when the server presented that key during this session: the interface cannot make
// Vigie trust a key of its own.
type HostWizard struct {
	ws   *workspace.Workspace
	emit func(name string, data any)

	mu       sync.Mutex
	received map[string]sshx.HostKey // by fingerprint, as the server presented them
	trusted  map[string]bool         // fingerprints the user trusted
}

// NewHostWizard returns the service over the workspace; emit sends an event to the window.
func NewHostWizard(ws *workspace.Workspace, emit func(name string, data any)) *HostWizard {
	return &HostWizard{ws: ws, emit: emit, received: map[string]sshx.HostKey{}, trusted: map[string]bool{}}
}

// Check validates the details: required fields, port, and a name free in the chosen folder.
func (w *HostWizard) Check(ctx context.Context, d HostDetails) *Problem {
	h, err := d.host()
	if err != nil {
		return ProblemOf(err)
	}
	if err := h.Validate(); err != nil {
		return ProblemOf(err)
	}
	tree, err := w.ws.DB().LoadTree(ctx)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(tree.AddHost(h))
}

// HostKeys asks the server for its keys.
func (w *HostWizard) HostKeys(ctx context.Context, d HostDetails) KeysResult {
	h, err := d.host()
	if err != nil {
		return KeysResult{Problem: ProblemOf(err)}
	}
	name, keys, err := w.ws.HostKeys(ctx, h)
	if err != nil {
		return KeysResult{Problem: ProblemOf(err)}
	}
	result := KeysResult{Name: name, Keys: []HostKey{}}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, k := range keys {
		fp, err := k.Fingerprint()
		if err != nil {
			return KeysResult{Problem: ProblemOf(err)}
		}
		w.received[fp] = k
		result.Keys = append(result.Keys, HostKey{Type: k.Type, Fingerprint: fp})
	}
	return result
}

// Trust records, under name, the key with this fingerprint, which the server must have presented.
func (w *HostWizard) Trust(name, fingerprint string) *Problem {
	w.mu.Lock()
	key, ok := w.received[fingerprint]
	w.mu.Unlock()
	if !ok {
		return ProblemOf(errKeyNotReceived)
	}
	if err := w.ws.Trust(name, key); err != nil {
		return ProblemOf(err)
	}
	w.mu.Lock()
	w.trusted[fingerprint] = true
	w.mu.Unlock()
	return nil
}

// TestConnection checks SSH, PHP and WP-CLI; ssh refuses the server until its key is trusted.
func (w *HostWizard) TestConnection(ctx context.Context, d HostDetails) TestResult {
	h, err := d.host()
	if err != nil {
		return TestResult{Problem: ProblemOf(err)}
	}
	report, err := w.ws.TestConnection(ctx, h)
	result := TestResult{PHPVersion: report.PHPVersion, WPCLI: report.WPCLI, WPCLIVersion: report.WPCLIVersion}
	if errors.Is(err, hostcheck.ErrNoWPCLI) {
		result.Problem = &Problem{Code: "problem.noWpCli", Detail: strings.Join(report.Tried, ", ")}
		return result
	}
	result.Problem = ProblemOf(err)
	return result
}

// Discover searches the account for WordPress installs, sending DiscoveryEvent as it checks
// them; it adds nothing.
func (w *HostWizard) Discover(ctx context.Context, d HostDetails) DiscoverResult {
	h, err := d.host()
	if err != nil {
		return DiscoverResult{Problem: ProblemOf(err)}
	}
	found, truncated, err := w.ws.DiscoverAccount(ctx, h, func(checked, total int) {
		w.emit(DiscoveryEvent, DiscoveryProgress{Checked: checked, Total: total})
	})
	return discovered(found, truncated, err)
}

// DiscoverHost searches the account of a stored host for installs, its own sites marked as
// already added, sending DiscoveryEvent as it checks them; it adds nothing.
func (w *HostWizard) DiscoverHost(ctx context.Context, hostID string) DiscoverResult {
	n, err := findNode(ctx, w.ws, hostID)
	if err != nil {
		return DiscoverResult{Problem: ProblemOf(err)}
	}
	if n.Kind != model.KindHost {
		return DiscoverResult{Problem: ProblemOf(errNodeNotFound)}
	}
	found, truncated, err := w.ws.DiscoverHost(ctx, n.ID, func(checked, total int) {
		w.emit(DiscoveryEvent, DiscoveryProgress{Checked: checked, Total: total})
	})
	return discovered(found, truncated, err)
}

func discovered(found []discover.Found, truncated bool, err error) DiscoverResult {
	if err != nil {
		return DiscoverResult{Problem: ProblemOf(err)}
	}
	result := DiscoverResult{Installs: make([]Install, len(found)), Truncated: truncated}
	for i, f := range found {
		result.Installs[i] = Install{Path: f.Path, State: string(f.State), Name: f.Name, URL: f.URL,
			Multisite: f.Multisite, Subsites: f.Subsites, Message: f.Message}
	}
	return result
}

// AddSites adds the installs chosen to a stored host, after its own sites, all or nothing.
func (w *HostWizard) AddSites(ctx context.Context, hostID string, choices []Choice) *Problem {
	n, err := findNode(ctx, w.ws, hostID)
	if err != nil {
		return ProblemOf(err)
	}
	if n.Kind != model.KindHost {
		return ProblemOf(errNodeNotFound)
	}
	tree, err := w.ws.DB().LoadTree(ctx)
	if err != nil {
		return ProblemOf(err)
	}
	after := len(tree.SitesUnder(n))
	return ProblemOf(w.ws.AddSites(ctx, n.ID, sitesOf(n.ID, choices, after)))
}

// KeepKey records, for a stored host whose address changed, the key the user trusted: the server
// must have presented it in this session, as in Add.
func (w *HostWizard) KeepKey(ctx context.Context, hostID, fingerprint string) *Problem {
	w.mu.Lock()
	trusted := w.trusted[fingerprint]
	w.mu.Unlock()
	if !trusted {
		return ProblemOf(errKeyNotReceived)
	}
	n, err := findNode(ctx, w.ws, hostID)
	if err != nil {
		return ProblemOf(err)
	}
	if n.Kind != model.KindHost {
		return ProblemOf(errNodeNotFound)
	}
	return ProblemOf(w.ws.SetHostKey(ctx, n.ID, fingerprint))
}

// sitesOf turns the installs chosen into sites of a host, placed from position first.
func sitesOf(hostID model.ID, choices []Choice, first int) []model.Site {
	sites := make([]model.Site, len(choices))
	for i, c := range choices {
		sites[i] = model.Site{
			ID: model.NewID(), HostID: hostID, Name: strings.TrimSpace(c.Name), Path: c.Path, URL: c.URL,
			Multisite: c.Multisite, Position: first + i,
		}
	}
	return sites
}

// Add stores the host, with the fingerprint the user trusted, and the installs chosen.
func (w *HostWizard) Add(ctx context.Context, d HostDetails, fingerprint string, choices []Choice) AddResult {
	w.mu.Lock()
	trusted := w.trusted[fingerprint]
	w.mu.Unlock()
	if !trusted {
		return AddResult{Problem: ProblemOf(errKeyNotReceived)}
	}
	h, err := d.host()
	if err != nil {
		return AddResult{Problem: ProblemOf(err)}
	}
	h.HostKeyFingerprint = fingerprint
	if err := w.ws.AddHost(ctx, h, sitesOf(h.ID, choices, 0)); err != nil {
		return AddResult{Problem: ProblemOf(err)}
	}
	return AddResult{HostID: h.ID.String()}
}
