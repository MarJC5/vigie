// Package exchange is Vigie's export format: the site manager in a JSON file, read again by an
// import on this machine or another one, and by add-ons. Version 1 is described in docs/data.md,
// with its JSON Schema.
package exchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
)

// The format's name and the version this Vigie writes; every older version stays readable.
const (
	Format  = "vigie"
	Version = 1
)

var (
	// ErrNotAnExport is returned for a file that is not a Vigie export.
	ErrNotAnExport = errors.New("not a Vigie export")
	// ErrNewerVersion is returned for an export written by a newer Vigie.
	ErrNewerVersion = errors.New("written by a newer Vigie")
	// ErrInvalid is returned for an export whose content does not hold together.
	ErrInvalid = errors.New("invalid export")
)

// File is an export.
type File struct {
	Format   string    `json:"format"`
	Version  int       `json:"version"`
	Exported time.Time `json:"exported"`
	App      string    `json:"app"`   // the Vigie version that wrote it
	Scope    string    `json:"scope"` // the path of the node exported; "" for everything
	Folders  []Folder  `json:"folders"`
	Hosts    []Host    `json:"hosts"`
	Sites    []Site    `json:"sites"`
	Ignores  []Ignore  `json:"ignores"`
	History  *History  `json:"history,omitempty"`
}

// Folder is a folder; Parent is "" at the top of the export.
type Folder struct {
	ID       string `json:"id"`
	Parent   string `json:"parent"`
	Name     string `json:"name"`
	Color    string `json:"color"` // none, red, green, blue, yellow, cyan, magenta, orange
	Notes    string `json:"notes"`
	Position int    `json:"position"`
}

// Host is an SSH account; Folder is "" at the top of the export.
type Host struct {
	ID            string    `json:"id"`
	Folder        string    `json:"folder"`
	Name          string    `json:"name"`
	Alias         string    `json:"alias"`
	Hostname      string    `json:"hostname"`
	Port          int       `json:"port"`
	User          string    `json:"user"`
	KeyPath       string    `json:"keyPath"`
	WPCLI         string    `json:"wpCli"`
	MaxConcurrent int       `json:"maxConcurrent"`
	TrustedKeys   []HostKey `json:"trustedKeys"`
	Color         string    `json:"color"`
	Notes         string    `json:"notes"`
	Position      int       `json:"position"`
}

// HostKey is a public key of the server Vigie trusted, under the name its known_hosts uses
// ("host", or "[host]:port"). Its fingerprint is computed from the key, never read from a file.
type HostKey struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Key  string `json:"key"`
}

// Site is a WordPress install of a host.
type Site struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	URL       string    `json:"url"`
	WPCLI     string    `json:"wpCli"`
	Color     string    `json:"color"`
	Notes     string    `json:"notes"`
	Tags      []string  `json:"tags"`
	Position  int       `json:"position"`
	Multisite bool      `json:"multisite"`
	Subsites  []Subsite `json:"subsites"`
}

// Subsite is a site of a network.
type Subsite struct {
	BlogID   int    `json:"blogId"`
	URL      string `json:"url"`
	Public   bool   `json:"public"`
	Archived bool   `json:"archived"`
	Spam     bool   `json:"spam"`
	Deleted  bool   `json:"deleted"`
	Pinned   bool   `json:"pinned"`
}

// Ignore sets a finding of a site aside.
type Ignore struct {
	Site         string    `json:"site"`
	Key          string    `json:"key"`
	UntilVersion string    `json:"untilVersion"` // "": permanently
	Reason       string    `json:"reason"`
	Created      time.Time `json:"created"`
}

// History is what Vigie recorded of the sites exported.
type History struct {
	Scans      []Scan      `json:"scans"`
	Operations []Operation `json:"operations"`
}

