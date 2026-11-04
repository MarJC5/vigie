package sitecopy_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// server is a fake account: it answers the copy's commands, and streams what each archive holds.
type server struct {
	mu       sync.Mutex
	gnu      bool
	commands []string
	// cutAt makes the stream of that archive's command fail, once.
	cutAt string
	// dumpExit is the dump's exit status; dumpTail ends the dump.
	dumpExit string
	dumpTail string
	tarExit  int
}

func gz(s string) []byte {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Write([]byte(s))
	zw.Close()
	return b.Bytes()
}

func (s *server) Run(_ context.Context, line string, input []byte) (sshx.Result, error) {
	s.mu.Lock()
	s.commands = append(s.commands, line)
	s.mu.Unlock()
	ok := func(out string) (sshx.Result, error) { return sshx.Result{Stdout: []byte(out)}, nil }
	switch {
	case strings.Contains(line, "'eval-file'"):
		if !bytes.Equal(input, wpcli.CopyPathsScript()) {
			return sshx.Result{ExitCode: 1}, nil
		}
		return ok(`{"root":"/s","content":"/s/wp-content","plugins":"/s/wp-content/plugins","muPlugins":"/s/wp-content/mu-plugins","themes":"/s/wp-content/themes","uploads":"/s/wp-content/uploads","prefix":"wp_"}`)
	case strings.Contains(line, "'core' 'version'"):
		return ok("7.1.3\n")
	case strings.Contains(line, "'--tables'"):
		return ok(`[{"Name":"wp_posts","Size":"300 B"},{"Name":"wp_options","Size":"100 B"}]`)
	case strings.Contains(line, "'db' 'size'"):
		return ok("400\n")
	case strings.HasPrefix(line, "(find --version"):
		if s.gnu {
			return ok("find (GNU findutils) 4.10.0\n---\ntar (GNU tar) 1.35\n")
		}
		return ok("---\n")
	case strings.HasPrefix(line, "cd '/s/wp-content' && du"):
		return ok("8\tplugins/\n12\tuploads/\n4\tthemes/\n4\tcache/\n---\n32\t.\n")
	case strings.HasPrefix(line, "cd '/s' && du"):
		return ok("32\twp-content/\n8\twp-includes/\n---\n44\t.\n")
	case strings.HasPrefix(line, "find '/s/wp-content/mu-plugins'"):
		return sshx.Result{ExitCode: 1, Stderr: []byte("find: '/s/wp-content/mu-plugins': No such file or directory")}, nil
	case strings.HasPrefix(line, "find '/s/wp-content/uploads'"):
		return ok("400 2026/a.jpg\x00400 2026/b.jpg\x00")
	case strings.HasPrefix(line, "find '/s/wp-content/plugins'"):
		return ok("100 a/a.php\x00")
	case strings.HasPrefix(line, "find '/s/wp-content/themes'"):
		return ok("50 t/style.css\x00")
	case strings.HasPrefix(line, "find '/s/wp-content'"):
		return ok("10 index.php\x00")
	}
	return sshx.Result{ExitCode: 127, Stderr: []byte("unknown: " + line)}, nil
}

func (s *server) Stream(_ context.Context, line string, input io.Reader, out io.Writer) (sshx.Result, error) {
	s.mu.Lock()
	s.commands = append(s.commands, line)
	cut := s.cutAt != "" && strings.Contains(line, s.cutAt)
	if cut {
		s.cutAt = ""
	}
	s.mu.Unlock()
	if strings.Contains(line, "'db' 'export'") {
		out.Write(gz("-- MySQL dump\nINSERT INTO wp_posts VALUES (1);\n" + s.dumpTail))
		return sshx.Result{Stderr: []byte("vigie-dump-exit " + s.dumpExit + "\n")}, nil
	}
	names := ""
	if input != nil {
		b, _ := io.ReadAll(input)
		names = string(b)
	}
	out.Write([]byte("tar of " + line + " names " + names))
	if cut {
		return sshx.Result{}, errors.New("connection lost")
	}
	return sshx.Result{ExitCode: s.tarExit}, nil
}

func copier(s *server, free uint64) sitecopy.Copier {
	return sitecopy.Copier{
		Server: s, Version: "0.2.0",
		WP:   func(c wpcli.Command) (string, error) { return c.Remote("wp", "/s") },
		Now:  func() time.Time { return time.Date(2026, 11, 3, 20, 41, 0, 0, time.UTC) },
		Free: func(string) (uint64, error) { return free, nil },
	}
}

func newServer() *server {
	return &server{gnu: true, dumpExit: "0", dumpTail: "-- Dump completed on 2026-11-03 20:41:02\n"}
}

func job(root string) sitecopy.Job {
	return sitecopy.Job{Root: root, Host: "Shared hosting", Site: "boulangerie-exemple.ch", Address: "https://boulangerie-exemple.ch",
		Content: sitecopy.WithContent, LeftOut: []string{"cache"}, Limit: 500}
}

