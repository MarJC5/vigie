package wpcli_test

import (
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/wpcli"
)

func TestCLIInfoNeedsNoWordPress(t *testing.T) {
	got, err := wpcli.CLIInfo().Remote("php ~/wp-cli.phar", "")
	want := "'php' 'wp-cli.phar' 'cli' 'info' '--format=json'"
	if err != nil || got != want {
		t.Fatalf("Remote() = %s, %v; want %s", got, err, want)
	}
	if _, err := wpcli.CLIInfo().Remote("wp", "site"); !errors.Is(err, wpcli.ErrBadPath) {
		t.Fatalf("cli info with a path: %v, want ErrBadPath", err)
	}
}

func TestParseCLIInfo(t *testing.T) {
	info, err := wpcli.ParseCLIInfo([]byte(load(t, "plain", "cli-info").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	if info.WPCLIVersion != "2.12.0" || info.PHPVersion == "" {
		t.Fatalf("ParseCLIInfo() = %+v, want WP-CLI 2.12.0 and a PHP version", info)
	}
}

func TestParseCLIInfoRejectsOtherOutput(t *testing.T) {
	for _, out := range []string{"", "Error: 'info' is not a registered wp command.", `{"php_version":"8.4.27"}`, "[]"} {
		if _, err := wpcli.ParseCLIInfo([]byte(out)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseCLIInfo(%q) error = %v, want ErrUnexpectedOutput", out, err)
		}
	}
}

func FuzzParseCLIInfo(f *testing.F) {
	f.Add([]byte(load(f, "plain", "cli-info").Stdout))
	f.Add([]byte(`{"php_version":1,"wp_cli_version":null}`))
	f.Fuzz(func(t *testing.T, out []byte) {
		info, err := wpcli.ParseCLIInfo(out)
		if err == nil && (info.PHPVersion == "" || info.WPCLIVersion == "") {
			t.Fatalf("accepted an incomplete answer: %+v", info)
		}
	})
}
