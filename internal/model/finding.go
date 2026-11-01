package model

import (
	"errors"
	"strings"
	"time"
)

// FindingLevel is how much a finding calls for action (ADR-0015).
type FindingLevel string

const (
	LevelUrgent     FindingLevel = "urgent"
	LevelAttention  FindingLevel = "attention"
	LevelInfo       FindingLevel = "info"        // read and fine, or good to know; never counted
	LevelNotChecked FindingLevel = "not-checked" // the check could not run; Facts say why
)

// FindingKind is what a finding is about: a vulnerability, a changed file, a setting...
type FindingKind string

// The kinds of findings, with their subject, identifier and facts.
const (
	// KindVulnerability: Subject "core", "plugin:<slug>", "theme:<slug>" or "php"; Identifier the
	// record's uuid; Facts name, severity, score, kev, unfixed, fixed, sources (links, space
	// separated), fetched (when the answer is from an older cache).
	KindVulnerability FindingKind = "vulnerability"
	// KindPHPSupport: Subject "php"; Facts status ("s" security fixes only, "d" unmaintained), end.
	KindPHPSupport FindingKind = "php-support"
	// KindPluginClosed: Facts date, reason (wordpress.org's, such as "security-issue"), other-plugin
	// ("true" when the files say the closed plugin is another one of the same name).
	KindPluginClosed FindingKind = "plugin-closed"
	// KindPluginAbandoned: Facts last-release.
	KindPluginAbandoned FindingKind = "plugin-abandoned"
	// KindCoreFile: the WordPress files that differ from wordpress.org's, in one finding; Facts
	// changed, missing, added (counts), files (the first ones, comma separated). A file often
	// deleted on purpose has its own finding: Identifier its path, Facts change.
	KindCoreFile FindingKind = "core-file"
	// KindPluginFile: the files of a plugin that differ from wordpress.org's, one finding per
	// plugin; Facts added, changed (counts), files (the first ones, comma separated), mismatch
	// ("true" when nearly every file differs: another plugin of the same name), uncomparable
	// ("true" when wordpress.org's list does not match this version's files at all).
	KindPluginFile FindingKind = "plugin-file"
	// KindPluginUnverified: wordpress.org publishes no checksums for the plugin's version.
	KindPluginUnverified FindingKind = "plugin-unverified"
	// KindUploadsPHP: Identifier the file, relative to the uploads folder. KindUploadsPHPMore: the
	// list stopped at its limit.
	KindUploadsPHP     FindingKind = "uploads-php"
	KindUploadsPHPMore FindingKind = "uploads-php-more"
	// KindAdministrator: Subject "user:<login>"; Facts email, registered, two-factor (on, off,
	// unknown), super.
	KindAdministrator FindingKind = "administrator"
	// Settings, Subject "site": file editing allowed from the administration, PHP errors shown on
	// the pages, hidden from search engines, an address without HTTPS, WP-Cron events overdue
	// (Facts count, hook, since), autoloaded options too heavy (Facts bytes, limit).
	KindFileEditing      FindingKind = "file-editing"
	KindErrorsDisplayed  FindingKind = "errors-displayed"
	KindHiddenFromSearch FindingKind = "hidden-from-search"
	KindNoHTTPS          FindingKind = "no-https"
	KindCronOverdue      FindingKind = "cron-overdue"
	KindAutoloadSize     FindingKind = "autoload-size"
	// KindDatabaseVersion: Facts version.
	KindDatabaseVersion FindingKind = "database-version"
	// KindCertificate: Version the expiry date; Facts expires, issuer, problem.
	KindCertificate FindingKind = "certificate"
	// KindDomain: Version the expiry date; Facts domain, expires, or reason when the registry does
	// not publish it.
	KindDomain FindingKind = "domain"
	// KindDebugLog: the site serves its debug.log to anyone; Facts url.
	KindDebugLog FindingKind = "debug-log"
	// KindDebugFatal: fatal or parse errors written to debug.log in the last 7 days; Facts count,
	// last (RFC 3339), message (the latest's, cut).
	KindDebugFatal FindingKind = "debug-fatal"
	// KindCheck: a check that could not run, Identifier its name; Facts reason ("off": the
	// destination is turned off or not agreed to; "error"), error.
	KindCheck FindingKind = "check"
)

