//go:build integration

package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// copyServer runs a copy's commands on the bench's account, as the app does.
type copyServer struct {
	runner sshx.ExecRunner
	target sshx.Target
	// cut makes the stream of the n-th archive fail, once (1 the first); 0 never.
	cut, streams int
}

func (s *copyServer) Run(ctx context.Context, line string, input []byte) (sshx.Result, error) {
	return s.runner.RunInput(ctx, s.target, line, input)
}

func (s *copyServer) Stream(ctx context.Context, line string, input io.Reader, out io.Writer) (sshx.Result, error) {
	s.streams++
	if s.streams == s.cut {
		out.Write([]byte("part of it"))
		return sshx.Result{}, errors.New("connection lost")
	}
	return s.runner.RunStream(ctx, s.target, line, input, out)
}

const copied = "~/sites/plain"

func copyBench(t *testing.T) (*copyServer, sitecopy.Copier) {
	t.Helper()
	a := sshAccess(t)
	s := &copyServer{runner: a.runner(t, a.trusted(t)), target: a.target(a.key)}
	return s, sitecopy.Copier{
		Server: s, Now: time.Now, Version: "test", Free: sitecopy.DiskFree,
		WP: func(c wpcli.Command) (string, error) { return c.Remote("wp", copied) },
	}
}

// remoteOut runs a command in the account and answers its output, failing the test otherwise.
func remoteOut(t *testing.T, s *copyServer, line string, input []byte) string {
	t.Helper()
	res, err := s.Run(context.Background(), line, input)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("%s: exit %d, %v: %s", line, res.ExitCode, err, res.Stderr)
	}
	return string(res.Stdout)
}

// siteState is the site folder as the server has it: every file, its size and its time.
func siteState(t *testing.T, s *copyServer, root string) string {
	t.Helper()
	q, _ := sshx.Quote(root)
	return remoteOut(t, s, "find "+q+" -printf '%y %s %T@ %P\\n' | sort", nil)
}

// tarFiles lists the files of the archives of a part, with their sizes.
func tarFiles(t *testing.T, dir string, part sitecopy.Part, m sitecopy.Manifest) map[string]int64 {
	t.Helper()
	files := map[string]int64{}
	for _, r := range m.Archives {
		if r.Part != part {
			continue
		}
		f, err := os.Open(filepath.Join(dir, r.Name))
		if err != nil {
			t.Fatal(err)
		}
		zr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		tr := tar.NewReader(zr)
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if h.Typeflag == tar.TypeReg {
				files[strings.TrimPrefix(h.Name, "./")] = h.Size
			}
		}
		f.Close()
	}
	return files
}

