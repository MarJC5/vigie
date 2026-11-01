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
	Notes     string `json:"notes"`    // Markdown, shown on its page and read by the search
	Multisite bool   `json:"multisite"`
	Subsites  int    `json:"subsites"`
	Status    string `json:"status"` // sites: ok, warn or unknown from the latest scan; empty when never scanned
	// Updates pending from the latest scan: a site's, or the sum of a host's sites.
	Updates int `json:"updates"`
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
	hostUpdates := map[string]int{}
	tree.Walk(func(n model.Node, _ int) {
		node := Node{ID: n.ID.String(), Kind: n.Kind.String(), Path: tree.PathOf(n)}
		switch n.Kind {
		case model.KindFolder:
			f, _ := tree.Folder(n.ID)
			node.Name, node.Color, node.OwnColor, node.Notes = f.Name, tree.EffectiveFolderColor(f.ID).String(), f.Color.String(), f.Notes
			node.ParentID = idOrEmpty(f.ParentID)
		case model.KindHost:
			h, _ := tree.Host(n.ID)
			node.Name, node.Color, node.OwnColor, node.Notes = h.Name, tree.EffectiveHostColor(h.ID).String(), h.Color.String(), h.Notes
			node.ParentID = idOrEmpty(h.FolderID)
		case model.KindSite:
			site, _ := tree.Site(n.ID)
			node.Name, node.Color, node.OwnColor, node.Notes = site.Name, tree.EffectiveSiteColor(site.ID).String(), site.Color.String(), site.Notes
			node.ParentID = site.HostID.String()
			node.Multisite, node.Subsites = site.Multisite, len(site.Subsites)
			if scan, ok := latest[site.ID]; ok {
				node.Status = string(scan.Status())
				c := scan.PendingUpdates()
				node.Updates = c.Core + c.Plugins + c.Themes + c.Languages
				hostUpdates[node.ParentID] += node.Updates
			}
		}
		nodes = append(nodes, node)
	})
	for i, n := range nodes {
		if n.Kind == "host" {
			nodes[i].Updates = hostUpdates[n.ID]
		}
	}
	return Tree{Nodes: nodes}, nil
}

func idOrEmpty(id model.ID) string {
	if id == model.NoID {
		return ""
	}
	return id.String()
}
