package i18n_test

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/locales"
)

func catalogs(t *testing.T) *i18n.Catalogs {
	t.Helper()
	c, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPluralCategoriesMatchSharedCases(t *testing.T) {
	data, err := os.ReadFile("../../locales/testdata/plurals.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]any
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "fr"} {
		for n, want := range cases[lang].(map[string]any) {
			count, _ := strconv.Atoi(n)
			if got := i18n.PluralCategory(lang, count); got != want {
				t.Errorf("PluralCategory(%s, %d) = %q, want %q", lang, count, got, want)
			}
		}
	}
}

func TestTranslate(t *testing.T) {
	c := catalogs(t)
	if got := c.T("fr", "nav.sites", nil); got != "Sites" {
		t.Errorf("T(fr, nav.sites) = %q", got)
	}
	if got := c.T("en", "manager.running", map[string]any{"count": 2}); got != "2 scans running" {
		t.Errorf("T(en, manager.running, 2) = %q", got)
	}
	if got := c.T("fr", "manager.running", map[string]any{"count": 1}); got != "1 relevé en cours" {
		t.Errorf("T(fr, manager.running, 1) = %q", got)
	}
	if got := c.T("fr", "manager.running", map[string]any{"count": 0}); got != "0 relevé en cours" {
		t.Errorf("T(fr, manager.running, 0) = %q, French uses the one form for 0", got)
	}
}

func TestTranslateFallsBack(t *testing.T) {
	c := catalogs(t)
	if got := c.T("de", "nav.sites", nil); got != "Sites" {
		t.Errorf("unsupported language: %q, want English", got)
	}
	if got := c.T("fr", "no.such.key", nil); got != "no.such.key" {
		t.Errorf("missing key: %q, want the key itself", got)
	}
}

// Both catalogs have the same keys; plural keys carry the forms their language needs (English:
// one, other; French: one, many, other).
func TestCatalogsHaveTheSameKeys(t *testing.T) {
	c := catalogs(t)
	base := func(lang string) []string {
		var keys []string
		for _, k := range c.Keys(lang) {
			for _, form := range []string{"_one", "_many", "_other"} {
				k = strings.TrimSuffix(k, form)
			}
			if !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		return keys
	}
	if en, fr := base("en"), base("fr"); !slices.Equal(en, fr) {
		t.Fatalf("catalogs differ:\nen %q\nfr %q", en, fr)
	}
	forms := map[string][]string{"en": {"_one", "_other"}, "fr": {"_one", "_many", "_other"}}
	for lang, needed := range forms {
		keys := c.Keys(lang)
		for _, k := range keys {
			if !strings.HasSuffix(k, "_other") {
				continue
			}
			stem := strings.TrimSuffix(k, "_other")
			for _, form := range needed {
				if !slices.Contains(keys, stem+form) {
					t.Errorf("%s: %s%s is missing", lang, stem, form)
				}
			}
		}
	}
}

func TestLanguageFromEnvironment(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LANG": "fr_CH.UTF-8"}, "fr"},
		{map[string]string{"LANG": "de_CH.UTF-8"}, "en"},
		{map[string]string{"LANG": "en_US.UTF-8", "LC_MESSAGES": "fr_FR.UTF-8"}, "fr"},
		{map[string]string{"LC_MESSAGES": "en_GB", "LC_ALL": "fr_FR"}, "fr"},
		{map[string]string{"LANGUAGE": "de:fr:en", "LC_ALL": "en_US"}, "fr"},
		{map[string]string{"LANG": "C"}, "en"},
		{map[string]string{"LANGUAGE": "", "LANG": "fr_FR"}, "fr"},
		{map[string]string{}, "en"},
	}
	for _, tt := range tests {
		getenv := func(k string) string { return tt.env[k] }
		if got := i18n.LanguageFromEnv(getenv); got != tt.want {
			t.Errorf("LanguageFromEnv(%v) = %q, want %q", tt.env, got, tt.want)
		}
	}
}

func TestSupportedLanguage(t *testing.T) {
	tests := map[string]string{"fr-CH": "fr", "fr": "fr", "en-US": "en", "de-CH": "", "": ""}
	for tag, want := range tests {
		if got := i18n.Supported(tag); got != want {
			t.Errorf("Supported(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestSystemLanguageIsSupported(t *testing.T) {
	if got := i18n.SystemLanguage(); !slices.Contains(i18n.Languages, got) {
		t.Fatalf("SystemLanguage() = %q, want one of %q", got, i18n.Languages)
	}
}
