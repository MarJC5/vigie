package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/buildinfo"
	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

// ErrNotExportable is returned for a site: an export holds everything, a folder or a host.
var ErrNotExportable = errors.New("only everything, a folder or a host is exported")

// Export reads the site manager at path ("" for everything, a folder or a host) into an export,
// with the keys trusted for its hosts, the ignores of its sites, and their history when asked.
func (w *Workspace) Export(ctx context.Context, path string, history bool) (exchange.File, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return exchange.File{}, err
	}
	scope := model.Root
	if path != "" {
		if scope, err = tree.Resolve(path); err != nil {
			return exchange.File{}, err
		}
	}
	if scope.Kind == model.KindSite {
		return exchange.File{}, fmt.Errorf("%q: %w", path, ErrNotExportable)
	}
	f := exchange.File{Exported: w.deps.Now().UTC().Truncate(time.Second), App: buildinfo.Version(), Scope: tree.PathOf(scope),
		Folders: []exchange.Folder{}, Hosts: []exchange.Host{}, Sites: []exchange.Site{}, Ignores: []exchange.Ignore{}}
	if path == "" {
		f.Scope = ""
	}
	// What is under the scope, and the scope itself; a parent outside the export becomes "".
	in := map[model.ID]bool{}
	under := func(n model.Node) bool {
		p := tree.PathOf(n)
		return f.Scope == "" || p == f.Scope || strings.HasPrefix(p, f.Scope+"/")
	}
	inside := func(id model.ID) string {
		if in[id] {
			return id.String()
		}
		return ""
	}
	var sites []model.Site
	tree.Walk(func(n model.Node, _ int) {
		if !under(n) {
			return
		}
		in[n.ID] = true
		switch n.Kind {
		case model.KindFolder:
			folder, _ := tree.Folder(n.ID)
			f.Folders = append(f.Folders, exchange.FolderOf(folder, inside(folder.ParentID)))
		case model.KindHost:
			host, _ := tree.Host(n.ID)
			f.Hosts = append(f.Hosts, exchange.HostOf(host, inside(host.FolderID), w.trustedKeys(ctx, host)))
		case model.KindSite:
			site, _ := tree.Site(n.ID)
			sites = append(sites, site)
			f.Sites = append(f.Sites, exchange.SiteOf(site))
		}
	})
	if history {
		f.History = &exchange.History{Scans: []exchange.Scan{}, Operations: []exchange.Operation{}}
	}
	for _, site := range sites {
		ignores, err := w.db.Ignores(ctx, site.ID)
		if err != nil {
			return exchange.File{}, err
		}
		for _, i := range ignores {
			f.Ignores = append(f.Ignores, exchange.IgnoreOf(i))
		}
		if !history {
			continue
		}
		scans, err := w.db.Scans(ctx, site.ID)
		if err != nil {
			return exchange.File{}, err
		}
		for _, s := range scans {
			f.History.Scans = append(f.History.Scans, exchange.ScanOf(s))
		}
		ops, err := w.db.Operations(ctx, site.ID)
		if err != nil {
			return exchange.File{}, err
		}
		// Stored the newest first; an export keeps the order they were made in.
		for i := len(ops) - 1; i >= 0; i-- {
			f.History.Operations = append(f.History.Operations, exchange.OperationOf(ops[i]))
		}
	}
	return f, nil
}

// trustedKeys returns the keys of Vigie's known_hosts for a host, the one the host trusted when
// its fingerprint is known. A host whose name cannot be read (an alias ssh does not resolve any
// more) gets none: it asks again at its first connection after an import.
func (w *Workspace) trustedKeys(ctx context.Context, h model.Host) []exchange.HostKey {
	name := sshx.KnownHostsName(h.Hostname, h.EffectivePort())
	if h.Alias != "" {
		if w.deps.Resolver == nil {
			return nil
		}
		resolved, err := w.deps.Resolver.Resolve(ctx, h.Alias)
		if err != nil {
			return nil
		}
		name = resolved.KnownHostsName()
	}
	keys, err := sshx.TrustedKeys(w.deps.KnownHosts, name)
	if err != nil {
		return nil
	}
	var out []exchange.HostKey
	for _, k := range keys {
		if fp, err := k.Fingerprint(); err == nil && (h.HostKeyFingerprint == "" || fp == h.HostKeyFingerprint) {
			out = append(out, exchange.HostKey{Name: name, Type: k.Type, Key: k.Key})
		}
	}
	return out
}
