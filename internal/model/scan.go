package model

import "time"

// ScanOutcome is how a scan of a site ended.
type ScanOutcome string

const (
	ScanCompleted  ScanOutcome = "completed"
	ScanCannotLoad ScanOutcome = "cannot-load" // WP-CLI could not load WordPress
	ScanFailed     ScanOutcome = "failed"      // the connection or a command failed
	ScanCancelled  ScanOutcome = "cancelled"
)

// ItemKind is what a scan item is about.
type ItemKind string

const (
	ItemCore     ItemKind = "core"
	ItemPlugin   ItemKind = "plugin"
	ItemTheme    ItemKind = "theme"
	ItemLanguage ItemKind = "language"
)

// UpdateState is the update state of an item, as WP-CLI reports it.
type UpdateState string

const (
	UpdateNone        UpdateState = "none"
	UpdateAvailable   UpdateState = "available"
	UpdateUnavailable UpdateState = "unavailable" // a newer version needs another PHP or WordPress
	// UpdateNewerThanSource: the installed version is newer than wordpress.org's.
	UpdateNewerThanSource UpdateState = "newer-than-source"
	UpdateUnknown         UpdateState = "unknown" // a state this version of Vigie does not know
)

// ScanItem is WordPress itself, a plugin, a theme or a translation, as found by a scan.
type ScanItem struct {
	Kind           ItemKind
	Slug           string // plugin or theme; for a translation, its plugin or theme (empty for WordPress)
	Title          string
	Status         string // plugins and themes: active, inactive...; core: the offer's type (minor, major)
	Version        string // installed version
	Language       string // translations: for example "fr_FR"
	Update         UpdateState
	UpdateVersion  string
	AutoUpdate     bool
	OnWordPressOrg bool // plugins: wordpress.org knows it
}

// UpdateSourceUnknown tells that no update can be seen for this plugin: wordpress.org does not
// know it, and the code of a premium plugin's own updater does not run during a scan.
func (i ScanItem) UpdateSourceUnknown() bool {
	return i.Kind == ItemPlugin && !i.OnWordPressOrg && i.Update == UpdateNone
}

// Scan is one scan of a site; every scan is kept.
type Scan struct {
	ID       ID
	SiteID   ID
	Started  time.Time
	Finished time.Time
	Outcome  ScanOutcome
	Failure  string // why the connection failed, when it did
	Error    string // the message, when the scan did not complete

	WordPress string // installed version
	PHPCLI    string // PHP on the command line
	WPCLI     string // WP-CLI version
	Home      string // the site's address

	// Whether each update check is proven to have reached wordpress.org.
	CoreChecked      bool
	PluginsChecked   bool
	ThemesChecked    bool
	LanguagesChecked bool

	Items []ScanItem
}

// SiteStatus is the summary of a site's latest scan.
type SiteStatus string

const (
	StatusUnknown   SiteStatus = "unknown"
	StatusUpToDate  SiteStatus = "ok"
	StatusAttention SiteStatus = "warn"
)

// Status summarizes the scan: unknown when it did not complete; needs attention when an update is
// pending or cannot be installed, a check did not reach wordpress.org, or a plugin's update
// source is unknown; up to date otherwise.
func (s Scan) Status() SiteStatus {
	if s.Outcome != ScanCompleted {
		return StatusUnknown
	}
	if !s.CoreChecked || !s.PluginsChecked || !s.ThemesChecked || !s.LanguagesChecked {
		return StatusAttention
	}
	for _, item := range s.Items {
		switch {
		case item.Update == UpdateAvailable, item.Update == UpdateUnavailable, item.Update == UpdateUnknown:
			return StatusAttention
		case item.UpdateSourceUnknown():
			return StatusAttention
		}
	}
	return StatusUpToDate
}

// UpdateCounts counts the updates that can be installed now.
type UpdateCounts struct {
	Core, Plugins, Themes, Languages int
}

// PendingUpdates counts the available updates by kind.
func (s Scan) PendingUpdates() UpdateCounts {
	var c UpdateCounts
	for _, item := range s.Items {
		if item.Update != UpdateAvailable {
			continue
		}
		switch item.Kind {
		case ItemCore:
			c.Core++
		case ItemPlugin:
			c.Plugins++
		case ItemTheme:
			c.Themes++
		case ItemLanguage:
			c.Languages++
		}
	}
	return c
}