// Prepare reads, read-only, what the dialog shows (point 1).
func TestPrepareReadsTheSizes(t *testing.T) {
	p, err := copier(newServer(), 1<<30).Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.WordPress != "7.1.3" || p.Database != 400 || len(p.Tables) != 2 || !p.Tools.GNUFind || p.Root != 44*1024 {
		t.Fatalf("Prepare() = %+v", p)
	}
	if got, want := p.Size(sitecopy.WithContent, []string{"cache"}), int64(400+28*1024); got != want {
		t.Errorf("Size(content, without cache) = %d, want %d", got, want)
	}
	if got := p.Size(sitecopy.DatabaseOnly, nil); got != 400 {
		t.Errorf("Size(database) = %d", got)
	}
}

// A copy writes each archive, checked, then copy.json records them all; nothing is left .partial.
func TestACopyWritesEveryArchive(t *testing.T) {
	s := newServer()
	c := copier(s, 1<<30)
	p, err := c.Prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var seen []sitecopy.Progress
	dir, err := c.Start(context.Background(), job(t.TempDir()), p, func(pr sitecopy.Progress) { seen = append(seen, pr) })
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("Shared hosting", "boulangerie-exemple.ch", "2026-11-03 20.41"); !strings.HasSuffix(dir, want) {
		t.Errorf("folder %s, want …/%s", dir, want)
	}
	m, err := sitecopy.ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range m.Archives {
		names = append(names, r.Name)
		data, err := os.ReadFile(filepath.Join(dir, r.Name))
		sum := sha256.Sum256(data)
		if err != nil || int64(len(data)) != r.Bytes || hex.EncodeToString(sum[:]) != r.SHA256 {
			t.Errorf("%s: %d bytes, digest %s; recorded %d, %s (%v)", r.Name, len(data), hex.EncodeToString(sum[:]), r.Bytes, r.SHA256, err)
		}
	}
	// mu-plugins is not there: no archive; uploads is split in two.
	want := []string{"database.sql.gz", "plugins.tar.gz", "themes.tar.gz", "uploads.tar.gz", "uploads-2.tar.gz", "others.tar.gz"}
	if !slices.Equal(names, want) {
		t.Fatalf("archives %v, want %v", names, want)
	}
	if m.WordPress != "7.1.3" || m.Prefix != "wp_" || m.Site != "https://boulangerie-exemple.ch" || !slices.Equal(m.LeftOut, []string{"cache"}) {
		t.Errorf("manifest %+v", m)
	}
	if partial, _ := filepath.Glob(filepath.Join(dir, "*.partial")); len(partial) != 0 {
		t.Errorf("left %v", partial)
	}
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("the copy's folder is %v, want readable by the user only", info.Mode().Perm())
	}
	if last := seen[len(seen)-1]; last.Done != 6 || last.Total != 6 {
		t.Errorf("last progress %+v", last)
	}
	// The files of a part are archived from their names, sent on the archive's input.
	uploads, _ := os.ReadFile(filepath.Join(dir, "uploads-2.tar.gz"))
	zr, _ := gzip.NewReader(bytes.NewReader(uploads))
	content, _ := io.ReadAll(zr)
	if !strings.Contains(string(content), "names 2026/b.jpg\x00") {
		t.Errorf("uploads-2 holds %q", content)
	}
}