// Scan is a scan of a site, with what it read and found.
type Scan struct {
	ID               string     `json:"id"`
	Site             string     `json:"site"`
	Started          time.Time  `json:"started"`
	Finished         time.Time  `json:"finished"`
	Outcome          string     `json:"outcome"`
	Failure          string     `json:"failure"`
	Error            string     `json:"error"`
	WordPress        string     `json:"wordpress"`
	PHPCLI           string     `json:"phpCli"`
	WPCLI            string     `json:"wpCli"`
	Home             string     `json:"home"`
	CoreChecked      bool       `json:"coreChecked"`
	PluginsChecked   bool       `json:"pluginsChecked"`
	ThemesChecked    bool       `json:"themesChecked"`
	LanguagesChecked bool       `json:"languagesChecked"`
	FilesChecked     *time.Time `json:"filesChecked,omitempty"`
	Items            []Item     `json:"items"`
	Findings         []Finding  `json:"findings"`
}

// Item is a part of a site a scan read: WordPress, a plugin, a theme, a translation.
type Item struct {
	Kind           string `json:"kind"`
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	Version        string `json:"version"`
	Language       string `json:"language"`
	Update         string `json:"update"`
	UpdateVersion  string `json:"updateVersion"`
	AutoUpdate     bool   `json:"autoUpdate"`
	OnWordPressOrg bool   `json:"onWordPressOrg"`
}

// Finding is a result of a security scan.
type Finding struct {
	Kind       string            `json:"kind"`
	Subject    string            `json:"subject"`
	Identifier string            `json:"identifier"`
	Version    string            `json:"version"`
	Level      string            `json:"level"`
	Facts      map[string]string `json:"facts"`
}

// Operation is an update or a rollback of a site, with its steps.
type Operation struct {
	ID        string     `json:"id"`
	Site      string     `json:"site"`
	Kind      string     `json:"kind"`
	Started   time.Time  `json:"started"`
	Finished  *time.Time `json:"finished,omitempty"`
	Outcome   string     `json:"outcome"`
	Summary   string     `json:"summary"`
	BackupDir string     `json:"backupDir"`
	Steps     []Step     `json:"steps"`
}

// Step is a step of an operation.
type Step struct {
	Step          string     `json:"step"`
	ItemKind      string     `json:"itemKind"`
	Slug          string     `json:"slug"`
	VersionBefore string     `json:"versionBefore"`
	VersionAfter  string     `json:"versionAfter"`
	Started       time.Time  `json:"started"`
	Finished      *time.Time `json:"finished,omitempty"`
	Outcome       string     `json:"outcome"`
	Message       string     `json:"message"`
}

// Write writes an export, indented so it reads in a text editor.
func Write(w io.Writer, f File) error {
	f.Format, f.Version = Format, Version
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(f)
}

// maxSize bounds what is read: far above a site manager with a year of history.
const maxSize = 256 << 20

// Read reads an export of any version this Vigie knows, and checks that it holds together. A
// field it does not know is refused: a newer Vigie's file says so rather than losing data.
func Read(r io.Reader) (File, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return File{}, err
	}
	if len(data) > maxSize {
		return File{}, fmt.Errorf("larger than %d MB: %w", maxSize>>20, ErrInvalid)
	}
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	// The first value only: what follows it is refused below, as data that is not the export.
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&head); err != nil || head.Format != Format {
		return File{}, ErrNotAnExport
	}
	switch {
	case head.Version > Version:
		return File{}, fmt.Errorf("version %d: %w", head.Version, ErrNewerVersion)
	case head.Version < 1:
		return File{}, fmt.Errorf("version %d: %w", head.Version, ErrNotAnExport)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if dec.More() {
		return File{}, fmt.Errorf("data after the export: %w", ErrInvalid)
	}
	return f, f.Check()
}

