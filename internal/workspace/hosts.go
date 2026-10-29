package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/hostcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

// ErrSiteOfAnotherHost is returned when a site given to AddHost belongs to another host.
var ErrSiteOfAnotherHost = errors.New("site of another host")

// HostKeys asks the server of a host (not stored yet) for its keys, and returns them with the
// name ssh looks them up by in known_hosts. An alias is resolved by ssh itself first.
func (w *Workspace) HostKeys(ctx context.Context, h model.Host) (string, []sshx.HostKey, error) {
	resolved := sshx.Resolved{Hostname: h.Hostname, Port: h.EffectivePort(), User: h.User}
	if h.Alias != "" {
		var err error
		if resolved, err = w.deps.Resolver.Resolve(ctx, h.Alias); err != nil {
			return "", nil, err
		}
	}
	keys, err := w.deps.KeyScanner.Scan(ctx, resolved.Hostname, resolved.Port)
	if err != nil {
		return "", nil, err
	}
	return resolved.KnownHostsName(), keys, nil
}

// Trust records a key the user accepted in Vigie's own known_hosts.
func (w *Workspace) Trust(name string, key sshx.HostKey) error {
	if w.deps.KnownHosts == "" {
		return fmt.Errorf("trust %s: no known_hosts file configured", name)
	}
	return sshx.Trust(w.deps.KnownHosts, name, key)
}

// TestConnection checks SSH, PHP and WP-CLI on a host that is not stored yet.
func (w *Workspace) TestConnection(ctx context.Context, h model.Host) (hostcheck.Report, error) {
	return hostcheck.Check(ctx, w.deps.Runner, sshx.TargetFor(h), h.WPCLI)
}

// DiscoverAccount searches the account of a host that is not stored yet; progress, when set, is
// told how many installs were checked out of those found.
func (w *Workspace) DiscoverAccount(ctx context.Context, h model.Host, progress func(checked, total int)) ([]discover.Found, bool, error) {
	return w.discoverAccount(ctx, h, nil, progress)
}

// AddHost stores a new host and the sites chosen, all or nothing.
func (w *Workspace) AddHost(ctx context.Context, h model.Host, sites []model.Site) error {
	for _, s := range sites {
		if s.HostID != h.ID {
			return fmt.Errorf("%q: %w", s.Name, ErrSiteOfAnotherHost)
		}
	}
	return w.db.AddHostWithSites(ctx, h, sites)
}
