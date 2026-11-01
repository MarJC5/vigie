// Package workspace is Vigie at work: the user's data and the means to reach their servers. The
// command line and the window both go through it, so a scan is the same everywhere.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
)

// ErrNotAHost is returned when discovery is asked for a folder or a site.
var ErrNotAHost = errors.New("not a host")

// parallelSites caps the sites scanned at the same time across hosts; each host keeps its own
// limit of commands at a time.
const parallelSites = 8

// SSH settings shared by every command.
var sshOptions = sshx.Options{
	ConnectTimeout:      10 * time.Second,
	ServerAliveInterval: 15 * time.Second,
	ControlPersist:      60 * time.Second,
}

// commandTimeout bounds one WP-CLI command; a plugin list asks wordpress.org about each plugin.
const commandTimeout = 2 * time.Minute

// KeyScanner asks a server for its host keys (sshx.KeyScanner).
type KeyScanner interface {
	Scan(ctx context.Context, host string, port int) ([]sshx.HostKey, error)
}

// Resolver reads the configuration of an alias of ~/.ssh/config (sshx.ExecRunner).
type Resolver interface {
	Resolve(ctx context.Context, alias string) (sshx.Resolved, error)
}

// Deps are the means to reach servers; Open sets the real ones, tests replace them.
type Deps struct {
	Runner     sshx.Runner
	KeyScanner KeyScanner
	Resolver   Resolver
	KnownHosts string // Vigie's own known_hosts file
	Home       string // the user's home, where ~/.ssh/config is read
	Now        func() time.Time
	// Updates makes an updater that reports its steps to onStep, replacing the real one in tests;
	// Checker replaces the real one's checks.
	Updates func(onStep func(model.Operation, model.OperationStep)) Updates
	Checker update.Checker
	// Lookups makes the outside lookups of a scan; nil makes none, and their checks are "not
	// checked".
	Lookups scan.Lookups
}

// Workspace holds the database and the means to reach servers.
type Workspace struct {
	db      *store.DB
	deps    Deps
	limiter *sshx.Limiter
	close   func() error
}

// Open opens the data directory (created 0700 if needed), migrates the database, and sets ssh up
// with Vigie's own known_hosts and a connection sharing directory in the user cache directory.
func Open(ctx context.Context, dataDir, version string) (*Workspace, error) {
	if err := store.EnsureDataDir(dataDir); err != nil {
		return nil, err
	}
	db, err := store.Open(ctx, filepath.Join(dataDir, "vigie.db"))
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, store.Migrations(), version); err != nil {
		db.Close()
		return nil, err
	}
	options, err := sshSettings(dataDir)
	if err != nil {
		db.Close()
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		db.Close()
		return nil, err
	}
	runner := sshx.ExecRunner{Options: options, Timeout: commandTimeout}
	ws := New(db, Deps{
		Runner: runner, KeyScanner: sshx.KeyScanner{}, Resolver: runner,
		KnownHosts: options.KnownHostsFile, Home: home, Now: time.Now,
	})
	ws.deps.Lookups = newLookups(db, version, time.Now, ws.LookupAllowed)
	ws.close = db.Close
	return ws, nil
}

func sshSettings(dataDir string) (sshx.Options, error) {
	options := sshOptions
	cache, err := os.UserCacheDir()
	if err != nil {
		return options, err
	}
	if options.ControlDir, err = sshx.ControlDir(cache); err != nil {
		return options, err
	}
	if err := os.MkdirAll(options.ControlDir, 0o700); err != nil {
		return options, err
	}
	options.KnownHostsFile, err = sshx.KnownHostsFile(dataDir)
	return options, err
}

// New returns a workspace over an open database.
func New(db *store.DB, deps Deps) *Workspace {
	return &Workspace{db: db, deps: deps, limiter: sshx.NewLimiter()}
}

// DB is the workspace's database.
func (w *Workspace) DB() *store.DB { return w.db }

