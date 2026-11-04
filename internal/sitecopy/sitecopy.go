// Package sitecopy makes a copy of a site on the user's computer (ADR-0041): which parts of the site
// it holds, in which archives, split by size, and its manifest, copy.json.
package sitecopy

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// Content is what the user chose to copy (point 1).
type Content string

const (
	DatabaseOnly Content = "database" // the database
	WithContent  Content = "content"  // the database and wp-content
	WholeSite    Content = "site"     // the database and the whole site folder, WordPress included
)

// Part is a part of a site, in its own archives (point 2).
type Part string

const (
	Database  Part = "database"
	Plugins   Part = "plugins"
	Themes    Part = "themes"
	MuPlugins Part = "mu-plugins"
	Uploads   Part = "uploads"
	Others    Part = "others"    // the rest of wp-content
	WordPress Part = "wordpress" // the site folder without wp-content, wp-config.php included
)

// PartFolder is a part of a site's files: the folder it is read from, and the folders inside it
// that belong to other parts or were left out.
type PartFolder struct {
	Part Part
	Dir  string
	Skip []string
}

// Folders are the parts of the site's files a content holds, in the order they are copied; leftOut
// are names of folders of wp-content left out by the user (caches, other backups). A part whose
// folder is left out is not copied.
func Folders(p wpcli.SitePaths, content Content, leftOut []string) []PartFolder {
	if content == DatabaseOnly {
		return nil
	}
	// A folder left out is a name of wp-content's own folders, nothing else.
	leftOut = slices.DeleteFunc(slices.Clone(leftOut), func(name string) bool {
		return name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00\n")
	})
	left := map[string]bool{}
	for _, name := range leftOut {
		left[path.Join(p.Content, name)] = true
	}
	var parts []PartFolder
	var inContent []string // the parts' folders inside wp-content, skipped from others
	for _, f := range []PartFolder{{Part: Plugins, Dir: p.Plugins}, {Part: Themes, Dir: p.Themes}, {Part: MuPlugins, Dir: p.MuPlugins}, {Part: Uploads, Dir: p.Uploads}} {
		if inside(f.Dir, p.Content) {
			inContent = append(inContent, f.Dir)
		}
		if !left[f.Dir] {
			parts = append(parts, f)
		}
	}
	skip := inContent
	for _, name := range leftOut {
		if dir := path.Join(p.Content, name); !slices.Contains(skip, dir) {
			skip = append(skip, dir)
		}
	}
	parts = append(parts, PartFolder{Part: Others, Dir: p.Content, Skip: skip})
	if content == WholeSite {
		var skipRoot []string
		if inside(p.Content, p.Root) {
			skipRoot = []string{p.Content}
		}
		parts = append(parts, PartFolder{Part: WordPress, Dir: p.Root, Skip: skipRoot})
	}
	return parts
}

// inside tells whether dir is a folder below parent.
func inside(dir, parent string) bool {
	return strings.HasPrefix(dir, parent+"/")
}

