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

// Workspace holds the database and the SSH runner.
type Workspace struct {
	db      *store.DB
	runner  sshx.Runner
	limiter *sshx.Limiter
	now     func() time.Time
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
	ws := New(db, sshx.ExecRunner{Options: options, Timeout: commandTimeout}, time.Now)
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

// New returns a workspace over an open database and a runner (tests use a fake runner).
func New(db *store.DB, runner sshx.Runner, now func() time.Time) *Workspace {
	return &Workspace{db: db, runner: runner, limiter: sshx.NewLimiter(), now: now}
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
}

// Scan scans the site at path, the sites of a host, or every site under a folder ("" for all),
// stores each scan (also a failed or cancelled one) and the subsites of networks, and returns the
// scans in tree order.
func (w *Workspace) Scan(ctx context.Context, path string, progress func(scan.Progress)) ([]Scanned, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	node := model.Root
	if path != "" {
		if node, err = tree.Resolve(path); err != nil {
			return nil, err
		}
	}
	sites := tree.SitesUnder(node)
	scanner := scan.Scanner{Runner: w.runner, Limiter: w.limiter, Now: w.now}

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
			Max: maxConcurrent(host), WPCLI: tree.EffectiveWPCLI(site.ID),
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
	for i, r := range results {
		if err := w.db.SaveScan(keep, r.Scan); err != nil {
			return results, err
		}
		if sites[i].Multisite && r.Scan.Outcome == model.ScanCompleted {
			if err := w.db.SaveSubsites(keep, sites[i].ID, r.Subsites); err != nil {
				return results, err
			}
		}
	}
	return results, nil
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
	d := discover.Discoverer{Runner: w.runner, Limiter: w.limiter}
	wpcli := host.WPCLI
	if wpcli == "" {
		wpcli = "wp"
	}
	return d.Host(ctx, discover.Account{
		Target: sshx.TargetFor(host), HostKey: host.ID.String(), Max: maxConcurrent(host), WPCLI: wpcli,
	}, tree.SitesUnder(node))
}

// maxConcurrent is the host's limit of commands at a time, 2 by default.
func maxConcurrent(h model.Host) int {
	if h.MaxConcurrent > 0 {
		return h.MaxConcurrent
	}
	return 2
}
