package app_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/workspace"
)

var agreedAt = time.Date(2026, 10, 30, 14, 12, 0, 0, time.UTC)

func on(v app.LookupsView) []string {
	var ids []string
	for _, d := range v.Destinations {
		if d.On {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

func TestTheUsersChoicesAboutLookups(t *testing.T) {
	ws, _, _ := benchSites(t, sshxtest.NewFake())
	s := app.NewSecurity(ws, func() time.Time { return agreedAt })
	ctx := context.Background()

	v, err := s.Lookups(ctx)
	if err != nil || v.Agreed != "" || v.Asked != "" || len(on(v)) != 5 {
		t.Fatalf("Lookups() at first = %+v, %v; want nothing agreed, every destination on", v, err)
	}
	if p := s.NotNow(ctx); p != nil {
		t.Fatal(p)
	}
	if v, _ = s.Lookups(ctx); v.Agreed != "" || v.Asked != "2026-10-30T14:12:00Z" || ws.LookupAllowed(lookup.WPVulnerability) {
		t.Fatalf("after \"not now\": %+v", v)
	}
	// The consent screen's switches: the domain registries left off.
	if p := s.Agree(ctx, []string{"wpvulnerability", "wordpress.org", "site"}); p != nil {
		t.Fatal(p)
	}
	v, _ = s.Lookups(ctx)
	if v.Agreed != "2026-10-30T14:12:00Z" || !slices.Equal(on(v), []string{"wpvulnerability", "wordpress.org", "site"}) {
		t.Fatalf("after agreeing: %+v", v)
	}
	if !ws.LookupAllowed(lookup.Site) || ws.LookupAllowed(lookup.Registry) {
		t.Error("the destinations allowed do not follow the switches")
	}
	if p := s.SetDestination(ctx, "registry", true); p != nil {
		t.Fatal(p)
	}
	if p := s.SetDestination(ctx, "site", false); p != nil {
		t.Fatal(p)
	}
	if v, _ = s.Lookups(ctx); !slices.Equal(on(v), []string{"wpvulnerability", "wordpress.org", "registry"}) {
		t.Errorf("after the settings' switches: %v", on(v))
	}
	if p := s.Agree(ctx, []string{"example.com"}); code(p) != "problem.unknownDestination" {
		t.Errorf("Agree(unknown) = %+v", p)
	}
	if p := s.Withdraw(ctx); p != nil {
		t.Fatal(p)
	}
	if v, _ = s.Lookups(ctx); v.Agreed != "" || ws.LookupAllowed(lookup.WPVulnerability) {
		t.Errorf("after withdrawing: %+v", v)
	}
}

// A finding set aside shows as such in the site's detail and no longer counts in its status.
func TestIgnoreAFinding(t *testing.T) {
	ws, _, host := benchSites(t, sshxtest.NewFake())
	ctx := context.Background()
	if _, err := ws.Scan(ctx, "Bench", workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	m := app.NewSiteManager(ws.DB())
	rows, err := m.Sites(ctx, host.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	outdated := rows[0]
	// The bench's image adds wp-config-docker.php where only WordPress's own files belong.
	if outdated.Status != "danger" || outdated.Worst == nil || outdated.Worst.Kind != "core-file" {
		t.Fatalf("outdated = %+v, worst %+v", outdated, outdated.Worst)
	}
	key := outdated.Worst.Key

	s := app.NewSecurity(ws, func() time.Time { return agreedAt })
	if p := s.Ignore(ctx, outdated.ID, key, "", "  "); code(p) != "problem.reasonRequired" {
		t.Fatalf("Ignore() permanently without a reason = %+v", p)
	}
	if p := s.Ignore(ctx, outdated.ID, key, "", " added by the container image "); p != nil {
		t.Fatal(p)
	}
	site, err := m.SiteDetail(ctx, outdated.ID)
	if err != nil {
		t.Fatal(err)
	}
	if site.Scan.Status != "warn" || site.Scan.Counts.Urgent != 0 || site.Scan.Counts.Ignored != 1 {
		t.Fatalf("after ignoring: status %q, counts %+v", site.Scan.Status, site.Scan.Counts)
	}
	i := slices.IndexFunc(site.Scan.Findings, func(f app.FindingView) bool { return f.Key == key })
	if f := site.Scan.Findings[i]; f.Ignore == nil || f.Ignore.Reason != "added by the container image" || f.Ignore.Created != "2026-10-30T14:12:00Z" {
		t.Errorf("ignored finding = %+v", f)
	}
	if p := s.Unignore(ctx, outdated.ID, key); p != nil {
		t.Fatal(p)
	}
	if site, _ = m.SiteDetail(ctx, outdated.ID); site.Scan.Status != "danger" {
		t.Errorf("after stopping ignoring: %q", site.Scan.Status)
	}
}

// The findings come with what the interface needs to write them: their facts, the counts by level,
// and when the files were compared.
func TestSiteDetailCarriesTheFindings(t *testing.T) {
	ws, _, host := benchSites(t, sshxtest.NewFake())
	ctx := context.Background()
	if _, err := ws.Scan(ctx, "Bench", workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := app.NewSiteManager(ws.DB()).Sites(ctx, host.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	site, err := app.NewSiteManager(ws.DB()).SiteDetail(ctx, rows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	scan := site.Scan
	if scan.FilesChecked == "" || scan.Counts.Urgent != 1 || scan.Counts.NotChecked != 5 {
		t.Fatalf("files checked %q, counts %+v", scan.FilesChecked, scan.Counts)
	}
	for _, f := range scan.Findings {
		if f.Facts == nil || !strings.Contains(f.Key, f.Kind) {
			t.Errorf("finding %+v", f)
		}
		if f.Kind == "check" && f.Facts["reason"] != "off" {
			t.Errorf("a check not run before the user agreed = %+v, want reason off", f)
		}
	}
	if scan.Findings[0].Level != "urgent" {
		t.Errorf("first finding = %+v, want the worst first", scan.Findings[0])
	}
	// A plugin or theme carries the worst finding about it.
	for _, e := range scan.Extensions {
		var worst *app.FindingView
		for _, f := range scan.Findings {
			if f.Subject == e.Kind+":"+e.Slug && f.Ignore == nil && (f.Level == "urgent" || f.Level == "attention") {
				worst = &f
				break
			}
		}
		if (worst == nil) != (e.Worst == nil) || (worst != nil && worst.Key != e.Worst.Key) {
			t.Errorf("%s %s: worst finding %+v, want %+v", e.Kind, e.Slug, e.Worst, worst)
		}
	}
}
