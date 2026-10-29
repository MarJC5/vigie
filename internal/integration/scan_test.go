//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
)

// scanFixture scans one fixture site over SSH, as the app does.
func scanFixture(t *testing.T, path string, multisite bool) (model.Scan, []model.Subsite) {
	t.Helper()
	a := sshAccess(t)
	s := scan.Scanner{Runner: a.runner(t, a.trusted(t)), Limiter: sshx.NewLimiter(), Now: timeNow}
	return s.Site(context.Background(), scan.Job{
		Site:   model.Site{ID: model.NewID(), Name: path, Path: path, Multisite: multisite},
		Target: a.target(a.key), HostKey: "bench", Max: 2, WPCLI: "wp",
	}, nil)
}

func TestScanOverSSH(t *testing.T) {
	outdated, _ := scanFixture(t, "~/sites/outdated", false)
	if outdated.Outcome != model.ScanCompleted || outdated.Status() != model.StatusAttention {
		t.Fatalf("outdated: %q (%s), status %q", outdated.Outcome, outdated.Error, outdated.Status())
	}
	if !outdated.CoreChecked || !outdated.PluginsChecked || !outdated.ThemesChecked || !outdated.LanguagesChecked {
		t.Errorf("outdated: checks %v %v %v %v, want all proven", outdated.CoreChecked, outdated.PluginsChecked,
			outdated.ThemesChecked, outdated.LanguagesChecked)
	}
	if c := outdated.PendingUpdates(); c.Plugins < 1 || c.Themes < 1 || c.Languages < 1 {
		t.Errorf("outdated: pending %+v, want plugin, theme and translation updates", c)
	}

	offline, _ := scanFixture(t, "~/sites/offline", false)
	if offline.Outcome != model.ScanCompleted || offline.CoreChecked || offline.PluginsChecked || offline.ThemesChecked {
		t.Errorf("offline: %q, checks %v %v %v; want completed and no check proven",
			offline.Outcome, offline.CoreChecked, offline.PluginsChecked, offline.ThemesChecked)
	}

	broken, _ := scanFixture(t, "~/sites/broken", false)
	if broken.Outcome != model.ScanCannotLoad || !strings.Contains(broken.Error, "database") {
		t.Errorf("broken: %q (%s), want cannot-load", broken.Outcome, broken.Error)
	}

	network, subsites := scanFixture(t, "~/sites/network", true)
	if network.Outcome != model.ScanCompleted || len(subsites) != 4 {
		t.Errorf("network: %q (%s), %d subsites; want 4", network.Outcome, network.Error, len(subsites))
	}

	parent, _ := scanFixture(t, "~/sites/parent-config/public", false)
	if parent.Outcome != model.ScanCompleted || parent.Home != "https://parent.example.test" {
		t.Errorf("wp-config.php one level up: %q (%s), home %q", parent.Outcome, parent.Error, parent.Home)
	}

	// The only theme lives in a plugin's folder: read again with the plugins.
	pluginTheme, _ := scanFixture(t, "~/sites/plugin-theme", false)
	themes := 0
	for _, i := range pluginTheme.Items {
		if i.Kind == model.ItemTheme && i.Slug == "fixture-theme" && i.Status == "active" {
			themes++
		}
	}
	if pluginTheme.Outcome != model.ScanCompleted || themes != 1 || !pluginTheme.LanguagesChecked {
		t.Errorf("theme in a plugin: %q (%s), %d active fixture-theme, languages checked %v",
			pluginTheme.Outcome, pluginTheme.Error, themes, pluginTheme.LanguagesChecked)
	}
}
