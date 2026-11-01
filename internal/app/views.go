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
	// Changes are what its latest complete scan found changed since the complete scan before
	// (ADR-0027), made at ChangesSince (RFC 3339); empty, and "", when there is none to compare.
	Changes      []ChangeView `json:"changes"`
	ChangesSince string       `json:"changesSince"`
}

// PHPWebView is the PHP serving the pages, as the dashboard noted it (ADR-0028).
type PHPWebView struct {
	Version string `json:"version"` // "" when not read
	Seen    string `json:"seen"`    // RFC 3339, "" when unknown
	// Unknown is why it was not read: not-noted, object-cache, unmatched; "" when read, or when the
	// check could not run (a finding says so).
	Unknown string `json:"unknown"`
}

// DebugLogView is the site's debug.log (ADR-0028).
type DebugLogView struct {
	Path    string `json:"path"` // relative to the site's folder when within it
	Size    int64  `json:"size"` // bytes
	Written bool   `json:"written"`
}

// ScanView is a scan as the interface shows it.
type ScanView struct {
	Finished  string     `json:"finished"` // RFC 3339
	Outcome   string     `json:"outcome"`
	Status    string     `json:"status"`
	Problem   *Problem   `json:"problem"` // why the scan did not complete
	WordPress string     `json:"wordpress"`
	PHPCLI    string     `json:"phpCli"`
	PHPWeb    PHPWebView `json:"phpWeb"`
	WPCLI     string     `json:"wpCli"`
	Home      string     `json:"home"`
	// AdminEmail is where WordPress sends its notices; AdminEmailPending the address waiting for
	// its confirmation, "" when none (ADR-0028).
	AdminEmail        string        `json:"adminEmail"`
	AdminEmailPending string        `json:"adminEmailPending"`
	DebugLog          *DebugLogView `json:"debugLog"` // nil when there is none
	Checked           ChecksView    `json:"checked"`
	Pending           PendingView   `json:"pending"`
	Plugins           int           `json:"plugins"` // installed
	Themes            int           `json:"themes"`
	// Updates lists what needs attention: available, impossible for now, or in an unknown state.
	Updates []UpdateView `json:"updates"`
	// SourceUnknown names the plugins whose updates Vigie cannot see (not on wordpress.org).
	SourceUnknown []string `json:"sourceUnknown"`
	// Extensions are the plugins, then the themes, as the scan read them (ADR-0024).
	Extensions []ExtensionView `json:"extensions"`
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

// IgnoreView is why and until when a finding or an update is set aside.
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

// ExtensionView is a plugin or theme of a site, from its scan.
type ExtensionView struct {
	Kind           string       `json:"kind"` // plugin, theme
	Slug           string       `json:"slug"`
	Title          string       `json:"title"`
	Status         string       `json:"status"` // plugins: active, active-network, inactive, must-use, dropin; themes: active, parent, inactive
	Version        string       `json:"version"`
	Update         string       `json:"update"` // none, available, unavailable, unknown
	UpdateVersion  string       `json:"updateVersion"`
	AutoUpdate     bool         `json:"autoUpdate"`
	OnWordPressOrg bool         `json:"onWordPressOrg"`
	Worst          *FindingView `json:"worst"`  // the worst finding about it not set aside; nil when none
	Ignore         *IgnoreView  `json:"ignore"` // its update offered, ignored (ADR-0026); nil when it counts
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
	// OnWordPressOrg: a plugin wordpress.org knows, whose changelog is there.
	OnWordPressOrg bool `json:"onWordPressOrg"`
	// Key ignores an update offered with Security.Ignore (ADR-0026); "" for a translation, updated
	// with the others.
	Key    string      `json:"key"`
	Ignore *IgnoreView `json:"ignore"` // nil unless ignored
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
		PHPWeb:     PHPWebView{Version: s.PHPWeb.Version, Seen: rfc3339(s.PHPWeb.Seen), Unknown: string(s.PHPWeb.Unknown)},
		AdminEmail: s.AdminEmail, AdminEmailPending: s.AdminEmailPending,
		Checked:       ChecksView{Core: s.CoreChecked, Plugins: s.PluginsChecked, Themes: s.ThemesChecked, Languages: s.LanguagesChecked},
		Pending:       PendingView{Core: p.Core, Plugins: p.Plugins, Themes: p.Themes, Languages: p.Languages},
		Updates:       []UpdateView{},
		SourceUnknown: []string{},
		FilesChecked:  rfc3339(s.FilesChecked),
		Findings:      findingViews(s),
		Extensions:    []ExtensionView{},
	}
	if s.DebugLog.Path != "" {
		v.DebugLog = &DebugLogView{Path: s.DebugLog.Path, Size: s.DebugLog.Size, Written: s.DebugLog.Written}
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
			u := UpdateView{
				Kind: string(item.Kind), Slug: item.Slug, Title: item.Title, Language: item.Language,
				Version: item.Version, UpdateVersion: item.UpdateVersion, Update: string(item.Update), Ignore: ignoreView(item.Ignore),
				OnWordPressOrg: item.OnWordPressOrg,
			}
			if item.Kind != model.ItemLanguage && item.Update == model.UpdateAvailable {
				u.Key = model.UpdateKey(item)
			}
			v.Updates = append(v.Updates, u)
		}
		if item.UpdateSourceUnknown() {
			name := item.Title
			if name == "" {
				name = item.Slug
			}
			v.SourceUnknown = append(v.SourceUnknown, name)
		}
	}
	v.Extensions = extensionViews(s.Items, v.Findings)
	return v
}

