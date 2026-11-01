package app

import (
	"cmp"
	"context"
	"slices"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/version"
)

// ExtensionsAcross are the plugins and themes of the sites under a host or folder (ADR-0024).
type ExtensionsAcross struct {
	Sites   int               `json:"sites"`   // sites whose latest complete scan was read
	LeftOut []SiteLeftOut     `json:"leftOut"` // sites without one
	Items   []ExtensionAcross `json:"items"`   // by title
}

// SiteLeftOut is a site whose plugins are not known.
type SiteLeftOut struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"` // never (scanned), failed (its latest scan did not complete)
}

// ExtensionAcross is a plugin or theme with the sites that run it.
type ExtensionAcross struct {
	Kind           string            `json:"kind"` // plugin, theme
	Slug           string            `json:"slug"`
	Title          string            `json:"title"`
	OnWordPressOrg bool              `json:"onWordPressOrg"`
	Updates        int               `json:"updates"` // sites with an update offered
	Worst          *FindingView      `json:"worst"`   // the worst on any of its sites, as on a site
	Sites          []ExtensionOnSite `json:"sites"`   // the oldest version first
}

// ExtensionOnSite is a plugin or theme as one site runs it.
type ExtensionOnSite struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Status        string       `json:"status"`
	Update        string       `json:"update"`
	UpdateVersion string       `json:"updateVersion"`
	Worst         *FindingView `json:"worst"`
}

// Extensions lists the plugins and themes of the sites under a host or folder ("" for every
// site), from the latest complete scan of each: nothing is read on the servers.
func (s *SiteManager) Extensions(ctx context.Context, nodeID string) (ExtensionsAcross, error) {
	tree, err := s.repo.LoadTree(ctx)
	if err != nil {
		return ExtensionsAcross{}, err
	}
	node, err := nodeByID(tree, nodeID)
	if err != nil {
		return ExtensionsAcross{}, err
	}
	latest, err := s.repo.LatestScans(ctx)
	if err != nil {
		return ExtensionsAcross{}, err
	}
	across := ExtensionsAcross{LeftOut: []SiteLeftOut{}, Items: []ExtensionAcross{}}
	byKey := map[string]int{}
	for _, site := range tree.SitesUnder(node) {
		scan, ok := latest[site.ID]
		switch {
		case !ok:
			across.LeftOut = append(across.LeftOut, SiteLeftOut{ID: site.ID.String(), Name: site.Name, Reason: "never"})
			continue
		case scan.Outcome != model.ScanCompleted:
			across.LeftOut = append(across.LeftOut, SiteLeftOut{ID: site.ID.String(), Name: site.Name, Reason: "failed"})
			continue
		}
		across.Sites++
		for _, e := range extensionViews(scan.Items, findingViews(scan)) {
			key := e.Kind + ":" + e.Slug
			i, seen := byKey[key]
			if !seen {
				i = len(across.Items)
				byKey[key] = i
				across.Items = append(across.Items, ExtensionAcross{Kind: e.Kind, Slug: e.Slug, Title: e.Title, Sites: []ExtensionOnSite{}})
			}
			item := &across.Items[i]
			item.OnWordPressOrg = item.OnWordPressOrg || e.OnWordPressOrg
			if e.Update == string(model.UpdateAvailable) && e.Ignore == nil {
				item.Updates++
			}
			if e.Worst != nil && (item.Worst == nil || levelRank(e.Worst.Level) > levelRank(item.Worst.Level)) {
				item.Worst = e.Worst
			}
			item.Sites = append(item.Sites, ExtensionOnSite{
				ID: site.ID.String(), Name: site.Name, Version: e.Version, Status: e.Status,
				Update: e.Update, UpdateVersion: e.UpdateVersion, Worst: e.Worst,
			})
		}
	}
	for i := range across.Items {
		slices.SortStableFunc(across.Items[i].Sites, func(a, b ExtensionOnSite) int {
			return cmp.Or(version.Compare(a.Version, b.Version), cmp.Compare(a.Name, b.Name))
		})
	}
	slices.SortStableFunc(across.Items, func(a, b ExtensionAcross) int {
		return cmp.Or(cmp.Compare(title(a), title(b)), cmp.Compare(a.Kind, b.Kind))
	})
	return across, nil
}

// title is what a plugin or theme is sorted by: its title, else its slug.
func title(e ExtensionAcross) string {
	if e.Title != "" {
		return e.Title
	}
	return e.Slug
}

// levelRank orders the levels a worst finding can have.
func levelRank(level string) int {
	switch model.FindingLevel(level) {
	case model.LevelUrgent:
		return 2
	case model.LevelAttention:
		return 1
	}
	return 0
}
