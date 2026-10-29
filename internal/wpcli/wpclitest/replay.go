// Package wpclitest replays the WP-CLI outputs recorded from the container bench (make record),
// for tests that must never reach a server.
package wpclitest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// Fixtures are the recorded sites of the bench.
var Fixtures = []string{"plain", "parent-config", "network", "outdated", "offline", "broken", "plugin-theme", "updates"}

// commands are the read commands Vigie runs, by the name of their recording.
var commands = map[string]wpcli.Command{
	"core-is-installed":         wpcli.CoreIsInstalled(false),
	"core-is-installed-network": wpcli.CoreIsInstalled(true),
	"core-version":              wpcli.CoreVersion(),
	"option-home":               wpcli.OptionGet("home"),
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
		remote, err := cmd.Remote(wp, root)
		if err != nil {
			t.Fatal(err)
		}
		fake.Set(remote, Recorded(t, fixture, name))
	}
	remote, err := wpcli.CLIInfo().Remote(wp, "")
	if err != nil {
		t.Fatal(err)
	}
	fake.Set(remote, Recorded(t, fixture, "cli-info"))
}
