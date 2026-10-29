package wpcli_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

func TestUpdateCommandLines(t *testing.T) {
	tests := []struct {
		cmd  wpcli.Command
		want string
	}{
		// Plugins run their own updaters (premium plugins); for themes, the active theme's too.
		{wpcli.UpdatePreview(wpcli.ExtensionPlugins), "'wp' 'plugin' 'update' '--all' '--dry-run' '--format=json' '--path=site' '--skip-themes'"},
		{wpcli.UpdatePreview(wpcli.ExtensionThemes), "'wp' 'theme' 'update' '--all' '--dry-run' '--format=json' '--path=site'"},
		{wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet"), "'wp' 'plugin' 'update' 'akismet' '--format=json' '--path=site' '--skip-themes'"},
		{wpcli.UpdateCore("7.1.4"), "'wp' 'core' 'update' '--version=7.1.4' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.UpdateDatabase(true), "'wp' 'core' 'update-db' '--network' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.UpdateLanguages(wpcli.LanguagesOfPlugins), "'wp' 'language' 'plugin' 'update' '--all' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.Maintenance(true), "'wp' 'maintenance-mode' 'activate' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.DatabaseExport(".vigie/backups/a/1.sql"), "'wp' 'db' 'export' '.vigie/backups/a/1.sql' '--porcelain' '--path=site' '--skip-plugins' '--skip-themes'"},
		// A theme can live in a plugin's folder.
		{wpcli.ExtensionDir(wpcli.ExtensionThemes, "gutheme"), "'wp' 'theme' 'path' 'gutheme' '--dir' '--path=site' '--skip-themes'"},
		{wpcli.ConfigConstant("WP_DEBUG_LOG"), "'wp' 'config' 'get' 'WP_DEBUG_LOG' '--type=constant' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
	}
	for _, tt := range tests {
		got, err := tt.cmd.Remote("wp", "~/site")
		if err != nil || got != tt.want {
			t.Errorf("got %s, %v\nwant %s", got, err, tt.want)
		}
	}
}

func TestUpdateCommandsRefuseBadArguments(t *testing.T) {
	for name, cmd := range map[string]wpcli.Command{
		"slug with a space":      wpcli.UpdateOne(wpcli.ExtensionPlugins, "a b"),
		"slug as an option":      wpcli.UpdateOne(wpcli.ExtensionPlugins, "--all"),
		"unknown kind":           wpcli.UpdatePreview("core"),
		"version as text":        wpcli.UpdateCore("latest"),
		"dump out of backups":    wpcli.DatabaseExport("/tmp/x.sql"),
		"dump going up":          wpcli.DatabaseExport(".vigie/backups/../x.sql"),
		"constant in lower case": wpcli.ConfigConstant("wp_debug"),
		"path of a slug option":  wpcli.ExtensionDir(wpcli.ExtensionThemes, "-x"),
	} {
		if _, err := cmd.Remote("wp", "~/site"); !errors.Is(err, wpcli.ErrBadArgument) {
			t.Errorf("%s: %v, want ErrBadArgument", name, err)
		}
	}
}

func TestParseUpdatePreview(t *testing.T) {
	plugins, err := wpcli.ParseUpdatePreview([]byte(load(t, "updates", "preview-plugins").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	want := []wpcli.Pending{
		{Name: "akismet", Version: "5.3", UpdateVersion: "5.7.2"},
		{Name: "fixture-breaks", Version: "1.0.0", UpdateVersion: "2.0.0"},
		{Name: "fixture-missing", Version: "1.0.0", UpdateVersion: "9.9.9"},
		{Name: "fixture-pro", Version: "1.0.0", UpdateVersion: "1.1.0"},
	}
	if !reflect.DeepEqual(plugins, want) {
		t.Errorf("plugins = %+v", plugins)
	}
	// With --format=json, WP-CLI prints nothing when there is no update.
	if none, err := wpcli.ParseUpdatePreview(nil); err != nil || len(none) != 0 {
		t.Errorf("empty output = %+v, %v; want none", none, err)
	}
	if _, err := wpcli.ParseUpdatePreview([]byte(`[{"status":"active"}]`)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
		t.Errorf("entry without a name: %v, want ErrUnexpectedOutput", err)
	}
}

func TestParseUpdateOne(t *testing.T) {
	tests := []struct {
		name string
		want wpcli.Updated
	}{
		{"update-plugin", wpcli.Updated{Name: "akismet", From: "5.3", To: "5.7.2", Outcome: wpcli.UpdateDone}},
		{"update-plugin-premium", wpcli.Updated{Name: "fixture-pro", From: "1.0.0", To: "1.1.0", Outcome: wpcli.UpdateDone}},
		{"update-theme", wpcli.Updated{Name: "twentytwentyfour", From: "1.0", To: "1.6", Outcome: wpcli.UpdateDone}},
		// Nothing printed: there was no update to make.
		{"update-plugin-again", wpcli.Updated{Outcome: wpcli.UpdateNothing}},
		{"update-plugin-missing", wpcli.Updated{Name: "fixture-missing", From: "1.0.0", To: "9.9.9", Outcome: wpcli.UpdateFailed,
			Message: "Error: No plugins updated (1 failed)."}},
	}
	for _, tt := range tests {
		got, err := wpcli.ParseUpdateOne(result(load(t, "updates", tt.name)))
		if err != nil || got != tt.want {
			t.Errorf("%s = %+v, %v\nwant %+v", tt.name, got, err, tt.want)
		}
	}
	cannot := sshx.Result{ExitCode: 255, Stderr: []byte("PHP Fatal error: Uncaught Error in fixture-breaks.php")}
	if _, err := wpcli.ParseUpdateOne(cannot); !errors.Is(err, wpcli.ErrCannotLoad) {
		t.Errorf("WordPress not loading: %v, want ErrCannotLoad", err)
	}
}

func TestDone(t *testing.T) {
	for _, name := range []string{"update-core", "update-database", "update-languages-core", "update-languages-themes", "maintenance-on", "db-import"} {
		if err := wpcli.Done(result(load(t, "updates", name))); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	failed := sshx.Result{ExitCode: 1, Stderr: []byte("Error: Another update is currently in progress.\n")}
	if err := wpcli.Done(failed); !errors.Is(err, wpcli.ErrFailed) || err.Error() != "WP-CLI command failed: Error: Another update is currently in progress." {
		t.Errorf("failed = %v", err)
	}
}

func TestMaintenanceIsActive(t *testing.T) {
	on, err := wpcli.MaintenanceIsActive(result(load(t, "updates", "maintenance-active-on")))
	if err != nil || !on {
		t.Errorf("on = %v, %v", on, err)
	}
	off, err := wpcli.MaintenanceIsActive(result(load(t, "updates", "maintenance-active-off")))
	if err != nil || off {
		t.Errorf("off = %v, %v", off, err)
	}
}

func TestParseDatabaseExport(t *testing.T) {
	file, err := wpcli.ParseDatabaseExport(result(load(t, "updates", "db-export")))
	if err != nil || file != ".vigie/backups/updates/before.sql" {
		t.Errorf("export = %q, %v", file, err)
	}
	// A host without mysqldump nor mariadb-dump.
	if _, err := wpcli.ParseDatabaseExport(result(load(t, "updates", "db-export-without-dump"))); !errors.Is(err, wpcli.ErrNoDumpTool) {
		t.Errorf("without a dump tool: %v, want ErrNoDumpTool", err)
	}
}

func TestParseDir(t *testing.T) {
	dir, err := wpcli.ParseDir([]byte(load(t, "updates", "theme-dir").Stdout))
	if err != nil || dir != "/home/vigie/sites/updates/wp-content/themes/twentytwentyfour" {
		t.Errorf("theme dir = %q, %v", dir, err)
	}
	for _, out := range []string{"", "relative/dir\n", "/a\n/b\n", "/home/../etc\n"} {
		if _, err := wpcli.ParseDir([]byte(out)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseDir(%q) = %v, want ErrUnexpectedOutput", out, err)
		}
	}
}

func TestParseDebugLog(t *testing.T) {
	debug, log := result(load(t, "updates", "constant-debug")), result(load(t, "updates", "constant-debug-log"))
	missing := result(load(t, "updates", "constant-missing"))
	text := func(s string) sshx.Result { return sshx.Result{Stdout: []byte(s + "\n")} }
	tests := []struct {
		name       string
		debug, log sshx.Result
		want       wpcli.DebugLog
	}{
		{"both true: wp-content/debug.log", debug, log, wpcli.DebugLog{Enabled: true}},
		{"a file of its own", debug, text(`"\/var\/log\/site.log"`), wpcli.DebugLog{Enabled: true, Path: "/var/log/site.log"}},
		{"WP_DEBUG off", text("false"), log, wpcli.DebugLog{}},
		{"WP_DEBUG_LOG not defined", debug, missing, wpcli.DebugLog{}},
		{"WP_DEBUG_LOG false", debug, text("false"), wpcli.DebugLog{}},
	}
	for _, tt := range tests {
		got, err := wpcli.ParseDebugLog(tt.debug, tt.log)
		if err != nil || got != tt.want {
			t.Errorf("%s = %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
	}
	if _, err := wpcli.ParseDebugLog(text("[1]"), log); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
		t.Errorf("WP_DEBUG as a list: %v, want ErrUnexpectedOutput", err)
	}
}

func FuzzUpdateParsers(f *testing.F) {
	for _, name := range []string{"preview-plugins", "preview-themes", "update-plugin", "update-plugin-missing", "theme-dir", "db-export", "constant-debug-log"} {
		r := load(f, "updates", name)
		f.Add([]byte(r.Stdout), []byte(r.Stderr), r.Exit)
	}
	f.Fuzz(func(t *testing.T, stdout, stderr []byte, exit int) {
		res := sshx.Result{Stdout: stdout, Stderr: stderr, ExitCode: exit}
		if pending, err := wpcli.ParseUpdatePreview(stdout); err == nil {
			for _, p := range pending {
				if p.Name == "" || p.UpdateVersion == "" {
					t.Fatalf("pending update %+v accepted", p)
				}
			}
		}
		if u, err := wpcli.ParseUpdateOne(res); err == nil {
			switch u.Outcome {
			case wpcli.UpdateDone:
				if exit != 0 {
					t.Fatalf("update done with exit %d", exit)
				}
			case wpcli.UpdateFailed, wpcli.UpdateNothing:
			default:
				t.Fatalf("outcome %q", u.Outcome)
			}
		}
		if file, err := wpcli.ParseDatabaseExport(res); err == nil && (exit != 0 || !strings.HasPrefix(file, ".vigie/backups/") || strings.Contains(file, "..")) {
			t.Fatalf("dump %q accepted with exit %d", file, exit)
		}
		if dir, err := wpcli.ParseDir(stdout); err == nil && (!strings.HasPrefix(dir, "/") || strings.Contains(dir, "/../")) {
			t.Fatalf("folder %q accepted", dir)
		}
		if active, err := wpcli.MaintenanceIsActive(res); err == nil && active != (exit == 0) {
			t.Fatalf("maintenance %v with exit %d", active, exit)
		}
		_, _ = wpcli.ParseDebugLog(res, res)
	})
}
