package wpcli_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

func TestSecurityCommandLines(t *testing.T) {
	for _, tt := range []struct {
		cmd  wpcli.Command
		want string
	}{
		{wpcli.CoreVerifyChecksums(), "'wp' 'core' 'verify-checksums' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.PluginVerifyChecksums(), "'wp' 'plugin' 'verify-checksums' '--all' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
	} {
		if got, err := tt.cmd.Remote("wp", "~/site"); err != nil || got != tt.want {
			t.Errorf("got %s, %v\nwant %s", got, err, tt.want)
		}
	}
}

// The recorded commands are the ones Vigie runs: a recording holds its command line.
func TestSecurityRecordingsAreCurrent(t *testing.T) {
	for name, cmd := range map[string]wpcli.Command{
		"core-checksums":   wpcli.CoreVerifyChecksums(),
		"plugin-checksums": wpcli.PluginVerifyChecksums(),
	} {
		want, err := cmd.Remote("wp", "~/sites/security")
		if err != nil {
			t.Fatal(err)
		}
		if got := load(t, "security", name).Args; len(got) != 1 || got[0] != want {
			t.Errorf("%s recorded %v, Vigie runs %s: run make record", name, got, want)
		}
	}
}

func TestParseCoreChecksums(t *testing.T) {
	got, err := wpcli.ParseCoreChecksums(result(load(t, "security", "core-checksums")))
	if err != nil {
		t.Fatal(err)
	}
	want := []wpcli.FileProblem{
		{File: "wp-admin/freedoms.php", Change: wpcli.FileMissing},
		{File: "wp-includes/functions.php", Change: wpcli.FileChanged},
		{File: "wp-config-docker.php", Change: wpcli.FileAdded},
		{File: "wp-includes/fixture-added.php", Change: wpcli.FileAdded},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCoreChecksums() = %+v\nwant %+v", got, want)
	}
	// WP-CLI says Success when the only problem is an added file: the warning still counts.
	got, err = wpcli.ParseCoreChecksums(result(load(t, "security", "core-checksums-clean")))
	if err != nil || !reflect.DeepEqual(got, []wpcli.FileProblem{{File: "wp-config-docker.php", Change: wpcli.FileAdded}}) {
		t.Fatalf("ParseCoreChecksums(clean) = %+v, %v", got, err)
	}
}

func TestCoreChecksumsThatCouldNotRun(t *testing.T) {
	res := sshx.Result{ExitCode: 1, Stderr: []byte("Error: Couldn't get checksums from WordPress.org.\n")}
	if _, err := wpcli.ParseCoreChecksums(res); !errors.Is(err, wpcli.ErrNotVerified) {
		t.Errorf("ParseCoreChecksums() without checksums = %v, want ErrNotVerified", err)
	}
	if _, err := wpcli.ParseCoreChecksums(sshx.Result{ExitCode: 0, Stdout: []byte("hello\n")}); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
		t.Errorf("ParseCoreChecksums() of an unknown answer = %v, want ErrUnexpectedOutput", err)
	}
}

func TestParsePluginChecksums(t *testing.T) {
	got, err := wpcli.ParsePluginChecksums(result(load(t, "security", "plugin-checksums")))
	if err != nil {
		t.Fatal(err)
	}
	want := wpcli.PluginChecksums{
		Problems: []wpcli.FileProblem{
			{Plugin: "akismet", File: "fixture-added.php", Change: wpcli.FileAdded},
			{Plugin: "akismet", File: "akismet.php", Change: wpcli.FileChanged},
		},
		Total: 3, Verified: 1, Failed: 1, Skipped: []string{"fixture-premium"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParsePluginChecksums() = %+v\nwant %+v", got, want)
	}
	got, err = wpcli.ParsePluginChecksums(result(load(t, "security", "plugin-checksums-clean")))
	if err != nil || !reflect.DeepEqual(got, wpcli.PluginChecksums{Total: 2, Verified: 2}) {
		t.Fatalf("ParsePluginChecksums(clean) = %+v, %v", got, err)
	}
	// Success, with one plugin skipped: it is not verified, and Vigie says so.
	got, err = wpcli.ParsePluginChecksums(result(load(t, "security", "plugin-checksums-unknown")))
	if err != nil || !reflect.DeepEqual(got, wpcli.PluginChecksums{Total: 3, Verified: 2, Skipped: []string{"fixture-themes"}}) {
		t.Fatalf("ParsePluginChecksums(unknown) = %+v, %v", got, err)
	}
}
