package workspace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/store"
)

// ErrNothingToMove refuses a move of several items holding no folder or host: sites stay under their
// hosts (ADR-0034).
var ErrNothingToMove = errors.New("only folders and hosts are moved")

// The actions on several items selected (ADR-0034): each one an edit of Vigie's database in one
// transaction, all done or none.

// MoveMany puts the folders and hosts among nodes in a folder (model.NoID: at the root); the sites
// among them stay under their hosts.
func (w *Workspace) MoveMany(ctx context.Context, nodes []model.Node, folder model.ID) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	var e store.Edits
	for _, n := range nodes {
		switch n.Kind {
		case model.KindFolder:
			f, ok := tree.Folder(n.ID)
			if !ok {
				return fmt.Errorf("folder %s: %w", n.ID, model.ErrNotFound)
			}
			f.ParentID = folder
			e.Folders = append(e.Folders, f)
		case model.KindHost:
			h, ok := tree.Host(n.ID)
			if !ok {
				return fmt.Errorf("host %s: %w", n.ID, model.ErrNotFound)
			}
			h.FolderID = folder
			e.Hosts = append(e.Hosts, h)
		}
	}
	if len(e.Folders)+len(e.Hosts) == 0 {
		return ErrNothingToMove
	}
	return w.db.ApplyEdits(ctx, e)
}

// SetColorMany gives every folder, host and site among nodes its own color (model.ColorNone:
// inherit it).
func (w *Workspace) SetColorMany(ctx context.Context, nodes []model.Node, c model.Color) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	var e store.Edits
	for _, n := range nodes {
		switch n.Kind {
		case model.KindFolder:
			f, ok := tree.Folder(n.ID)
			if !ok {
				return fmt.Errorf("folder %s: %w", n.ID, model.ErrNotFound)
			}
			f.Color = c
			e.Folders = append(e.Folders, f)
		case model.KindHost:
			h, ok := tree.Host(n.ID)
			if !ok {
				return fmt.Errorf("host %s: %w", n.ID, model.ErrNotFound)
			}
			h.Color = c
			e.Hosts = append(e.Hosts, h)
		case model.KindSite:
			s, ok := tree.Site(n.ID)
			if !ok {
				return fmt.Errorf("site %s: %w", n.ID, model.ErrNotFound)
			}
			s.Color = c
			e.Sites = append(e.Sites, s)
		}
	}
	return w.db.ApplyEdits(ctx, e)
}

// SitesOf returns the sites the nodes hold, each once, in the tree's order.
func SitesOf(tree *model.Tree, nodes []model.Node) []model.Site {
	held := map[model.ID]bool{}
	for _, n := range nodes {
		for _, s := range tree.SitesUnder(n) {
			held[s.ID] = true
		}
	}
	var sites []model.Site
	for _, s := range tree.SitesUnder(model.Root) {
		if held[s.ID] {
			sites = append(sites, s)
		}
	}
	return sites
}

// TagMany adds a tag to every site the nodes hold (add), or takes it off those that have it.
func (w *Workspace) TagMany(ctx context.Context, nodes []model.Node, tag string, add bool) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	sites := SitesOf(tree, nodes)
	e := store.Edits{Tags: map[model.ID][]string{}}
	if add {
		chosen := map[model.ID]bool{}
		for _, s := range sites {
			chosen[s.ID] = true
		}
		var used []string
		for _, s := range tree.SitesUnder(model.Root) {
			if !chosen[s.ID] {
				used = append(used, s.Tags...)
			}
		}
		for _, s := range sites {
			tags, err := model.SiteTags(append(slices.Clone(s.Tags), tag), used)
			if err != nil {
				return err
			}
			e.Tags[s.ID] = tags
		}
	} else {
		for _, s := range sites {
			kept := slices.DeleteFunc(slices.Clone(s.Tags), func(t string) bool { return model.TagKey(t) == model.TagKey(tag) })
			if len(kept) != len(s.Tags) {
				e.Tags[s.ID] = kept
			}
		}
	}
	return w.db.ApplyEdits(ctx, e)
}

// DeleteMany removes sites, hosts with their sites, and folders from Vigie, all or none: a folder
// is deleted only when nothing is left in it once the others are.
func (w *Workspace) DeleteMany(ctx context.Context, nodes []model.Node) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	return w.db.ApplyEdits(ctx, store.Edits{Delete: deepestFirst(tree, nodes)})
}

// DeleteManyWithBackups removes the backups of the sites the nodes hold on their servers, a command
// per host, then deletes the nodes (ADR-0033). The sites of a server that cannot be reached are
// in left; the nodes are deleted anyway.
func (w *Workspace) DeleteManyWithBackups(ctx context.Context, nodes []model.Node) (left []model.ID, err error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	// The sites of the sites and hosts deleted: a folder is deleted empty, and holds none.
	var gone []model.Node
	for _, n := range nodes {
		if n.Kind == model.KindSite || n.Kind == model.KindHost {
			gone = append(gone, n)
		}
	}
	byHost := map[model.ID][]model.ID{}
	var hosts []model.ID
	for _, s := range SitesOf(tree, gone) {
		if _, seen := byHost[s.HostID]; !seen {
			hosts = append(hosts, s.HostID)
		}
		byHost[s.HostID] = append(byHost[s.HostID], s.ID)
	}
	for _, id := range hosts {
		host, _ := tree.Host(id)
		line, err := remote.RemoveSiteBackups(byHost[id]...)
		if err != nil {
			return nil, err
		}
		if res, err := w.runOn(ctx, host, line); err != nil || res.ExitCode != 0 {
			left = append(left, byHost[id]...)
		}
	}
	return left, w.DeleteMany(ctx, nodes)
}

// deepestFirst orders nodes to delete: the folders deepest first, so that a folder whose folders go
// too is empty by its turn.
func deepestFirst(tree *model.Tree, nodes []model.Node) []model.Node {
	ordered := slices.Clone(nodes)
	depth := func(n model.Node) int { return strings.Count(tree.PathOf(n), "/") }
	slices.SortStableFunc(ordered, func(a, b model.Node) int { return depth(b) - depth(a) })
	return ordered
}