// serverFiles lists the files of a folder on the server, the folders skip left out.
func serverFiles(t *testing.T, s *copyServer, dir string, skip []string) map[string]int64 {
	t.Helper()
	line, err := remote.ListFiles(dir, skip)
	if err != nil {
		t.Fatal(err)
	}
	list, err := remote.ParseFileList([]byte(remoteOut(t, s, line, nil)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]int64{}
	for _, f := range list {
		files[f.Name] = f.Bytes
	}
	return files
}

// A copy of the database and wp-content, split by a small limit: the archives hold exactly the
// site's files, the dumps put back into an empty database give the site's rows, and the server's
// folder is as it was (ADR-0041).
func TestCopyOverSSH(t *testing.T) {
	ctx := context.Background()
	s, c := copyBench(t)
	p, err := c.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The read-only script on a real WordPress: the folders where WordPress has them.
	if p.Paths.Content != p.Paths.Root+"/wp-content" || p.Paths.Plugins != p.Paths.Content+"/plugins" || p.Paths.Prefix != "wp_" {
		t.Fatalf("paths %+v", p.Paths)
	}
	if !p.Tools.GNUFind || !p.Tools.GNUTar || p.Database <= 0 || len(p.Tables) == 0 {
		t.Fatalf("Prepare() = %+v", p)
	}
	before := siteState(t, s, p.Paths.Root)

	root := t.TempDir()
	const limit = 256 << 10
	dir, err := c.Start(ctx, sitecopy.Job{Root: root, Host: "Bench", Site: "plain", Address: "https://plain.example.test",
		Content: sitecopy.WithContent, Limit: limit}, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := sitecopy.ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(m.Archives))
	for i, r := range m.Archives {
		names[i] = r.Name
	}
	if !slices.Contains(names, "plugins-2.tar.gz") || !slices.Contains(names, "database-2.sql.gz") {
		t.Errorf("archives %v: want plugins and the database split by the limit", names)
	}

	// The files of each part, as the server has them.
	for _, part := range []struct {
		part sitecopy.Part
		dir  string
		skip []string
	}{
		{sitecopy.Plugins, p.Paths.Plugins, nil},
		{sitecopy.Themes, p.Paths.Themes, nil},
		{sitecopy.Others, p.Paths.Content, []string{p.Paths.Plugins, p.Paths.Themes, p.Paths.MuPlugins, p.Paths.Uploads}},
	} {
		got, want := tarFiles(t, dir, part.part, m), serverFiles(t, s, part.dir, part.skip)
		if len(want) == 0 || !mapsEqual(got, want) {
			t.Errorf("%s: %d files archived, %d on the server", part.part, len(got), len(want))
		}
	}

	// The dumps, put back into an empty database of the account, give the site's rows.
	config := func(name string) string {
		return strings.TrimSpace(remoteOut(t, s, "wp config get "+name+" --path="+strings.TrimPrefix(copied, "~/"), nil))
	}
	host, user, pass := config("DB_HOST"), config("DB_USER"), config("DB_PASSWORD")
	mysql := func(extra string) string {
		q, _ := sshx.Quote("-h", host, "-u", user, "-p"+pass)
		return "mysql " + q + " " + extra
	}
	remoteOut(t, s, mysql(`-e 'DROP DATABASE IF EXISTS vigie_copy_check; CREATE DATABASE vigie_copy_check'`), nil)
	t.Cleanup(func() { s.Run(context.Background(), mysql(`-e 'DROP DATABASE IF EXISTS vigie_copy_check'`), nil) })
	for _, r := range m.Archives {
		if r.Part != sitecopy.Database {
			continue
		}
		dump, err := os.ReadFile(filepath.Join(dir, r.Name))
		if err != nil {
			t.Fatal(err)
		}
		remoteOut(t, s, "gzip -dc | "+mysql("vigie_copy_check"), dump)
	}
	count := func(db string) string {
		return strings.TrimSpace(remoteOut(t, s, mysql("-N -e 'SELECT COUNT(*) FROM wp_options; SELECT COUNT(*) FROM wp_posts; SELECT COUNT(*) FROM wp_users' "+db), nil))
	}
	siteDB := config("DB_NAME")
	if got, want := count("vigie_copy_check"), count(siteDB); got != want {
		t.Errorf("rows put back: %q, the site's: %q", got, want)
	}
	tables := strings.Fields(remoteOut(t, s, mysql("-N -e 'SHOW TABLES' vigie_copy_check"), nil))
	if len(tables) != len(p.Tables) {
		t.Errorf("%d tables put back, %d on the site", len(tables), len(p.Tables))
	}

	if after := siteState(t, s, p.Paths.Root); after != before {
		t.Error("the copy changed the site's folder on the server")
	}
}

// A copy cut at its third archive keeps the two before; the resume reads only those missing.
func TestCopyCutThenResumedOverSSH(t *testing.T) {
	ctx := context.Background()
	s, c := copyBench(t)
	p, err := c.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.cut = 3
	dir, err := c.Start(ctx, sitecopy.Job{Root: t.TempDir(), Host: "Bench", Site: "plain", Content: sitecopy.WithContent, Limit: 1 << 20}, p, nil)
	if err == nil {
		t.Fatal("the copy was not cut")
	}
	m, _ := sitecopy.ReadManifest(dir)
	if len(m.Archives) != 2 {
		t.Fatalf("kept %d archives, want 2", len(m.Archives))
	}
	streams := s.streams
	if err := c.Resume(ctx, dir, nil); err != nil {
		t.Fatal(err)
	}
	listed, _ := sitecopy.List(filepath.Dir(dir))
	if len(listed) != 1 || !listed[0].Complete {
		t.Fatalf("after the resume: %+v", listed)
	}
	if read := s.streams - streams; read != listed[0].Planned-2 {
		t.Errorf("the resume read %d archives, want %d", read, listed[0].Planned-2)
	}
}

func mapsEqual(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}
