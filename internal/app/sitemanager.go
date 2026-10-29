// Package app holds the services the window calls. Their exported methods and types become the
// interface's TypeScript bindings.
package app

import (
	"context"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

// Node is one line of the site manager tree, as the interface shows it.
type Node struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId"` // empty at the root
	Kind      string `json:"kind"`     // "folder", "host" or "site"
	Name      string `json:"name"`
	Color     string `json:"color"`    // effective color: its own, else inherited
	OwnColor  string `json:"ownColor"` // its own color, "none" when it inherits
	Path      string `json:"path"`     // command line path, for example "Clients/Shared hosting/atelier-exemple.ch"
	Multisite bool   `json:"multisite"`
	Subsites  int    `json:"subsites"`
	Status    string `json:"status"` // sites: ok, warn or unknown from the latest scan; empty when never scanned
}

// Tree lists the nodes depth first, parents before their children.
type Tree struct {
	Nodes []Node `json:"nodes"`
}

// siteData is what the site manager reads: the tree and the scans.
type siteData interface {
	store.Repository
	store.ScanRepository
}

// SiteManager serves the site manager to the interface.
type SiteManager struct {
	repo siteData
}

// NewSiteManager returns the service over the stored data.
func NewSiteManager(repo siteData) *SiteManager {
	return &SiteManager{repo: repo}
}

// Tree returns the whole site manager.
func (s *SiteManager) Tree(ctx context.Context) (Tree, error) {
	tree, err := s.repo.LoadTree(ctx)
	if err != nil {
		return Tree{}, err
	}
	latest, err := s.repo.LatestScans(ctx)
	if err != nil {
		return Tree{}, err
	}
	nodes := []Node{}
	tree.Walk(func(n model.Node, _ int) {
		node := Node{ID: n.ID.String(), Kind: n.Kind.String(), Path: tree.PathOf(n)}
		switch n.Kind {
		case model.KindFolder:
			f, _ := tree.Folder(n.ID)
			node.Name, node.Color, node.OwnColor = f.Name, tree.EffectiveFolderColor(f.ID).String(), f.Color.String()
			node.ParentID = idOrEmpty(f.ParentID)
		case model.KindHost:
			h, _ := tree.Host(n.ID)
			node.Name, node.Color, node.OwnColor = h.Name, tree.EffectiveHostColor(h.ID).String(), h.Color.String()
			node.ParentID = idOrEmpty(h.FolderID)
		case model.KindSite:
			site, _ := tree.Site(n.ID)
			node.Name, node.Color, node.OwnColor = site.Name, tree.EffectiveSiteColor(site.ID).String(), site.Color.String()
			node.ParentID = site.HostID.String()
			node.Multisite, node.Subsites = site.Multisite, len(site.Subsites)
			if scan, ok := latest[site.ID]; ok {
				node.Status = string(scan.Status())
			}
		}
		nodes = append(nodes, node)
	})
	return Tree{Nodes: nodes}, nil
}

func idOrEmpty(id model.ID) string {
	if id == model.NoID {
		return ""
	}
	return id.String()
}
