package model

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

const defaultWPCLI = "wp"

// NodeKind tells what a tree node is.
type NodeKind int

const (
	KindFolder NodeKind = iota
	KindHost
	KindSite
)

func (k NodeKind) String() string {
	switch k {
	case KindFolder:
		return "folder"
	case KindHost:
		return "host"
	case KindSite:
		return "site"
	}
	return fmt.Sprintf("NodeKind(%d)", int(k))
}

// Node points to a folder, host or site of the tree.
type Node struct {
	Kind NodeKind
	ID   ID
}

// Root is the top of the tree: the folder without ID that holds root folders and hosts.
var Root = Node{Kind: KindFolder, ID: NoID}

// Tree is the site manager in memory: nested folders, hosts in folders (or at the root), sites
// under hosts. Folders and hosts sharing a parent folder share one namespace, so paths are
// unambiguous; names are compared without case.
type Tree struct {
	folders map[ID]Folder
	hosts   map[ID]Host
	sites   map[ID]Site
}

// NewTree returns an empty tree.
func NewTree() *Tree {
	return &Tree{folders: map[ID]Folder{}, hosts: map[ID]Host{}, sites: map[ID]Site{}}
}

// AddFolder adds a folder under its parent (or at the root).
func (t *Tree) AddFolder(f Folder) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if f.ParentID != NoID {
		if _, ok := t.folders[f.ParentID]; !ok {
			return fmt.Errorf("parent folder %s: %w", f.ParentID, ErrNotFound)
		}
	}
	if t.nameTakenInFolder(f.ParentID, f.Name, f.ID) {
		return fmt.Errorf("%q: %w", f.Name, ErrDuplicateName)
	}
	t.folders[f.ID] = f
	return nil
}

// AddHost adds a host to a folder (or at the root).
func (t *Tree) AddHost(h Host) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if h.FolderID != NoID {
		if _, ok := t.folders[h.FolderID]; !ok {
			return fmt.Errorf("folder %s: %w", h.FolderID, ErrNotFound)
		}
	}
	if t.nameTakenInFolder(h.FolderID, h.Name, h.ID) {
		return fmt.Errorf("%q: %w", h.Name, ErrDuplicateName)
	}
	t.hosts[h.ID] = h
	return nil
}

// AddSite adds a site to a host.
func (t *Tree) AddSite(s Site) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if _, ok := t.hosts[s.HostID]; !ok {
		return fmt.Errorf("host %s: %w", s.HostID, ErrNotFound)
	}
	for _, other := range t.sites {
		if other.HostID == s.HostID && other.ID != s.ID && NamesEqual(other.Name, s.Name) {
			return fmt.Errorf("%q: %w", s.Name, ErrDuplicateName)
		}
	}
	t.sites[s.ID] = s
	return nil
}

func (t *Tree) nameTakenInFolder(folderID ID, name string, self ID) bool {
	for _, f := range t.folders {
		if f.ParentID == folderID && f.ID != self && NamesEqual(f.Name, name) {
			return true
		}
	}
	for _, h := range t.hosts {
		if h.FolderID == folderID && h.ID != self && NamesEqual(h.Name, name) {
			return true
		}
	}
	return false
}

// NamesEqual tells whether two names collide: case and surrounding spaces are ignored.
func NamesEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// Folder returns the folder with the given ID.
func (t *Tree) Folder(id ID) (Folder, bool) {
	f, ok := t.folders[id]
	return f, ok
}

// Host returns the host with the given ID.
func (t *Tree) Host(id ID) (Host, bool) {
	h, ok := t.hosts[id]
	return h, ok
}

// Site returns the site with the given ID.
func (t *Tree) Site(id ID) (Site, bool) {
	s, ok := t.sites[id]
	return s, ok
}

// EffectiveFolderColor returns the folder's color, or its nearest colored ancestor's.
func (t *Tree) EffectiveFolderColor(id ID) Color {
	for id != NoID {
		f, ok := t.folders[id]
		if !ok {
			return ColorNone
		}
		if f.Color != ColorNone {
			return f.Color
		}
		id = f.ParentID
	}
	return ColorNone
}

// EffectiveHostColor returns the host's color, or its folder's.
func (t *Tree) EffectiveHostColor(id ID) Color {
	h, ok := t.hosts[id]
	if !ok {
		return ColorNone
	}
	if h.Color != ColorNone {
		return h.Color
	}
	return t.EffectiveFolderColor(h.FolderID)
}

// EffectiveSiteColor returns the site's color, or its host's.
func (t *Tree) EffectiveSiteColor(id ID) Color {
	s, ok := t.sites[id]
	if !ok {
		return ColorNone
	}
	if s.Color != ColorNone {
		return s.Color
	}
	return t.EffectiveHostColor(s.HostID)
}

// EffectiveWPCLI returns the WP-CLI command for a site: its own, else its host's, else "wp".
func (t *Tree) EffectiveWPCLI(siteID ID) string {
	s := t.sites[siteID]
	if s.WPCLI != "" {
		return s.WPCLI
	}
	if h := t.hosts[s.HostID]; h.WPCLI != "" {
		return h.WPCLI
	}
	return defaultWPCLI
}

