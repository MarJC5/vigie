//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/wpcli"
)

var record = flag.Bool("record", false, "write the WP-CLI outputs of the fixtures to internal/wpcli/testdata/recorded")

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

// The security commands, recorded on the security site and, for comparison, on plain ones; name to
// site and command line. Remote commands run from the site's real folder, as Vigie runs them.
var securityRecordings = []struct {
	name, site string
	cmd        wpcli.Command
	shell      func(root string) (string, error)
}{
	{"core-checksums", "security", wpcli.CoreVerifyChecksums(), nil},
	{"core-checksums-clean", "plain", wpcli.CoreVerifyChecksums(), nil},
	{"plugin-checksums", "security", wpcli.PluginVerifyChecksums(), nil},
	{"plugin-checksums-clean", "plain", wpcli.PluginVerifyChecksums(), nil},
	// Every plugin skipped: wordpress.org knows none of them.
	{"plugin-checksums-unknown", "plugin-theme", wpcli.PluginVerifyChecksums(), nil},
	{"administrators", "security", wpcli.Administrators(), nil},
	{"administrators-plain", "plain", wpcli.Administrators(), nil},
	{"super-admins", "network", wpcli.SuperAdmins(), nil},
	{"super-admins-single", "security", wpcli.SuperAdmins(), nil},
	{"two-factor-2", "security", wpcli.TwoFactorMeta(2), nil},
	{"two-factor-3", "security", wpcli.TwoFactorMeta(3), nil},
	{"two-factor-4", "security", wpcli.TwoFactorMeta(4), nil},
	{"two-factor-5", "security", wpcli.TwoFactorMeta(5), nil},
	{"db-prefix", "security", wpcli.DatabasePrefix(), nil},
	{"wordfence-two-factor", "security", wpcli.WordfenceTwoFactorUsers("wp_"), nil},
	// No Wordfence table.
	{"wordfence-two-factor-none", "plain", wpcli.WordfenceTwoFactorUsers("wp_"), nil},
	{"config-names", "security", wpcli.ConfigNames(), nil},
	{"constant-debug", "security", wpcli.ConfigConstant("WP_DEBUG"), nil},
	{"constant-missing", "security", wpcli.ConfigConstant("DISALLOW_FILE_EDIT"), nil},
	{"option-blog-public", "security", wpcli.OptionGet("blog_public"), nil},
	{"option-auto-update-major", "security", wpcli.OptionGet("auto_update_core_major"), nil},
	{"option-missing", "security", wpcli.OptionGet("fixture_absent"), nil},
	{"option-sizes", "security", wpcli.OptionSizes(false), nil},
	{"transient-sizes", "security", wpcli.OptionSizes(true), nil},
	{"cron-events", "security", wpcli.CronEvents(), nil},
	{"db-version", "security", wpcli.DatabaseVersion(), nil},
	{"uploads-php", "security", wpcli.Command{}, remote.UploadsPHP},
	{"uploads-php-clean", "plain", wpcli.Command{}, remote.UploadsPHP},
}

// TestRecordSecurity refreshes the recorded outputs of the security commands; with -record only.
func TestRecordSecurity(t *testing.T) {
	if !*record {
		t.Skip("run with -record to refresh the recorded outputs")
	}
	ctx := context.Background()
	dir := filepath.Join("..", "wpcli", "testdata", "recorded", "security")
	for _, r := range securityRecordings {
		var line string
		var err error
		if r.shell != nil {
			line, err = r.shell(sites + "/" + r.site)
		} else {
			line, err = r.cmd.Remote("wp", "~/sites/"+r.site)
		}
		if err != nil {
			t.Fatal(err)
		}
		res, err := runAs(ctx, "sh", "-c", line)
		if err != nil {
			t.Fatal(err)
		}
		// A remote command's recording goes with the package that builds it.
		to := dir
		if r.shell != nil {
			to = filepath.Join("..", "remote", "testdata", "recorded")
		}
		writeRecording(t, filepath.Join(to, r.name+".json"),
			recorded{Args: []string{line}, Exit: res.exit, Stdout: res.stdout, Stderr: res.stderr})
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