// Close closes what Open opened.
func (w *Workspace) Close() error {
	if w.close == nil {
		return nil
	}
	return w.close()
}

// Scanned is the scan of one site, with the site's path in the tree and, for a network, the
// subsites read.
type Scanned struct {
	Path     string
	Scan     model.Scan
	Subsites []model.Subsite
	// Changes are what the scan found changed since the site's complete scan before (ADR-0027).
	Changes []model.Change
}

// ScanOptions change what a scan does.
type ScanOptions struct {
	// CheckFiles compares the files with wordpress.org's even when the last comparison is less
	// than a day old.
	CheckFiles bool
	// By is who started the scan, recorded with its run; the user when empty.
	By model.RunBy
}

// Scan scans the site at path, the sites of a host, or every site under a folder ("" for all),
// stores each scan (also a failed or cancelled one) and the subsites of networks, and returns the
// scans in tree order, with the findings the ignores of each site cover marked.
func (w *Workspace) Scan(ctx context.Context, path string, opts ScanOptions, progress func(scan.Progress)) ([]Scanned, error) {
	node := model.Root
	if path != "" {
		tree, err := w.db.LoadTree(ctx)
		if err != nil {
			return nil, err
		}
		if node, err = tree.Resolve(path); err != nil {
			return nil, err
		}
	}
	return w.ScanNode(ctx, node, opts, progress)
}

// ScanNode is Scan for a node of the tree (model.Root for all).
func (w *Workspace) ScanNode(ctx context.Context, node model.Node, opts ScanOptions, progress func(scan.Progress)) ([]Scanned, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	sites := tree.SitesUnder(node)
	latest, err := w.db.LatestScans(ctx)
	if err != nil {
		return nil, err
	}
	// Each scan of a node is a run, for the Activity page.
	run := model.Run{ID: model.NewID(), By: opts.By, Started: w.deps.Now().UTC(), Outcome: model.RunRunning, Sites: len(sites)}
	if run.By == "" {
		run.By = model.RunByUser
	}
	if node != model.Root {
		run.Path = tree.PathOf(node)
	}
	if err := w.db.StartRun(ctx, run); err != nil {
		return nil, err
	}
	scanner := w.scanner()

	var mu sync.Mutex // progress comes from several scans at once
	report := func(p scan.Progress) {
		if progress != nil {
			mu.Lock()
			defer mu.Unlock()
			progress(p)
		}
	}
	results := make([]Scanned, len(sites))
	slots := make(chan struct{}, parallelSites)
	var wg sync.WaitGroup
	for i, site := range sites {
		host, _ := tree.Host(site.HostID)
		job := scan.Job{
			Site: site, Target: sshx.TargetFor(host), HostKey: host.ID.String(),
			Max: maxConcurrent(host), WPCLI: tree.EffectiveWPCLI(site.ID), Previous: latest[site.ID],
			CheckFiles: opts.CheckFiles,
		}
		results[i].Path = tree.PathOf(model.Node{Kind: model.KindSite, ID: site.ID})
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			results[i].Scan, results[i].Subsites = scanner.Site(ctx, job, report)
		})
	}
	wg.Wait()

	// Kept even when the scan was cancelled: the history shows it.
	keep := context.WithoutCancel(ctx)
	saved := w.saveScans(keep, run.ID, sites, latest, results)
	ended := model.RunCompleted
	if ctx.Err() != nil {
		ended = model.RunCancelled
	}
	if err := errors.Join(saved, w.db.EndRun(keep, run.ID, w.deps.Now().UTC(), ended)); err != nil {
		return results, err
	}
	// The schedule of automatic scans counts from the end of the latest complete scan of everything.
	if node == model.Root && ctx.Err() == nil {
		if err := w.db.SetSetting(keep, everythingScannedSetting, w.deps.Now().UTC().Format(time.RFC3339)); err != nil {
			return results, err
		}
	}
	return results, nil
}

