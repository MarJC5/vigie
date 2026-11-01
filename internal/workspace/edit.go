package workspace

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
)

// CreateFolder stores a new folder under parent (model.NoID: at the root).
func (w *Workspace) CreateFolder(ctx context.Context, parent model.ID, name string) (model.Folder, error) {
	f := model.Folder{ID: model.NewID(), ParentID: parent, Name: strings.TrimSpace(name)}
	return f, w.db.SaveFolder(ctx, f)
}

// Rename gives a folder, host or site a new name, free among its siblings.
func (w *Workspace) Rename(ctx context.Context, n model.Node, name string) error {
	name = strings.TrimSpace(name)
	return w.edit(ctx, n,
		func(f *model.Folder) { f.Name = name },
		func(h *model.Host) { h.Name = name },
		func(s *model.Site) { s.Name = name })
}

// Move puts a folder or a host in another folder (model.NoID: at the root). A site stays under
// its host.
func (w *Workspace) Move(ctx context.Context, n model.Node, folder model.ID) error {
	if n.Kind == model.KindSite {
		return model.ErrSiteMoved
	}
	return w.edit(ctx, n,
		func(f *model.Folder) { f.ParentID = folder },
		func(h *model.Host) { h.FolderID = folder },
		nil)
}

// SetColor gives a folder, host or site its own color (model.ColorNone: inherit it).
func (w *Workspace) SetColor(ctx context.Context, n model.Node, c model.Color) error {
	return w.edit(ctx, n,
		func(f *model.Folder) { f.Color = c },
		func(h *model.Host) { h.Color = c },
		func(s *model.Site) { s.Color = c })
}

// SetNotes writes the Markdown notes of a folder, host or site, and nothing else: at most
// model.MaxNotes characters, line breaks as "\n".
func (w *Workspace) SetNotes(ctx context.Context, n model.Node, notes string) error {
	notes = strings.ReplaceAll(notes, "\r\n", "\n")
	if utf8.RuneCountInString(notes) > model.MaxNotes {
		return model.ErrNotesTooLong
	}
	return w.edit(ctx, n,
		func(f *model.Folder) { f.Notes = notes },
		func(h *model.Host) { h.Notes = notes },
		func(s *model.Site) { s.Notes = notes })
}

// edit changes the stored node with the function of its kind, then saves it.
func (w *Workspace) edit(ctx context.Context, n model.Node, folder func(*model.Folder), host func(*model.Host), site func(*model.Site)) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	switch n.Kind {
	case model.KindFolder:
		f, ok := tree.Folder(n.ID)
		if !ok {
			return fmt.Errorf("folder %s: %w", n.ID, model.ErrNotFound)
		}
		folder(&f)
		return w.db.SaveFolder(ctx, f)
	case model.KindHost:
		h, ok := tree.Host(n.ID)
		if !ok {
			return fmt.Errorf("host %s: %w", n.ID, model.ErrNotFound)
		}
		host(&h)
		return w.db.SaveHost(ctx, h)
	case model.KindSite:
		s, ok := tree.Site(n.ID)
		if !ok {
			return fmt.Errorf("site %s: %w", n.ID, model.ErrNotFound)
		}
		site(&s)
		return w.db.SaveSite(ctx, s)
	}
	return fmt.Errorf("%s %s: %w", n.Kind, n.ID, model.ErrNotFound)
}

// Delete removes a site, a host with its sites, or an empty folder, from Vigie only: nothing
// changes on the servers.
func (w *Workspace) Delete(ctx context.Context, n model.Node) error {
	switch n.Kind {
	case model.KindFolder:
		return w.db.DeleteFolder(ctx, n.ID)
	case model.KindHost:
		return w.db.DeleteHost(ctx, n.ID)
	case model.KindSite:
		return w.db.DeleteSite(ctx, n.ID)
	}
	return fmt.Errorf("%s %s: %w", n.Kind, n.ID, model.ErrNotFound)
}

// EditHost changes the name and connection of a stored host; its folder, color, notes and
// trusted key stay. A new address (alias, hostname or port) clears the trusted key, which the
// user checks again: cleared tells so.
func (w *Workspace) EditHost(ctx context.Context, edited model.Host) (cleared bool, err error) {
	err = w.edit(ctx, model.Node{Kind: model.KindHost, ID: edited.ID}, nil, func(h *model.Host) {
		cleared = h.Alias != edited.Alias || h.Hostname != edited.Hostname || h.EffectivePort() != edited.EffectivePort()
		h.Name, h.Alias, h.Hostname, h.Port, h.User = edited.Name, edited.Alias, edited.Hostname, edited.Port, edited.User
		h.KeyPath, h.WPCLI, h.MaxConcurrent = edited.KeyPath, edited.WPCLI, edited.MaxConcurrent
		if cleared {
			h.HostKeyFingerprint = ""
		}
	}, nil)
	if err != nil {
		return false, err
	}
	return cleared, nil
}

// SetHostKey records the fingerprint of the key the user trusted for a stored host.
func (w *Workspace) SetHostKey(ctx context.Context, hostID model.ID, fingerprint string) error {
	return w.edit(ctx, model.Node{Kind: model.KindHost, ID: hostID}, nil,
		func(h *model.Host) { h.HostKeyFingerprint = fingerprint }, nil)
}

// DiscoverHost searches the account of a stored host; its sites come back as already added.
func (w *Workspace) DiscoverHost(ctx context.Context, hostID model.ID, progress func(checked, total int)) ([]discover.Found, bool, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, false, err
	}
	h, ok := tree.Host(hostID)
	if !ok {
		return nil, false, fmt.Errorf("host %s: %w", hostID, model.ErrNotFound)
	}
	return w.discoverAccount(ctx, h, tree.SitesUnder(model.Node{Kind: model.KindHost, ID: hostID}), progress)
}

// AddSites stores new sites of a stored host, all or nothing.
func (w *Workspace) AddSites(ctx context.Context, hostID model.ID, sites []model.Site) error {
	for _, s := range sites {
		if s.HostID != hostID {
			return fmt.Errorf("%q: %w", s.Name, ErrSiteOfAnotherHost)
		}
	}
	return w.db.AddSites(ctx, sites)
}