// A copy cut halfway keeps its archives, and a resume reads only those missing (point 6).
func TestACutCopyResumes(t *testing.T) {
	s := newServer()
	s.cutAt = "cd '/s/wp-content/uploads'"
	c := copier(s, 1<<30)
	p, _ := c.Prepare(context.Background())
	dir, err := c.Start(context.Background(), job(t.TempDir()), p, nil)
	if err == nil || !strings.Contains(err.Error(), "uploads.tar.gz") {
		t.Fatalf("cut copy: %v, want the error of uploads.tar.gz", err)
	}
	m, _ := sitecopy.ReadManifest(dir)
	if len(m.Archives) != 3 {
		t.Fatalf("kept %d archives, want the 3 before the cut", len(m.Archives))
	}
	if partial, _ := filepath.Glob(filepath.Join(dir, "*.partial")); len(partial) != 0 {
		t.Errorf("left %v after the cut", partial)
	}
	s.commands = nil
	if err := c.Resume(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
	m, _ = sitecopy.ReadManifest(dir)
	if len(m.Archives) != 6 {
		t.Fatalf("after the resume, %d archives, want 6", len(m.Archives))
	}
	for _, line := range s.commands {
		if strings.Contains(line, "'db' 'export'") || strings.HasPrefix(line, "find ") {
			t.Errorf("the resume ran %s: it reads only what is missing, from the plan", line)
		}
	}
	if err := c.Resume(context.Background(), dir, nil); !errors.Is(err, sitecopy.ErrComplete) {
		t.Errorf("resume of a complete copy: %v, want ErrComplete", err)
	}
}

// Not enough free space: nothing is read (point 6).
func TestACopyRefusedWithoutTheSpace(t *testing.T) {
	s := newServer()
	c := copier(s, 1000)
	p, _ := c.Prepare(context.Background())
	root := t.TempDir()
	if _, err := c.Start(context.Background(), job(root), p, nil); !errors.Is(err, sitecopy.ErrNoSpace) {
		t.Fatalf("Start() on a full disk: %v, want ErrNoSpace", err)
	}
	for _, line := range s.commands {
		if strings.Contains(line, "'db' 'export'") {
			t.Error("a dump was read")
		}
	}
	if copies, _ := filepath.Glob(filepath.Join(root, "*", "*", "*")); len(copies) != 0 {
		t.Errorf("copies made: %v", copies)
	}
}

// A dump that failed, or that does not end as a complete one, is refused.
func TestAFailedDumpIsRefused(t *testing.T) {
	for name, s := range map[string]*server{
		"exit 2":     {gnu: true, dumpExit: "2", dumpTail: "-- Dump completed\n"},
		"incomplete": {gnu: true, dumpExit: "0", dumpTail: "INSERT INTO wp_posts VALUES (2"},
	} {
		c := copier(s, 1<<30)
		p, _ := c.Prepare(context.Background())
		dir, err := c.Start(context.Background(), job(t.TempDir()), p, nil)
		if !errors.Is(err, sitecopy.ErrDump) {
			t.Errorf("%s: %v, want ErrDump", name, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "database.sql.gz")); err == nil {
			t.Errorf("%s: the dump was kept", name)
		}
	}
}

// tar's status 1 (a file changed while read) is a warning; 2 is a failure.
func TestTarExitStatuses(t *testing.T) {
	for code, fails := range map[int]bool{1: false, 2: true} {
		s := newServer()
		s.tarExit = code
		c := copier(s, 1<<30)
		p, _ := c.Prepare(context.Background())
		_, err := c.Start(context.Background(), job(t.TempDir()), p, nil)
		if fails != errors.Is(err, sitecopy.ErrArchive) {
			t.Errorf("tar exit %d: %v", code, err)
		}
	}
}

// Without GNU find and tar: one archive of each part's whole folder, a folder not there left out.
func TestACopyWithoutGNUTools(t *testing.T) {
	s := newServer()
	s.gnu = false
	c := copier(s, 1<<30)
	p, _ := c.Prepare(context.Background())
	dir, err := c.Start(context.Background(), job(t.TempDir()), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := sitecopy.ReadManifest(dir)
	var names []string
	for _, r := range m.Archives {
		names = append(names, r.Name)
	}
	if want := []string{"database.sql.gz", "plugins.tar.gz", "themes.tar.gz", "uploads.tar.gz", "others.tar.gz"}; !slices.Equal(names, want) {
		t.Fatalf("archives %v, want %v", names, want)
	}
	for _, line := range s.commands {
		if strings.HasPrefix(line, "find ") {
			t.Errorf("listed without GNU find: %s", line)
		}
	}
	var others string
	for _, line := range s.commands {
		if strings.HasPrefix(line, "cd '/s/wp-content' && tar") {
			others = line
		}
	}
	if !strings.Contains(others, "'--exclude=./cache'") || !strings.Contains(others, "'--exclude=./uploads'") {
		t.Errorf("others archived with %s", others)
	}
}

func TestDiskFree(t *testing.T) {
	free, err := sitecopy.DiskFree(t.TempDir())
	if err != nil || free == 0 {
		t.Fatalf("DiskFree() = %d, %v", free, err)
	}
}

// The copies of a site, the newest first, a cut one said incomplete; other folders left out.
func TestListTheCopiesOfASite(t *testing.T) {
	root := t.TempDir()
	s := newServer()
	c := copier(s, 1<<30)
	p, _ := c.Prepare(context.Background())
	first, err := c.Start(context.Background(), job(root), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.cutAt = "cd '/s/wp-content/uploads'"
	c.Now = func() time.Time { return time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC) }
	second, _ := c.Start(context.Background(), job(root), p, nil)
	site := sitecopy.SiteFolder(root, "Shared hosting", "boulangerie-exemple.ch")
	os.Mkdir(filepath.Join(site, "not a copy"), 0o700)
	got, err := sitecopy.List(site)
	if err != nil || len(got) != 2 {
		t.Fatalf("List() = %+v, %v", got, err)
	}
	if got[0].Dir != second || got[0].Complete || got[0].Planned != 6 || got[1].Dir != first || !got[1].Complete {
		t.Errorf("List() = %+v", got)
	}
	if none, err := sitecopy.List(filepath.Join(root, "nothing")); err != nil || len(none) != 0 {
		t.Errorf("List() of no folder = %+v, %v", none, err)
	}
}
