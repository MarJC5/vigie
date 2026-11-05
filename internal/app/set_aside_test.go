package app_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// A version rolled back on its own is set aside: the site's update says when and why, and it still
// counts among the updates offered (ADR-0042, point 5).
func TestAnUpdateSetAsideSaysWhy(t *testing.T) {
	ctx := context.Background()
	_, ws, ids := editor(t)
	site := uuid.MustParse(ids["site"])
	scan := model.Scan{ID: model.NewID(), SiteID: site, Outcome: model.ScanCompleted, Finished: time.Now(),
		CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
		Items: []model.ScanItem{
			{Kind: model.ItemPlugin, Slug: "acf-pro", Title: "ACF PRO", Status: "active", Version: "6.8.10", Update: model.UpdateAvailable, UpdateVersion: "6.9.0"},
			{Kind: model.ItemPlugin, Slug: "woocommerce", Title: "WooCommerce", Status: "active", Version: "9.8.1", Update: model.UpdateAvailable, UpdateVersion: "9.8.3"},
		}}
	if err := ws.DB().SaveScan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 11, 4, 14, 12, 24, 0, time.UTC)
	if err := ws.DB().SaveSetAside(ctx, model.SetAside{SiteID: site, Kind: model.ItemPlugin, Slug: "acf-pro", Version: "6.9.0",
		OperationID: model.NewID(), Reason: "home 500", Created: when}); err != nil {
		t.Fatal(err)
	}
	v, err := app.NewSiteManager(ws.DB()).SiteDetail(ctx, ids["site"])
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range v.Scan.Updates {
		switch u.Slug {
		case "acf-pro":
			if u.SetAside == nil || u.SetAside.Reason != "home 500" || u.SetAside.Created != "2026-11-04T14:12:24Z" {
				t.Errorf("acf-pro = %+v, want set aside", u.SetAside)
			}
		case "woocommerce":
			if u.SetAside != nil {
				t.Errorf("woocommerce set aside: %+v", u.SetAside)
			}
		}
	}
	if v.Scan.Pending.Plugins != 2 {
		t.Errorf("pending plugins = %d, want 2: a version set aside still counts", v.Scan.Pending.Plugins)
	}
}