// Group groups files in their order, each group at most limit bytes; a file larger than the limit
// makes a group alone (point 3).
func Group(files []remote.File, limit int64) [][]remote.File {
	var groups [][]remote.File
	var current []remote.File
	var size int64
	for _, f := range files {
		if len(current) > 0 && size+f.Bytes > limit {
			groups = append(groups, current)
			current, size = nil, 0
		}
		current = append(current, f)
		size += f.Bytes
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

// ArchiveName is the name of the i-th archive of a part, from 0: uploads.tar.gz, uploads-2.tar.gz;
// database.sql.gz, database-2.sql.gz.
func ArchiveName(part Part, i int) string {
	ext := ".tar.gz"
	if part == Database {
		ext = ".sql.gz"
	}
	if i == 0 {
		return string(part) + ext
	}
	return string(part) + "-" + strconv.Itoa(i+1) + ext
}

// Archive is one archive of a copy: what it is read from, and how much it should weigh before
// compression.
type Archive struct {
	Part Part
	Name string
	// The files of a part's archive, from Dir; nil: the whole folder, its Skip left out (an account
	// without GNU find and tar).
	Dir   string
	Skip  []string
	Files []remote.File
	// The tables of a database archive; nil: the whole database.
	Tables []string
	Bytes  int64
}

// DatabaseArchives are the dumps of the database: one, or several of whole tables when the database
// is larger than the limit. A table whose name a dump refuses keeps the database in one dump.
func DatabaseArchives(tables []wpcli.Table, limit int64) []Archive {
	var total int64
	files := make([]remote.File, len(tables))
	for i, t := range tables {
		total += t.Bytes
		files[i] = remote.File{Name: t.Name, Bytes: t.Bytes}
		if !wpcli.DumpableTable(t.Name) {
			return []Archive{{Part: Database, Name: ArchiveName(Database, 0), Bytes: totalOf(tables)}}
		}
	}
	if total <= limit {
		return []Archive{{Part: Database, Name: ArchiveName(Database, 0), Bytes: total}}
	}
	var archives []Archive
	for i, g := range Group(files, limit) {
		a := Archive{Part: Database, Name: ArchiveName(Database, i)}
		for _, f := range g {
			a.Tables = append(a.Tables, f.Name)
			a.Bytes += f.Bytes
		}
		archives = append(archives, a)
	}
	return archives
}

func totalOf(tables []wpcli.Table) int64 {
	var n int64
	for _, t := range tables {
		n += t.Bytes
	}
	return n
}

// Inputs are what the server answered before a copy.
type Inputs struct {
	Folders []PartFolder
	Listed  map[Part][]remote.File // the files of each part, when Split
	Tables  []wpcli.Table
	Limit   int64
	Split   bool // the account has GNU find and tar
}

// Plan is every archive of a copy, in the order they are read: the database first, then each part.
// Split, a part is grouped by size and an empty part has no archive; not split, each part is one
// archive of its whole folder.
func Plan(in Inputs) []Archive {
	archives := DatabaseArchives(in.Tables, in.Limit)
	for _, f := range in.Folders {
		if !in.Split {
			archives = append(archives, Archive{Part: f.Part, Name: ArchiveName(f.Part, 0), Dir: f.Dir, Skip: f.Skip})
			continue
		}
		for i, g := range Group(in.Listed[f.Part], in.Limit) {
			a := Archive{Part: f.Part, Name: ArchiveName(f.Part, i), Dir: f.Dir, Skip: f.Skip, Files: g}
			for _, file := range g {
				a.Bytes += file.Bytes
			}
			archives = append(archives, a)
		}
	}
	return archives
}

// ManifestName is the manifest's file in a copy's folder.
const ManifestName = "copy.json"

// Manifest is copy.json: what a copy holds, so that it can be checked on its own (point 5).
type Manifest struct {
	Vigie     string    `json:"vigie"`
	Site      string    `json:"site"`
	Host      string    `json:"host"`
	WordPress string    `json:"wordpress"`
	Prefix    string    `json:"prefix"`
	Content   Content   `json:"content"`
	LeftOut   []string  `json:"leftOut,omitempty"`
	Started   time.Time `json:"started"`
	Archives  []Record  `json:"archives"`
}

// Record is an archive of the copy as it was written: its size once compressed, its digest and
// when it was read; a resumed copy mixes two moments (point 6).
type Record struct {
	Name   string    `json:"name"`
	Part   Part      `json:"part"`
	Bytes  int64     `json:"bytes"`
	SHA256 string    `json:"sha256"`
	Read   time.Time `json:"read"`
}

// WriteManifest writes copy.json in dir, readable by the user only, replacing the one there at once.
func WriteManifest(dir string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, ManifestName+".partial")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, ManifestName))
}

// ReadManifest reads the copy.json of dir.
func ReadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", filepath.Join(dir, ManifestName), err)
	}
	return m, nil
}
