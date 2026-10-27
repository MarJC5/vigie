// Package model holds the site manager: folders, hosts (SSH accounts) and sites (WordPress installs).
package model

import (
	"errors"
	"fmt"
	"strings"
	"uuid"
)

// ID identifies every record. Version 7 UUIDs sort by creation time and can be merged across
// machines (ADR-0007).
type ID = uuid.UUID

// NoID marks an absent parent (a folder or host at the root).
var NoID = uuid.Nil()

// NewID returns a new version 7 UUID.
func NewID() ID {
	return uuid.NewV7()
}

var (
	ErrEmptyName     = errors.New("name is empty")
	ErrSlashInName   = errors.New("name contains a slash")
	ErrDuplicateName = errors.New("name already used by a sibling")
	ErrNotFound      = errors.New("not found")
	ErrNoConnection  = errors.New("host needs either an SSH alias or a hostname")
	ErrNoUser        = errors.New("host with a hostname needs a user")
	ErrInvalidPort   = errors.New("port must be between 1 and 65535")
	ErrUnknownColor  = errors.New("unknown color")
)

// ValidateName checks a folder, host or site name. Names are used in command line paths
// ("Clients/Infomaniak/misits.ch"), so they cannot contain a slash.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrEmptyName
	}
	if strings.Contains(name, "/") {
		return ErrSlashInName
	}
	return nil
}

// Color is one of the eight site manager colors, in FileZilla's order so imports map one to one.
type Color int

const (
	ColorNone Color = iota
	ColorRed
	ColorGreen
	ColorBlue
	ColorYellow
	ColorCyan
	ColorMagenta
	ColorOrange
)

var colorNames = [...]string{"none", "red", "green", "blue", "yellow", "cyan", "magenta", "orange"}

// Colors returns every color in order.
func Colors() []Color {
	colors := make([]Color, len(colorNames))
	for i := range colorNames {
		colors[i] = Color(i)
	}
	return colors
}

func (c Color) String() string {
	if c < 0 || int(c) >= len(colorNames) {
		return fmt.Sprintf("Color(%d)", int(c))
	}
	return colorNames[c]
}

// ParseColor returns the color with the given name.
func ParseColor(name string) (Color, error) {
	for i, n := range colorNames {
		if n == name {
			return Color(i), nil
		}
	}
	return ColorNone, fmt.Errorf("%w: %q", ErrUnknownColor, name)
}

// Folder groups hosts and other folders.
type Folder struct {
	ID       ID
	ParentID ID // NoID at the root
	Name     string
	Color    Color
	Notes    string // Markdown
	Position int
}

// Host is one SSH account; its sites inherit its connection.
type Host struct {
	ID       ID
	FolderID ID // NoID at the root
	Name     string

	// Either an alias from ~/.ssh/config, or hostname and user (port 22 when zero).
	Alias    string
	Hostname string
	Port     int
	User     string
	KeyPath  string // optional; when set, only this key is offered

	WPCLI              string // WP-CLI command on the server; "wp" when empty
	MaxConcurrent      int    // commands at a time on this account; 2 when zero
	HostKeyFingerprint string // trusted on first connection
	Color              Color
	Notes              string
	Position           int
}

// Validate checks the name and the connection details.
func (h Host) Validate() error {
	if err := ValidateName(h.Name); err != nil {
		return err
	}
	hasAlias, hasHostname := h.Alias != "", h.Hostname != ""
	if hasAlias == hasHostname {
		return ErrNoConnection
	}
	if hasHostname && h.User == "" {
		return ErrNoUser
	}
	if h.Port < 0 || h.Port > 65535 {
		return ErrInvalidPort
	}
	return nil
}

// EffectivePort returns the SSH port, 22 by default.
func (h Host) EffectivePort() int {
	if h.Port == 0 {
		return 22
	}
	return h.Port
}

// Site is one WordPress install on a host. A multisite network is one site with its subsites.
type Site struct {
	ID       ID
	HostID   ID
	Name     string
	Path     string // WordPress root on the server (the directory containing wp-includes)
	URL      string
	WPCLI    string // overrides the host's command when set
	Color    Color
	Notes    string
	Tags     []string
	Position int

	Multisite bool
	Subsites  []Subsite
}

// Subsite is a site of a multisite network, read at each scan.
type Subsite struct {
	BlogID   int
	URL      string
	Public   bool
	Archived bool
	Spam     bool
	Deleted  bool
	Pinned   bool // checked over HTTP after updates (ADR-0009)
}
