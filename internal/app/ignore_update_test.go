package app_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// An update is ignored with the services of the findings, by the key its view gives; the views say
// so, and the site counts it no more.
func TestIgnoreAnUpdate(t *testing.T) {
	ctx := context.Background()
	_, ws, ids := editor(t)
	site := uuid.MustParse(ids["site"])
	scan := model.Scan{ID: model.NewID(), SiteID: site, Outcome: model.ScanCompleted, Finished: time.Now(),
		CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
		Items: []model.ScanItem{
			{Kind: model.ItemCore, Version: "7.1.4", Update: model.UpdateNone},
			{Kind: model.ItemPlugin, Slug: "woocommerce", Title: "WooCommerce", Status: "active", Version: "9.8.1", Update: model.UpdateAvailable, UpdateVersion: "10.2.0", OnWordPressOrg: true},
			{Kind: model.ItemLanguage, Slug: "woocommerce", Language: "fr_FR", Update: model.UpdateAvailable},
		}}
	if err := ws.DB().SaveScan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	m := app.NewSiteManager(ws.DB())
	view := func() app.SiteView {
		t.Helper()
		v, err := m.SiteDetail(ctx, ids["site"])
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	byKind := func(v app.SiteView, kind string) app.UpdateView {
		for _, u := range v.Scan.Updates {
			if u.Kind == kind {
				return u
			}
		}
		t.Fatalf("no %s update in %+v", kind, v.Scan.Updates)
		return app.UpdateView{}
	}
	before := view()
	plugin := byKind(before, "plugin")
	if plugin.Key == "" || plugin.Ignore != nil || before.Scan.Pending.Plugins != 1 {
		t.Fatalf("before: %+v, pending %+v", plugin, before.Scan.Pending)
	}
	if byKind(before, "language").Key != "" {
		t.Error("a translation has an ignore key: translations are updated together")
	}

	s := app.NewSecurity(ws, time.Now)
	if p := s.Ignore(ctx, ids["site"], plugin.Key, "", "payment not tested"); p != nil {
		t.Fatal(p)
	}
	after := view()
	ignored := byKind(after, "plugin")
	if ignored.Ignore == nil || ignored.Ignore.Reason != "payment not tested" || after.Scan.Pending.Plugins != 0 {
		t.Fatalf("after: %+v, pending %+v", ignored, after.Scan.Pending)
	}
	if e := after.Scan.Extensions[0]; e.Ignore == nil {
		t.Errorf("the Plugins tab's line = %+v, want it ignored", e)
	}
	if p := s.Unignore(ctx, ids["site"], plugin.Key); p != nil {
		t.Fatal(p)
	}
	if v := view(); byKind(v, "plugin").Ignore != nil || v.Scan.Pending.Plugins != 1 {
		t.Errorf("after putting it back: %+v", byKind(v, "plugin"))
	}
}
