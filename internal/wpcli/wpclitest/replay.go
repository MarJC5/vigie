// Package wpclitest replays the WP-CLI outputs recorded from the container bench (make record),
// for tests that must never reach a server.
package wpclitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// Fixtures are the recorded sites of the bench.
var Fixtures = []string{"plain", "parent-config", "network", "outdated", "offline", "broken", "plugin-theme", "updates", "security"}

// commands are the read commands Vigie runs, by the name of their recording.
var commands = map[string]wpcli.Command{
	"core-is-installed":         wpcli.CoreIsInstalled(false),
	"core-is-installed-network": wpcli.CoreIsInstalled(true),
	"core-version":              wpcli.CoreVersion(),
	"option-home":               wpcli.OptionGet("home"),
	"option-siteurl":            wpcli.OptionGet("siteurl"),
	"option-stylesheet":         wpcli.OptionGet("stylesheet"),
	"core-check-update":         wpcli.CoreCheckUpdate(),
	"update-core-data":          wpcli.UpdateData(wpcli.UpdatesOfCore),
	"plugin-list":               wpcli.PluginList(),
	"update-plugins-data":       wpcli.UpdateData(wpcli.UpdatesOfPlugins),
	"theme-list":                wpcli.ThemeList(),
	"theme-list-plugins":        wpcli.ThemeList().WithPlugins(),
	"update-themes-data":        wpcli.UpdateData(wpcli.UpdatesOfThemes),
	"language-core":             wpcli.LanguageUpdates(wpcli.LanguagesOfCore),
	"language-plugin":           wpcli.LanguageUpdates(wpcli.LanguagesOfPlugins),
	"language-theme":            wpcli.LanguageUpdates(wpcli.LanguagesOfThemes),
	"language-theme-plugins":    wpcli.LanguageUpdates(wpcli.LanguagesOfThemes).WithPlugins(),
	"site-list":                 wpcli.SiteList(),
}

// SecurityReads are the security read commands of a scan, by the name of their recording; the
// two-factor of each administrator, Wordfence's table and the uploads follow from what they read.
var SecurityReads = map[string]wpcli.Command{
	"core-checksums":              wpcli.CoreVerifyChecksums(),
	"plugin-checksums":            wpcli.PluginVerifyChecksums(),
	"administrators":              wpcli.Administrators(),
	"super-admins":                wpcli.SuperAdmins(),
	"db-prefix":                   wpcli.DatabasePrefix(),
	"config-names":                wpcli.ConfigNames(),
	"constant-wp-debug":           wpcli.ConfigConstant("WP_DEBUG"),
	"constant-wp-debug-display":   wpcli.ConfigConstant("WP_DEBUG_DISPLAY"),
	"constant-disallow-file-edit": wpcli.ConfigConstant("DISALLOW_FILE_EDIT"),
	"option-blog-public":          wpcli.OptionGet("blog_public"),
	"cron-events":                 wpcli.CronEvents(),
	"option-sizes":                wpcli.OptionSizes(false),
	"transient-sizes":             wpcli.OptionSizes(true),
	"db-version":                  wpcli.DatabaseVersion(),
	"constant-wp-debug-log":       wpcli.ConfigConstant("WP_DEBUG_LOG"),
	"option-admin-email":          wpcli.OptionGet("admin_email"),
	"option-new-admin-email":      wpcli.OptionGet("new_admin_email"),
	"php-checks":                  wpcli.PHPChecks(),
}

// Networks are the recorded fixtures that are multisite networks: their scans read the subsites
// with the script of ADR-0029, recorded as "subsites".
var Networks = []string{"network"}

