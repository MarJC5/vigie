package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

// ErrNoSSHConfig is returned when the workspace was given no home to read ~/.ssh/config from.
var ErrNoSSHConfig = errors.New("no home directory to read ~/.ssh/config from")

// SSHConfigHost is a name of ~/.ssh/config with the account ssh reaches through it.
type SSHConfigHost struct {
	Alias    string
	Hostname string
	Port     int
	User     string
	Present  string // the path of a host that already reaches it; "" when none does
	Error    string // why ssh could not read its settings
}

// SSHConfigHosts lists the names of ~/.ssh/config and its includes, with the settings ssh uses
// for each (ssh -G), and the host already reaching it: the same alias, or the same hostname, port
// and user.
func (w *Workspace) SSHConfigHosts(ctx context.Context) ([]SSHConfigHost, error) {
	if w.deps.Home == "" || w.deps.Resolver == nil {
		return nil, ErrNoSSHConfig
	}
	names, err := sshx.ConfigNames(w.deps.Home)
	if err != nil {
		return nil, err
	}
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	var stored []model.Host
	tree.Walk(func(n model.Node, _ int) {
		if n.Kind == model.KindHost {
			h, _ := tree.Host(n.ID)
			stored = append(stored, h)
		}
	})
	out := make([]SSHConfigHost, 0, len(names))
	for _, name := range names {
		entry := SSHConfigHost{Alias: name}
		resolved, err := w.deps.Resolver.Resolve(ctx, name)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			entry.Error = err.Error()
		}
		entry.Hostname, entry.Port, entry.User = resolved.Hostname, resolved.Port, resolved.User
		for _, h := range stored {
			reaches := h.Alias == name || (err == nil && h.Alias == "" && strings.EqualFold(h.Hostname, resolved.Hostname) &&
				h.EffectivePort() == resolved.Port && h.User == resolved.User)
			if reaches {
				entry.Present = tree.PathOf(model.Node{Kind: model.KindHost, ID: h.ID})
				break
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// SSHConfigFile is a file to import holding a host for each alias, named after it: the import
// shows what it adds and adds it as it does any export.
func SSHConfigFile(aliases []string) exchange.File {
	f := exchange.File{Folders: []exchange.Folder{}, Hosts: []exchange.Host{}, Sites: []exchange.Site{}, Ignores: []exchange.Ignore{}}
	for i, alias := range aliases {
		f.Hosts = append(f.Hosts, exchange.Host{ID: model.NewID().String(), Name: alias, Alias: alias, Color: model.ColorNone.String(),
			Position: i, TrustedKeys: []exchange.HostKey{}})
	}
	return f
}
