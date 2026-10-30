package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/addon/addontest"
	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/workspace"
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

// Started by the system at login (Linux's autostart entry passes it), the window opens hidden.
func TestParseAtLogin(t *testing.T) {
	opts, err := cli.Parse([]string{"--at-login"})
	if err != nil || !opts.AtLogin || !opts.OpensWindow() {
		t.Fatalf("Parse(--at-login) = %+v, %v; want the window, at login", opts, err)
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

// run runs a command line with a workspace over the bench recordings, in lang.
func run(t *testing.T, lang string, args ...string) (int, string, string) {
	t.Helper()
	opts, err := cli.Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), opts, cli.Env{
		Out: &out, Err: &errOut, Lang: lang, Texts: texts(t),
		Open: func(context.Context) (*workspace.Workspace, error) { return benchWorkspace(t), nil },
	})
	return code, out.String(), errOut.String()
}

func TestRunVersion(t *testing.T) {
	code, out, _ := run(t, "en", "version")
	if code != 0 || out != "vigie dev\n" {
		t.Fatalf("version = %d, %q", code, out)
	}
}

func TestRunUnknownCommandIsAnError(t *testing.T) {
	code, _, errOut := run(t, "en", "nope")
	if code != 3 || !strings.Contains(errOut, `unknown command "nope"`) || !strings.Contains(errOut, "Usage") {
		t.Fatalf("nope = %d, stderr %q", code, errOut)
	}
}

func TestRunHelpInTheUsersLanguage(t *testing.T) {
	code, out, _ := run(t, "fr", "help")
	if code != 0 || !strings.Contains(out, "Usage") || !strings.Contains(out, "Relève un site") {
		t.Fatalf("help = %d, %q", code, out)
	}
}

// Every ssh failure has its explanation, in every language.
func TestEveryFailureIsExplained(t *testing.T) {
	c := texts(t)
	for _, lang := range []string{"en", "fr"} {
		for _, f := range sshx.Failures {
			key := "failure." + string(f)
			if c.T(lang, key, nil) == key {
				t.Errorf("%s: %s has no text", lang, key)
			}
		}
	}
}

// A name that is not a command of Vigie runs vigie-<name> from the PATH, as Git does.
func TestAnAddOnRunsFromThePATH(t *testing.T) {
	program := addontest.Build(t, "../addons/testdata/fake")
	dir := t.TempDir()
	data, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"vigie-good", "vigie-fails"} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	run := func(args ...string) (int, string, string) {
		opts, err := cli.Parse(args)
		if err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		code := cli.Run(context.Background(), opts, cli.Env{Out: &out, Err: &errOut, Lang: "en", Texts: texts(t)})
		return code, out.String(), errOut.String()
	}
	if code, out, _ := run("good", "--vigie-info"); code != 0 || !strings.Contains(out, `"name": "good"`) {
		t.Errorf("vigie good --vigie-info: exit %d\n%s", code, out)
	}
	if code, _, errOut := run("fails", "--names", "x"); code != 3 || errOut != "vigie-fails: the list cannot be read\n" {
		t.Errorf("vigie fails: exit %d, %q", code, errOut)
	}
	for _, name := range []string{"missing", "../good", "-good"} {
		if code, _, errOut := run(name); code != 3 || !strings.Contains(errOut, "unknown command") {
			t.Errorf("vigie %s: exit %d, %s", name, code, errOut)
		}
	}
}
