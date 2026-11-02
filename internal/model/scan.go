package model

import "time"

// PHPWeb is the PHP version serving a site's pages, as its dashboard last noted it: WordPress keeps
// its check of that version a week (ADR-0028).
type PHPWeb struct {
	Version string    // empty when not read
	Seen    time.Time // when the dashboard noted it, a week before its note expires; zero when unknown
	Unknown PHPWebUnknown
}

// NetworkRead is what each subsite of a network has active, and who administers it (ADR-0029).
type NetworkRead struct {
	Plugins []string      // slugs of the plugins active network-wide
	Sites   []SubsiteRead // the subsites not archived, spam nor deleted
}

// SubsiteRead is one subsite as a scan read it.
type SubsiteRead struct {
	BlogID                         int
	Plugins                        []string // slugs of the plugins active on it, besides the network's
	Theme, ThemeName, ThemeVersion string
	AdminEmail                     string
	Admins                         []string // logins; their administrator findings tell their two-factor
}

// DebugLog is the log of PHP errors WordPress writes, as a scan found it (ADR-0028).
type DebugLog struct {
	Path    string // where it is, relative to the site's folder when within it; empty when there is none
	Size    int64
	Written bool // WordPress writes to it (WP_DEBUG and WP_DEBUG_LOG); a file left from before otherwise
}

// PHPWebUnknown is why the PHP serving the pages was not read.
type PHPWebUnknown string

const (
	PHPWebRead        PHPWebUnknown = ""
	PHPWebNotNoted    PHPWebUnknown = "not-noted"    // no note this week: nobody opened the dashboard, or it could not reach wordpress.org
	PHPWebObjectCache PHPWebUnknown = "object-cache" // the notes are in an external object cache, which WP-CLI does not list
	PHPWebUnmatched   PHPWebUnknown = "unmatched"    // a note of a version Vigie cannot name: not the command line's
)

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
	// Ignore is the ignore of the site that covers its update offered (ADR-0026), set by
	// ApplyIgnores; nil when it counts.
	Ignore *Ignore
}

// UpdateSourceUnknown tells that no update can be seen for this plugin: wordpress.org does not
// know it, and the code of a premium plugin's own updater does not run during a scan. Must-use
// plugins and drop-ins are left out: WordPress never updates them.
func (i ScanItem) UpdateSourceUnknown() bool {
	return i.Kind == ItemPlugin && !i.OnWordPressOrg && i.Update == UpdateNone &&
		i.Status != "must-use" && i.Status != "dropin"
}

// Scan is one scan of a site; every scan is kept.
// Backup is the backups of one update on the server, as a scan read them (ADR-0033).
type Backup struct {
	At    time.Time `json:"at"` // the update's time, the name of its folder
	Bytes int64     `json:"bytes"`
}

type Scan struct {
	ID       ID
	SiteID   ID
	RunID    ID // the run that made it; NoID for scans made before runs were recorded
	Started  time.Time
	Finished time.Time
	Outcome  ScanOutcome
	Failure  string // why the connection failed, when it did
	Error    string // the message, when the scan did not complete

	WordPress string // installed version
	PHPCLI    string // PHP on the command line
	PHPWeb    PHPWeb // PHP serving the pages, as the dashboard noted it
	// AdminEmail is where WordPress sends its notices (admin_email); AdminEmailPending the address
	// waiting for its confirmation (new_admin_email), when there is one.
	AdminEmail        string
	AdminEmailPending string
	DebugLog          DebugLog
	// Network is what a scan of a network read of its subsites (ADR-0029); nil for a single site,
	// or when the read failed (the administrators are then "not checked").
	Network *NetworkRead
	// Backups are the site's backups on the server, Vigie's own before each update, oldest first
	// (ADR-0033); nil when not read.
	Backups []Backup
	// What the scan noted of itself when it was made, for the journal (ADR-0030): its status with
	// the ignores of then, and what changed since the site's complete scan before (empty when
	// nothing did, or there was none). Recorded is false for a scan stored before ADR-0030 until
	// it is computed once.
	Recorded        bool
	RecordedStatus  SiteStatus
	RecordedChanges []Change
	WPCLI           string // WP-CLI version
	Home            string // the site's address
	SiteURL         string // the WordPress address (siteurl), where wp-admin lives

	// Whether each update check is proven to have reached wordpress.org.
	CoreChecked      bool
	PluginsChecked   bool
	ThemesChecked    bool
	LanguagesChecked bool

	Items []ScanItem

	// FilesChecked is when the files were last compared with wordpress.org's: at most once a day,
	// a scan in between carries those findings over. Zero when never.
	FilesChecked time.Time
	Findings     []Finding

	// Attempt is a later scan of the site that did not complete (cancelled, failed): this scan,
	// the latest complete one, stays what is shown, and the attempt says why nothing newer is.
	// Only the storage sets it.
	Attempt *Scan
}

// SiteStatus is the summary of a site's latest scan.
type SiteStatus string

const (
	StatusUnknown   SiteStatus = "unknown"
	StatusUpToDate  SiteStatus = "ok"
	StatusAttention SiteStatus = "warn"
	StatusUrgent    SiteStatus = "danger"
	StatusPartial   SiteStatus = "partial" // not fully checked: a check of the scan could not run
)

// Status summarizes the scan from its worst part, the ignored findings left aside: unknown when it
// did not complete; urgent when a finding is; needs attention when a finding does, an update is
// pending or cannot be installed, or an update check did not reach wordpress.org; not fully
// checked when a check could not run; up to date otherwise. A plugin whose update source is
// unknown is shown as such but does not change the status: most client sites have one, and "needs
// attention" must keep its meaning.
func (s Scan) Status() SiteStatus {
	if s.Outcome != ScanCompleted {
		return StatusUnknown
	}
	c := s.CountFindings()
	switch {
	case c.Urgent > 0:
		return StatusUrgent
	case c.Attention > 0 || !s.CoreChecked || !s.PluginsChecked || !s.ThemesChecked || !s.LanguagesChecked:
		return StatusAttention
	}
	for _, item := range s.Items {
		switch {
		case item.Ignore != nil:
		case item.Update == UpdateAvailable, item.Update == UpdateUnavailable, item.Update == UpdateUnknown:
			return StatusAttention
		}
	}
	if c.NotChecked > 0 {
		return StatusPartial
	}
	return StatusUpToDate
}

// UpdateCounts counts the updates that can be installed now.
type UpdateCounts struct {
	Core, Plugins, Themes, Languages int
}

// PendingUpdates counts the available updates by kind, the ignored ones left out.
func (s Scan) PendingUpdates() UpdateCounts {
	var c UpdateCounts
	for _, item := range s.Items {
		if item.Update != UpdateAvailable || item.Ignore != nil {
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
