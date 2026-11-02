package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/MarJC5/vigie/internal/cli"
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
	return isTerminal(os.Stdin)
}

// isTerminal tells whether a file is a terminal. A character device is not enough: LaunchServices
// gives /dev/null, which is one too.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), termiosRequest)
	return err == nil
}

// terminal is where the command line runs (ADR-0037): its progress shows when stdout and stderr
// are terminals and TERM is not "dumb", cut to the width of stderr's terminal.
func terminal(getenv func(string) string) cli.Terminal {
	return cli.Terminal{
		Interactive: isTerminal(os.Stdout) && isTerminal(os.Stderr) && getenv("TERM") != "dumb",
		Width:       func() int { return int(winsize().Col) },
		Height:      func() int { return int(winsize().Row) },
	}
}

// winsize is the size of stderr's terminal; zero when unknown.
func winsize() unix.Winsize {
	size, err := unix.IoctlGetWinsize(int(os.Stderr.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return unix.Winsize{}
	}
	return *size
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
