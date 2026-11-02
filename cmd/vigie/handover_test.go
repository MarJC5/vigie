package main

import (
	"os"
	"testing"
)

// The app bundle of a binary: the folder ending in .app three levels up, or none (ADR-0036).
func TestBundleOf(t *testing.T) {
	cases := map[string]string{
		"/Applications/Vigie.app/Contents/MacOS/Vigie":             "/Applications/Vigie.app",
		"/Users/marie/Applications/Vigie.app/Contents/MacOS/Vigie": "/Users/marie/Applications/Vigie.app",
		"/usr/bin/vigie":              "",
		"/home/marie/vigie/bin/Vigie": "",
	}
	for exe, want := range cases {
		if got := bundleOf(exe); got != want {
			t.Errorf("bundleOf(%q) = %q, want %q", exe, got, want)
		}
	}
}

// vigie alone hands the window over to LaunchServices only on macOS, typed in a terminal, from a
// bundle: the app LaunchServices starts has no terminal, and opens its window itself.
func TestHandsOver(t *testing.T) {
	const bundle = "/Applications/Vigie.app"
	cases := []struct {
		goos     string
		terminal bool
		bundle   string
		want     bool
	}{
		{"darwin", true, bundle, true},
		{"darwin", false, bundle, false}, // started by LaunchServices: opens its window
		{"darwin", true, "", false},      // a binary outside a bundle (a build of the sources)
		{"linux", true, "", false},
	}
	for _, c := range cases {
		if got := handsOver(c.goos, c.terminal, c.bundle); got != c.want {
			t.Errorf("handsOver(%q, %v, %q) = %v, want %v", c.goos, c.terminal, c.bundle, got, c.want)
		}
	}
}

// /dev/null, what LaunchServices gives an app, is a character device but no terminal: the app it
// starts opens its window instead of handing it over again.
func TestDevNullIsNoTerminal(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	stdin := os.Stdin
	os.Stdin = null
	defer func() { os.Stdin = stdin }()
	if inTerminal() {
		t.Fatal("inTerminal() with /dev/null = true, want false")
	}
}
