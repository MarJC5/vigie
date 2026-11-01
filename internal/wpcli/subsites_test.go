package wpcli_test

import (
	"errors"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// reading are the only functions the subsites script may call: WordPress's reading functions, the
// methods of WP_Theme that read its header, and PHP's own array functions (ADR-0029). A function
// that writes, runs a command or reads a file has no place in it.
var reading = []string{
	"get_site_option", "get_super_admins", "get_sites", "switch_to_blog", "restore_current_blog",
	"wp_get_theme", "get_stylesheet", "get", "get_users", "get_user_by", "get_user_meta", "get_option", "wp_json_encode",
	"array", "array_keys", "array_values",
	"vigie_account", // the script's own, which reads an account with the functions above
}

func TestTheSubsitesScriptOnlyReads(t *testing.T) {
	script := string(wpcli.SubsitesScript())
	if !strings.HasPrefix(script, "<?php\n") {
		t.Fatal("the script must start with <?php")
	}
	// Comments aside, every name followed by "(" is a call.
	code := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`).ReplaceAllString(script, "")
	called := map[string]bool{}
	for _, m := range regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\(`).FindAllStringSubmatch(code, -1) {
		called[m[1]] = true
	}
	for name := range called {
		if !slices.Contains(reading, name) && name != "foreach" && name != "if" && name != "function" {
			t.Errorf("the script calls %s, not a reading function", name)
		}
	}
	if !called["get_sites"] || !strings.Contains(code, "'number'   => 0") {
		t.Error("the subsites must be read without get_sites' default limit of 100")
	}
}

func TestSubsitesCommandLine(t *testing.T) {
	got, err := wpcli.Subsites().Remote("wp", "~/site")
	if want := "'wp' 'eval-file' '-' '--path=site' '--skip-plugins' '--skip-themes'"; err != nil || got != want {
		t.Errorf("got %s, %v\nwant %s", got, err, want)
	}
}

func TestParseSubsites(t *testing.T) {
	out := `{"plugins":["woocommerce/woocommerce.php"],"supers":[{"id":1,"login":"admin","email":"admin@reseau.example.test",
		 "registered":"2021-02-04 08:00:00","providers":["Two_Factor_Totp"],"wp2fa":""},{"id":4,"login":"jdupont",
		 "email":"jdupont@reseau.example.test","registered":"2021-02-04 09:00:00","providers":"","wp2fa":""}],"sites":[
		{"id":1,"plugins":["akismet/akismet.php"],"theme":{"slug":"twentytwentyfive","name":"Twenty Twenty-Five","version":"1.2"},
		 "email":"admin@reseau.example.test","admins":[{"id":1,"login":"admin","email":"admin@reseau.example.test",
		 "registered":"2021-02-04 08:00:00","providers":["Two_Factor_Totp"],"wp2fa":""}]},
		{"id":2,"plugins":["contact-form-7/wp-contact-form-7.php","hello.php"],"theme":{"slug":"astra","name":"Astra","version":"4.8.1"},
		 "email":"direction@alpha.example.test","admins":[{"id":7,"login":"marie","email":"marie@alpha.example.test",
		 "registered":"2024-09-12 10:00:00","providers":"","wp2fa":"totp"},{"id":8,"login":"paul","email":"paul@alpha.example.test",
		 "registered":"0000-00-00 00:00:00","providers":"Two_Factor_Email","wp2fa":""}]}]}`
	got, err := wpcli.ParseSubsites(sshx.Result{Stdout: []byte(out)})
	if err != nil {
		t.Fatal(err)
	}
	want := wpcli.Network{
		Plugins: []string{"woocommerce"},
		Supers: []wpcli.SubsiteAdmin{
			{Account: wpcli.Account{ID: 1, Login: "admin", Email: "admin@reseau.example.test", Registered: time.Date(2021, 2, 4, 8, 0, 0, 0, time.UTC)},
				TwoFactor: wpcli.TwoFactor{Providers: []string{"Two_Factor_Totp"}}},
			{Account: wpcli.Account{ID: 4, Login: "jdupont", Email: "jdupont@reseau.example.test", Registered: time.Date(2021, 2, 4, 9, 0, 0, 0, time.UTC)}},
		},
		Sites: []wpcli.Subsite{
			{ID: 1, Plugins: []string{"akismet"}, Theme: "twentytwentyfive", ThemeName: "Twenty Twenty-Five", ThemeVersion: "1.2",
				Email: "admin@reseau.example.test", Admins: []wpcli.SubsiteAdmin{
					{Account: wpcli.Account{ID: 1, Login: "admin", Email: "admin@reseau.example.test", Registered: time.Date(2021, 2, 4, 8, 0, 0, 0, time.UTC)},
						TwoFactor: wpcli.TwoFactor{Providers: []string{"Two_Factor_Totp"}}}}},
			{ID: 2, Plugins: []string{"contact-form-7", "hello"}, Theme: "astra", ThemeName: "Astra", ThemeVersion: "4.8.1",
				Email: "direction@alpha.example.test", Admins: []wpcli.SubsiteAdmin{
					{Account: wpcli.Account{ID: 7, Login: "marie", Email: "marie@alpha.example.test", Registered: time.Date(2024, 9, 12, 10, 0, 0, 0, time.UTC)},
						TwoFactor: wpcli.TwoFactor{WP2FA: "totp"}},
					{Account: wpcli.Account{ID: 8, Login: "paul", Email: "paul@alpha.example.test"},
						TwoFactor: wpcli.TwoFactor{Providers: []string{"Two_Factor_Email"}}}}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseSubsites() =\n%+v\nwant\n%+v", got, want)
	}

	if _, err := wpcli.ParseSubsites(sshx.Result{ExitCode: 1, Stderr: []byte("Error: This is not a multisite installation.")}); !errors.Is(err, wpcli.ErrCannotLoad) {
		t.Errorf("ParseSubsites() of a failed command = %v, want ErrCannotLoad", err)
	}
	for _, bad := range []string{`[]`, `{"sites":[{"id":0}]}`, `{"sites":[{"id":2,"admins":[{"id":3,"login":"x","registered":"yesterday"}]}]}`} {
		if _, err := wpcli.ParseSubsites(sshx.Result{Stdout: []byte(bad)}); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseSubsites(%s) = %v, want ErrUnexpectedOutput", bad, err)
		}
	}
}
