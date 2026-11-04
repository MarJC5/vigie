package app_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/trash"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// copyService is the service over a site whose account answers what a copy of its database asks.
func copyService(t *testing.T) (*app.Copies, *workspace.Workspace, model.Site, string) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	host := model.Host{ID: model.NewID(), Name: "Shared hosting", Alias: "shared"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "boulangerie-exemple.ch", Path: "~/www/boulangerie"}
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
	fake.Set(wp(wpcli.CopyPaths()), ok(`{"root":"/h/www/b","content":"/h/www/b/wp-content","plugins":"/h/www/b/wp-content/plugins","muPlugins":"/h/www/b/wp-content/mu-plugins","themes":"/h/www/b/wp-content/themes","uploads":"/h/www/b/wp-content/uploads","prefix":"wp_"}`))
	fake.Set(wp(wpcli.CoreVersion()), ok("7.1.3\n"))
	fake.Set(wp(wpcli.DatabaseSize()), ok("12400\n"))
	fake.Set(wp(wpcli.DatabaseTables()), ok(`[{"Name":"wp_posts","Size":"12400 B"}]`))
	fake.Set(remote.CopyTools(), ok("---\n"))
	content, _ := remote.ContentSizes("/h/www/b/wp-content")
	fake.Set(content, ok("400\tuploads/\n100\tplugins/\n---\n510\t.\n"))
	root, _ := remote.ContentSizes("/h/www/b")
	fake.Set(root, ok("510\twp-content/\n---\n600\t.\n"))
	var dump bytes.Buffer
	zw := gzip.NewWriter(&dump)
	zw.Write([]byte("-- MySQL dump\n-- Dump completed on 2026-11-03 20:41:02\n"))
	zw.Close()
	fake.Set(remote.DumpCompressed(wp(wpcli.DatabaseDump(nil))), sshx.Result{Stdout: dump.Bytes(), Stderr: []byte("vigie-dump-exit 0\n")})
	home := t.TempDir()
	ws := workspace.New(db, workspace.Deps{Runner: fake, Home: home, Now: time.Now})
	return app.NewCopies(ws, nil), ws, site, home
}

// waitCopies waits for the copies started to end.
func waitCopies(t *testing.T, c *app.Copies, completed int) app.CopiesState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := c.State(); s.Completed >= completed && len(s.Running) == 0 {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the copy did not end")
	return app.CopiesState{}
}

