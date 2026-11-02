package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// bundleOf returns the app bundle a binary is in ("/Applications/Vigie.app" for
// "/Applications/Vigie.app/Contents/MacOS/Vigie"), or "" when it is not in one.
func bundleOf(exe string) string {
	macOS := filepath.Dir(exe)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)
	if filepath.Base(macOS) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(bundle, ".app") {
		return ""
	}
	return bundle
}

// handsOver tells whether vigie alone gives the window to LaunchServices (ADR-0036): on macOS,
// typed in a terminal, from an app bundle, so that the window is not tied to the terminal.
func handsOver(goos string, terminal bool, bundle string) bool {
	return goos == "darwin" && terminal && bundle != ""
}

// inTerminal tells whether the input is a terminal: vigie typed in one, not started by the system.
func inTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// openBundle asks LaunchServices to open the app with the arguments given: it opens the window, or
// brings forward the one already open.
func openBundle(bundle string, args []string) error {
	cmd := append([]string{bundle}, args...)
	if len(args) > 0 {
		cmd = append([]string{bundle, "--args"}, args...)
	}
	return exec.Command("/usr/bin/open", cmd...).Run()
}
