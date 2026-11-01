package app

import (
	"net/url"
	"slices"
	"strings"

	"github.com/MarJC5/vigie/internal/model"
)

// SubsiteView is one subsite of a network (ADR-0029): its place and state, from the list a scan
// read, and what the latest scan read of it.
type SubsiteView struct {
	BlogID   int    `json:"blogId"`
	URL      string `json:"url"`
	Label    string `json:"label"` // its path on the network ("/", "/alpha"), or its host when it has a domain of its own
	State    string `json:"state"` // public, hidden (from search engines), archived, spam, deleted
	AdminURL string `json:"adminUrl"`
	// Read is whether the latest scan read what it has: never an archived, spam or deleted one.
	Read         bool     `json:"read"`
	Theme        string   `json:"theme"`
	ThemeName    string   `json:"themeName"`
	ThemeVersion string   `json:"themeVersion"`
	AdminEmail   string   `json:"adminEmail"`
	Plugins      []string `json:"plugins"` // slugs active on it, besides those active network-wide
	Admins       []string `json:"admins"`  // logins; their administrator findings tell their two-factor
}

// TreeSubsite is a subsite as the site list shows it under its network.
type TreeSubsite struct {
	BlogID int    `json:"blogId"`
	Label  string `json:"label"`
	State  string `json:"state"`
}

// subsiteState names a subsite's state, the most telling first.
func subsiteState(s model.Subsite) string {
	switch {
	case s.Deleted:
		return "deleted"
	case s.Spam:
		return "spam"
	case s.Archived:
		return "archived"
	case !s.Public:
		return "hidden"
	}
	return "public"
}

// subsiteLabel names a subsite by its path on the network's domain, else by its own host.
func subsiteLabel(network, address string) string {
	n, err1 := url.Parse(network)
	u, err2 := url.Parse(address)
	if err1 != nil || err2 != nil || u.Host == "" {
		return address
	}
	path := strings.TrimSuffix(u.Path, "/")
	if strings.EqualFold(u.Host, n.Host) {
		if path == "" {
			return "/"
		}
		return path
	}
	return u.Host + path
}

// networkAddress is the network's address, which its subsites are named against: the site's, else
// the main site's (blog 1).
func networkAddress(site model.Site, scan *model.Scan) string {
	if site.URL != "" {
		return site.URL
	}
	if scan != nil && scan.Home != "" {
		return scan.Home
	}
	for _, s := range site.Subsites {
		if s.BlogID == 1 {
			return s.URL
		}
	}
	return ""
}

func treeSubsites(site model.Site) []TreeSubsite {
	network := networkAddress(site, nil)
	list := []TreeSubsite{}
	for _, s := range site.Subsites {
		list = append(list, TreeSubsite{BlogID: s.BlogID, Label: subsiteLabel(network, s.URL), State: subsiteState(s)})
	}
	return list
}

// subsiteViews lists a network's subsites with what scan read of each; nil scan reads none.
func subsiteViews(site model.Site, scan *model.Scan) []SubsiteView {
	network := networkAddress(site, scan)
	read := map[int]model.SubsiteRead{}
	if scan != nil && scan.Network != nil {
		for _, r := range scan.Network.Sites {
			read[r.BlogID] = r
		}
	}
	views := []SubsiteView{}
	for _, s := range site.Subsites {
		v := SubsiteView{BlogID: s.BlogID, URL: s.URL, Label: subsiteLabel(network, s.URL), State: subsiteState(s),
			AdminURL: strings.TrimRight(s.URL, "/") + "/wp-admin/", Plugins: []string{}, Admins: []string{}}
		if r, ok := read[s.BlogID]; ok {
			v.Read, v.Theme, v.ThemeName, v.ThemeVersion, v.AdminEmail = true, r.Theme, r.ThemeName, r.ThemeVersion, r.AdminEmail
			v.Plugins = append(v.Plugins, r.Plugins...)
			v.Admins = append(v.Admins, r.Admins...)
		}
		views = append(views, v)
	}
	return views
}

// activeOn tells on which subsites each plugin and theme is active, when the scan read them; a
// plugin active network-wide is on none of its own.
func activeOn(extensions []ExtensionView, network *model.NetworkRead) {
	if network == nil {
		return
	}
	for i, e := range extensions {
		extensions[i].Subsites = []int{}
		for _, s := range network.Sites {
			if e.Kind == "plugin" && slices.Contains(s.Plugins, e.Slug) || e.Kind == "theme" && s.Theme == e.Slug {
				extensions[i].Subsites = append(extensions[i].Subsites, s.BlogID)
			}
		}
	}
}
