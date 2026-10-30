//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

var (
	record = flag.Bool("record", false, "write the WP-CLI outputs of the fixtures to internal/wpcli/testdata/recorded")
	only   = flag.String("fixtures", "", "record the read commands of these fixtures only, comma separated")
)

// recorded is one WP-CLI call on a fixture, as the parsers' tests replay it.
type recorded struct {
	Args   []string `json:"args"`
	Exit   int      `json:"exit"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
}

// The read commands Vigie runs, with the same flags; --path is added per fixture.
var recordings = []struct {
	name string
	args []string
}{
	{"cli-info", []string{"cli", "info", "--format=json"}},
	{"core-version", []string{"core", "version", "--skip-plugins", "--skip-themes"}},
	{"core-is-installed", []string{"core", "is-installed", "--skip-plugins", "--skip-themes"}},
	{"core-is-installed-network", []string{"core", "is-installed", "--network", "--skip-plugins", "--skip-themes"}},
	{"option-home", []string{"option", "get", "home", "--skip-plugins", "--skip-themes"}},
	{"option-stylesheet", []string{"option", "get", "stylesheet", "--skip-plugins", "--skip-themes"}},
	{"core-check-update", []string{"core", "check-update", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"plugin-list", []string{"plugin", "list", "--format=json", "--skip-plugins", "--skip-themes",
		// Not wporg_last_updated: WP-CLI reads it from Trac, one request per plugin.
		"--fields=name,title,status,version,update,update_version,auto_update,wporg_status"}},
	{"theme-list", []string{"theme", "list", "--format=json", "--skip-plugins", "--skip-themes",
		"--fields=name,title,status,version,update,update_version,auto_update"}},
	{"language-core", []string{"language", "core", "list", "--update=available", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"language-plugin", []string{"language", "plugin", "list", "--all", "--update=available", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"language-theme", []string{"language", "theme", "list", "--all", "--update=available", "--format=json", "--skip-plugins", "--skip-themes"}},
	// With the plugins: a theme can live in a plugin's folder.
	{"theme-list-plugins", []string{"theme", "list", "--format=json", "--skip-themes",
		"--fields=name,title,status,version,update,update_version,auto_update"}},
	{"language-theme-plugins", []string{"language", "theme", "list", "--all", "--update=available", "--format=json", "--skip-themes"}},
	{"site-list", []string{"site", "list", "--format=json", "--skip-plugins", "--skip-themes",
		"--fields=blog_id,url,registered,last_updated,public,archived,spam,deleted"}},
	{"update-core-data", []string{"transient", "get", "update_core", "--network", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"update-plugins-data", []string{"transient", "get", "update_plugins", "--network", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"update-themes-data", []string{"transient", "get", "update_themes", "--network", "--format=json", "--skip-plugins", "--skip-themes"}},
}

var fixtureRoots = map[string]string{
	"plain":         sites + "/plain",
	"parent-config": sites + "/parent-config/public",
	"network":       sites + "/network",
	"outdated":      sites + "/outdated",
	"offline":       sites + "/offline",
	"broken":        sites + "/broken",
	"plugin-theme":  sites + "/plugin-theme",
	"updates":       sites + "/updates",
	"security":      sites + "/security",
}

// TestRecord refreshes the recorded outputs; it only runs with -record (make record).
func TestRecord(t *testing.T) {
	if !*record {
		t.Skip("run with -record to refresh the recorded outputs")
	}
	// The search of the account by discovery, from the home directory as sshd starts it.
	find, err := runAs(context.Background(), "sh", "-c", discover.FindCommand())
	if err != nil {
		t.Fatal(err)
	}
	writeRecording(t, filepath.Join("..", "discover", "testdata", "recorded", "find.json"),
		recorded{Args: []string{discover.FindCommand()}, Exit: find.exit, Stdout: find.stdout, Stderr: find.stderr})

	dir := filepath.Join("..", "wpcli", "testdata", "recorded")
	for fixture, root := range fixtureRoots {
		if *only != "" && !slices.Contains(strings.Split(*only, ","), fixture) {
			continue
		}
		if err := os.MkdirAll(filepath.Join(dir, fixture), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, r := range recordings {
			res, err := runAs(context.Background(), append(append([]string{"wp"}, r.args...), "--path="+root)...)
			if err != nil {
				t.Fatal(err)
			}
			writeRecording(t, filepath.Join(dir, fixture, r.name+".json"),
				recorded{Args: r.args, Exit: res.exit, Stdout: res.stdout, Stderr: res.stderr})
		}
	}
}

// The update commands, recorded on the updates site in this order after the reads: each one
// changes the site for the next. Run as Vigie runs them, from their typed command; name to
// command.
var updateRecordings = []struct {
	name string
	cmd  wpcli.Command
	env  string // a PATH for this command only
}{
	{"preview-plugins", wpcli.UpdatePreview(wpcli.ExtensionPlugins), ""},
	{"preview-themes", wpcli.UpdatePreview(wpcli.ExtensionThemes), ""},
	{"constant-debug", wpcli.ConfigConstant("WP_DEBUG"), ""},
	{"constant-debug-log", wpcli.ConfigConstant("WP_DEBUG_LOG"), ""},
	{"constant-missing", wpcli.ConfigConstant("WP_CONTENT_DIR"), ""},
	{"plugin-dir", wpcli.ExtensionDir(wpcli.ExtensionPlugins, "akismet"), ""},
	{"theme-dir", wpcli.ExtensionDir(wpcli.ExtensionThemes, "twentytwentyfour"), ""},
	{"maintenance-active-off", wpcli.MaintenanceActive(), ""},
	{"maintenance-on", wpcli.Maintenance(true), ""},
	{"maintenance-active-on", wpcli.MaintenanceActive(), ""},
	{"maintenance-off", wpcli.Maintenance(false), ""},
	{"db-export", wpcli.DatabaseExport(".vigie/backups/updates/before.sql"), ""},
	{"db-export-without-dump", wpcli.DatabaseExport(".vigie/backups/updates/nodump.sql"), "/opt/nodump/bin"},
	{"db-import", wpcli.DatabaseImport(".vigie/backups/updates/before.sql"), ""},
	{"update-plugin", wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet"), ""},
	{"update-plugin-again", wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet"), ""},
	{"update-plugin-premium", wpcli.UpdateOne(wpcli.ExtensionPlugins, "fixture-pro"), ""},
	{"update-plugin-missing", wpcli.UpdateOne(wpcli.ExtensionPlugins, "fixture-missing"), ""},
	{"update-theme", wpcli.UpdateOne(wpcli.ExtensionThemes, "twentytwentyfour"), ""},
	{"update-languages-core", wpcli.UpdateLanguages(wpcli.LanguagesOfCore), ""},
	{"update-languages-plugins", wpcli.UpdateLanguages(wpcli.LanguagesOfPlugins), ""},
	{"update-languages-themes", wpcli.UpdateLanguages(wpcli.LanguagesOfThemes), ""},
	{"update-core", wpcli.UpdateCore("7.1.4"), ""},
	{"update-database", wpcli.UpdateDatabase(false), ""},
	{"update-database-again", wpcli.UpdateDatabase(false), ""},
	// Last: once installed, version 2.0.0 stops anything that loads the plugins.
	{"update-plugin-breaks", wpcli.UpdateOne(wpcli.ExtensionPlugins, "fixture-breaks"), ""},
}

// TestRecordUpdates refreshes the recorded outputs of the update commands; with -record only.
func TestRecordUpdates(t *testing.T) {
	if !*record {
		t.Skip("run with -record to refresh the recorded outputs")
	}
	ctx := context.Background()
	if res, err := runAs(ctx, "mkdir", "-p", ".vigie/backups/updates"); err != nil || res.exit != 0 {
		t.Fatalf("backups folder: %v %s", err, res.stderr)
	}
	dir := filepath.Join("..", "wpcli", "testdata", "recorded", "updates")
	for _, r := range updateRecordings {
		remote, err := r.cmd.Remote("wp", "~/sites/updates")
		if err != nil {
			t.Fatal(err)
		}
		line := remote
		if r.env != "" {
			line = "PATH=" + r.env + " " + remote
		}
		res, err := runAs(ctx, "sh", "-c", line)
		if err != nil {
			t.Fatal(err)
		}
		writeRecording(t, filepath.Join(dir, r.name+".json"),
			recorded{Args: []string{remote}, Exit: res.exit, Stdout: res.stdout, Stderr: res.stderr})
	}
}

// TestRecordSecurity refreshes the recorded outputs of the security reads of every fixture, as Vigie
// runs them, and of the commands compared on one site only; with -record only.
func TestRecordSecurity(t *testing.T) {
	if !*record {
		t.Skip("run with -record to refresh the recorded outputs")
	}
	ctx := context.Background()
	dir := filepath.Join("..", "wpcli", "testdata", "recorded")
	run := func(file, line string) recorded {
		t.Helper()
		res, err := runAs(ctx, "sh", "-c", line)
		if err != nil {
			t.Fatal(err)
		}
		r := recorded{Args: []string{line}, Exit: res.exit, Stdout: res.stdout, Stderr: res.stderr}
		writeRecording(t, file, r)
		return r
	}
	for fixture, root := range fixtureRoots {
		path := "~/" + strings.TrimPrefix(root, "/home/vigie/")
		lines := map[string]wpclitest.Line{}
		for name, cmd := range wpclitest.SecurityReads {
			lines[name] = wpclitest.Line{Cmd: cmd}
		}
		lines["real-root"] = wpclitest.RealRoot(path)
		out := map[string]string{}
		for name, l := range lines {
			line, err := l.Remote("wp", path)
			if err != nil {
				t.Fatal(err)
			}
			out[name] = run(filepath.Join(dir, fixture, name+".json"), line).Stdout
		}
		for name, l := range wpclitest.Derived([]byte(out["administrators"]), []byte(out["db-prefix"]), []byte(out["real-root"])) {
			line, err := l.Remote("wp", path)
			if err != nil {
				t.Fatal(err)
			}
			run(filepath.Join(dir, fixture, name+".json"), line)
		}
	}

	// An option that does not exist.
	line, err := wpcli.OptionGet("fixture_absent").Remote("wp", "~/sites/security")
	if err != nil {
		t.Fatal(err)
	}
	run(filepath.Join(dir, "security", "option-missing.json"), line)
	// The uploads, for the remote package's own tests.
	for name, site := range map[string]string{"uploads-php": "security", "uploads-php-clean": "plain"} {
		line, err := remote.UploadsPHP(sites + "/" + site)
		if err != nil {
			t.Fatal(err)
		}
		run(filepath.Join("..", "remote", "testdata", "recorded", name+".json"), line)
	}
}

func writeRecording(t *testing.T, file string, r recorded) {
	t.Helper()
	if strings.Contains(r.Stdout+r.Stderr, dbPassword) {
		t.Fatalf("%s: output contains the database password", file)
	}
	// The fake two-factor secrets of the security site: Vigie never reads them.
	if strings.Contains(r.Stdout+r.Stderr, "fixture-totp-secret") {
		t.Fatalf("%s: output contains a two-factor secret", file)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
