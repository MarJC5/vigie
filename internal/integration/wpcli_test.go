//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// live runs a command as Vigie builds it, through the account's shell as ssh would.
func live(t *testing.T, cmd wpcli.Command, root string) sshx.Result {
	t.Helper()
	remote, err := cmd.Remote("wp", root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := runAs(context.Background(), "sh", "-c", remote)
	if err != nil {
		t.Fatal(err)
	}
	return sshx.Result{Stdout: []byte(res.stdout), Stderr: []byte(res.stderr), ExitCode: res.exit}
}

// The parsers read today's WP-CLI output, not only the recordings.
func TestParsersReadLiveOutputs(t *testing.T) {
	outdated := sites + "/outdated"
	parse := map[string]struct {
		cmd   wpcli.Command
		parse func([]byte) error
	}{
		"core version": {wpcli.CoreVersion(), func(b []byte) error { _, err := wpcli.ParseCoreVersion(b); return err }},
		"home":         {wpcli.OptionGet("home"), func(b []byte) error { _, err := wpcli.ParseHome(b); return err }},
		"core updates": {wpcli.CoreCheckUpdate(), func(b []byte) error { _, err := wpcli.ParseCoreUpdates(b); return err }},
		"plugins":      {wpcli.PluginList(), func(b []byte) error { _, err := wpcli.ParsePlugins(b); return err }},
		"themes":       {wpcli.ThemeList(), func(b []byte) error { _, err := wpcli.ParseThemes(b); return err }},
		"core languages": {wpcli.LanguageUpdates(wpcli.LanguagesOfCore),
			func(b []byte) error { _, err := wpcli.ParseLanguageUpdates(b); return err }},
		"plugin languages": {wpcli.LanguageUpdates(wpcli.LanguagesOfPlugins),
			func(b []byte) error { _, err := wpcli.ParseLanguageUpdates(b); return err }},
	}
	for name, p := range parse {
		res := live(t, p.cmd, outdated)
		if err := wpcli.Check(res); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := p.parse(res.Stdout); err != nil {
			t.Errorf("%s: %v\n%s", name, err, res.Stdout)
		}
	}
	res := live(t, wpcli.SiteList(), sites+"/network")
	if subsites, err := wpcli.ParseSites(res.Stdout); err != nil || len(subsites) != 4 {
		t.Errorf("site list: %d subsites, %v", len(subsites), err)
	}
	if ok, err := wpcli.IsInstalled(live(t, wpcli.CoreIsInstalled(false), sites+"/broken")); ok || err == nil {
		t.Errorf("broken site: IsInstalled() = %v, %v; want ErrCannotLoad", ok, err)
	}
}
