package wpcli_test

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
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
		{wpcli.Administrators(), "'wp' 'user' 'list' '--role=administrator' '--fields=ID,user_login,user_email,user_registered' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.SuperAdmins(), "'wp' 'super-admin' 'list' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
		// Only the methods turned on: never a TOTP key or backup codes.
		{wpcli.TwoFactorMeta(3), "'wp' 'user' 'meta' 'list' '3' '--keys=_two_factor_enabled_providers,wp_2fa_enabled_methods' '--format=json' '--unserialize' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.DatabasePrefix(), "'wp' 'db' 'prefix' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.WordfenceTwoFactorUsers("wp_"), "'wp' 'db' 'query' 'SELECT DISTINCT user_id FROM `wp_wfls_2fa_secrets`' '--skip-column-names' '--path=site' '--skip-plugins' '--skip-themes'"},
		// Names only: wp-config.php also holds the database password.
		{wpcli.ConfigNames(), "'wp' 'config' 'list' '--fields=name,type' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.OptionSizes(false), "'wp' 'option' 'list' '--fields=option_name,autoload,size_bytes' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.OptionSizes(true), "'wp' 'option' 'list' '--fields=option_name,autoload,size_bytes' '--format=json' '--transients' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.CronEvents(), "'wp' 'cron' 'event' 'list' '--fields=hook,next_run_gmt' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
		{wpcli.DatabaseVersion(), "'wp' 'db' 'query' 'SELECT VERSION()' '--skip-column-names' '--path=site' '--skip-plugins' '--skip-themes'"},
		// The keys and expiries only: the values are wordpress.org's answers.
		{wpcli.PHPChecks(), "'wp' 'transient' 'list' '--network' '--search=php_check_*' '--fields=name,expiration' '--format=json' '--path=site' '--skip-plugins' '--skip-themes'"},
	} {
		if got, err := tt.cmd.Remote("wp", "~/site"); err != nil || got != tt.want {
			t.Errorf("got %s, %v\nwant %s", got, err, tt.want)
		}
	}
}

