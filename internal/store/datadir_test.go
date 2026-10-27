package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MarJC5/vigie/internal/store"
)

func configDir(dir string) func() (string, error) {
	return func() (string, error) { return dir, nil }
}

func TestDataDir(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		override string
		want     string
	}{
		{"macOS", "darwin", "", "/Users/me/Library/Application Support/Vigie"},
		{"Linux", "linux", "", "/home/me/.config/vigie"},
		{"override wins", "darwin", "/data/vigie", "/data/vigie"},
	}
	for _, tt := range tests {
		base := "/Users/me/Library/Application Support"
		if tt.goos == "linux" {
			base = "/home/me/.config"
		}
		got, err := store.DataDir(tt.goos, configDir(base), tt.override)
		if err != nil || got != tt.want {
			t.Errorf("%s: DataDir() = %q, %v; want %q", tt.name, got, err, tt.want)
		}
	}
}

func TestDataDirWithoutConfigDir(t *testing.T) {
	failing := func() (string, error) { return "", errors.New("no home") }
	if _, err := store.DataDir("linux", failing, ""); err == nil {
		t.Fatal("DataDir() without a config directory: want an error")
	}
}

func TestEnsureDataDirIsPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Application Support", "Vigie")
	if err := store.EnsureDataDir(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("data directory permissions = %o, want 700", perm)
	}
}
