package app_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// network stores a network with four subsites, one archived, and a scan that read the others.
func network(t *testing.T) (*app.SiteManager, model.Site) {
	t.Helper()
	ctx := context.Background()
	db := repository(t)
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "reseau.example.test", Path: "~/sites/reseau",
		URL: "https://reseau.example.test", Multisite: true}
	if err := db.AddHostWithSites(ctx, host, []model.Site{site}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSubsites(ctx, site.ID, []model.Subsite{
		{BlogID: 1, URL: "https://reseau.example.test/", Public: true},
		{BlogID: 2, URL: "https://reseau.example.test/alpha/", Public: true},
		{BlogID: 3, URL: "https://ecole-beta.example.test/", Public: false},
		{BlogID: 4, URL: "https://reseau.example.test/gamma/", Public: true, Archived: true},
	}); err != nil {
		t.Fatal(err)
	}
	finished := time.Date(2026, 10, 29, 9, 0, 3, 0, time.UTC)
	err := db.SaveScan(ctx, model.Scan{
		ID: model.NewID(), SiteID: site.ID, Started: finished.Add(-3 * time.Second), Finished: finished,
		Outcome: model.ScanCompleted, WordPress: "7.1.3", Home: "https://reseau.example.test",
		Items: []model.ScanItem{
			{Kind: model.ItemPlugin, Slug: "akismet", Title: "Akismet", Status: "active-network", Version: "5.7.2", Update: model.UpdateNone},
			{Kind: model.ItemPlugin, Slug: "hello", Title: "Hello Dolly", Status: "inactive", Version: "1.7.2", Update: model.UpdateNone},
			{Kind: model.ItemPlugin, Slug: "contact-form-7", Title: "Contact Form 7", Status: "inactive", Version: "6.0.4", Update: model.UpdateNone},
			{Kind: model.ItemTheme, Slug: "astra", Title: "Astra", Status: "inactive", Version: "4.8.1", Update: model.UpdateNone},
			{Kind: model.ItemTheme, Slug: "twentytwentyfive", Title: "Twenty Twenty-Five", Status: "active", Version: "1.5", Update: model.UpdateNone},
		},
		Network: &model.NetworkRead{Plugins: []string{"akismet"}, Sites: []model.SubsiteRead{
			{BlogID: 1, Plugins: []string{}, Theme: "twentytwentyfive", ThemeName: "Twenty Twenty-Five", ThemeVersion: "1.5",
				AdminEmail: "admin@reseau.example.test", Admins: []string{"admin"}},
			{BlogID: 2, Plugins: []string{"hello", "contact-form-7"}, Theme: "astra", ThemeName: "Astra", ThemeVersion: "4.8.1",
				AdminEmail: "direction@alpha.example.test", Admins: []string{"admin", "marie"}},
			{BlogID: 3, Plugins: []string{"contact-form-7"}, Theme: "astra", ThemeName: "Astra", ThemeVersion: "4.8.1",
				AdminEmail: "beta@example.test", Admins: []string{"marie"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return app.NewSiteManager(db), site
}

// A network's page lists its subsites with what the latest scan read of each (ADR-0029).
func TestANetworksSubsites(t *testing.T) {
	m, site := network(t)
	got, err := m.SiteDetail(context.Background(), site.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	want := []app.SubsiteView{
		{BlogID: 1, URL: "https://reseau.example.test/", Label: "/", State: "public", Read: true, AdminURL: "https://reseau.example.test/wp-admin/",
			Theme: "twentytwentyfive", ThemeName: "Twenty Twenty-Five", ThemeVersion: "1.5", AdminEmail: "admin@reseau.example.test",
			Plugins: []string{}, Admins: []string{"admin"}},
		{BlogID: 2, URL: "https://reseau.example.test/alpha/", Label: "/alpha", State: "public", Read: true,
			AdminURL: "https://reseau.example.test/alpha/wp-admin/", Theme: "astra", ThemeName: "Astra", ThemeVersion: "4.8.1",
			AdminEmail: "direction@alpha.example.test", Plugins: []string{"hello", "contact-form-7"}, Admins: []string{"admin", "marie"}},
		// On a domain of its own: named by it.
		{BlogID: 3, URL: "https://ecole-beta.example.test/", Label: "ecole-beta.example.test", State: "hidden", Read: true,
			AdminURL: "https://ecole-beta.example.test/wp-admin/", Theme: "astra", ThemeName: "Astra", ThemeVersion: "4.8.1",
			AdminEmail: "beta@example.test", Plugins: []string{"contact-form-7"}, Admins: []string{"marie"}},
		// Archived: listed with its state, nothing read.
		{BlogID: 4, URL: "https://reseau.example.test/gamma/", Label: "/gamma", State: "archived",
			AdminURL: "https://reseau.example.test/gamma/wp-admin/", Plugins: []string{}, Admins: []string{}},
	}
	if !reflect.DeepEqual(got.Subsites, want) {
		t.Fatalf("subsites =\n%+v\nwant\n%+v", got.Subsites, want)
	}
	// Each plugin and theme says on which subsites it is active.
	active := map[string][]int{}
	for _, e := range got.Scan.Extensions {
		active[e.Kind+":"+e.Slug] = e.Subsites
	}
	for key, ids := range map[string][]int{"plugin:akismet": {}, "plugin:hello": {2}, "plugin:contact-form-7": {2, 3},
		"theme:astra": {2, 3}, "theme:twentytwentyfive": {1}} {
		if !slices.Equal(active[key], ids) {
			t.Errorf("%s active on %v, want %v", key, active[key], ids)
		}
	}
}

// In the site list, a network carries its subsites, to unfold.
func TestTheTreeCarriesANetworksSubsites(t *testing.T) {
	m, site := network(t)
	tree, err := m.Tree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tree.Nodes {
		if n.ID != site.ID.String() {
			continue
		}
		want := []app.TreeSubsite{{BlogID: 1, Label: "/", State: "public"}, {BlogID: 2, Label: "/alpha", State: "public"},
			{BlogID: 3, Label: "ecole-beta.example.test", State: "hidden"}, {BlogID: 4, Label: "/gamma", State: "archived"}}
		if !reflect.DeepEqual(n.SubsiteList, want) {
			t.Fatalf("subsites = %+v\nwant %+v", n.SubsiteList, want)
		}
		return
	}
	t.Fatal("the network is not in the tree")
}