// Derived are the security commands that follow from the recordings of a fixture, read by name:
// the two-factor of each administrator, Wordfence's table with the prefix read, the PHP files in
// the uploads and debug.log, found from the real folder and the constants read. A read that failed
// gives none.
func Derived(read func(name string) sshx.Result) map[string]Line {
	lines := map[string]Line{}
	if admins, err := wpcli.ParseAdministrators(read("administrators").Stdout); err == nil {
		for _, a := range admins {
			lines["two-factor-"+strconv.Itoa(a.ID)] = Line{Cmd: wpcli.TwoFactorMeta(a.ID)}
		}
	}
	if p, err := wpcli.ParseDatabasePrefix(read("db-prefix").Stdout); err == nil {
		lines["wordfence-two-factor"] = Line{Cmd: wpcli.WordfenceTwoFactorUsers(p)}
	}
	dir, err := wpcli.ParseDir(read("real-root").Stdout)
	if err != nil {
		return lines
	}
	lines["uploads-php"] = Line{Shell: func() (string, error) { return remote.UploadsPHP(dir) }}
	if log, err := wpcli.ParseDebugLog(read("constant-wp-debug"), read("constant-wp-debug-log")); err == nil {
		if file, err := wpcli.DebugLogFile(dir, log); err == nil {
			lines["debug-log"] = Line{Shell: func() (string, error) { return remote.DebugLog(file) }}
		}
	}
	return lines
}

// Line is a command of a scan: WP-CLI's, run at the site's path, or a shell command line.
type Line struct {
	Cmd   wpcli.Command
	Shell func() (string, error)
}

// Remote is the command line run on the server for a site at root.
func (l Line) Remote(wp, root string) (string, error) {
	if l.Shell != nil {
		return l.Shell()
	}
	return l.Cmd.Remote(wp, root)
}

// RealRoot is the command that reads the site's real folder, recorded as "real-root".
func RealRoot(root string) Line {
	return Line{Shell: func() (string, error) { return remote.RealRoot(root) }}
}

// Recorded returns one recorded call of a fixture.
func Recorded(t testing.TB, fixture, name string) sshx.Result {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "testdata", "recorded", fixture, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Exit           int
		Stdout, Stderr string
	}
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return sshx.Result{Stdout: []byte(r.Stdout), Stderr: []byte(r.Stderr), ExitCode: r.Exit}
}

// Replay makes fake answer every read command for a site at root, run with the WP-CLI command
// wp, with the recordings of fixture.
func Replay(t testing.TB, fake *sshxtest.Fake, fixture, root, wp string) {
	t.Helper()
	for name, cmd := range commands {
		line, err := cmd.Remote(wp, root)
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(line, Recorded(t, fixture, name))
	}
	line, err := wpcli.CLIInfo().Remote(wp, "")
	if err != nil {
		t.Fatal(err)
	}
	fake.Set(line, Recorded(t, fixture, "cli-info"))

	lines := map[string]Line{"real-root": RealRoot(root)}
	for name, cmd := range SecurityReads {
		lines[name] = Line{Cmd: cmd}
	}
	for name, l := range Derived(func(name string) sshx.Result { return Recorded(t, fixture, name) }) {
		lines[name] = l
	}
	for name, l := range lines {
		line, err := l.Remote(wp, root)
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(line, Recorded(t, fixture, name))
	}
	if slices.Contains(Networks, fixture) {
		line, err := wpcli.Subsites().Remote(wp, root)
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(line, Recorded(t, fixture, "subsites"))
	}
}

// ReplayUpdates makes fake answer update commands of the updates fixture (at ~/sites/updates,
// with the WP-CLI command wp) with their recordings. Each recording holds the command line its
// typed command gave when recorded, so a command that changed gets no answer.
func ReplayUpdates(t testing.TB, fake *sshxtest.Fake, names ...string) {
	t.Helper()
	for _, name := range names {
		_, here, _, _ := runtime.Caller(0)
		data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "testdata", "recorded", "updates", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var r struct {
			Args []string
		}
		if err := json.Unmarshal(data, &r); err != nil || len(r.Args) != 1 {
			t.Fatalf("%s: one command line expected: %v", name, err)
		}
		fake.Set(r.Args[0], Recorded(t, "updates", name))
	}
}
