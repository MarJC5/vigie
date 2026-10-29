package wpcli_test

import (
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/wpcli"
)

func TestRemoteCommands(t *testing.T) {
	tests := []struct {
		name  string
		cmd   wpcli.Command
		wpcli string
		path  string
		want  string
	}{
		{
			"core version, Infomaniak command, path under home",
			wpcli.CoreVersion(), "wp-cli", "~/sites/misits.ch",
			"'wp-cli' 'core' 'version' '--path=sites/misits.ch' '--skip-plugins' '--skip-themes'",
		},
		{
			"is-installed on a network, absolute path, phar command",
			wpcli.CoreIsInstalled(true), "php /home/u/wp-cli.phar", "/var/www/site",
			"'php' '/home/u/wp-cli.phar' 'core' 'is-installed' '--network' '--path=/var/www/site' '--skip-plugins' '--skip-themes'",
		},
		{
			"command in the home directory",
			wpcli.CoreVersion(), "php ~/wp-cli.phar", "~/site",
			"'php' 'wp-cli.phar' 'core' 'version' '--path=site' '--skip-plugins' '--skip-themes'",
		},
		{
			"option get, relative path",
			wpcli.OptionGet("home"), "wp", "sites/a b",
			"'wp' 'option' 'get' 'home' '--path=sites/a b' '--skip-plugins' '--skip-themes'",
		},
		{
			"theme list with the plugins, which can hold themes",
			wpcli.ThemeList().WithPlugins(), "wp", "~/site",
			"'wp' 'theme' 'list' '--format=json' '--fields=name,title,status,version,update,update_version,auto_update' '--path=site' '--skip-themes'",
		},
	}
	for _, tt := range tests {
		got, err := tt.cmd.Remote(tt.wpcli, tt.path)
		if err != nil || got != tt.want {
			t.Errorf("%s:\n got %s, %v\nwant %s", tt.name, got, err, tt.want)
		}
	}
}

func TestRemoteRejectsBadInput(t *testing.T) {
	if _, err := wpcli.CoreVersion().Remote("", "site"); !errors.Is(err, wpcli.ErrNoCommand) {
		t.Errorf("empty WP-CLI command: %v, want ErrNoCommand", err)
	}
	for _, path := range []string{"", "~other/site", "~"} {
		if _, err := wpcli.CoreVersion().Remote("wp", path); !errors.Is(err, wpcli.ErrBadPath) {
			t.Errorf("path %q: %v, want ErrBadPath", path, err)
		}
	}
}

func TestOptionNamesAreChecked(t *testing.T) {
	for _, name := range []string{"home", "admin_email", "blog-public"} {
		if _, err := wpcli.OptionGet(name).Remote("wp", "site"); err != nil {
			t.Errorf("OptionGet(%q): %v", name, err)
		}
	}
	for _, name := range []string{"", "--url=x", "home; id", "a b"} {
		if _, err := wpcli.OptionGet(name).Remote("wp", "site"); !errors.Is(err, wpcli.ErrBadArgument) {
			t.Errorf("OptionGet(%q): %v, want ErrBadArgument", name, err)
		}
	}
}
