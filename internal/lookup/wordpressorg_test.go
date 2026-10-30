package lookup_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/lookup"
)

// pluginInfo is the request for one plugin, with every field Vigie does not read turned off.
func pluginInfo(slug string) string {
	return "https://api.wordpress.org/plugins/info/1.2/?action=plugin_information" +
		"&request%5Bfields%5D%5Bactive_installs%5D=0&request%5Bfields%5D%5Bbanners%5D=0" +
		"&request%5Bfields%5D%5Bcontributors%5D=0&request%5Bfields%5D%5Bdescription%5D=0" +
		"&request%5Bfields%5D%5Bdonate_link%5D=0&request%5Bfields%5D%5Bicons%5D=0" +
		"&request%5Bfields%5D%5Bratings%5D=0&request%5Bfields%5D%5Breviews%5D=0" +
		"&request%5Bfields%5D%5Bscreenshots%5D=0&request%5Bfields%5D%5Bsections%5D=0" +
		"&request%5Bfields%5D%5Btags%5D=0&request%5Bfields%5D%5Bversions%5D=0" +
		"&request%5Bslug%5D=" + slug
}

func TestAPluginOnWordPressOrg(t *testing.T) {
	c, r, now := client(map[string]answer{pluginInfo("polylang"): {200, "wordpress.org/plugin-polylang.json", ""}})
	info, err := c.PluginInfo(context.Background(), "polylang")
	if err != nil {
		t.Fatal(err)
	}
	sent(t, r, pluginInfo("polylang"))
	want := time.Date(2026, 10, 21, 15, 54, 0, 0, time.UTC)
	if !info.OnWordPressOrg || info.Closed || info.Latest != "3.9.1" || !info.LastUpdated.Equal(want) {
		t.Fatalf("info = %+v", info)
	}
	if info.Abandoned(*now) {
		t.Error("a plugin released nine days ago is abandoned")
	}
}

func TestAClosedPlugin(t *testing.T) {
	c, _, _ := client(map[string]answer{pluginInfo("display-widgets"): {404, "wordpress.org/plugin-display-widgets.json", ""}})
	info, err := c.PluginInfo(context.Background(), "display-widgets")
	if err != nil {
		t.Fatal(err)
	}
	if !info.OnWordPressOrg || !info.Closed || info.ClosedDate != "2021-01-30" || info.ClosedReason != "security-issue" {
		t.Fatalf("info = %+v, want closed on 2021-01-30 for a security issue", info)
	}
}

func TestAPluginUnknownToWordPressOrg(t *testing.T) {
	c, _, _ := client(map[string]answer{pluginInfo("zzz-premium"): {404, "wordpress.org/plugin-unknown.json", ""}})
	info, err := c.PluginInfo(context.Background(), "zzz-premium")
	if err != nil || info.OnWordPressOrg || info.Closed {
		t.Fatalf("info = %+v, %v; want a plugin wordpress.org does not know, without error", info, err)
	}
}

// Abandoned: no release on wordpress.org for two years (plan 05, decision 7).
func TestAnAbandonedPlugin(t *testing.T) {
	body := `{"name":"Old","slug":"old","version":"1.2","last_updated":"2024-10-29 9:00am GMT"}`
	c, _, now := client(map[string]answer{pluginInfo("old"): {200, "", body}})
	info, err := c.PluginInfo(context.Background(), "old")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Abandoned(*now) {
		t.Errorf("last release on 2024-10-29, now %s: want abandoned", now.Format(time.DateOnly))
	}
	if info.Abandoned(now.AddDate(0, 0, -2)) {
		t.Error("abandoned one day before its two years")
	}
}

func TestWordPressOrgRefusalsAndSwitch(t *testing.T) {
	c, r, _ := client(nil, lookup.WordPressOrg)
	if _, err := c.PluginInfo(context.Background(), "polylang"); !errors.Is(err, lookup.ErrOff) {
		t.Errorf("PluginInfo() turned off = %v, want ErrOff", err)
	}
	if _, err := c.PluginInfo(context.Background(), "a&request[slug]=b"); !errors.Is(err, lookup.ErrBadInput) {
		t.Errorf("PluginInfo() of a crafted slug = %v, want ErrBadInput", err)
	}
	sent(t, r)
}
