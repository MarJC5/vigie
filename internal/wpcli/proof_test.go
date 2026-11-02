package wpcli_test

import (
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/wpcli"
)

func TestUpdateDataCommandsAsRecorded(t *testing.T) {
	for name, cmd := range map[string]wpcli.Command{
		"update-core-data":    wpcli.UpdateData(wpcli.UpdatesOfCore),
		"update-plugins-data": wpcli.UpdateData(wpcli.UpdatesOfPlugins),
		"update-themes-data":  wpcli.UpdateData(wpcli.UpdatesOfThemes),
	} {
		got, err := cmd.Remote("wp", "site")
		if err != nil {
			t.Fatal(err)
		}
		want := "'wp' " + quoteAll(load(t, "plain", name).Args) + " '--path=site'"
		if !sameWords(got, want) {
			t.Errorf("%s:\n got %s\nwant %s", name, got, want)
		}
	}
}

func quoteAll(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += "'" + a + "'"
	}
	return s
}

// A check is proven by what only a successful answer from wordpress.org writes.
func TestUpdateChecksAreProvenOnlyBySuccess(t *testing.T) {
	core := func(fixture string) []byte { return []byte(load(t, fixture, "update-core-data").Stdout) }
	plugins := func(fixture string) []byte { return []byte(load(t, fixture, "update-plugins-data").Stdout) }
	themes := func(fixture string) []byte { return []byte(load(t, fixture, "update-themes-data").Stdout) }

	for _, tc := range []struct {
		name string
		got  bool
		want bool
	}{
		{"core, wordpress.org answered", wpcli.CoreCheckProven(core("outdated"), "7.1.3"), true},
		{"core, network site", wpcli.CoreCheckProven(core("network"), "7.1.3"), true},
		{"core, no outbound HTTP", wpcli.CoreCheckProven(core("offline"), "7.1.3"), false},
		{"core, answer for another version", wpcli.CoreCheckProven(core("outdated"), "7.0"), false},
		{"core, nothing stored", wpcli.CoreCheckProven(nil, "7.1.3"), false},
		{"plugins, wordpress.org answered", wpcli.ExtensionsCheckProven(plugins("outdated")), true},
		{"plugins, no outbound HTTP", wpcli.ExtensionsCheckProven(plugins("offline")), false},
		{"themes, wordpress.org answered", wpcli.ExtensionsCheckProven(themes("network")), true},
		{"themes, no outbound HTTP", wpcli.ExtensionsCheckProven(themes("offline")), false},
		{"themes, not JSON", wpcli.ExtensionsCheckProven([]byte("Warning: x")), false},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: proven = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func FuzzUpdateChecks(f *testing.F) {
	for _, fixture := range []string{"outdated", "offline"} {
		f.Add([]byte(load(f, fixture, "update-core-data").Stdout))
		f.Add([]byte(load(f, fixture, "update-plugins-data").Stdout))
	}
	f.Fuzz(func(t *testing.T, out []byte) {
		wpcli.CoreCheckProven(out, "7.1.3")
		wpcli.ExtensionsCheckProven(out)
	})
}

// The translation updates are those WordPress stored with its update checks, as its Updates
// screen lists them (ADR-0032): the same as the language commands found on the bench.
func TestParseUpdateTranslations(t *testing.T) {
	for _, c := range []struct {
		data string
		want []wpcli.LanguageUpdate
	}{
		{"update-core-data", []wpcli.LanguageUpdate{{Item: "", Language: "fr_FR"}}},
		{"update-plugins-data", []wpcli.LanguageUpdate{{Item: "akismet", Language: "fr_FR"}}},
		{"update-themes-data", []wpcli.LanguageUpdate{}},
	} {
		got, err := wpcli.ParseUpdateTranslations([]byte(load(t, "outdated", c.data).Stdout))
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s: ParseUpdateTranslations() = %+v, %v; want %+v", c.data, got, err, c.want)
		}
	}
	// No stored data prints nothing.
	if got, err := wpcli.ParseUpdateTranslations(nil); err != nil || len(got) != 0 {
		t.Errorf("ParseUpdateTranslations(nothing) = %+v, %v; want none", got, err)
	}
}

// wordpress.org knows a plugin it answered for, with an update or without; a closed or unknown
// plugin is in neither (ADR-0032).
func TestPluginsKnown(t *testing.T) {
	known := wpcli.PluginsKnown([]byte(load(t, "outdated", "update-plugins-data").Stdout))
	for name, want := range map[string]bool{"akismet": true, "hello": true, "fixture-premium": false, "fixture-must-use": false} {
		if known[name] != want {
			t.Errorf("%s known = %v, want %v", name, known[name], want)
		}
	}
	// PHP writes an empty list for no plugin at all.
	if known := wpcli.PluginsKnown([]byte(`{"response":[],"no_update":{"seo/seo.php":{}}}`)); !known["seo"] || len(known) != 1 {
		t.Errorf("PluginsKnown() = %v, want seo only", known)
	}
	if known := wpcli.PluginsKnown([]byte(load(t, "offline", "update-plugins-data").Stdout)); len(known) != 0 {
		t.Errorf("offline: PluginsKnown() = %v, want none: the check never reached wordpress.org", known)
	}
}