// Check refuses an export whose parts do not hold together: IDs that are not IDs or come twice,
// a parent, host or site that is not in the export, a node without a name, a value the schema
// does not list. Read checks what it reads.
func (f File) Check() error {
	seen := map[string]bool{}
	id := func(kind, s string) error {
		if _, err := uuid.Parse(s); err != nil {
			return fmt.Errorf("%s ID %q: %w", kind, s, ErrInvalid)
		}
		if seen[s] {
			return fmt.Errorf("%s ID %s twice: %w", kind, s, ErrInvalid)
		}
		seen[s] = true
		return nil
	}
	folders, hosts, sites := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range f.Folders {
		if err := id("folder", x.ID); err != nil {
			return err
		}
		folders[x.ID] = true
	}
	for _, x := range f.Hosts {
		if err := id("host", x.ID); err != nil {
			return err
		}
		hosts[x.ID] = true
	}
	for _, x := range f.Sites {
		if err := id("site", x.ID); err != nil {
			return err
		}
		sites[x.ID] = true
	}
	for _, x := range f.Folders {
		if x.Name == "" || (x.Parent != "" && !folders[x.Parent]) || x.Parent == x.ID || !knownColor(x.Color) {
			return fmt.Errorf("folder %s: %w", x.ID, ErrInvalid)
		}
	}
	for _, x := range f.Hosts {
		if x.Name == "" || (x.Folder != "" && !folders[x.Folder]) || !knownColor(x.Color) {
			return fmt.Errorf("host %s: %w", x.ID, ErrInvalid)
		}
		for _, k := range x.TrustedKeys {
			if !keyTypes[k.Type] {
				return fmt.Errorf("host %s, key type %q: %w", x.ID, k.Type, ErrInvalid)
			}
		}
	}
	for _, x := range f.Sites {
		if x.Name == "" || x.Path == "" || !hosts[x.Host] || !knownColor(x.Color) {
			return fmt.Errorf("site %s: %w", x.ID, ErrInvalid)
		}
	}
	for _, x := range f.Ignores {
		if !sites[x.Site] || x.Key == "" {
			return fmt.Errorf("ignore of %s: %w", x.Site, ErrInvalid)
		}
	}
	if f.History != nil {
		for _, x := range f.History.Scans {
			if err := id("scan", x.ID); err != nil {
				return err
			}
			if !sites[x.Site] {
				return fmt.Errorf("scan %s: %w", x.ID, ErrInvalid)
			}
			for _, fd := range x.Findings {
				if !levels[model.FindingLevel(fd.Level)] {
					return fmt.Errorf("scan %s, level %q: %w", x.ID, fd.Level, ErrInvalid)
				}
			}
		}
		for _, x := range f.History.Operations {
			if err := id("operation", x.ID); err != nil {
				return err
			}
			if !sites[x.Site] {
				return fmt.Errorf("operation %s: %w", x.ID, ErrInvalid)
			}
		}
	}
	return f.checkFolderLoops()
}

// The values the schema lists: what Vigie shows and counts by them.
var (
	keyTypes = map[string]bool{"ssh-ed25519": true, "ecdsa-sha2-nistp256": true, "ecdsa-sha2-nistp384": true,
		"ecdsa-sha2-nistp521": true, "ssh-rsa": true}
	levels = map[model.FindingLevel]bool{model.LevelUrgent: true, model.LevelAttention: true, model.LevelInfo: true,
		model.LevelNotChecked: true}
)

func knownColor(name string) bool {
	_, err := model.ParseColor(name)
	return err == nil
}

// checkFolderLoops refuses folders that are each other's parents.
func (f File) checkFolderLoops() error {
	parent := map[string]string{}
	for _, x := range f.Folders {
		parent[x.ID] = x.Parent
	}
	for _, x := range f.Folders {
		at := x.ID
		for range len(f.Folders) + 1 {
			if at = parent[at]; at == "" {
				break
			}
			if at == x.ID {
				return fmt.Errorf("folder %s is its own parent: %w", x.ID, ErrInvalid)
			}
		}
		if at != "" {
			return fmt.Errorf("folder %s in a loop: %w", x.ID, ErrInvalid)
		}
	}
	return nil
}
