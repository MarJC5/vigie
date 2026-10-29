package wpcli_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/wpcli"
)

func result(r recorded) sshx.Result {
	return sshx.Result{Stdout: []byte(r.Stdout), Stderr: []byte(r.Stderr), ExitCode: r.Exit}
}

func TestReadCommandsAsRecorded(t *testing.T) {
	// The bench recorded these exact argument lists (internal/integration/record_test.go).
	for name, cmd := range map[string]wpcli.Command{
		"core-version":      wpcli.CoreVersion(),
		"core-check-update": wpcli.CoreCheckUpdate(),
		"plugin-list":       wpcli.PluginList(),
		"theme-list":        wpcli.ThemeList(),
		"language-core":     wpcli.LanguageUpdates(wpcli.LanguagesOfCore),
		"language-plugin":   wpcli.LanguageUpdates(wpcli.LanguagesOfPlugins),
		"language-theme":    wpcli.LanguageUpdates(wpcli.LanguagesOfThemes),
		"site-list":         wpcli.SiteList(),
	} {
		want, err := sshx.Quote(append(append([]string{"wp"}, load(t, "plain", name).Args...), "--path=site")...)
		if err != nil {
			t.Fatal(err)
		}
		got, err := cmd.Remote("wp", "site")
		if err != nil {
			t.Fatal(err)
		}
		// The recordings put --path last; Remote puts it before the read flags.
		if !sameWords(got, want) {
			t.Errorf("%s:\n got %s\nwant %s", name, got, want)
		}
	}
}

// sameWords compares two quoted command lines as sets of words, in any order after the command.
func sameWords(a, b string) bool {
	count := map[string]int{}
	for _, w := range splitQuoted(a) {
		count[w]++
	}
	for _, w := range splitQuoted(b) {
		count[w]--
	}
	for _, n := range count {
		if n != 0 {
			return false
		}
	}
	return true
}

func splitQuoted(s string) []string {
	var words []string
	inside, word := false, []rune{}
	for _, r := range s {
		switch {
		case r == '\'':
			inside = !inside
			if !inside {
				words = append(words, string(word))
				word = word[:0]
			}
		case inside:
			word = append(word, r)
		}
	}
	return words
}

func TestIsInstalled(t *testing.T) {
	for _, tc := range []struct {
		fixture, name string
		want          bool
		err           error
	}{
		{"plain", "core-is-installed", true, nil},
		{"plain", "core-is-installed-network", false, nil},
		{"network", "core-is-installed-network", true, nil},
		{"broken", "core-is-installed", false, wpcli.ErrCannotLoad},
	} {
		got, err := wpcli.IsInstalled(result(load(t, tc.fixture, tc.name)))
		if got != tc.want || !errors.Is(err, tc.err) {
			t.Errorf("%s/%s: IsInstalled() = %v, %v; want %v, %v", tc.fixture, tc.name, got, err, tc.want, tc.err)
		}
	}
}

func TestCommandErrorCarriesWPCLIsMessage(t *testing.T) {
	err := wpcli.Check(result(load(t, "broken", "plugin-list")))
	if !errors.Is(err, wpcli.ErrCannotLoad) || err.Error() == "" {
		t.Fatalf("Check() = %v, want ErrCannotLoad with the message", err)
	}
	if err := wpcli.Check(result(load(t, "plain", "plugin-list"))); err != nil {
		t.Fatalf("Check() on a successful command = %v", err)
	}
}

func TestParseCoreVersionAndHome(t *testing.T) {
	version, err := wpcli.ParseCoreVersion([]byte(load(t, "plain", "core-version").Stdout))
	if err != nil || version != "7.1.3" {
		t.Fatalf("ParseCoreVersion() = %q, %v", version, err)
	}
	home, err := wpcli.ParseHome([]byte(load(t, "parent-config", "option-home").Stdout))
	if err != nil || home != "https://parent.example.test" {
		t.Fatalf("ParseHome() = %q, %v", home, err)
	}
	for _, out := range []string{"", "Error: x", "7.1.3; rm -rf /", "<b>7</b>"} {
		if _, err := wpcli.ParseCoreVersion([]byte(out)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseCoreVersion(%q) error = %v", out, err)
		}
	}
	for _, out := range []string{"", "javascript:alert(1)", "not a url"} {
		if _, err := wpcli.ParseHome([]byte(out)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseHome(%q) error = %v", out, err)
		}
	}
}

