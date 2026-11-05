package model

import (
	"errors"
	"time"
)

// ErrInvalidSetAside is returned for a set-aside that is not a plugin or a theme with a slug and a
// version.
var ErrInvalidSetAside = errors.New("a set-aside is a plugin or theme version")

// SetAside is a version of a plugin or theme that broke its site and was rolled back on its own
// (ADR-0042, point 5): Update all leaves it out until a newer version is offered or the user
// updates it alone.
type SetAside struct {
	SiteID      ID
	Kind        ItemKind
	Slug        string
	Version     string // the version rolled back
	OperationID ID     // the update it broke
	Reason      string // the check that failed
	Created     time.Time
}

// Validate refuses what cannot roll back on its own: WordPress, the translations, an item without
// a slug or a version.
func (s SetAside) Validate() error {
	if (s.Kind != ItemPlugin && s.Kind != ItemTheme) || s.Slug == "" || s.Version == "" {
		return ErrInvalidSetAside
	}
	return nil
}

// Holds tells whether a set-aside still stands after a scan: another version offered, the version
// set aside installed (updated elsewhere), or the item removed end it. A scan that sees no update
// keeps it: a premium plugin offers its update itself, which a scan does not run.
func (s SetAside) Holds(items []ScanItem) bool {
	for _, it := range items {
		if it.Kind == s.Kind && it.Slug == s.Slug {
			if it.Version == s.Version {
				return false
			}
			return it.UpdateVersion == "" || it.UpdateVersion == s.Version
		}
	}
	return false
}
