package sitecopy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

var (
	// ErrNoSpace is returned when the computer has less free space than the copy needs.
	ErrNoSpace = errors.New("not enough free space on this computer")
	// ErrDump is returned when the database dump failed or came incomplete.
	ErrDump = errors.New("the database dump failed")
	// ErrArchive is returned when tar failed on the server.
	ErrArchive = errors.New("the archive failed")
	// ErrComplete is returned when a copy to resume misses nothing.
	ErrComplete = errors.New("the copy is complete")
)

// Server runs the copy's commands on the site's account: short ones, their output read whole, and
// streams, their output written as it comes (sshx.ExecRunner.RunStream).
type Server interface {
	Run(ctx context.Context, remote string, input []byte) (sshx.Result, error)
	Stream(ctx context.Context, remote string, input io.Reader, out io.Writer) (sshx.Result, error)
}

// WP builds the command line of a WP-CLI command for the site copied.
type WP func(wpcli.Command) (string, error)

// Copier copies one site to the computer.
type Copier struct {
	Server  Server
	WP      WP
	Now     func() time.Time
	Version string // Vigie's, written in copy.json
	// Free answers the free space of the disk of a folder, in bytes.
	Free func(dir string) (uint64, error)
}

// Preview is what the server answered before a copy: what the copy dialog shows.
type Preview struct {
	Paths     wpcli.SitePaths
	WordPress string
	Database  int64
	Tables    []wpcli.Table
	Content   remote.ContentSize // the folders of wp-content
	Root      int64              // the whole site folder
	Tools     remote.Tools
}

// Size is how much a content reads, before compression, the folders of wp-content left out
// excluded.
func (p Preview) Size(content Content, leftOut []string) int64 {
	n := p.Database
	if content == DatabaseOnly {
		return n
	}
	files := p.Content.Total
	if content == WholeSite {
		files = p.Root
	}
	for _, f := range p.Content.Folders {
		if slices.Contains(leftOut, f.Name) {
			files -= f.Bytes
		}
	}
	return n + max(files, 0)
}

// Prepare asks the server what a copy needs, read-only (ADR-0041, point 1).
func (c Copier) Prepare(ctx context.Context) (Preview, error) {
	var p Preview
	paths, err := c.wp(ctx, wpcli.CopyPaths(), wpcli.CopyPathsScript())
	if err != nil {
		return p, err
	}
	if p.Paths, err = wpcli.ParseCopyPaths(paths); err != nil {
		return p, err
	}
	version, err := c.wp(ctx, wpcli.CoreVersion(), nil)
	if err != nil {
		return p, err
	}
	p.WordPress = strings.TrimSpace(string(version))
	size, err := c.wp(ctx, wpcli.DatabaseSize(), nil)
	if err != nil {
		return p, err
	}
	if p.Database, err = wpcli.ParseDatabaseSize(size); err != nil {
		return p, err
	}
	tables, err := c.wp(ctx, wpcli.DatabaseTables(), nil)
	if err != nil {
		return p, err
	}
	if p.Tables, err = wpcli.ParseDatabaseTables(tables); err != nil {
		return p, err
	}
	tools, err := c.run(ctx, remote.CopyTools(), nil)
	if err != nil {
		return p, err
	}
	p.Tools = remote.ParseCopyTools(tools)
	for _, dir := range []struct {
		path string
		to   func(remote.ContentSize)
	}{
		{p.Paths.Content, func(s remote.ContentSize) { p.Content = s }},
		{p.Paths.Root, func(s remote.ContentSize) { p.Root = s.Total }},
	} {
		cmd, err := remote.ContentSizes(dir.path)
		if err != nil {
			return p, err
		}
		out, err := c.run(ctx, cmd, nil)
		if err != nil {
			return p, err
		}
		s, err := remote.ParseContentSizes(out)
		if err != nil {
			return p, err
		}
		dir.to(s)
	}
	return p, nil
}

// wp runs a WP-CLI command of the site and answers its output; a failure is an error.
func (c Copier) wp(ctx context.Context, cmd wpcli.Command, input []byte) ([]byte, error) {
	line, err := c.WP(cmd)
	if err != nil {
		return nil, err
	}
	return c.run(ctx, line, input)
}

// run runs a command and answers its output; a non-zero exit status is an error.
func (c Copier) run(ctx context.Context, line string, input []byte) ([]byte, error) {
	res, err := c.Server.Run(ctx, line, input)
	if err != nil {
		return nil, err
	}
	if err := wpcli.Done(res); err != nil {
		return nil, err
	}
	return res.Stdout, nil
}

// Job is a copy the user asked for.
type Job struct {
	Root    string // the folder of the copies (Settings, Data)
	Host    string // the host's name and the site's, for the folders and copy.json
	Site    string
	Address string
	Content Content
	LeftOut []string
	Limit   int64
}

// planName is the plan of a copy, kept in its folder so that a resume reads the same files: the
// files listed then, not those of the server at the resume.
const planName = ".plan.json"