// Finding is one result of a scan beyond the versions and updates. It holds data, not text: the
// interface and the command line write it in their language.
type Finding struct {
	Kind       FindingKind
	Subject    string // "core", "plugin:<slug>", "theme:<slug>", "php", "site", a user, a file...
	Identifier string // what tells two findings of one subject apart: a vulnerability's uuid, a path
	Version    string // the subject's installed version, when it has one
	Level      FindingLevel
	Facts      map[string]string // the details shown: score, fixed version, date, reason...
	// Ignore sets the finding aside when an ignore of its site covers it: it stays listed and never
	// counts. Not stored with the scan: an ignore applies to the scans before it too.
	Ignore *Ignore
}

// Key names a finding the same way from one scan to the next, whatever its version and facts: an
// ignore refers to it.
func (f Finding) Key() string {
	return string(f.Kind) + "|" + f.Subject + "|" + f.Identifier
}

// ErrReasonRequired is returned for a permanent ignore without a reason.
var ErrReasonRequired = errors.New("a reason is required to ignore a finding permanently")

// Ignore sets a finding of a site aside: until the version it was made for changes, or
// permanently with a reason.
type Ignore struct {
	SiteID       ID
	Key          string
	UntilVersion string // empty: permanent
	Reason       string
	Created      time.Time
}

// Validate refuses a permanent ignore without a reason.
func (i Ignore) Validate() error {
	if i.UntilVersion == "" && strings.TrimSpace(i.Reason) == "" {
		return ErrReasonRequired
	}
	return nil
}

// Covers tells whether the ignore applies to a finding: the same key, and the same version unless
// it is permanent.
func (i Ignore) Covers(f Finding) bool {
	return i.Key == f.Key() && (i.UntilVersion == "" || i.UntilVersion == f.Version)
}

// UpdateKey is what an ignore of an update offered is kept by (ADR-0026): the item, whatever
// version is offered; a key of the same form as a finding's.
func UpdateKey(item ScanItem) string {
	return "update|" + string(item.Kind) + ":" + item.Slug + "|"
}

// CoversUpdate tells whether the ignore applies to the update an item is offered: the same item,
// and the version ignored unless it is permanent.
func (i Ignore) CoversUpdate(item ScanItem) bool {
	return item.Update == UpdateAvailable && i.Key == UpdateKey(item) && (i.UntilVersion == "" || i.UntilVersion == item.UpdateVersion)
}

// ApplyIgnores marks the findings and the updates of the scan that the ignores of its site cover.
func (s *Scan) ApplyIgnores(ignores []Ignore) {
	for i := range s.Findings {
		s.Findings[i].Ignore = nil
		for _, ig := range ignores {
			if ig.SiteID == s.SiteID && ig.Covers(s.Findings[i]) {
				s.Findings[i].Ignore = &ig
				break
			}
		}
	}
	for i := range s.Items {
		s.Items[i].Ignore = nil
		for _, ig := range ignores {
			if ig.SiteID == s.SiteID && ig.CoversUpdate(s.Items[i]) {
				s.Items[i].Ignore = &ig
				break
			}
		}
	}
}

// FindingCounts counts the findings that are not ignored, by level; information is not counted.
type FindingCounts struct {
	Urgent, Attention, NotChecked int
}

// CountFindings counts the findings that are not ignored.
func (s Scan) CountFindings() FindingCounts {
	var c FindingCounts
	for _, f := range s.Findings {
		if f.Ignore != nil {
			continue
		}
		switch f.Level {
		case LevelUrgent:
			c.Urgent++
		case LevelAttention:
			c.Attention++
		case LevelNotChecked:
			c.NotChecked++
		}
	}
	return c
}
