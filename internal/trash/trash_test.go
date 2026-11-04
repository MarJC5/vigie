package trash_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/trash"
)

var at = time.Date(2026, 11, 4, 9, 30, 5, 0, time.Local)

func home(t *testing.T) (trash.Home, string) {
	t.Helper()
	root := t.TempDir()
	return trash.Home{DataHome: filepath.Join(root, "share"), Now: func() time.Time { return at }}, root
}

// A folder goes to the home trash: its files there, and a .trashinfo that says where it was and
// when (freedesktop.org Trash specification).
func TestAFolderGoesToTheHomeTrash(t *testing.T) {
	h, root := home(t)
	copy := filepath.Join(root, "Vigie", "Shared hosting", "site", "2026-11-03 20.41")
	if err := os.MkdirAll(copy, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(copy, "database.sql.gz"), []byte("x"), 0o600)
	if err := h.Put(copy); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copy); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the folder is still there: %v", err)
	}
	trashed := filepath.Join(h.DataHome, "Trash", "files", "2026-11-03 20.41")
	if _, err := os.Stat(filepath.Join(trashed, "database.sql.gz")); err != nil {
		t.Fatalf("not in the trash: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(h.DataHome, "Trash", "info", "2026-11-03 20.41.trashinfo"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(info)
	if !strings.HasPrefix(text, "[Trash Info]\nPath=/") || !strings.Contains(text, "/Shared%20hosting/site/2026-11-03%2020.41\n") || !strings.HasSuffix(text, "DeletionDate=2026-11-04T09:30:05\n") {
		t.Fatalf(".trashinfo =\n%s", text)
	}
}

// A name already in the trash: the next one gets a number, its .trashinfo too.
func TestNamesThatCollide(t *testing.T) {
	h, root := home(t)
	for i := range 2 {
		dir := filepath.Join(root, string(rune('a'+i)), "copy")
		os.MkdirAll(dir, 0o700)
		if err := h.Put(dir); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"copy", "copy.2"} {
		if _, err := os.Stat(filepath.Join(h.DataHome, "Trash", "files", name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(h.DataHome, "Trash", "info", name+".trashinfo")); err != nil {
			t.Errorf("%s.trashinfo: %v", name, err)
		}
	}
}

// No trash that can take it: ErrNoTrash, and the folder stays where it was.
func TestNoTrashToTakeIt(t *testing.T) {
	h, root := home(t)
	os.MkdirAll(h.DataHome, 0o500) // the trash cannot be made there
	t.Cleanup(func() { os.Chmod(h.DataHome, 0o700) })
	dir := filepath.Join(root, "copy")
	os.MkdirAll(dir, 0o700)
	if err := h.Put(dir); !errors.Is(err, trash.ErrNoTrash) {
		t.Fatalf("Put() = %v, want ErrNoTrash", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the folder moved: %v", err)
	}
}

func TestDataHome(t *testing.T) {
	if got := trash.DataHome("/home/u", ""); got != "/home/u/.local/share" {
		t.Errorf("without XDG_DATA_HOME: %s", got)
	}
	if got := trash.DataHome("/home/u", "/data"); got != "/data" {
		t.Errorf("with XDG_DATA_HOME: %s", got)
	}
	if got := trash.DataHome("/home/u", "relative"); got != "/home/u/.local/share" {
		t.Errorf("a relative XDG_DATA_HOME is ignored: %s", got)
	}
}
