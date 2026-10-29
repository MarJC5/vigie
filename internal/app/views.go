package app

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// SiteView is a site with its latest scan.
type SiteView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"` // WordPress root on the server
	URL       string    `json:"url"`
	Multisite bool      `json:"multisite"`
	Scan      *ScanView `json:"scan"` // nil: never scanned
}

// ScanView is a scan as the interface shows it.
type ScanView struct {
	Finished  string      `json:"finished"` // RFC 3339
	Outcome   string      `json:"outcome"`
	Status    string      `json:"status"`
	Problem   *Problem    `json:"problem"` // why the scan did not complete
	WordPress string      `json:"wordpress"`
	PHPCLI    string      `json:"phpCli"`
	WPCLI     string      `json:"wpCli"`
	Home      string      `json:"home"`
	Checked   ChecksView  `json:"checked"`
	Pending   PendingView `json:"pending"`
	Plugins   int         `json:"plugins"` // installed
	Themes    int         `json:"themes"`
	// Updates lists what needs attention: available, impossible for now, or in an unknown state.
	Updates []UpdateView `json:"updates"`
	// SourceUnknown names the plugins whose updates Vigie cannot see (not on wordpress.org).
	SourceUnknown []string `json:"sourceUnknown"`
}

// ChecksView tells which update checks reached wordpress.org.
type ChecksView struct {
	Core      bool `json:"core"`
	Plugins   bool `json:"plugins"`
	Themes    bool `json:"themes"`
	Languages bool `json:"languages"`
}

// PendingView counts the updates that can be installed.
type PendingView struct {
	Core      int `json:"core"`
	Plugins   int `json:"plugins"`
	Themes    int `json:"themes"`
	Languages int `json:"languages"`
}

// UpdateView is one update to look at.
type UpdateView struct {
	Kind          string `json:"kind"` // core, plugin, theme, language
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Language      string `json:"language"`
	Version       string `json:"version"`
	UpdateVersion string `json:"updateVersion"`
	Update        string `json:"update"` // available, unavailable, unknown
}

// SiteRow is a line of the table of a host or folder.
type SiteRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"` // in the tree, for example "Clients/Bench/a.example.test"
	Status    string `json:"status"`
	WordPress string `json:"wordpress"`
	PHPCLI    string `json:"phpCli"`
	Pending   int    `json:"pending"`
}

// ScanProblem is why a scan did not complete, with the text keys of the catalogs; nil when it
// completed.
func ScanProblem(s model.Scan) *Problem {
	switch s.Outcome {
	case model.ScanCompleted:
		return nil
	case model.ScanCannotLoad:
		return &Problem{Code: "outcome.cannotLoad", Detail: s.Error}
	case model.ScanCancelled:
		return &Problem{Code: "outcome.cancelled", Detail: s.Error}
	}
	if s.Failure != "" {
		return &Problem{Code: "failure." + s.Failure, Detail: s.Error}
	}
	return &Problem{Code: "outcome.failed", Detail: s.Error}
}

func scanView(s model.Scan) *ScanView {
	p := s.PendingUpdates()
	v := &ScanView{
		Finished: s.Finished.UTC().Format(time.RFC3339), Outcome: string(s.Outcome), Status: string(s.Status()),
		Problem: ScanProblem(s), WordPress: s.WordPress, PHPCLI: s.PHPCLI, WPCLI: s.WPCLI, Home: s.Home,
		Checked:       ChecksView{Core: s.CoreChecked, Plugins: s.PluginsChecked, Themes: s.ThemesChecked, Languages: s.LanguagesChecked},
		Pending:       PendingView{Core: p.Core, Plugins: p.Plugins, Themes: p.Themes, Languages: p.Languages},
		Updates:       []UpdateView{},
		SourceUnknown: []string{},
	}
	for _, item := range s.Items {
		switch item.Kind {
		case model.ItemPlugin:
			v.Plugins++
		case model.ItemTheme:
			v.Themes++
		}
		switch item.Update {
		case model.UpdateAvailable, model.UpdateUnavailable, model.UpdateUnknown:
			v.Updates = append(v.Updates, UpdateView{
				Kind: string(item.Kind), Slug: item.Slug, Title: item.Title, Language: item.Language,
				Version: item.Version, UpdateVersion: item.UpdateVersion, Update: string(item.Update),
			})
		}
		if item.UpdateSourceUnknown() {
			name := item.Title
			if name == "" {
				name = item.Slug
			}
			v.SourceUnknown = append(v.SourceUnknown, name)
		}
	}
	return v
}

// SiteDetail returns a site with its latest scan.
func (s *SiteManager) SiteDetail(ctx context.Context, siteID string) (SiteView, error) {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return SiteView{}, fmt.Errorf("site %q: %w", siteID, err)
	}
	tree, err := s.repo.LoadTree(ctx)
	if err != nil {
		return SiteView{}, err
	}
	site, ok := tree.Site(id)
	if !ok {
		return SiteView{}, fmt.Errorf("site %s: %w", siteID, model.ErrNotFound)
	}
	latest, err := s.repo.LatestScans(ctx)
	if err != nil {
		return SiteView{}, err
	}
	view := SiteView{ID: siteID, Name: site.Name, Path: site.Path, URL: site.URL, Multisite: site.Multisite}
	if scan, ok := latest[id]; ok {
		view.Scan = scanView(scan)
	}
	return view, nil
}

// Sites returns the table of the sites under a host or folder ("" for all).
func (s *SiteManager) Sites(ctx context.Context, nodeID string) ([]SiteRow, error) {
	tree, err := s.repo.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	node, err := nodeByID(tree, nodeID)
	if err != nil {
		return nil, err
	}
	latest, err := s.repo.LatestScans(ctx)
	if err != nil {
		return nil, err
	}
	rows := []SiteRow{}
	for _, site := range tree.SitesUnder(node) {
		row := SiteRow{ID: site.ID.String(), Name: site.Name, Path: tree.PathOf(model.Node{Kind: model.KindSite, ID: site.ID})}
		if scan, ok := latest[site.ID]; ok {
			p := scan.PendingUpdates()
			row.Status, row.WordPress, row.PHPCLI = string(scan.Status()), scan.WordPress, scan.PHPCLI
			row.Pending = p.Core + p.Plugins + p.Themes + p.Languages
		}
		rows = append(rows, row)
	}
	return rows, nil
}