// Resolve finds the node at a command line path such as "Clients/Infomaniak/misits.ch".
func (t *Tree) Resolve(path string) (Node, error) {
	notFound := fmt.Errorf("path %q: %w", path, ErrNotFound)
	if strings.TrimSpace(path) == "" {
		return Node{}, notFound
	}
	parts := strings.Split(path, "/")
	folder := NoID
	for i, part := range parts {
		if f, ok := t.childFolder(folder, part); ok {
			folder = f.ID
			if i == len(parts)-1 {
				return Node{Kind: KindFolder, ID: f.ID}, nil
			}
			continue
		}
		h, ok := t.childHost(folder, part)
		if !ok {
			return Node{}, notFound
		}
		switch len(parts) - i {
		case 1:
			return Node{Kind: KindHost, ID: h.ID}, nil
		case 2:
			if s, ok := t.childSite(h.ID, parts[i+1]); ok {
				return Node{Kind: KindSite, ID: s.ID}, nil
			}
		}
		return Node{}, notFound
	}
	return Node{}, notFound
}

// PathOf returns the command line path of a node, the reverse of Resolve.
func (t *Tree) PathOf(n Node) string {
	var parts []string
	folder := NoID
	switch n.Kind {
	case KindSite:
		s := t.sites[n.ID]
		h := t.hosts[s.HostID]
		parts = []string{h.Name, s.Name}
		folder = h.FolderID
	case KindHost:
		h := t.hosts[n.ID]
		parts = []string{h.Name}
		folder = h.FolderID
	case KindFolder:
		folder = n.ID
	}
	for folder != NoID {
		f := t.folders[folder]
		parts = append([]string{f.Name}, parts...)
		folder = f.ParentID
	}
	return strings.Join(parts, "/")
}

func (t *Tree) childFolder(parent ID, name string) (Folder, bool) {
	for _, f := range t.folders {
		if f.ParentID == parent && NamesEqual(f.Name, name) {
			return f, true
		}
	}
	return Folder{}, false
}

func (t *Tree) childHost(folder ID, name string) (Host, bool) {
	for _, h := range t.hosts {
		if h.FolderID == folder && NamesEqual(h.Name, name) {
			return h, true
		}
	}
	return Host{}, false
}

func (t *Tree) childSite(host ID, name string) (Site, bool) {
	for _, s := range t.sites {
		if s.HostID == host && NamesEqual(s.Name, name) {
			return s, true
		}
	}
	return Site{}, false
}

// Walk visits every node depth first, parents before children: in a folder, its subfolders then
// its hosts; under a host, its sites. Siblings are ordered by position, then by name without case.
func (t *Tree) Walk(fn func(n Node, depth int)) {
	t.walkFolder(NoID, 0, fn)
}

// SitesUnder returns the site itself, the sites of a host, or every site under a folder (Root
// for all), in tree order.
func (t *Tree) SitesUnder(n Node) []Site {
	var sites []Site
	collect := func(node Node, _ int) {
		if node.Kind == KindSite {
			sites = append(sites, t.sites[node.ID])
		}
	}
	switch n.Kind {
	case KindSite:
		if s, ok := t.sites[n.ID]; ok {
			sites = append(sites, s)
		}
	case KindHost:
		h, ok := t.hosts[n.ID]
		if !ok {
			return nil
		}
		t.walkFolder(h.FolderID, 0, func(node Node, depth int) {
			if node.Kind == KindSite && t.sites[node.ID].HostID == n.ID {
				collect(node, depth)
			}
		})
	case KindFolder:
		t.walkFolder(n.ID, 0, collect)
	}
	return sites
}

func (t *Tree) walkFolder(folder ID, depth int, fn func(Node, int)) {
	var folders []Folder
	for _, f := range t.folders {
		if f.ParentID == folder {
			folders = append(folders, f)
		}
	}
	slices.SortFunc(folders, func(a, b Folder) int { return siblingOrder(a.Position, a.Name, b.Position, b.Name) })
	for _, f := range folders {
		fn(Node{Kind: KindFolder, ID: f.ID}, depth)
		t.walkFolder(f.ID, depth+1, fn)
	}

	var hosts []Host
	for _, h := range t.hosts {
		if h.FolderID == folder {
			hosts = append(hosts, h)
		}
	}
	slices.SortFunc(hosts, func(a, b Host) int { return siblingOrder(a.Position, a.Name, b.Position, b.Name) })
	for _, h := range hosts {
		fn(Node{Kind: KindHost, ID: h.ID}, depth)
		var sites []Site
		for _, s := range t.sites {
			if s.HostID == h.ID {
				sites = append(sites, s)
			}
		}
		slices.SortFunc(sites, func(a, b Site) int { return siblingOrder(a.Position, a.Name, b.Position, b.Name) })
		for _, s := range sites {
			fn(Node{Kind: KindSite, ID: s.ID}, depth+1)
		}
	}
}

func siblingOrder(posA int, nameA string, posB int, nameB string) int {
	if posA != posB {
		return cmp.Compare(posA, posB)
	}
	return cmp.Compare(strings.ToLower(nameA), strings.ToLower(nameB))
}
