package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MarJC5/vigie/internal/app"
)

// The vigie command is linked when a folder of the PATH holds a vigie that leads to this app
// (ADR-0036); otherwise the settings show the one command that links it.
func TestTheCommandLinkedOrTheCommandThatLinksIt(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "Applications", "Vigie.app", "Contents", "MacOS", "Vigie")
	local := filepath.Join(root, "usr-local-bin")
	brew := filepath.Join(root, "opt-homebrew-bin")
	for _, dir := range []string{filepath.Dir(exe), local, brew} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Nothing linked yet: the command that links it into the first folder.
	got := app.CommandState(app.CommandEnv{Executable: exe, Dirs: []string{local, brew}})
	want := "sudo ln -sf '" + exe + "' '" + filepath.Join(local, "vigie") + "'"
	if got.Linked || got.Command != want || got.Path != filepath.Join(local, "vigie") {
		t.Fatalf("not linked: %+v, want the command %q", got, want)
	}

	// Linked by Homebrew: found, nothing to run.
	if err := os.Symlink(exe, filepath.Join(brew, "vigie")); err != nil {
		t.Fatal(err)
	}
	got = app.CommandState(app.CommandEnv{Executable: exe, Dirs: []string{local, brew}})
	if !got.Linked || got.Path != filepath.Join(brew, "vigie") || got.Command != "" {
		t.Fatalf("linked by Homebrew: %+v", got)
	}

	// A vigie that leads elsewhere, another install, is not this app's.
	other := filepath.Join(root, "other")
	if err := os.WriteFile(other, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(brew, "vigie")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(local, "vigie")); err != nil {
		t.Fatal(err)
	}
	if got = app.CommandState(app.CommandEnv{Executable: exe, Dirs: []string{local, brew}}); got.Linked {
		t.Fatalf("another vigie: %+v, want not linked", got)
	}
}

// An AppImage links into the user's ~/.local/bin, no password; a Linux package installs
// /usr/bin/vigie itself.
func TestTheCommandOnLinux(t *testing.T) {
	root := t.TempDir()
	image := filepath.Join(root, "Applications", "Vigie-x86_64.AppImage")
	user := filepath.Join(root, "home", ".local", "bin")
	if err := os.MkdirAll(filepath.Dir(image), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	got := app.CommandState(app.CommandEnv{Executable: "/tmp/.mount_Vigie/usr/bin/Vigie", AppImage: image, Dirs: []string{user}})
	want := "mkdir -p '" + user + "' && ln -sf '" + image + "' '" + filepath.Join(user, "vigie") + "'"
	if got.Linked || got.Command != want {
		t.Fatalf("AppImage: %+v, want %q", got, want)
	}

	// A package: the binary is /usr/bin/vigie itself.
	bin := filepath.Join(root, "usr-bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(bin, "vigie")
	if err := os.WriteFile(pkg, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := app.CommandState(app.CommandEnv{Executable: pkg, Dirs: []string{bin, user}}); !got.Linked || got.Path != pkg {
		t.Fatalf("package: %+v, want linked at %s", got, pkg)
	}
}
