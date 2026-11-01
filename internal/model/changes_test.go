package model_test

import (
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

var (
	t0 = time.Date(2026, 10, 31, 3, 0, 0, 0, time.UTC)
	t1 = t0.Add(24 * time.Hour)
)

func admin(login string, super bool) model.Finding {
	f := model.Finding{Kind: model.KindAdministrator, Subject: "user:" + login, Level: model.LevelInfo, Facts: map[string]string{"two-factor": "on"}}
	if super {
		f.Facts["super"] = "true"
	}
	return f
}

func item(kind model.ItemKind, slug, title, status, version string) model.ScanItem {
	return model.ScanItem{Kind: kind, Slug: slug, Title: title, Status: status, Version: version, Update: model.UpdateNone}
}

// scanAt is a complete scan of one site, finished at the time given.
func scanAt(at time.Time, items []model.ScanItem, findings ...model.Finding) model.Scan {
	return model.Scan{SiteID: site, Started: at.Add(-time.Minute), Finished: at, Outcome: model.ScanCompleted, Items: items, Findings: findings}
}

var site = model.NewID()

// kinds lists the changes as "kind subject", in their order.
func kinds(changes []model.Change) []string {
	var out []string
	for _, c := range changes {
		out = append(out, string(c.Kind)+" "+c.Subject)
	}
	return out
}

func TestNothingChanged(t *testing.T) {
	items := []model.ScanItem{item(model.ItemCore, "", "", "", "7.1.4"), item(model.ItemPlugin, "akismet", "Akismet", "active", "5.7.2")}
	if got := model.Changes(scanAt(t0, items, admin("marie", false)), scanAt(t1, items, admin("marie", false)), nil); len(got) != 0 {
		t.Errorf("Changes() = %+v, want none", got)
	}
}

func TestAdministratorsAddedAndRemoved(t *testing.T) {
	got := model.Changes(scanAt(t0, nil, admin("marie", false), admin("stagiaire", false)), scanAt(t1, nil, admin("marie", false), admin("root2", true)), nil)
	if want := []string{"admin-added user:root2", "admin-removed user:stagiaire"}; !slices.Equal(kinds(got), want) {
		t.Fatalf("Changes() = %q, want %q", kinds(got), want)
	}
	if !got[0].Super {
		t.Error("the new administrator is a super administrator: not said")
	}
}

func TestPluginsAndThemesInstalledRemovedAndSwitched(t *testing.T) {
	before := []model.ScanItem{item(model.ItemPlugin, "akismet", "Akismet", "active", "5.7.2"), item(model.ItemTheme, "twentytwentyfour", "Twenty Twenty-Four", "active", "1.3"),
		item(model.ItemTheme, "astra", "Astra", "inactive", "4.8")}
	after := []model.ScanItem{item(model.ItemPlugin, "wp-file-manager", "WP File Manager", "active", "7.2.1"), item(model.ItemTheme, "twentytwentyfour", "Twenty Twenty-Four", "inactive", "1.3"),
		item(model.ItemTheme, "astra", "Astra", "active", "4.8")}
	got := model.Changes(scanAt(t0, before), scanAt(t1, after), nil)
	want := []string{"installed plugin:wp-file-manager", "removed plugin:akismet", "theme-switched theme:astra"}
	if !slices.Equal(kinds(got), want) {
		t.Fatalf("Changes() = %q, want %q", kinds(got), want)
	}
	if got[0].Title != "WP File Manager" || got[0].To != "7.2.1" || got[2].From != "Twenty Twenty-Four" || got[2].To != "Astra" {
		t.Errorf("changes = %+v", got)
	}
}

func TestVersionsChangedByVigieOrOutside(t *testing.T) {
	before := []model.ScanItem{item(model.ItemCore, "", "", "", "7.1.3"), item(model.ItemPlugin, "akismet", "Akismet", "active", "5.3")}
	after := []model.ScanItem{item(model.ItemCore, "", "", "", "7.1.4"), item(model.ItemPlugin, "akismet", "Akismet", "active", "5.7.2")}
	op := model.Operation{SiteID: site, Kind: model.OperationUpdate, Started: t0.Add(time.Hour), Finished: t0.Add(time.Hour + time.Minute),
		Steps: []model.OperationStep{{Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: "akismet", VersionBefore: "5.3", VersionAfter: "5.7.2"}}}
	// An update of Vigie before the earlier scan says nothing about this one.
	old := model.Operation{SiteID: site, Kind: model.OperationUpdate, Started: t0.Add(-time.Hour),
		Steps: []model.OperationStep{{Step: model.StepUpdate, ItemKind: model.ItemCore, VersionBefore: "7.1.2", VersionAfter: "7.1.3"}}}
	got := model.Changes(scanAt(t0, before), scanAt(t1, after), []model.Operation{old, op})
	if want := []string{"version core", "version plugin:akismet"}; !slices.Equal(kinds(got), want) {
		t.Fatalf("Changes() = %q, want %q", kinds(got), want)
	}
	if got[0].ByVigie || !got[1].ByVigie || got[0].From != "7.1.3" || got[0].To != "7.1.4" {
		t.Errorf("changes = %+v; WordPress outside Vigie, Akismet by Vigie", got)
	}
}

func TestFilesNewlyDifferent(t *testing.T) {
	core := func(changed string) model.Finding {
		return model.Finding{Kind: model.KindCoreFile, Subject: "core", Level: model.LevelUrgent, Facts: map[string]string{"changed": changed, "added": "0", "missing": "0"}}
	}
	plugin := model.Finding{Kind: model.KindPluginFile, Subject: "plugin:akismet", Level: model.LevelAttention, Facts: map[string]string{"changed": "1", "added": "0"}}
	akismet := []model.ScanItem{item(model.ItemPlugin, "akismet", "Akismet Anti-spam", "active", "5.7.2")}
	got := model.Changes(scanAt(t0, akismet, core("1")), scanAt(t1, akismet, core("3"), plugin), nil)
	if want := []string{"files core", "files plugin:akismet"}; !slices.Equal(kinds(got), want) {
		t.Fatalf("Changes() = %q, want %q", kinds(got), want)
	}
	// Named as the scan names the plugin, so the change says it on its own (ADR-0030).
	if got[0].Title != "" || got[1].Title != "Akismet Anti-spam" {
		t.Errorf("titles = %q, %q; want none for WordPress, the plugin's", got[0].Title, got[1].Title)
	}
	// The same files, carried over from a comparison of the day before: nothing new.
	if got := model.Changes(scanAt(t0, nil, core("3")), scanAt(t1, nil, core("3")), nil); len(got) != 0 {
		t.Errorf("the same files = %+v, want no change", got)
	}
}

func TestNewFindingsNotSetAside(t *testing.T) {
	vuln := model.Finding{Kind: model.KindVulnerability, Subject: "plugin:woocommerce", Identifier: "uuid-1", Version: "9.8.1", Level: model.LevelUrgent}
	editing := model.Finding{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention}
	info := model.Finding{Kind: model.KindCertificate, Subject: "site", Level: model.LevelInfo}
	ignored := model.Finding{Kind: model.KindHiddenFromSearch, Subject: "site", Level: model.LevelAttention, Ignore: &model.Ignore{Reason: "staging"}}
	got := model.Changes(scanAt(t0, nil, editing), scanAt(t1, nil, editing, vuln, info, ignored), nil)
	if want := []string{"finding plugin:woocommerce"}; !slices.Equal(kinds(got), want) {
		t.Fatalf("Changes() = %q, want %q", kinds(got), want)
	}
	if got[0].Finding == nil || got[0].Finding.Identifier != "uuid-1" {
		t.Errorf("the new finding = %+v", got[0])
	}
}

func TestNoChangesAgainstAnIncompleteScan(t *testing.T) {
	failed := model.Scan{SiteID: site, Outcome: model.ScanFailed}
	if got := model.Changes(failed, scanAt(t1, nil, admin("root2", false)), nil); got != nil {
		t.Errorf("Changes() after a failed scan = %+v, want nil", got)
	}
}

// What each scan of a site tells of itself, noted in order as the workspace notes it when a scan is
// made (ADR-0030): its status, its changes since the complete scan before, a failed one between.
func TestNoteAll(t *testing.T) {
	editing := model.Finding{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention}
	first := scanAt(t0, []model.ScanItem{item(model.ItemPlugin, "akismet", "Akismet", "active", "5.3")})
	failed := model.Scan{SiteID: site, Started: t1, Finished: t1, Outcome: model.ScanFailed, Failure: "refused"}
	last := scanAt(t1.Add(time.Hour), []model.ScanItem{item(model.ItemPlugin, "akismet", "Akismet", "active", "5.7.2")}, editing)
	scans := []model.Scan{first, failed, last}
	model.NoteAll(scans, nil)
	for i, s := range scans {
		if !s.Recorded || s.RecordedStatus != s.Status() {
			t.Errorf("scan %d noted %v, %q; want its status %q", i, s.Recorded, s.RecordedStatus, s.Status())
		}
	}
	if len(scans[0].RecordedChanges) != 0 || len(scans[1].RecordedChanges) != 0 {
		t.Errorf("the first and the failed scan noted changes: %+v, %+v", scans[0].RecordedChanges, scans[1].RecordedChanges)
	}
	if want := []string{"version plugin:akismet", "finding site"}; !slices.Equal(kinds(scans[2].RecordedChanges), want) {
		t.Errorf("the last scan noted %q, want %q (against the first, the failed one skipped)", kinds(scans[2].RecordedChanges), want)
	}
}
