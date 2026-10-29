package app_test

import (
	"testing"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/i18n"
	"github.com/MarJC5/vigie/locales"
)

// Every label of the menu bar is translated; every item does one thing.
func TestMenuBarIsTranslated(t *testing.T) {
	texts, err := i18n.Load(locales.Files())
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]bool{}
	for _, section := range app.MenuBar {
		for _, lang := range []string{"en", "fr"} {
			if section.Key != "" && texts.T(lang, section.Key, nil) == section.Key {
				t.Errorf("%s: %s has no text", lang, section.Key)
			}
		}
		for _, item := range section.Items {
			if item.Separator {
				continue
			}
			for _, lang := range []string{"en", "fr"} {
				if texts.T(lang, item.Key, map[string]any{"app": "Vigie"}) == item.Key {
					t.Errorf("%s: %s has no text", lang, item.Key)
				}
			}
			if (item.Action == "") == (item.Role == "") {
				t.Errorf("%s: wants either an action or a standard role", item.Key)
			}
			if item.Action != "" {
				if actions[item.Action] {
					t.Errorf("action %q twice", item.Action)
				}
				actions[item.Action] = true
			}
		}
	}
	for _, want := range []string{app.ActionNewHost, app.ActionNewFolder, app.ActionSearch, app.ActionScan, app.ActionScanAll} {
		if !actions[want] {
			t.Errorf("no menu item sends %q", want)
		}
	}
}