// Progress tells how far a copy went.
type Progress struct {
	Archive  string // the archive being read
	Done     int    // archives complete
	Total    int
	Read     int64 // bytes read of the archive being read
	Expected int64 // its size before compression, 0 for a dump
}

// Start plans the copy, checks the free space, makes its folder, then reads every archive. It
// answers the copy's folder, also when it failed halfway: what arrived is kept (point 6).
func (c Copier) Start(ctx context.Context, job Job, p Preview, progress func(Progress)) (string, error) {
	archives, err := c.plan(ctx, job, p)
	if err != nil {
		return "", err
	}
	var need int64
	for _, a := range archives {
		need += a.Bytes
	}
	site := SiteFolder(job.Root, job.Host, job.Site)
	if err := os.MkdirAll(site, 0o700); err != nil {
		return "", err
	}
	if free, err := c.Free(site); err == nil && uint64(need) > free {
		return "", fmt.Errorf("%w: %d bytes needed, %d free", ErrNoSpace, need, free)
	}
	started := c.Now()
	dir := filepath.Join(site, started.Format("2006-01-02 15.04"))
	if _, err := os.Stat(dir); err == nil {
		dir = filepath.Join(site, started.Format("2006-01-02 15.04.05"))
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(dir, planName), archives); err != nil {
		return dir, err
	}
	m := Manifest{Vigie: c.Version, Site: job.Address, Host: job.Host, WordPress: p.WordPress, Prefix: p.Paths.Prefix,
		Content: job.Content, LeftOut: job.LeftOut, Started: started, Archives: []Record{}}
	if err := WriteManifest(dir, m); err != nil {
		return dir, err
	}
	return dir, c.fetch(ctx, dir, archives, m, progress)
}

// Resume reads the archives of a cut copy that are missing, from its plan.
func (c Copier) Resume(ctx context.Context, dir string, progress func(Progress)) error {
	m, err := ReadManifest(dir)
	if err != nil {
		return err
	}
	var archives []Archive
	if err := readJSON(filepath.Join(dir, planName), &archives); err != nil {
		return err
	}
	if len(Missing(archives, m)) == 0 {
		return ErrComplete
	}
	return c.fetch(ctx, dir, archives, m, progress)
}

// Missing are the archives of a plan that copy.json does not record.
func Missing(archives []Archive, m Manifest) []Archive {
	have := map[string]bool{}
	for _, r := range m.Archives {
		have[r.Name] = true
	}
	var missing []Archive
	for _, a := range archives {
		if !have[a.Name] {
			missing = append(missing, a)
		}
	}
	return missing
}

// fetch reads the archives copy.json does not record yet, one after the other, recording each.
func (c Copier) fetch(ctx context.Context, dir string, archives []Archive, m Manifest, progress func(Progress)) error {
	missing := Missing(archives, m)
	done := len(archives) - len(missing)
	for _, a := range missing {
		report := func(n int64) {
			if progress != nil {
				progress(Progress{Archive: a.Name, Done: done, Total: len(archives), Read: n, Expected: expected(a)})
			}
		}
		report(0)
		rec, err := c.archive(ctx, dir, a, report)
		if err != nil {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
		m.Archives = append(m.Archives, rec)
		if err := WriteManifest(dir, m); err != nil {
			return err
		}
		done++
	}
	if progress != nil {
		progress(Progress{Done: done, Total: len(archives)})
	}
	return nil
}

// expected is the size an archive's progress is measured against: a dump comes compressed, so has
// none.
func expected(a Archive) int64 {
	if a.Part == Database {
		return 0
	}
	return a.Bytes
}

// archive reads one archive into <name>.partial, then checks and renames it.
func (c Copier) archive(ctx context.Context, dir string, a Archive, report func(int64)) (Record, error) {
	partial := filepath.Join(dir, a.Name+".partial")
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return Record{}, err
	}
	digest := sha256.New()
	file := &counting{w: io.MultiWriter(f, digest)}
	read := c.Now()
	if a.Part == Database {
		err = c.dump(ctx, a, &counting{w: file, report: report})
	} else {
		err = c.files(ctx, a, file, report)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && a.Part == Database {
		err = dumpComplete(partial)
	}
	if err != nil {
		os.Remove(partial)
		return Record{}, err
	}
	if err := os.Rename(partial, filepath.Join(dir, a.Name)); err != nil {
		return Record{}, err
	}
	return Record{Name: a.Name, Part: a.Part, Bytes: file.n, SHA256: hex.EncodeToString(digest.Sum(nil)), Read: read}, nil
}

// dump streams a database dump, already compressed on the server, and checks its exit status.
func (c Copier) dump(ctx context.Context, a Archive, out io.Writer) error {
	line, err := c.WP(wpcli.DatabaseDump(a.Tables))
	if err != nil {
		return err
	}
	res, err := c.Server.Stream(ctx, remote.DumpCompressed(line), nil, out)
	if err != nil {
		return err
	}
	if code, ok := remote.ParseDumpExit(res.Stderr); !ok || code != 0 {
		return fmt.Errorf("%w: %s", ErrDump, firstLines(res.Stderr))
	}
	return nil
}

// files streams a tar archive of a part, compressed on the computer. tar's status 1 is "some files
// differ": a file changed while it was read, a warning, not a failure.
func (c Copier) files(ctx context.Context, a Archive, out io.Writer, report func(int64)) error {
	var line string
	var input io.Reader
	var err error
	if a.Files == nil {
		line, err = remote.ArchiveFolder(a.Dir, a.Skip)
	} else {
		line, err = remote.ArchiveFiles(a.Dir)
		var names bytes.Buffer
		for _, f := range a.Files {
			names.WriteString(f.Name)
			names.WriteByte(0)
		}
		input = &names
	}
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(out)
	res, err := c.Server.Stream(ctx, line, input, &counting{w: zw, report: report})
	if err != nil {
		return err
	}
	if res.ExitCode > 1 {
		return fmt.Errorf("%w (exit %d): %s", ErrArchive, res.ExitCode, firstLines(res.Stderr))
	}
	return zw.Close()
}

// dumpComplete checks that a dump ends as mysqldump ends a complete one: "-- Dump completed".
func dumpComplete(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDump, err)
	}
	tail := make([]byte, 0, 512)
	buf := make([]byte, 64<<10)
	for {
		n, err := zr.Read(buf)
		tail = append(tail, buf[:n]...)
		if len(tail) > 256 {
			tail = append(tail[:0], tail[len(tail)-256:]...)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrDump, err)
		}
	}
	if !bytes.Contains(tail, []byte("-- Dump completed")) {
		return fmt.Errorf("%w: the dump does not end as a complete one", ErrDump)
	}
	return nil
}

