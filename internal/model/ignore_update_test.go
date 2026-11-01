package model_test

import (
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

func pluginUpdate(slug, from, to string) model.ScanItem {
	return model.ScanItem{Kind: model.ItemPlugin, Slug: slug, Status: "active", Version: from, Update: model.UpdateAvailable, UpdateVersion: to, OnWordPressOrg: true}
}

func TestAnUpdateIgnoredUntilTheNextVersion(t *testing.T) {
	item := pluginUpdate("woocommerce", "9.8.1", "10.2.0")
	ignore := model.Ignore{Key: model.UpdateKey(item), UntilVersion: "10.2.0"}
	if !ignore.CoversUpdate(item) {
		t.Fatal("CoversUpdate() the version offered when ignored = false")
	}
	item.UpdateVersion = "10.2.1"
	if ignore.CoversUpdate(item) {
		t.Error("CoversUpdate() a newer version offered = true, want the ignore to lapse")
	}
	permanent := model.Ignore{Key: model.UpdateKey(item), Reason: "payment not tested"}
	if !permanent.CoversUpdate(item) {
		t.Error("CoversUpdate() of a permanent ignore = false")
	}
	if model.UpdateKey(item) == model.UpdateKey(pluginUpdate("akismet", "5.3", "5.7.2")) {
		t.Error("two plugins share an update key")
	}
	core := model.ScanItem{Kind: model.ItemCore, Version: "7.1.3", Update: model.UpdateAvailable, UpdateVersion: "7.1.4"}
	if model.UpdateKey(core) == "" || model.UpdateKey(core) == model.UpdateKey(item) {
		t.Errorf("UpdateKey(core) = %q", model.UpdateKey(core))
	}
}

func TestAnIgnoredUpdateCountsNowhere(t *testing.T) {
	site := model.NewID()
	held := pluginUpdate("page-builder-kit", "4.2.0", "5.0.0")
	s := model.Scan{SiteID: site, Outcome: model.ScanCompleted, CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
		Items: []model.ScanItem{{Kind: model.ItemCore, Version: "7.1.4", Update: model.UpdateNone}, held}}
	if s.Status() != model.StatusAttention {
		t.Fatalf("Status() with a pending update = %s", s.Status())
	}
	ignores := []model.Ignore{
		{SiteID: site, Key: model.UpdateKey(held), Reason: "the theme uses its internal API"},
		{SiteID: model.NewID(), Key: model.UpdateKey(pluginUpdate("akismet", "", "")), Reason: "another site's"},
	}
	s.ApplyIgnores(ignores)
	if s.Items[1].Ignore == nil || s.Items[0].Ignore != nil {
		t.Fatalf("ApplyIgnores marked %+v", s.Items)
	}
	if s.Status() != model.StatusUpToDate {
		t.Errorf("Status() with the update ignored = %s, want up to date", s.Status())
	}
	if c := s.PendingUpdates(); c.Plugins != 0 {
		t.Errorf("PendingUpdates() = %+v, want the ignored update left out", c)
	}
	// A newer version offered: the update counts again.
	s.Items[1].UpdateVersion = "5.0.1"
	s.ApplyIgnores([]model.Ignore{{SiteID: site, Key: model.UpdateKey(held), UntilVersion: "5.0.0"}})
	if s.Items[1].Ignore != nil || s.PendingUpdates().Plugins != 1 {
		t.Errorf("an ignore until 5.0.0 still covers 5.0.1: %+v", s.Items[1])
	}
}
