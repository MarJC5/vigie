package wpcli_test

import (
	"errors"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/wpcli"
)

// The script that tells where the parts of a site are only reads: WordPress's constants, and these
// functions (ADR-0041). wp_upload_dir is asked not to create the folder.
var copyReading = []string{"wp_upload_dir", "untrailingslashit", "get_theme_root", "wp_json_encode", "array"}

func TestTheCopyPathsScriptOnlyReads(t *testing.T) {
	script := string(wpcli.CopyPathsScript())
	if !strings.HasPrefix(script, "<?php\n") {
		t.Fatal("the script must start with <?php")
	}
	code := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`).ReplaceAllString(script, "")
	for _, m := range regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\(`).FindAllStringSubmatch(code, -1) {
		if !slices.Contains(copyReading, m[1]) {
			t.Errorf("the script calls %s, not a reading function", m[1])
		}
	}
	if !strings.Contains(code, "wp_upload_dir( null, false )") {
		t.Error("wp_upload_dir must be asked not to create the uploads folder")
	}
}

func TestCopyCommandLines(t *testing.T) {
	tests := []struct {
		cmd  wpcli.Command
		want string
	}{
		{wpcli.CopyPaths(), "'wp' 'eval-file' '-' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.DatabaseSize(), "'wp' 'db' 'size' '--size_format=b' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.DatabaseTables(), "'wp' 'db' 'size' '--tables' '--all-tables' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.DatabaseDump(nil), "'wp' 'db' 'export' '-' '--single-transaction' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.DatabaseDump([]string{"wp_posts", "wp_postmeta"}), "'wp' 'db' 'export' '-' '--single-transaction' '--tables=wp_posts,wp_postmeta' '--path=site' '--skip-plugins' '--skip-themes'"},
	}
	for _, tt := range tests {
		got, err := tt.cmd.Remote("wp", "~/site")
		if err != nil || got != tt.want {
			t.Errorf("got %s, %v\nwant %s", got, err, tt.want)
		}
	}
	for _, name := range []string{"", "wp posts", "a,b", "x';rm"} {
		if _, err := wpcli.DatabaseDump([]string{name}).Remote("wp", "~/site"); !errors.Is(err, wpcli.ErrBadArgument) {
			t.Errorf("DatabaseDump(%q): %v, want ErrBadArgument", name, err)
		}
	}
}

func TestParseCopyPaths(t *testing.T) {
	out := `{"root":"/home/u/www/site","content":"/home/u/www/site/wp-content","plugins":"/home/u/www/site/wp-content/plugins",
		"muPlugins":"/home/u/www/site/wp-content/mu-plugins","themes":"/home/u/www/site/wp-content/themes",
		"uploads":"/home/u/www/site/wp-content/uploads","prefix":"wp_"}`
	got, err := wpcli.ParseCopyPaths([]byte(out))
	want := wpcli.SitePaths{Root: "/home/u/www/site", Content: "/home/u/www/site/wp-content",
		Plugins: "/home/u/www/site/wp-content/plugins", MuPlugins: "/home/u/www/site/wp-content/mu-plugins",
		Themes: "/home/u/www/site/wp-content/themes", Uploads: "/home/u/www/site/wp-content/uploads", Prefix: "wp_"}
	if err != nil || got != want {
		t.Fatalf("ParseCopyPaths() = %+v, %v", got, err)
	}
	// A folder that is not an absolute, clean path is refused, as is an answer that is not the script's.
	for _, bad := range []string{
		strings.Replace(out, `"/home/u/www/site/wp-content/uploads"`, `"wp-content/uploads"`, 1),
		strings.Replace(out, `"/home/u/www/site/wp-content/uploads"`, `"/home/u/www/site/../x"`, 1),
		strings.Replace(out, `"root":"/home/u/www/site"`, `"root":""`, 1),
		"Warning: something\n",
	} {
		if _, err := wpcli.ParseCopyPaths([]byte(bad)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseCopyPaths(%q): %v, want ErrUnexpectedOutput", bad, err)
		}
	}
}

func TestParseDatabaseSizes(t *testing.T) {
	if n, err := wpcli.ParseDatabaseSize([]byte("5865472\n")); err != nil || n != 5865472 {
		t.Fatalf("ParseDatabaseSize() = %d, %v", n, err)
	}
	// As WP-CLI 2.12.0 prints it without a size format: the bytes, then " B".
	tables, err := wpcli.ParseDatabaseTables([]byte(`[{"Name":"wp_posts","Size":"5242880 B"},{"Name":"wp_options","Size":"1081344 B"}]`))
	want := []wpcli.Table{{Name: "wp_posts", Bytes: 5242880}, {Name: "wp_options", Bytes: 1081344}}
	if err != nil || !reflect.DeepEqual(tables, want) {
		t.Fatalf("ParseDatabaseTables() = %+v, %v", tables, err)
	}
	for _, bad := range []string{"abc", `[{"Name":"wp_posts","Size":"5 MB"}]`, `[{"Name":"","Size":"5 B"}]`} {
		if _, err := wpcli.ParseDatabaseTables([]byte(bad)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseDatabaseTables(%q): %v, want ErrUnexpectedOutput", bad, err)
		}
	}
}