// saveScans stores the scans of a run with their subsites, each prepared first.
func (w *Workspace) saveScans(ctx context.Context, run model.ID, sites []model.Site, latest map[model.ID]model.Scan, results []Scanned) error {
	for i := range results {
		results[i].Scan.RunID = run
		previous, ok := latest[sites[i].ID]
		if err := w.prepare(ctx, &results[i], previous, ok); err != nil {
			return err
		}
		r := results[i]
		if err := w.db.SaveScan(ctx, r.Scan); err != nil {
			return err
		}
		if sites[i].Multisite && r.Scan.Outcome == model.ScanCompleted {
			if err := w.db.SaveSubsites(ctx, sites[i].ID, r.Subsites); err != nil {
				return err
			}
		}
	}
	return nil
}

// prepare readies a scan to be stored: what changed since its site's complete scan before, with
// Vigie's operations on the site (ADR-0027), its site's ignores applied, and what it tells of itself
// noted for the journal (ADR-0030).
func (w *Workspace) prepare(ctx context.Context, r *Scanned, previous model.Scan, hasPrevious bool) error {
	if hasPrevious && r.Scan.Outcome == model.ScanCompleted {
		ops, err := w.db.Operations(ctx, r.Scan.SiteID)
		if err != nil {
			return err
		}
		r.Changes = model.Changes(previous, r.Scan, ops)
	}
	ignores, err := w.db.Ignores(ctx, r.Scan.SiteID)
	if err != nil {
		return err
	}
	r.Scan.ApplyIgnores(ignores)
	r.Scan.Recorded, r.Scan.RecordedStatus, r.Scan.RecordedChanges = true, r.Scan.Status(), r.Changes
	return nil
}

// Prune keeps the history in check (ADR-0031), after a run; a failure is left for the next one.
func (w *Workspace) Prune(ctx context.Context) (store.PruneReport, error) {
	return w.db.Prune(ctx, w.deps.Now())
}

// everythingScannedSetting holds when the latest complete scan of everything ended.
const everythingScannedSetting = "scan.everything"

// EverythingScanned returns when the latest complete scan of everything ended, if there was one.
func (w *Workspace) EverythingScanned(ctx context.Context) (time.Time, bool, error) {
	value, ok, err := w.db.Setting(ctx, everythingScannedSetting)
	if err != nil || !ok {
		return time.Time{}, false, err
	}
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%s: %w", everythingScannedSetting, err)
	}
	return at, true, nil
}

// Discover searches the account of the host at path for WordPress installs; it adds nothing.
func (w *Workspace) Discover(ctx context.Context, path string) ([]discover.Found, bool, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, false, err
	}
	node, err := tree.Resolve(path)
	if err != nil {
		return nil, false, err
	}
	if node.Kind != model.KindHost {
		return nil, false, fmt.Errorf("%q: %w", path, ErrNotAHost)
	}
	host, _ := tree.Host(node.ID)
	return w.discoverAccount(ctx, host, tree.SitesUnder(node), nil)
}

// discoverAccount searches the account of a host, stored or not; existing are its sites in Vigie.
func (w *Workspace) discoverAccount(ctx context.Context, host model.Host, existing []model.Site, progress func(checked, total int)) ([]discover.Found, bool, error) {
	d := discover.Discoverer{Runner: w.deps.Runner, Limiter: w.limiter, Progress: progress}
	return d.Host(ctx, discover.Account{
		Target: sshx.TargetFor(host), HostKey: host.ID.String(), Max: maxConcurrent(host), WPCLI: hostWPCLI(host),
	}, existing)
}

// hostWPCLI is the host's WP-CLI command, "wp" by default.
func hostWPCLI(h model.Host) string {
	if h.WPCLI != "" {
		return h.WPCLI
	}
	return "wp"
}

// maxConcurrent is the host's limit of commands at a time, 2 by default.
func maxConcurrent(h model.Host) int {
	if h.MaxConcurrent > 0 {
		return h.MaxConcurrent
	}
	return 2
}
