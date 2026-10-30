package app_test

import (
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

// A scan alerts only on what it made worse: a site turned urgent, a host turned unreachable.
func TestAlertsTellOnlyWhatGotWorse(t *testing.T) {
	host := model.Host{ID: model.NewID(), Name: "Shared hosting", Alias: "shared"}
	other := model.Host{ID: model.NewID(), Name: "Managed VPS", Alias: "vps"}
	shop := model.Site{ID: model.NewID(), HostID: host.ID, Name: "shop.example.test", Path: "~/sites/shop"}
	blog := model.Site{ID: model.NewID(), HostID: host.ID, Name: "blog.example.test", Path: "~/sites/blog", Position: 1}
	known := model.Site{ID: model.NewID(), HostID: host.ID, Name: "known.example.test", Path: "~/sites/known", Position: 2}
	vps := model.Site{ID: model.NewID(), HostID: other.ID, Name: "vps.example.test", Path: "~/sites/vps"}
	tree := model.NewTree()
	for _, err := range []error{tree.AddHost(host), tree.AddHost(other), tree.AddSite(shop), tree.AddSite(blog), tree.AddSite(known), tree.AddSite(vps)} {
		if err != nil {
			t.Fatal(err)
		}
	}

	urgent := func(site model.Site) model.Scan {
		return model.Scan{SiteID: site.ID, Outcome: model.ScanCompleted, CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
			Findings: []model.Finding{{Kind: model.KindDebugLog, Subject: "site", Level: model.LevelUrgent}}}
	}
	ok := func(site model.Site) model.Scan {
		return model.Scan{SiteID: site.ID, Outcome: model.ScanCompleted, CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true}
	}
	unreachable := func(site model.Site) model.Scan {
		return model.Scan{SiteID: site.ID, Outcome: model.ScanFailed, Failure: string(sshx.FailureTimedOut)}
	}
	before := map[model.ID]model.Scan{shop.ID: ok(shop), known.ID: urgent(known), vps.ID: ok(vps)}
	after := []model.Scan{urgent(shop), urgent(blog), urgent(known), unreachable(vps)}

	got := app.Alerts(tree, before, after)
	want := []app.Alert{
		{Kind: app.AlertUrgent, Name: "shop.example.test"},
		{Kind: app.AlertUrgent, Name: "blog.example.test"}, // never scanned before
		{Kind: app.AlertUnreachable, Name: "Managed VPS"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Alerts() = %+v, want %+v (known.example.test was already urgent)", got, want)
	}
	// Unreachable again: already known.
	if again := app.Alerts(tree, map[model.ID]model.Scan{vps.ID: unreachable(vps)}, []model.Scan{unreachable(vps)}); len(again) != 0 {
		t.Errorf("Alerts() for a host still unreachable = %+v, want none", again)
	}
}
