package workspace_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// copyBench is a site whose account answers what a copy of its database asks.
func copyBench(t *testing.T, free uint64) (*workspace.Workspace, *store.DB, model.Site) {
	t.Helper()
	ctx := context.Background()
	db := emptyDB(t)
	host := model.Host{ID: model.NewID(), Name: "Shared hosting", Alias: "shared"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "boulangerie-exemple.ch", URL: "https://boulangerie-exemple.ch", Path: "~/www/boulangerie"}
	for _, err := range []error{db.SaveHost(ctx, host), db.SaveSite(ctx, site)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := sshxtest.NewFake()
	wp := func(c wpcli.Command) string {
		line, err := c.Remote("wp", site.Path)
		if err != nil {
			t.Fatal(err)
		}
		return line
	}
	ok := func(out string) sshx.Result { return sshx.Result{Stdout: []byte(out)} }
	fake.Set(wp(wpcli.CopyPaths()), ok(`{"root":"/home/u/www/boulangerie","content":"/home/u/www/boulangerie/wp-content","plugins":"/home/u/www/boulangerie/wp-content/plugins","muPlugins":"/home/u/www/boulangerie/wp-content/mu-plugins","themes":"/home/u/www/boulangerie/wp-content/themes","uploads":"/home/u/www/boulangerie/wp-content/uploads","prefix":"wp_"}`))
	fake.Set(wp(wpcli.CoreVersion()), ok("7.1.3\n"))
	fake.Set(wp(wpcli.DatabaseSize()), ok("12400000\n"))
	fake.Set(wp(wpcli.DatabaseTables()), ok(`[{"Name":"wp_posts","Size":"12400000 B"}]`))
	fake.Set(remote.CopyTools(), ok("find (GNU findutils) 4.10.0\n---\ntar (GNU tar) 1.35\n"))
	for _, dir := range []string{"/home/u/www/boulangerie/wp-content", "/home/u/www/boulangerie"} {
		line, _ := remote.ContentSizes(dir)
		fake.Set(line, ok("4\tplugins/\n---\n8\t.\n"))
	}
	var dump bytes.Buffer
	zw := gzip.NewWriter(&dump)
	zw.Write([]byte("-- MySQL dump\n-- Dump completed on 2026-11-03 20:41:02\n"))
	zw.Close()
	fake.Set(remote.DumpCompressed(wp(wpcli.DatabaseDump(nil))), sshx.Result{Stdout: dump.Bytes(), Stderr: []byte("vigie-dump-exit 0\n")})
	at := time.Date(2026, 11, 3, 20, 41, 0, 0, time.UTC)
	return workspace.New(db, workspace.Deps{Runner: fake, Now: func() time.Time { return at },
		DiskFree: func(string) (uint64, error) { return free, nil }}), db, site
}

// A copy of a site's database: its archive on the computer, and the copy in the journal.
func TestACopyOfTheDatabase(t *testing.T) {
	ctx := context.Background()
	ws, db, site := copyBench(t, 1<<40)
	p, err := ws.PrepareCopy(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Database != 12400000 || p.WordPress != "7.1.3" {
		t.Fatalf("PrepareCopy() = %+v", p)
	}
	root := t.TempDir()
	kept, err := ws.CopySite(ctx, site.ID, workspace.CopyRequest{Root: root, Content: sitecopy.DatabaseOnly, Limit: 500 << 20}, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := kept.Folder
	if want := filepath.Join(root, "Shared hosting", "boulangerie-exemple.ch", "2026-11-03 20.41"); dir != want {
		t.Errorf("folder %s, want %s", dir, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "database.sql.gz")); err != nil {
		t.Fatal(err)
	}
	m, _ := sitecopy.ReadManifest(dir)
	if m.Site != "https://boulangerie-exemple.ch" {
		t.Errorf("copy.json names %q", m.Site)
	}
	copies, err := db.Copies(ctx, time.Time{})
	if err != nil || len(copies) != 1 {
		t.Fatalf("journal: %+v, %v", copies, err)
	}
	c := copies[0]
	if c.ID != kept.ID || c.Outcome != model.CopyCompleted || c.Archives != 1 || c.Bytes != m.Archives[0].Bytes || c.Folder != dir || c.SiteName != site.Name || c.Content != "database" {
		t.Errorf("journal entry %+v", c)
	}
}

// Refused for the disk: nothing read, nothing in the journal.
func TestACopyRefusedForTheDisk(t *testing.T) {
	ctx := context.Background()
	ws, db, site := copyBench(t, 1000)
	p, _ := ws.PrepareCopy(ctx, site.ID)
	if _, err := ws.CopySite(ctx, site.ID, workspace.CopyRequest{Root: t.TempDir(), Content: sitecopy.DatabaseOnly, Limit: 500 << 20}, p, nil); !errors.Is(err, sitecopy.ErrNoSpace) {
		t.Fatalf("CopySite() = %v, want ErrNoSpace", err)
	}
	if copies, _ := db.Copies(ctx, time.Time{}); len(copies) != 0 {
		t.Errorf("journal: %+v", copies)
	}
}

// The folder of the copies and the size of the archives: their defaults, refused values, stored.
func TestTheSettingsOfTheCopies(t *testing.T) {
	ctx := context.Background()
	db := emptyDB(t)
	ws := workspace.New(db, workspace.Deps{Home: "/Users/u", Now: time.Now})
	if dir, err := ws.CopyFolder(ctx); err != nil || dir != "/Users/u/Documents/Vigie" {
		t.Fatalf("CopyFolder() = %s, %v", dir, err)
	}
	if mb, err := ws.ArchiveSize(ctx); err != nil || mb != 500 {
		t.Fatalf("ArchiveSize() = %d, %v", mb, err)
	}
	for _, bad := range []string{"", "copies", "/Users/u/../x"} {
		if err := ws.SetCopyFolder(ctx, bad); !errors.Is(err, workspace.ErrCopyFolder) {
			t.Errorf("SetCopyFolder(%q) = %v", bad, err)
		}
	}
	if err := ws.SetArchiveSize(ctx, 5000); !errors.Is(err, workspace.ErrArchiveSize) {
		t.Errorf("SetArchiveSize(5000) = %v", err)
	}
	if err := ws.SetCopyFolder(ctx, "/Volumes/Disk/Copies"); err != nil {
		t.Fatal(err)
	}
	if err := ws.SetArchiveSize(ctx, 2000); err != nil {
		t.Fatal(err)
	}
	if dir, _ := ws.CopyFolder(ctx); dir != "/Volumes/Disk/Copies" {
		t.Errorf("CopyFolder() after a choice = %s", dir)
	}
	if mb, _ := ws.ArchiveSize(ctx); mb != 2000 {
		t.Errorf("ArchiveSize() after a choice = %d", mb)
	}
}
