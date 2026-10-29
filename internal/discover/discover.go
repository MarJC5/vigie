// Package discover finds the WordPress installs of an SSH account: one search of the home
// directory, then a check of each candidate with WP-CLI. It only reads; adding the installs found
// is the user's choice.
package discover

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

const (
	// MaxCandidates caps the installs checked after one search.
	MaxCandidates = 200
	// findTimeout bounds the search of a large account.
	findTimeout = 60 * time.Second
)

// ErrFind is returned when the search itself failed (for example a find without -maxdepth).
var ErrFind = errors.New("search for WordPress installs failed")

// FindCommand lists the wp-includes/version.php files under the home directory, six levels deep
// at most, skipping dependencies, version control, caches, backups and uploads. It looks for
// version.php, not wp-config.php, which WordPress also reads one level above the install.
func FindCommand() string {
	remote, err := sshx.Quote("find", ".", "-maxdepth", "6",
		"(", "-name", "node_modules", "-o", "-name", ".git", "-o", "-name", "cache", "-o", "-name", "backup*",
		"-o", "-path", "*/wp-content/uploads", ")", "-prune",
		"-o", "-type", "f", "-path", "*/wp-includes/version.php", "-print")
	if err != nil {
		panic(err) // fixed arguments
	}
	return remote
}

const versionFile = "/wp-includes/version.php"

// ParseFind returns the WordPress roots found, as paths under the home directory ("~/sites/a"),
// sorted, at most MaxCandidates; truncated tells that more were found. Lines that are not a
// version.php under the home directory (messages, odd names) are skipped.
func ParseFind(out []byte) (roots []string, truncated bool) {
	seen := map[string]bool{}
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSuffix(line, "\n")
		if !strings.HasPrefix(line, "./") || !strings.HasSuffix(line, versionFile) {
			continue
		}
		rel := path.Clean(strings.TrimSuffix(line, versionFile))
		if rel == ".." || strings.HasPrefix(rel, "../") || strings.ContainsAny(rel, "\r\x00") {
			continue
		}
		root := "~/" + rel // "~/." for an install in the home directory itself
		if !seen[root] {
			seen[root] = true
			roots = append(roots, root)
		}
	}
	slices.Sort(roots)
	if len(roots) > MaxCandidates {
		return roots[:MaxCandidates], true
	}
	return roots, false
}

// State is what discovery tells about a candidate.
type State string

const (
	StateNew          State = "new"           // a working install, not in Vigie yet
	StateAlreadyAdded State = "already-added" // a site of this host has this path
	StateNotInstalled State = "not-installed" // WordPress files without an installed site
	StateCannotLoad   State = "cannot-load"   // WP-CLI could not load WordPress
)

// Found is one candidate install.
type Found struct {
	Path      string
	State     State
	Message   string // WP-CLI's message when it cannot load WordPress
	Name      string // suggested name: the address's host, else the folder name
	URL       string
	Multisite bool
	Subsites  int
}

// Account is the SSH account to search.
type Account struct {
	Target  sshx.Target
	HostKey string // shares the host's limit of commands at a time (the host ID)
	Max     int
	WPCLI   string
}

// Discoverer searches accounts.
type Discoverer struct {
	Runner  sshx.Runner
	Limiter *sshx.Limiter
}

// Host searches an account; existing are the sites of that host already in Vigie.
func (d Discoverer) Host(ctx context.Context, account Account, existing []model.Site) ([]Found, bool, error) {
	findCtx, cancel := context.WithTimeout(ctx, findTimeout)
	defer cancel()
	res, err := d.run(findCtx, account, FindCommand())
	if err != nil {
		return nil, false, err
	}
	roots, truncated := ParseFind(res.Stdout)
	// find exits 1 when some folders cannot be read; what it printed is still valid.
	if res.ExitCode != 0 && len(roots) == 0 {
		return nil, false, fmt.Errorf("%w: %s", ErrFind, strings.TrimSpace(string(res.Stderr)))
	}

	added := map[string]string{}
	for _, s := range existing {
		added[cleanPath(s.Path)] = s.Name
	}
	found := make([]Found, 0, len(roots))
	for _, root := range roots {
		if name, ok := added[cleanPath(root)]; ok {
			found = append(found, Found{Path: root, State: StateAlreadyAdded, Name: name})
			continue
		}
		f, err := d.check(ctx, account, root)
		if err != nil {
			return found, truncated, err
		}
		found = append(found, f)
	}
	return found, truncated, nil
}

// check reads one candidate: installed, a network, its address.
func (d Discoverer) check(ctx context.Context, account Account, root string) (Found, error) {
	f := Found{Path: root, Name: path.Base(strings.TrimPrefix(root, "~/"))}
	res, err := d.wp(ctx, account, wpcli.CoreIsInstalled(false), root)
	if err != nil {
		return f, err
	}
	installed, err := wpcli.IsInstalled(res)
	switch {
	case errors.Is(err, wpcli.ErrCannotLoad):
		f.State, f.Message = StateCannotLoad, strings.TrimSpace(string(res.Stderr))
		return f, nil
	case err != nil:
		return f, err
	case !installed:
		f.State = StateNotInstalled
		return f, nil
	}
	f.State = StateNew

	if res, err = d.wp(ctx, account, wpcli.CoreIsInstalled(true), root); err != nil {
		return f, err
	}
	if f.Multisite, err = wpcli.IsInstalled(res); err != nil {
		return f, err
	}
	if res, err = d.wp(ctx, account, wpcli.OptionGet("home"), root); err != nil {
		return f, err
	}
	if err := wpcli.Check(res); err != nil {
		return f, err
	}
	if f.URL, err = wpcli.ParseHome(res.Stdout); err != nil {
		return f, err
	}
	if u, err := url.Parse(f.URL); err == nil && u.Host != "" {
		f.Name = u.Host
	}
	if !f.Multisite {
		return f, nil
	}
	if res, err = d.wp(ctx, account, wpcli.SiteList(), root); err != nil {
		return f, err
	}
	if err := wpcli.Check(res); err != nil {
		return f, err
	}
	subsites, err := wpcli.ParseSites(res.Stdout)
	f.Subsites = len(subsites)
	return f, err
}

func (d Discoverer) wp(ctx context.Context, account Account, cmd wpcli.Command, root string) (sshx.Result, error) {
	remote, err := cmd.Remote(account.WPCLI, root)
	if err != nil {
		return sshx.Result{}, err
	}
	return d.run(ctx, account, remote)
}

func (d Discoverer) run(ctx context.Context, account Account, remote string) (sshx.Result, error) {
	var res sshx.Result
	err := d.Limiter.Do(ctx, account.HostKey, account.Max, func() error {
		var runErr error
		res, runErr = d.Runner.Run(ctx, account.Target, remote)
		return runErr
	})
	return res, err
}

// cleanPath compares paths whatever their trailing slash.
func cleanPath(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return "~/" + path.Clean(rest)
	}
	return path.Clean(p)
}
