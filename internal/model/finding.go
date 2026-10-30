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

// Finding is one result of a scan beyond the versions and updates. It holds data, not text: the
// interface and the command line write it in their language.
type Finding struct {
	Kind       FindingKind
	Subject    string // "core", "plugin:<slug>", "theme:<slug>", "php", "site", a user, a file...
	Identifier string // what tells two findings of one subject apart: a vulnerability's uuid, a path
	Version    string // the subject's installed version, when it has one
	Level      FindingLevel
	Facts      map[string]string // the details shown: score, fixed version, date, reason...
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
