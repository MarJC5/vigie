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
