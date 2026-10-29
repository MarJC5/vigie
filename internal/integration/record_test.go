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
	{"core-check-update", []string{"core", "check-update", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"plugin-list", []string{"plugin", "list", "--format=json", "--skip-plugins", "--skip-themes",
		// Not wporg_last_updated: WP-CLI reads it from Trac, one request per plugin.
		"--fields=name,title,status,version,update,update_version,auto_update,wporg_status"}},
	{"theme-list", []string{"theme", "list", "--format=json", "--skip-plugins", "--skip-themes",
		"--fields=name,title,status,version,update,update_version,auto_update"}},
	{"language-core", []string{"language", "core", "list", "--update=available", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"language-plugin", []string{"language", "plugin", "list", "--all", "--update=available", "--format=json", "--skip-plugins", "--skip-themes"}},
	{"language-theme", []string{"language", "theme", "list", "--all", "--update=available", "--format=json", "--skip-plugins", "--skip-themes"}},
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
}

// TestRecord refreshes the recorded outputs; it only runs with -record (make record).
func TestRecord(t *testing.T) {
	if !*record {
		t.Skip("run with -record to refresh the recorded outputs")
	}
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
			if strings.Contains(res.stdout+res.stderr, dbPassword) {
				t.Fatalf("%s/%s: output contains the database password", fixture, r.name)
			}
			data, err := json.MarshalIndent(recorded{Args: r.args, Exit: res.exit, Stdout: res.stdout, Stderr: res.stderr}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fixture, r.name+".json"), append(data, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