// The recorded commands are the ones Vigie runs: a recording holds its command line.
func TestSecurityRecordingsAreCurrent(t *testing.T) {
	for name, rec := range map[string]struct {
		site string
		cmd  wpcli.Command
	}{
		"core-checksums":       {"security", wpcli.CoreVerifyChecksums()},
		"plugin-checksums":     {"security", wpcli.PluginVerifyChecksums()},
		"administrators":       {"security", wpcli.Administrators()},
		"super-admins":         {"security", wpcli.SuperAdmins()},
		"two-factor-3":         {"security", wpcli.TwoFactorMeta(3)},
		"db-prefix":            {"security", wpcli.DatabasePrefix()},
		"wordfence-two-factor": {"security", wpcli.WordfenceTwoFactorUsers("wp_")},
		"config-names":         {"security", wpcli.ConfigNames()},
		"option-sizes":         {"security", wpcli.OptionSizes(false)},
		"transient-sizes":      {"security", wpcli.OptionSizes(true)},
		"cron-events":          {"security", wpcli.CronEvents()},
		"db-version":           {"security", wpcli.DatabaseVersion()},
	} {
		want, err := rec.cmd.Remote("wp", "~/sites/"+rec.site)
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
	got, err = wpcli.ParseCoreChecksums(result(load(t, "plain", "core-checksums")))
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
		Total: 4, Verified: 1, Failed: 1, Skipped: []string{"fixture-premium", "wordfence"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParsePluginChecksums() = %+v\nwant %+v", got, want)
	}
	got, err = wpcli.ParsePluginChecksums(result(load(t, "plain", "plugin-checksums")))
	if err != nil || !reflect.DeepEqual(got, wpcli.PluginChecksums{Total: 2, Verified: 2}) {
		t.Fatalf("ParsePluginChecksums(clean) = %+v, %v", got, err)
	}
	// Success, with one plugin skipped: it is not verified, and Vigie says so.
	got, err = wpcli.ParsePluginChecksums(result(load(t, "plugin-theme", "plugin-checksums")))
	if err != nil || !reflect.DeepEqual(got, wpcli.PluginChecksums{Total: 3, Verified: 2, Skipped: []string{"fixture-themes"}}) {
		t.Fatalf("ParsePluginChecksums(unknown) = %+v, %v", got, err)
	}
}

func TestAccountCommandsRefuseBadArguments(t *testing.T) {
	for name, cmd := range map[string]wpcli.Command{
		"user 0":             wpcli.TwoFactorMeta(0),
		"prefix with a tick": wpcli.WordfenceTwoFactorUsers("wp_`; DROP"),
		"empty prefix":       wpcli.WordfenceTwoFactorUsers(""),
	} {
		if _, err := cmd.Remote("wp", "~/site"); !errors.Is(err, wpcli.ErrBadArgument) {
			t.Errorf("%s: %v, want ErrBadArgument", name, err)
		}
	}
}

func TestParseAdministrators(t *testing.T) {
	got, err := wpcli.ParseAdministrators([]byte(load(t, "security", "administrators").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	var logins []string
	for _, a := range got {
		logins = append(logins, a.Login)
	}
	// The editor is not listed.
	if !reflect.DeepEqual(logins, []string{"admin", "marie", "totp-admin", "wf-admin", "wp2fa-admin"}) {
		t.Fatalf("logins = %v", logins)
	}
	if got[1].ID != 2 || got[1].Email != "marie@example.test" || got[1].Registered.IsZero() || got[1].Registered.Location() != time.UTC {
		t.Errorf("marie = %+v", got[1])
	}
}

func TestParseSuperAdmins(t *testing.T) {
	got, err := wpcli.ParseSuperAdmins(result(load(t, "network", "super-admins")))
	if err != nil || !reflect.DeepEqual(got, []string{"admin"}) {
		t.Fatalf("ParseSuperAdmins() = %v, %v", got, err)
	}
	if _, err := wpcli.ParseSuperAdmins(result(load(t, "plain", "super-admins"))); !errors.Is(err, wpcli.ErrCannotLoad) {
		t.Errorf("ParseSuperAdmins() of a single site = %v, want ErrCannotLoad", err)
	}
}

func TestParseTwoFactorMeta(t *testing.T) {
	for _, tc := range []struct {
		name string
		want wpcli.TwoFactor
	}{
		{"two-factor-2", wpcli.TwoFactor{}},
		{"two-factor-3", wpcli.TwoFactor{Providers: []string{"Two_Factor_Totp", "Two_Factor_Backup_Codes"}}},
		{"two-factor-4", wpcli.TwoFactor{WP2FA: "totp"}},
		{"two-factor-5", wpcli.TwoFactor{}}, // Wordfence keeps no user meta
	} {
		got, err := wpcli.ParseTwoFactorMeta([]byte(load(t, "security", tc.name).Stdout))
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: ParseTwoFactorMeta() = %+v, %v; want %+v", tc.name, got, err, tc.want)
		}
	}
}

func TestParseWordfenceTwoFactorUsers(t *testing.T) {
	prefix, err := wpcli.ParseDatabasePrefix([]byte(load(t, "security", "db-prefix").Stdout))
	if err != nil || prefix != "wp_" {
		t.Fatalf("ParseDatabasePrefix() = %q, %v", prefix, err)
	}
	ids, err := wpcli.ParseUserIDs(result(load(t, "security", "wordfence-two-factor")))
	if err != nil || !reflect.DeepEqual(ids, []int{5}) {
		t.Fatalf("ParseUserIDs() = %v, %v", ids, err)
	}
	// No table: Wordfence is not there, and nobody uses its two-factor.
	ids, err = wpcli.ParseUserIDs(result(load(t, "plain", "wordfence-two-factor")))
	if err != nil || len(ids) != 0 {
		t.Fatalf("ParseUserIDs() without the table = %v, %v", ids, err)
	}
	// Without the mysql client, two-factor of Wordfence cannot be read.
	noClient := sshx.Result{ExitCode: 1, Stderr: []byte("/usr/bin/env: 'mysql': No such file or directory\n")}
	if _, err := wpcli.ParseUserIDs(noClient); !errors.Is(err, wpcli.ErrNotVerified) {
		t.Errorf("ParseUserIDs() without mysql = %v, want ErrNotVerified", err)
	}
}

func TestParseConfigNames(t *testing.T) {
	names, err := wpcli.ParseConfigNames([]byte(load(t, "security", "config-names").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(names, "WP_DEBUG") || !slices.Contains(names, "DB_PASSWORD") || slices.Contains(names, "table_prefix") {
		t.Errorf("constants = %v; want the constants' names, not the variables", names)
	}
}

func TestParseConstantAndOption(t *testing.T) {
	if v, ok, err := wpcli.ParseConstant(result(load(t, "security", "constant-wp-debug"))); err != nil || !ok || v != true {
		t.Errorf("ParseConstant(WP_DEBUG) = %v, %v, %v", v, ok, err)
	}
	if v, ok, err := wpcli.ParseConstant(result(load(t, "security", "constant-disallow-file-edit"))); err != nil || ok || v != nil {
		t.Errorf("ParseConstant(undefined) = %v, %v, %v", v, ok, err)
	}
	if v, ok, err := wpcli.ParseOption(result(load(t, "security", "option-blog-public"))); err != nil || !ok || v != "0" {
		t.Errorf("ParseOption(blog_public) = %q, %v, %v", v, ok, err)
	}
	if v, ok, err := wpcli.ParseOption(result(load(t, "security", "option-missing"))); err != nil || ok || v != "" {
		t.Errorf("ParseOption(missing) = %q, %v, %v; want not set", v, ok, err)
	}
}

func TestParseAutoloadedSize(t *testing.T) {
	size, err := wpcli.ParseAutoloadedSize(
		[]byte(load(t, "security", "option-sizes").Stdout), []byte(load(t, "security", "transient-sizes").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's 900,000-byte option and WordPress's own: above Site Health's 800,000.
	if size < 900_000 || size > 1_000_000 {
		t.Errorf("autoloaded size = %d", size)
	}
	// Only yes, on, auto-on and auto are loaded on every page (WordPress 6.6 and later).
	small := `[{"option_name":"a","autoload":"auto","size_bytes":"10"},{"option_name":"b","autoload":"auto-off","size_bytes":"20"},
		{"option_name":"c","autoload":"yes","size_bytes":"30"},{"option_name":"d","autoload":"no","size_bytes":"40"},
		{"option_name":"e","autoload":"auto-on","size_bytes":"50"},{"option_name":"f","autoload":"off","size_bytes":"60"}]`
	if size, err := wpcli.ParseAutoloadedSize([]byte(small), []byte("[]")); err != nil || size != 90 {
		t.Errorf("ParseAutoloadedSize(small) = %d, %v; want 90", size, err)
	}
}

func TestParseCronEvents(t *testing.T) {
	events, err := wpcli.ParseCronEvents([]byte(load(t, "security", "cron-events").Stdout))
	if err != nil || len(events) != 9 {
		t.Fatalf("ParseCronEvents() = %d events, %v", len(events), err)
	}
	if e := events[0]; e.Hook != "fixture_overdue" || !e.Next.Equal(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first event = %+v", e)
	}
}

func TestParseDatabaseVersion(t *testing.T) {
	if v, err := wpcli.ParseDatabaseVersion(result(load(t, "security", "db-version"))); err != nil || v != "11.8.9-MariaDB-ubu2404" {
		t.Errorf("ParseDatabaseVersion() = %q, %v", v, err)
	}
	noClient := sshx.Result{ExitCode: 1, Stderr: []byte("/usr/bin/env: 'mysql': No such file or directory\n")}
	if _, err := wpcli.ParseDatabaseVersion(noClient); !errors.Is(err, wpcli.ErrNotVerified) {
		t.Errorf("ParseDatabaseVersion() without mysql = %v, want ErrNotVerified", err)
	}
}

func TestParsePHPChecks(t *testing.T) {
	out := `[{"name":"php_check_1f3870be274f6c49b3e31a0c6728957f","expiration":"1793600000"},{"name":"php_check_abc","expiration":"false"}]`
	got, cached, err := wpcli.ParsePHPChecks(sshx.Result{Stdout: []byte(out)})
	want := []wpcli.PHPCheck{
		{Hash: "1f3870be274f6c49b3e31a0c6728957f", Expires: time.Unix(1793600000, 0).UTC()},
		{Hash: "abc"},
	}
	if err != nil || cached || !reflect.DeepEqual(got, want) {
		t.Errorf("ParsePHPChecks() = %+v, %v, %v\nwant %+v", got, cached, err, want)
	}

	// With an external object cache, WP-CLI lists the database only, and says so.
	warned := sshx.Result{Stdout: []byte("[]"), Stderr: []byte("Warning: Transients are stored in an external object cache, and this command only shows those stored in the database.\n")}
	if got, cached, err := wpcli.ParsePHPChecks(warned); err != nil || !cached || len(got) != 0 {
		t.Errorf("ParsePHPChecks() with an object cache = %+v, %v, %v", got, cached, err)
	}
	if _, _, err := wpcli.ParsePHPChecks(sshx.Result{ExitCode: 1, Stderr: []byte("Error: boom")}); err == nil {
		t.Error("ParsePHPChecks() of a failed command: no error")
	}
	if _, _, err := wpcli.ParsePHPChecks(sshx.Result{Stdout: []byte(`[{"name":"other","expiration":"12"}]`)}); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
		t.Errorf("ParsePHPChecks() of another transient = %v, want ErrUnexpectedOutput", err)
	}
}

func hashOf(version string) string {
	sum := md5.Sum([]byte(version))
	return hex.EncodeToString(sum[:])
}

func TestWebPHP(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour
	check := func(version string, noted time.Time) wpcli.PHPCheck {
		return wpcli.PHPCheck{Hash: hashOf(version), Expires: noted.Add(week)}
	}
	for _, tt := range []struct {
		name   string
		checks []wpcli.PHPCheck
		cached bool
		want   model.PHPWeb
	}{
		{"the command line's", []wpcli.PHPCheck{check("8.4.27", now.Add(-time.Hour))}, false,
			model.PHPWeb{Version: "8.4.27", Seen: now.Add(-time.Hour)}},
		{"another release", []wpcli.PHPCheck{check("8.2.29", now.Add(-48*time.Hour))}, false,
			model.PHPWeb{Version: "8.2.29", Seen: now.Add(-48 * time.Hour)}},
		{"a distribution's build of the command line", []wpcli.PHPCheck{check("8.1.2-1ubuntu2.20", now.Add(-time.Hour))}, false,
			model.PHPWeb{Unknown: model.PHPWebUnmatched}},
		{"the newest of several", []wpcli.PHPCheck{check("8.1.31", now.Add(-6*24*time.Hour)), check("8.3.14", now.Add(-time.Hour))}, false,
			model.PHPWeb{Version: "8.3.14", Seen: now.Add(-time.Hour)}},
		{"an expired one left", []wpcli.PHPCheck{check("7.4.33", now.Add(-8*24*time.Hour))}, false,
			model.PHPWeb{Unknown: model.PHPWebNotNoted}},
		{"none", nil, false, model.PHPWeb{Unknown: model.PHPWebNotNoted}},
		{"in an object cache", nil, true, model.PHPWeb{Unknown: model.PHPWebObjectCache}},
		{"one kept forever", []wpcli.PHPCheck{{Hash: hashOf("8.4.0")}}, false, model.PHPWeb{Version: "8.4.0"}},
	} {
		cli := "8.4.27"
		if strings.Contains(tt.name, "distribution") {
			cli = "8.1.2-1ubuntu2.19"
		}
		if got := wpcli.WebPHP(tt.checks, tt.cached, cli, now); got != tt.want {
			t.Errorf("%s: WebPHP() = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}