func TestParseCoreUpdates(t *testing.T) {
	updates, err := wpcli.ParseCoreUpdates([]byte(load(t, "outdated", "core-check-update").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	if want := []wpcli.CoreUpdate{{Version: "7.1.4", Type: "minor"}}; !reflect.DeepEqual(updates, want) {
		t.Fatalf("ParseCoreUpdates() = %+v, want %+v", updates, want)
	}
	if updates, err := wpcli.ParseCoreUpdates([]byte("[]\n")); err != nil || len(updates) != 0 {
		t.Fatalf("ParseCoreUpdates([]) = %+v, %v", updates, err)
	}
}

func TestParsePlugins(t *testing.T) {
	plugins, err := wpcli.ParsePlugins([]byte(load(t, "outdated", "plugin-list").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]wpcli.Extension{}
	for _, p := range plugins {
		byName[p.Name] = p
	}
	akismet, premium := byName["akismet"], byName["fixture-premium"]
	if akismet.Update != wpcli.UpdateAvailable || akismet.Version != "5.3" || akismet.UpdateVersion == "" || !akismet.OnWordPressOrg {
		t.Errorf("akismet = %+v, want an update from wordpress.org", akismet)
	}
	if premium.Update != wpcli.UpdateNone || premium.OnWordPressOrg || premium.Status != "active" || premium.Title != "Fixture Premium" {
		t.Errorf("fixture-premium = %+v, want an active plugin not on wordpress.org", premium)
	}
}

func TestParseThemes(t *testing.T) {
	themes, err := wpcli.ParseThemes([]byte(load(t, "outdated", "theme-list").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	pending := 0
	for _, th := range themes {
		if th.Update == wpcli.UpdateAvailable {
			pending++
			if th.Name != "twentytwentyfour" || th.Version != "1.0" {
				t.Errorf("pending theme = %+v", th)
			}
		}
	}
	if len(themes) != 3 || pending != 1 {
		t.Fatalf("ParseThemes() = %+v, want three themes, one with an update", themes)
	}
}

func TestUpdateStates(t *testing.T) {
	out := `[{"name":"a","status":"active","version":"1","update":"unavailable","update_version":"2","auto_update":"on"},` +
		`{"name":"b","status":"inactive","version":"9","update":"version higher than expected","update_version":"","auto_update":"off"},` +
		`{"name":"c","status":"inactive","version":"1","update":"something new","update_version":"","auto_update":"off"}]`
	plugins, err := wpcli.ParsePlugins([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	want := []wpcli.UpdateState{wpcli.UpdateUnavailable, wpcli.UpdateNewerThanSource, wpcli.UpdateUnknown}
	for i, p := range plugins {
		if p.Update != want[i] {
			t.Errorf("%s: update = %q, want %q", p.Name, p.Update, want[i])
		}
	}
	if !plugins[0].AutoUpdate || plugins[1].AutoUpdate {
		t.Errorf("auto updates = %v, %v; want on, off", plugins[0].AutoUpdate, plugins[1].AutoUpdate)
	}
}

func TestParseLanguageUpdates(t *testing.T) {
	core, err := wpcli.ParseLanguageUpdates([]byte(load(t, "outdated", "language-core").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := wpcli.ParseLanguageUpdates([]byte(load(t, "outdated", "language-plugin").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(core, []wpcli.LanguageUpdate{{Language: "fr_FR"}}) ||
		!reflect.DeepEqual(plugins, []wpcli.LanguageUpdate{{Item: "akismet", Language: "fr_FR"}}) {
		t.Fatalf("ParseLanguageUpdates() = %+v and %+v", core, plugins)
	}
}

func TestParseSites(t *testing.T) {
	sites, err := wpcli.ParseSites([]byte(load(t, "network", "site-list").Stdout))
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 4 {
		t.Fatalf("ParseSites() = %d sites, want 4", len(sites))
	}
	want := model.Subsite{BlogID: 4, URL: "https://network.example.test/gamma/", Public: true, Archived: true}
	if sites[3] != want {
		t.Fatalf("subsite 4 = %+v, want %+v", sites[3], want)
	}
}

func TestParsersRejectOtherOutput(t *testing.T) {
	for _, out := range []string{"", "Error: x", "{}", `[{"name":""}]`, `[{"blog_id":"x"}]`} {
		if _, err := wpcli.ParsePlugins([]byte(out)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParsePlugins(%q) error = %v", out, err)
		}
		if _, err := wpcli.ParseSites([]byte(out)); !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseSites(%q) error = %v", out, err)
		}
		if _, err := wpcli.ParseCoreUpdates([]byte(out)); out != "{}" && !errors.Is(err, wpcli.ErrUnexpectedOutput) {
			t.Errorf("ParseCoreUpdates(%q) error = %v", out, err)
		}
	}
}

// Every parser, on any input: an error or a result that keeps its own rules, never a panic.
func FuzzParsers(f *testing.F) {
	for _, fixture := range []string{"plain", "network", "outdated"} {
		for _, name := range []string{"core-version", "core-check-update", "option-home", "plugin-list", "theme-list", "language-core", "language-plugin", "site-list"} {
			f.Add([]byte(load(f, fixture, name).Stdout))
		}
	}
	f.Fuzz(func(t *testing.T, out []byte) {
		if v, err := wpcli.ParseCoreVersion(out); err == nil && v == "" {
			t.Fatal("empty version accepted")
		}
		if h, err := wpcli.ParseHome(out); err == nil && h == "" {
			t.Fatal("empty home accepted")
		}
		if ups, err := wpcli.ParseCoreUpdates(out); err == nil {
			for _, u := range ups {
				if u.Version == "" {
					t.Fatal("core update without version")
				}
			}
		}
		for _, parse := range []func([]byte) ([]wpcli.Extension, error){wpcli.ParsePlugins, wpcli.ParseThemes} {
			if list, err := parse(out); err == nil {
				for _, e := range list {
					if e.Name == "" {
						t.Fatal("extension without name")
					}
				}
			}
		}
		if langs, err := wpcli.ParseLanguageUpdates(out); err == nil {
			for _, l := range langs {
				if l.Language == "" {
					t.Fatal("language update without language")
				}
			}
		}
		if sites, err := wpcli.ParseSites(out); err == nil {
			for _, s := range sites {
				if s.BlogID <= 0 || s.URL == "" {
					t.Fatalf("subsite %+v accepted", s)
				}
			}
		}
	})
}
