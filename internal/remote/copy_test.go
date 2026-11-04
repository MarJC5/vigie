package remote_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MarJC5/vigie/internal/remote"
)

// The commands of a copy only read (ADR-0041): none of them writes on the server.
func TestCopyCommandLines(t *testing.T) {
	tools := remote.CopyTools()
	if want := "(find --version 2>/dev/null | head -n 1); echo ---; (tar --version 2>/dev/null | head -n 1); true"; tools != want {
		t.Errorf("CopyTools() = %s", tools)
	}
	tests := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"sizes", func() (string, error) { return remote.ContentSizes("/home/u/www/site/wp-content") },
			"cd '/home/u/www/site/wp-content' && du -sk -- */ 2>/dev/null; echo ---; du -sk . 2>/dev/null; true"},
		{"files", func() (string, error) { return remote.ListFiles("/home/u/www/site/wp-content/uploads", nil) },
			"find '/home/u/www/site/wp-content/uploads' -type f -printf '%s %P\\0'"},
		{"files but", func() (string, error) {
			return remote.ListFiles("/home/u/www/site/wp-content", []string{"/home/u/www/site/wp-content/uploads", "/home/u/www/site/wp-content/cache"})
		}, "find '/home/u/www/site/wp-content' '(' -path '/home/u/www/site/wp-content/uploads' -o -path '/home/u/www/site/wp-content/cache' ')' -prune -o -type f -printf '%s %P\\0'"},
		{"archive", func() (string, error) { return remote.ArchiveFiles("/home/u/www/site/wp-content/uploads") },
			"cd '/home/u/www/site/wp-content/uploads' && tar -cf - --ignore-failed-read --no-recursion --null -T -"},
	}
	for _, tt := range tests {
		got, err := tt.got()
		if err != nil || got != tt.want {
			t.Errorf("%s: got %s, %v\nwant %s", tt.name, got, err, tt.want)
		}
	}
	if got, want := remote.DumpCompressed("'wp' 'db' 'export' '-'"), `{ 'wp' 'db' 'export' '-'; echo "vigie-dump-exit $?" >&2; } | gzip -c`; got != want {
		t.Errorf("DumpCompressed() = %s\nwant %s", got, want)
	}
}

func TestCopyCommandsRefuseBadPaths(t *testing.T) {
	for _, dir := range []string{"", "/", "relative/uploads", "/home/u/../etc", "/home/u/a\nb"} {
		if _, err := remote.ContentSizes(dir); !errors.Is(err, remote.ErrBadPath) {
			t.Errorf("ContentSizes(%q): %v", dir, err)
		}
		if _, err := remote.ListFiles(dir, nil); !errors.Is(err, remote.ErrBadPath) {
			t.Errorf("ListFiles(%q): %v", dir, err)
		}
		if _, err := remote.ArchiveFiles(dir); !errors.Is(err, remote.ErrBadPath) {
			t.Errorf("ArchiveFiles(%q): %v", dir, err)
		}
	}
	// A folder left out must be inside the one listed.
	if _, err := remote.ListFiles("/home/u/www/site/wp-content", []string{"/home/u/other"}); !errors.Is(err, remote.ErrBadPath) {
		t.Errorf("ListFiles() leaving out a folder outside: %v", err)
	}
}

func TestParseCopyTools(t *testing.T) {
	gnu := remote.ParseCopyTools([]byte("find (GNU findutils) 4.10.0\n---\ntar (GNU tar) 1.35\n"))
	if !gnu.GNUFind || !gnu.GNUTar {
		t.Errorf("GNU find and tar: %+v", gnu)
	}
	// The WP-CLI image on Alpine: BusyBox find, which answers nothing to --version, and GNU tar.
	alpine := remote.ParseCopyTools([]byte("---\ntar (GNU tar) 1.35\n"))
	if alpine.GNUFind || !alpine.GNUTar {
		t.Errorf("BusyBox find, GNU tar: %+v", alpine)
	}
	if none := remote.ParseCopyTools([]byte("---\n")); none.GNUFind || none.GNUTar {
		t.Errorf("neither: %+v", none)
	}
}

func TestParseContentSizes(t *testing.T) {
	out := "1884160\tuploads/\n319488\tplugins/\n42188\tthemes/\n98612\tcache/\n---\n2363000\t.\n"
	got, err := remote.ParseContentSizes([]byte(out))
	want := remote.ContentSize{
		Folders: []remote.FolderSize{{Name: "uploads", Bytes: 1884160 * 1024}, {Name: "plugins", Bytes: 319488 * 1024}, {Name: "cache", Bytes: 98612 * 1024}, {Name: "themes", Bytes: 42188 * 1024}},
		Total:   2363000 * 1024,
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseContentSizes() = %+v, %v\nwant %+v", got, err, want)
	}
	if _, err := remote.ParseContentSizes([]byte("uploads\n")); err == nil {
		t.Error("an answer without its total: want an error")
	}
}

func TestParseFileList(t *testing.T) {
	got, err := remote.ParseFileList([]byte("3 2026/a b.jpg\x005 2026/q'uote.png\x000 empty\x00"))
	want := []remote.File{{Name: "2026/a b.jpg", Bytes: 3}, {Name: "2026/q'uote.png", Bytes: 5}, {Name: "empty", Bytes: 0}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseFileList() = %+v, %v", got, err)
	}
	for _, bad := range []string{"x a\x00", "3\x00", "-1 a\x00", "3 /etc/passwd\x00"} {
		if _, err := remote.ParseFileList([]byte(bad)); err == nil {
			t.Errorf("ParseFileList(%q): want an error", bad)
		}
	}
}

func TestParseDumpExit(t *testing.T) {
	if code, ok := remote.ParseDumpExit([]byte("Warning: something\nvigie-dump-exit 3\n")); !ok || code != 3 {
		t.Errorf("ParseDumpExit() = %d, %v", code, ok)
	}
	if _, ok := remote.ParseDumpExit([]byte("nothing\n")); ok {
		t.Error("no exit status written: want none")
	}
}
