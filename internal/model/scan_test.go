package model_test

import (
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

// upToDate is a completed scan where every check reached wordpress.org and nothing is pending.
func upToDate() model.Scan {
	return model.Scan{
		Outcome:     model.ScanCompleted,
		CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
		Items: []model.ScanItem{
			{Kind: model.ItemCore, Version: "7.1.3", Update: model.UpdateNone},
			{Kind: model.ItemPlugin, Slug: "akismet", Version: "5.7.2", Update: model.UpdateNone, OnWordPressOrg: true},
			{Kind: model.ItemTheme, Slug: "twentytwentyfive", Version: "1.5", Update: model.UpdateNone},
		},
	}
}

// found adds one finding per level.
func found(levels ...model.FindingLevel) func(*model.Scan) {
	return func(s *model.Scan) {
		for i, l := range levels {
			s.Findings = append(s.Findings, model.Finding{Kind: model.KindCheck, Subject: "site", Identifier: string(rune('a' + i)), Level: l})
		}
	}
}

func TestSiteStatus(t *testing.T) {
	with := func(change func(*model.Scan)) model.Scan {
		s := upToDate()
		change(&s)
		return s
	}
	for _, tc := range []struct {
		name string
		scan model.Scan
		want model.SiteStatus
	}{
		{"everything checked, nothing pending", upToDate(), model.StatusUpToDate},
		{"never scanned", model.Scan{}, model.StatusUnknown},
		{"WordPress could not be loaded", with(func(s *model.Scan) { s.Outcome = model.ScanCannotLoad }), model.StatusUnknown},
		{"connection failed", with(func(s *model.Scan) { s.Outcome = model.ScanFailed }), model.StatusUnknown},
		{"cancelled", with(func(s *model.Scan) { s.Outcome = model.ScanCancelled }), model.StatusUnknown},
		{"plugin update", with(func(s *model.Scan) { s.Items[1].Update = model.UpdateAvailable }), model.StatusAttention},
		{"update needs newer PHP", with(func(s *model.Scan) { s.Items[1].Update = model.UpdateUnavailable }), model.StatusAttention},
		{"update state unknown to Vigie", with(func(s *model.Scan) { s.Items[2].Update = model.UpdateUnknown }), model.StatusAttention},
		{"newer than wordpress.org", with(func(s *model.Scan) { s.Items[1].Update = model.UpdateNewerThanSource }), model.StatusUpToDate},
		{"translation update", with(func(s *model.Scan) {
			s.Items = append(s.Items, model.ScanItem{Kind: model.ItemLanguage, Language: "fr_FR", Update: model.UpdateAvailable})
		}), model.StatusAttention},
		{"core check failed", with(func(s *model.Scan) { s.CoreChecked = false }), model.StatusAttention},
		{"plugin check failed", with(func(s *model.Scan) { s.PluginsChecked = false }), model.StatusAttention},
		{"theme check failed", with(func(s *model.Scan) { s.ThemesChecked = false }), model.StatusAttention},
		{"translation check failed", with(func(s *model.Scan) { s.LanguagesChecked = false }), model.StatusAttention},
		// Shown on the plugin, but most client sites have one: it would keep them all orange.
		{"plugin not on wordpress.org", with(func(s *model.Scan) { s.Items[1].OnWordPressOrg = false }), model.StatusUpToDate},
		// The worst finding that is not ignored.
		{"urgent finding", with(found(model.LevelUrgent, model.LevelAttention, model.LevelNotChecked)), model.StatusUrgent},
		{"urgent finding and an update", with(func(s *model.Scan) {
			s.Items[1].Update = model.UpdateAvailable
			found(model.LevelUrgent)(s)
		}), model.StatusUrgent},
		{"finding that needs attention", with(found(model.LevelAttention, model.LevelNotChecked)), model.StatusAttention},
		{"a check not run", with(found(model.LevelNotChecked, model.LevelInfo)), model.StatusPartial},
		{"a check not run and an update", with(func(s *model.Scan) {
			s.Items[1].Update = model.UpdateAvailable
			found(model.LevelNotChecked)(s)
		}), model.StatusAttention},
		{"information only", with(found(model.LevelInfo)), model.StatusUpToDate},
		{"urgent finding ignored", with(func(s *model.Scan) {
			found(model.LevelUrgent)(s)
			s.Findings[0].Ignore = &model.Ignore{Key: s.Findings[0].Key(), Reason: "patched on purpose"}
		}), model.StatusUpToDate},
	} {
		if got := tc.scan.Status(); got != tc.want {
			t.Errorf("%s: Status() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestUpdateSourceUnknown(t *testing.T) {
	premium := model.ScanItem{Kind: model.ItemPlugin, Slug: "premium", Update: model.UpdateNone}
	if !premium.UpdateSourceUnknown() {
		t.Error("a plugin unknown to wordpress.org that reports no update: source unknown")
	}
	premium.Update = model.UpdateAvailable // its own updater reported one
	if premium.UpdateSourceUnknown() {
		t.Error("a plugin that reports an update has a known source")
	}
	theme := model.ScanItem{Kind: model.ItemTheme, Slug: "custom", Update: model.UpdateNone}
	if theme.UpdateSourceUnknown() {
		t.Error("themes have no wordpress.org status: never flagged")
	}
	// WordPress never updates them: no update to miss.
	for _, status := range []string{"must-use", "dropin"} {
		item := model.ScanItem{Kind: model.ItemPlugin, Slug: "advanced-cache.php", Status: status, Update: model.UpdateNone}
		if item.UpdateSourceUnknown() {
			t.Errorf("a %s plugin is never flagged", status)
		}
	}
}

func TestPendingUpdates(t *testing.T) {
	s := upToDate()
	s.Items = append(s.Items,
		model.ScanItem{Kind: model.ItemCore, Version: "7.1.3", UpdateVersion: "7.1.4", Update: model.UpdateAvailable},
		model.ScanItem{Kind: model.ItemPlugin, Slug: "a", Update: model.UpdateAvailable, OnWordPressOrg: true},
		model.ScanItem{Kind: model.ItemPlugin, Slug: "b", Update: model.UpdateUnavailable, OnWordPressOrg: true},
		model.ScanItem{Kind: model.ItemTheme, Slug: "c", Update: model.UpdateAvailable},
		model.ScanItem{Kind: model.ItemLanguage, Slug: "a", Language: "fr_FR", Update: model.UpdateAvailable},
	)
	got := s.PendingUpdates()
	want := model.UpdateCounts{Core: 1, Plugins: 1, Themes: 1, Languages: 1}
	if got != want {
		t.Fatalf("PendingUpdates() = %+v, want %+v (an update that cannot be installed is not pending)", got, want)
	}
}