// extensionViews lists the plugins, then the themes, each with the worst finding about it that
// needs the user (urgent or needs attention) and is not set aside.
func extensionViews(items []model.ScanItem, findings []FindingView) []ExtensionView {
	views := []ExtensionView{}
	for _, kind := range []model.ItemKind{model.ItemPlugin, model.ItemTheme} {
		for _, item := range items {
			if item.Kind != kind {
				continue
			}
			e := ExtensionView{
				Kind: string(item.Kind), Slug: item.Slug, Title: item.Title, Status: item.Status, Version: item.Version,
				Update: string(item.Update), UpdateVersion: item.UpdateVersion, AutoUpdate: item.AutoUpdate,
				OnWordPressOrg: item.OnWordPressOrg, Ignore: ignoreView(item.Ignore),
			}
			e.Worst = worst(findingsAbout(findings, e.Kind+":"+e.Slug))
			views = append(views, e)
		}
	}
	return views
}

// ignoreView is why and until when a finding or an update is set aside; nil when it is not.
func ignoreView(i *model.Ignore) *IgnoreView {
	if i == nil {
		return nil
	}
	return &IgnoreView{UntilVersion: i.UntilVersion, Reason: i.Reason, Created: rfc3339(i.Created)}
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
			v.Ignore = ignoreView(f.Ignore)
		}
		views = append(views, v)
	}
	return views
}

// findingsAbout keeps the findings of one subject, for example "plugin:akismet".
func findingsAbout(findings []FindingView, subject string) []FindingView {
	about := []FindingView{}
	for _, f := range findings {
		if f.Subject == subject {
			about = append(about, f)
		}
	}
	return about
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
	view := SiteView{ID: siteID, Name: site.Name, Path: site.Path, URL: site.URL, Multisite: site.Multisite, Changes: []ChangeView{}}
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
		if err := s.changes(ctx, &view, scan); err != nil {
			return SiteView{}, err
		}
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

// changes adds to a site's view what its latest complete scan changed since the complete scan
// before, with Vigie's operations on the site.
func (s *SiteManager) changes(ctx context.Context, view *SiteView, scan model.Scan) error {
	if scan.Outcome != model.ScanCompleted {
		return nil
	}
	before, ok, err := s.repo.CompleteScanBefore(ctx, scan.SiteID, scan.ID)
	if err != nil || !ok {
		return err
	}
	ops, err := s.repo.Operations(ctx, scan.SiteID)
	if err != nil {
		return err
	}
	view.Changes = changeViews(model.Changes(before, scan, ops), scan)
	view.ChangesSince = rfc3339(before.Finished)
	return nil
}