// The dialog's sizes, a copy in the background, its end told, then listed and in the journal.
func TestACopyFromTheWindow(t *testing.T) {
	ctx := context.Background()
	c, ws, site, home := copyService(t)
	var mu sync.Mutex
	var ends []app.RunEnd
	c.OnEnded = func(e app.RunEnd) { mu.Lock(); ends = append(ends, e); mu.Unlock() }

	if p := c.Start(site.ID.String(), "database", nil); code(p) != "problem.copyNoPreview" {
		t.Fatalf("Start() before the sizes: %+v", p)
	}
	v, p := c.Prepare(ctx, site.ID.String())
	if p != nil {
		t.Fatalf("Prepare() = %+v", p)
	}
	wantDest := filepath.Join(home, "Documents", "Vigie", "Shared hosting", "boulangerie-exemple.ch")
	if v.Database != 12400 || v.Content != 12400+510*1024 || v.Site != 12400+600*1024 || v.Split || v.Destination != wantDest || v.ArchiveSize != 500 || v.Free <= 0 {
		t.Fatalf("Prepare() = %+v", v)
	}
	// The folders of wp-content, the largest first, then what lies in wp-content itself.
	if len(v.Folders) != 3 || v.Folders[0].Name != "uploads" || v.Folders[2].Name != "" || v.Folders[2].Bytes != 10*1024 {
		t.Errorf("folders %+v", v.Folders)
	}

	if p := c.Start(site.ID.String(), "database", nil); p != nil {
		t.Fatalf("Start() = %+v", p)
	}
	s := waitCopies(t, c, 1)
	if len(s.Ends) != 1 || s.Ends[0].Problem != nil || !strings.HasPrefix(s.Ends[0].Folder, wantDest) {
		t.Fatalf("ends %+v", s.Ends)
	}
	list, err := c.List(ctx, site.ID.String())
	if err != nil || len(list) != 1 || !list[0].Complete || list[0].Archives != 1 || list[0].Content != "database" {
		t.Fatalf("List() = %+v, %v", list, err)
	}
	mu.Lock()
	if len(ends) != 1 || ends[0].Kind != app.EndCopy || ends[0].Archives != 1 || ends[0].Failed != 0 || ends[0].Name != site.Name {
		t.Errorf("ends told %+v", ends)
	}
	mu.Unlock()

	j, err := app.NewActivity(ws.DB(), time.Now).Journal(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	var copy *app.ActivityEntry
	for i := range j.Entries {
		if j.Entries[i].Kind == "copy" {
			copy = &j.Entries[i]
		}
	}
	if copy == nil || copy.Copy == nil || copy.Copy.Archives != 1 || copy.Outcome != "completed" || copy.Path != "Shared hosting/boulangerie-exemple.ch" {
		t.Fatalf("journal %+v", copy)
	}
}

// A folder that is not one of the site's copies is not resumed.
func TestResumeOnlyACopyOfTheSite(t *testing.T) {
	c, _, site, home := copyService(t)
	for _, folder := range []string{"/etc", filepath.Join(home, "Documents", "Vigie", "Shared hosting"), filepath.Join(home, "Documents", "Vigie", "Shared hosting", "boulangerie-exemple.ch", "..", "x")} {
		if p := c.Resume(site.ID.String(), folder); code(p) != "problem.notACopy" {
			t.Errorf("Resume(%s) = %+v", folder, p)
		}
	}
}

// A copy removed goes to the trash; without a trash, the problem says so and it can be deleted for
// good; the folder is shown; none of it for a folder that is not one of the site's copies.
func TestRemoveAndShowACopy(t *testing.T) {
	ctx := context.Background()
	c, _, site, _ := copyService(t)
	var trashed, shown []string
	noTrash := false
	c.Trash = func(p string) error {
		if noTrash {
			return trash.ErrNoTrash
		}
		trashed = append(trashed, p)
		return os.RemoveAll(p)
	}
	c.Reveal = func(p string) error { shown = append(shown, p); return nil }
	copyOnce := func() string {
		t.Helper()
		if _, p := c.Prepare(ctx, site.ID.String()); p != nil {
			t.Fatal(p)
		}
		before := c.State().Completed
		if p := c.Start(site.ID.String(), "database", nil); p != nil {
			t.Fatal(p)
		}
		return waitCopies(t, c, before+1).Ends[0].Folder
	}
	first := copyOnce()
	if p := c.Show(site.ID.String(), first); p != nil || len(shown) != 1 || shown[0] != first {
		t.Fatalf("Show() = %+v, shown %v", p, shown)
	}
	if p := c.Remove(site.ID.String(), first); p != nil || len(trashed) != 1 {
		t.Fatalf("Remove() = %+v, trashed %v", p, trashed)
	}
	if list, _ := c.List(ctx, site.ID.String()); len(list) != 0 {
		t.Errorf("listed after the removal: %+v", list)
	}
	second := copyOnce()
	noTrash = true
	if p := c.Remove(site.ID.String(), second); code(p) != "problem.noTrash" {
		t.Fatalf("Remove() without a trash = %+v", p)
	}
	if p := c.DeleteForGood(site.ID.String(), second); p != nil {
		t.Fatalf("DeleteForGood() = %+v", p)
	}
	if _, err := os.Stat(second); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("still there: %v", err)
	}
	for _, f := range []string{"/etc", first + "x"} {
		if p := c.DeleteForGood(site.ID.String(), f); code(p) != "problem.notACopy" {
			t.Errorf("DeleteForGood(%s) = %+v", f, p)
		}
	}
}
