package cli_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/cli"
)

func TestParse(t *testing.T) {
	tests := []struct {
		args    []string
		dataDir string
		command []string
	}{
		{nil, "", nil},
		{[]string{"version"}, "", []string{"version"}},
		{[]string{"--data-dir", "/tmp/v", "version"}, "/tmp/v", []string{"version"}},
		{[]string{"--data-dir=/tmp/v"}, "/tmp/v", nil},
		{[]string{"-psn_0_12345"}, "", nil}, // process serial number added by the macOS launcher
	}
	for _, tt := range tests {
		opts, err := cli.Parse(tt.args)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.args, err)
		}
		if opts.DataDir != tt.dataDir || !slices.Equal(opts.Command, tt.command) {
			t.Errorf("Parse(%q) = %+v, want data dir %q, command %q", tt.args, opts, tt.dataDir, tt.command)
		}
	}
}

func TestParseRejectsMissingValue(t *testing.T) {
	if _, err := cli.Parse([]string{"--data-dir"}); err == nil {
		t.Fatal("Parse(--data-dir) without a value: want an error")
	}
}

func TestOpensWindowWithoutCommand(t *testing.T) {
	opts, _ := cli.Parse([]string{"--data-dir", "/tmp/v"})
	if !opts.OpensWindow() {
		t.Fatal("no command: the window must open")
	}
	opts, _ = cli.Parse([]string{"version"})
	if opts.OpensWindow() {
		t.Fatal("a command runs in the terminal, without a window")
	}
}

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cli.Run(cli.Options{Command: []string{"version"}}, &out, &errOut)
	if code != 0 || out.String() != "vigie dev\n" {
		t.Fatalf("Run(version) = %d, %q", code, out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cli.Run(cli.Options{Command: []string{"nope"}}, &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "unknown command") || !strings.Contains(errOut.String(), "Usage") {
		t.Fatalf("Run(nope) = %d, stderr %q", code, errOut.String())
	}
}

func TestRunHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := cli.Run(cli.Options{Command: []string{"help"}}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "Usage") {
		t.Fatalf("Run(help) = %d, %q", code, out.String())
	}
}
