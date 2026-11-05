package model_test

import (
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

// A set-aside holds while a scan offers the very version set aside, and ends otherwise: a newer
// version offered, the plugin updated elsewhere, or removed (ADR-0042, point 5).
func TestSetAsideHoldsWhileItsVersionIsOffered(t *testing.T) {
	s := model.SetAside{Kind: model.ItemPlugin, Slug: "acf-pro", Version: "6.9.0"}
	plugin := func(version string, state model.UpdateState, offered string) model.ScanItem {
		return model.ScanItem{Kind: model.ItemPlugin, Slug: "acf-pro", Version: version, Update: state, UpdateVersion: offered}
	}
	for name, tc := range map[string]struct {
		items []model.ScanItem
		holds bool
	}{
		"the same version offered":        {[]model.ScanItem{plugin("6.8.10", model.UpdateAvailable, "6.9.0")}, true},
		"offered, but PHP is too old now": {[]model.ScanItem{plugin("6.8.10", model.UpdateUnavailable, "6.9.0")}, true},
		"a newer version offered":         {[]model.ScanItem{plugin("6.8.10", model.UpdateAvailable, "6.9.1")}, false},
		"updated elsewhere":               {[]model.ScanItem{plugin("6.9.0", model.UpdateNone, "")}, false},
		// A premium plugin offers its update itself: a scan does not see it, and keeps the set-aside.
		"no update seen by the scan": {[]model.ScanItem{plugin("6.8.10", model.UpdateNone, "")}, true},
		"its update not known":       {[]model.ScanItem{plugin("6.8.10", model.UpdateUnknown, "")}, true},
		"removed":                    {nil, false},
		"a theme of the same slug": {[]model.ScanItem{{Kind: model.ItemTheme, Slug: "acf-pro", Version: "6.8.10",
			Update: model.UpdateAvailable, UpdateVersion: "6.9.0"}}, false},
	} {
		if got := s.Holds(tc.items); got != tc.holds {
			t.Errorf("%s: Holds() = %v, want %v", name, got, tc.holds)
		}
	}
}
