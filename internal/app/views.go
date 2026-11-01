package app

import (
	"context"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// SiteView is a site with its latest scan.
type SiteView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"` // WordPress root on the server
	URL       string    `json:"url"`
	Multisite bool      `json:"multisite"`
	Scan      *ScanView `json:"scan"` // nil: never scanned
	// SSHCommand opens a shell on its account, in its folder, pasted in a terminal; "" when it
	// cannot be written (a control character in the folder or the key's path).
	SSHCommand string `json:"sshCommand"`
	// SSHURL is the ssh:// address of its host, for the system to open in the user's terminal.
	SSHURL string `json:"sshUrl"`
	// AdminURL opens its dashboard: the address the user gave, else the WordPress address the
	// latest scan read (else its own address) followed by /wp-admin/; "" when none is known.
	AdminURL string `json:"adminUrl"`
	// OwnAdminURL is the address the user gave, "" when the computed one is used.
	OwnAdminURL string `json:"ownAdminUrl"`
	// ComputedAdminURL is the WordPress address (else the site's) followed by /wp-admin/.
	ComputedAdminURL string `json:"computedAdminUrl"`
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
	// FilesChecked is when the files were last compared with wordpress.org's; "" never.
	FilesChecked string            `json:"filesChecked"`
	Findings     []FindingView     `json:"findings"` // the worst first
	Counts       FindingCountsView `json:"counts"`
	// Attempt is a later scan that did not complete: this one stays shown, the attempt says why
	// nothing newer is. Nil when this scan is the latest.
	Attempt *AttemptView `json:"attempt"`
}

// AttemptView is a scan that did not complete after the one shown.
type AttemptView struct {
	Finished string   `json:"finished"` // RFC 3339
	Problem  *Problem `json:"problem"`
}

// FindingView is a finding as the interface writes it in its language, from its kind and facts.
type FindingView struct {
	Key        string            `json:"key"`
	Kind       string            `json:"kind"`
	Subject    string            `json:"subject"` // core, plugin:<slug>, theme:<slug>, php, user:<login>, site
	Name       string            `json:"name"`    // the plugin's or theme's title, when the subject is one
	Identifier string            `json:"identifier"`
	Version    string            `json:"version"`
	Level      string            `json:"level"` // urgent, attention, not-checked, info
	Facts      map[string]string `json:"facts"`
	Ignore     *IgnoreView       `json:"ignore"` // nil unless set aside
}

// IgnoreView is why and until when a finding is set aside.
type IgnoreView struct {
	UntilVersion string `json:"untilVersion"` // "": permanently
	Reason       string `json:"reason"`
	Created      string `json:"created"` // RFC 3339
}

// FindingCountsView counts the findings by level, those set aside apart.
type FindingCountsView struct {
	Urgent     int `json:"urgent"`
	Attention  int `json:"attention"`
	NotChecked int `json:"notChecked"`
	Ignored    int `json:"ignored"`
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
	// Worst is the worst finding not set aside, urgent or needing attention; nil when none.
	Worst *FindingView `json:"worst"`
}

// ScanProblem is why a scan did not complete, with the text keys of the catalogs; nil when it
// completed.
func ScanProblem(s model.Scan) *Problem {
	switch s.Outcome {
	case model.ScanCompleted:
		return nil
	case model.ScanCannotLoad:
		if s.Failure == scan.FailureNotWordPress {
			return &Problem{Code: "outcome.notWordPress", Detail: s.Error}
		}
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
		FilesChecked:  rfc3339(s.FilesChecked),
		Findings:      findingViews(s),
	}
	if s.Attempt != nil {
		v.Attempt = &AttemptView{Finished: rfc3339(s.Attempt.Finished), Problem: ScanProblem(*s.Attempt)}
	}
	c := s.CountFindings()
	v.Counts = FindingCountsView{Urgent: c.Urgent, Attention: c.Attention, NotChecked: c.NotChecked}
	for _, f := range s.Findings {
		if f.Ignore != nil {
			v.Counts.Ignored++
		}
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

// findingViews are the findings of a scan, each plugin or theme with its title.
func findingViews(s model.Scan) []FindingView {
	titles := map[string]string{}
	for _, item := range s.Items {
		if item.Title != "" && (item.Kind == model.ItemPlugin || item.Kind == model.ItemTheme) {
			titles[string(item.Kind)+":"+item.Slug] = item.Title
		}
	}
	views := []FindingView{}
	for _, f := range s.Findings {
		v := FindingView{
			Key: f.Key(), Kind: string(f.Kind), Subject: f.Subject, Name: titles[f.Subject], Identifier: f.Identifier,
			Version: f.Version, Level: string(f.Level), Facts: f.Facts,
		}
		if v.Facts == nil {
			v.Facts = map[string]string{}
		}
		if f.Ignore != nil {
			v.Ignore = &IgnoreView{UntilVersion: f.Ignore.UntilVersion, Reason: f.Ignore.Reason, Created: rfc3339(f.Ignore.Created)}
		}
		views = append(views, v)
	}
	return views
}

// worst is the worst finding not set aside that calls for something, urgent or needing attention.
func worst(findings []FindingView) *FindingView {
	for _, level := range []model.FindingLevel{model.LevelUrgent, model.LevelAttention} {
		for i, f := range findings {
			if f.Ignore == nil && f.Level == string(level) {
				return &findings[i]
			}
		}
	}
	return nil
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
	if host, ok := tree.Host(site.HostID); ok {
		target := sshx.TargetFor(host)
		dir, err := wpcli.ServerPath(site.Path)
		if err != nil {
			dir = "" // the home directory
		}
		view.SSHCommand, _ = target.ShellCommand(dir)
		view.SSHURL = target.URL()
	}
	base := site.URL
	if scan, ok := latest[id]; ok {
		view.Scan = scanView(scan)
		if scan.SiteURL != "" {
			base = scan.SiteURL
		}
	}
	if base != "" {
		view.ComputedAdminURL = strings.TrimRight(base, "/") + "/wp-admin/"
	}
	view.OwnAdminURL, view.AdminURL = site.AdminURL, site.AdminURL
	if view.AdminURL == "" {
		view.AdminURL = view.ComputedAdminURL
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
			row.Worst = worst(findingViews(scan))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// HostView is a host with its connection and the table of its sites.
type HostView struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Alias    string    `json:"alias"`
	Hostname string    `json:"hostname"`
	Port     int       `json:"port"`
	User     string    `json:"user"`
	WPCLI    string    `json:"wpCli"` // the effective command
	Sites    []SiteRow `json:"sites"`
}

// HostDetail returns a host with its connection and its sites.
func (s *SiteManager) HostDetail(ctx context.Context, hostID string) (HostView, error) {
	id, err := uuid.Parse(hostID)
	if err != nil {
		return HostView{}, fmt.Errorf("host %q: %w", hostID, err)
	}
	tree, err := s.repo.LoadTree(ctx)
	if err != nil {
		return HostView{}, err
	}
	h, ok := tree.Host(id)
	if !ok {
		return HostView{}, fmt.Errorf("host %s: %w", hostID, model.ErrNotFound)
	}
	sites, err := s.Sites(ctx, hostID)
	if err != nil {
		return HostView{}, err
	}
	wp := h.WPCLI
	if wp == "" {
		wp = "wp"
	}
	return HostView{
		ID: hostID, Name: h.Name, Alias: h.Alias, Hostname: h.Hostname, Port: h.EffectivePort(), User: h.User,
		WPCLI: wp, Sites: sites,
	}, nil
}