// counting passes what it is given to w, counts it, and reports the count.
type counting struct {
	w      io.Writer
	n      int64
	report func(int64)
}

func (c *counting) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	if c.report != nil {
		c.report(c.n)
	}
	return n, err
}

// SiteFolder is the folder of a site's copies: <root>/<host>/<site>.
func SiteFolder(root, host, site string) string {
	return filepath.Join(root, folderName(host), folderName(site))
}

// Listed is a copy found in a site's folder: its manifest, how many archives its plan has, and
// whether it holds them all.
type Listed struct {
	Dir      string
	Manifest Manifest
	Planned  int
	Complete bool
}

// List reads the copies of a site's folder, the newest first; a folder that is not a copy (no
// copy.json) is left out, and a missing site folder is no copy.
func List(siteFolder string) ([]Listed, error) {
	entries, err := os.ReadDir(siteFolder)
	if errors.Is(err, os.ErrNotExist) {
		return []Listed{}, nil
	}
	if err != nil {
		return nil, err
	}
	copies := []Listed{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(siteFolder, e.Name())
		m, err := ReadManifest(dir)
		if err != nil {
			continue
		}
		var plan []Archive
		if err := readJSON(filepath.Join(dir, planName), &plan); err != nil {
			continue
		}
		copies = append(copies, Listed{Dir: dir, Manifest: m, Planned: len(plan), Complete: len(Missing(plan, m)) == 0})
	}
	slices.SortFunc(copies, func(a, b Listed) int { return b.Manifest.Started.Compare(a.Manifest.Started) })
	return copies, nil
}

// folderName is a host's or site's name as a folder name: no separator, no leading dot.
func folderName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 || r == ':' {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.TrimLeft(name, ".")
	if name == "" {
		return "site"
	}
	return name
}

func firstLines(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

func writeJSON(file string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0o600)
}

func readJSON(file string, v any) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// plan lists the files of each part, when the account has GNU find and tar, and answers the
// archives of the copy.
func (c Copier) plan(ctx context.Context, job Job, p Preview) ([]Archive, error) {
	folders := Folders(p.Paths, job.Content, job.LeftOut)
	split := p.Tools.GNUFind && p.Tools.GNUTar
	if !split {
		// Not listed: a part of wp-content whose folder is not there (no mu-plugins) is left out,
		// as wp-content's sizes tell.
		folders = slices.DeleteFunc(folders, func(f PartFolder) bool {
			if path.Dir(f.Dir) != p.Paths.Content {
				return false
			}
			return !slices.ContainsFunc(p.Content.Folders, func(s remote.FolderSize) bool { return s.Name == path.Base(f.Dir) })
		})
	}
	listed := map[Part][]remote.File{}
	if split {
		for _, f := range folders {
			cmd, err := remote.ListFiles(f.Dir, f.Skip)
			if err != nil {
				return nil, err
			}
			res, err := c.Server.Run(ctx, cmd, nil)
			if err != nil {
				return nil, err
			}
			// A part whose folder does not exist (no mu-plugins) has no files: find says so on its
			// error output, and the part has no archive.
			if res.ExitCode != 0 && len(res.Stdout) == 0 {
				continue
			}
			files, err := remote.ParseFileList(res.Stdout)
			if err != nil {
				return nil, err
			}
			listed[f.Part] = files
		}
	}
	return Plan(Inputs{Folders: folders, Listed: listed, Tables: p.Tables, Limit: job.Limit, Split: split}), nil
}
