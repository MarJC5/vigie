package wpcli

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

//go:embed copypaths.php
var copyPathsScript []byte

// CopyPathsScript is the read-only script CopyPaths runs, sent on its standard input (ADR-0041).
func CopyPathsScript() []byte {
	return bytes.Clone(copyPathsScript)
}

// CopyPaths asks where the parts of a site are, for a copy: WP-CLI runs CopyPathsScript, read on
// its standard input, so that a moved wp-content or uploads folder is followed.
func CopyPaths() Command {
	return Command{args: []string{"eval-file", "-"}, skipPlugins: true, skipThemes: true}
}

// SitePaths are the folders of a site's parts and its table prefix, as WordPress knows them.
type SitePaths struct {
	Root      string // the site's folder, where WordPress is
	Content   string // wp-content
	Plugins   string
	MuPlugins string
	Themes    string
	Uploads   string
	Prefix    string // the tables' prefix, the network's on a multisite
}

// ParseCopyPaths reads the script's answer; every folder must be an absolute, clean path.
func ParseCopyPaths(out []byte) (SitePaths, error) {
	var raw struct {
		Root, Content, Plugins, MuPlugins, Themes, Uploads, Prefix string
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &raw); err != nil {
		return SitePaths{}, fmt.Errorf("copy paths %.80q: %w", out, ErrUnexpectedOutput)
	}
	p := SitePaths{Root: raw.Root, Content: raw.Content, Plugins: raw.Plugins, MuPlugins: raw.MuPlugins,
		Themes: raw.Themes, Uploads: raw.Uploads, Prefix: raw.Prefix}
	for _, dir := range []string{p.Root, p.Content, p.Plugins, p.MuPlugins, p.Themes, p.Uploads} {
		if !strings.HasPrefix(dir, "/") || path.Clean(dir) != dir || dir == "/" || strings.ContainsAny(dir, "\n\x00") {
			return SitePaths{}, fmt.Errorf("copy paths: folder %q: %w", dir, ErrUnexpectedOutput)
		}
	}
	return p, nil
}

// DatabaseSize answers the size of the site's database in bytes.
func DatabaseSize() Command {
	return Command{args: []string{"db", "size", "--size_format=b"}, skipPlugins: true, skipThemes: true}
}

// ParseDatabaseSize reads DatabaseSize's answer, a bare number of bytes.
func ParseDatabaseSize(out []byte) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("database size %.40q: %w", out, ErrUnexpectedOutput)
	}
	return n, nil
}

// DatabaseTables lists every table of the database with its size, those without the prefix too:
// a dump exports them all.
func DatabaseTables() Command {
	return Command{args: []string{"db", "size", "--tables", "--all-tables", "--format=json"}, skipPlugins: true, skipThemes: true}
}

// Table is a table of the database and its size on disk.
type Table struct {
	Name  string
	Bytes int64
}

// ParseDatabaseTables reads DatabaseTables's answer: without a size format, WP-CLI writes each
// size as its bytes followed by " B".
func ParseDatabaseTables(out []byte) ([]Table, error) {
	var rows []struct{ Name, Size string }
	if err := json.Unmarshal(bytes.TrimSpace(out), &rows); err != nil {
		return nil, fmt.Errorf("database tables %.80q: %w", out, ErrUnexpectedOutput)
	}
	tables := make([]Table, 0, len(rows))
	for _, r := range rows {
		digits, ok := strings.CutSuffix(r.Size, " B")
		n, err := strconv.ParseInt(digits, 10, 64)
		if !ok || err != nil || n < 0 || r.Name == "" {
			return nil, fmt.Errorf("database table %q of %q: %w", r.Name, r.Size, ErrUnexpectedOutput)
		}
		tables = append(tables, Table{Name: r.Name, Bytes: n})
	}
	return tables, nil
}

// tableName is a table name Vigie passes to a dump; another one is refused, and the database is then
// dumped whole.
var tableName = regexp.MustCompile(`^[A-Za-z0-9_$-]+$`)

// DatabaseDump writes a dump of the database, or of the tables given, to the standard output:
// InnoDB tables are read in one transaction, without locking the site (ADR-0041).
func DatabaseDump(tables []string) Command {
	args := []string{"db", "export", "-", "--single-transaction"}
	if len(tables) > 0 {
		for _, t := range tables {
			if !tableName.MatchString(t) {
				return Command{err: fmt.Errorf("table %q: %w", t, ErrBadArgument)}
			}
		}
		args = append(args, "--tables="+strings.Join(tables, ","))
	}
	return Command{args: args, skipPlugins: true, skipThemes: true}
}
