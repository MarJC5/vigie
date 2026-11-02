package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

// seedScale stores sites fictional sites, 20 a host, 5 hosts a folder, each scanned every day for
// days days, to measure the window with many sites: about a third with updates pending, one in 20
// with an urgent vulnerability, one in 50 whose latest scan failed, a plugin updated on each site
// one day in 7, and WordPress updated on every site three days ago, as a security release is. The scans are noted at the end, as Vigie notes those stored before ADR-0030.
func seedScale(ctx context.Context, db *store.DB, sites, days int) error {
	var all []model.Site
	var folder model.ID
	for h := 0; h*20 < sites; h++ {
		if h%5 == 0 {
			folder = model.NewID()
			if err := db.SaveFolder(ctx, model.Folder{ID: folder, Name: fmt.Sprintf("Clients %c", 'A'+h/5), Position: h / 5}); err != nil {
				return err
			}
		}
		host := model.Host{ID: model.NewID(), FolderID: folder, Name: fmt.Sprintf("Hosting %02d", h), Alias: fmt.Sprintf("host%02d", h), Position: h % 5}
		var list []model.Site
		for i := 0; i < 20 && h*20+i < sites; i++ {
			n := h*20 + i
			list = append(list, model.Site{ID: model.NewID(), HostID: host.ID, Name: fmt.Sprintf("site-%03d.example.test", n),
				URL: fmt.Sprintf("https://site-%03d.example.test", n), Position: i, Tags: tagsOf(n)})
		}
		if err := db.AddHostWithSites(ctx, host, list); err != nil {
			return err
		}
		all = append(all, list...)
	}
	start := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	for d := 1; d <= days; d++ {
		run := model.Run{ID: model.NewID(), By: model.RunBySchedule, Started: start.Add(time.Duration(d) * 24 * time.Hour), Outcome: model.RunCompleted, Sites: len(all)}
		if err := db.StartRun(ctx, run); err != nil {
			return err
		}
		for n, s := range all {
			if err := db.SaveScan(ctx, scaleScan(s, n, d, days, run)); err != nil {
				return err
			}
		}
		if err := db.EndRun(ctx, run.ID, run.Started.Add(30*time.Minute), model.RunCompleted); err != nil {
			return err
		}
	}
	_, err := db.RecordScans(ctx)
	return err
}

func tagsOf(n int) []string {
	switch {
	case n%7 == 0:
		return []string{"shop"}
	case n%11 == 0:
		return []string{"association", "multilingual"}
	}
	return nil
}

// scaleScan is the scan of site n on day d: 30 plugins of a pool of 80, 3 themes, 5 languages, two
// administrators, and the findings of its kind.
func scaleScan(s model.Site, n, d, days int, run model.Run) model.Scan {
	last := d == days
	at := run.Started.Add(time.Duration(n) * 3 * time.Second)
	wordpress := "7.1.3"
	if d < days-3 {
		wordpress = "7.1.2"
	}
	scan := model.Scan{ID: model.NewID(), SiteID: s.ID, RunID: run.ID, Started: at, Finished: at.Add(20 * time.Second), Outcome: model.ScanCompleted,
		WordPress: wordpress, PHPCLI: "8.4.27", WPCLI: "2.12.0", Home: s.URL,
		CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true, FilesChecked: at}
	if last && n%50 == 49 {
		scan.Outcome, scan.Failure = model.ScanFailed, "unreachable"
		scan.Items, scan.CoreChecked, scan.PluginsChecked, scan.ThemesChecked, scan.LanguagesChecked = nil, false, false, false, false
		return scan
	}
	pending := n%3 == 0
	core := model.ScanItem{Kind: model.ItemCore, Version: wordpress, Update: model.UpdateNone}
	if pending {
		core = model.ScanItem{Kind: model.ItemCore, Status: "minor", Version: wordpress, Update: model.UpdateAvailable, UpdateVersion: "7.1.4"}
	}
	scan.Items = []model.ScanItem{core}
	for p := 0; p < 30; p++ {
		slug := fmt.Sprintf("plugin-%02d", (n*7+p*3)%80)
		// The first plugin moves to its next version once a week.
		minor := 0
		if p == 0 {
			minor = (d + n) / 7
		}
		item := model.ScanItem{Kind: model.ItemPlugin, Slug: slug, Title: "Plugin " + slug[7:], Status: "active",
			Version: "2." + strconv.Itoa(minor%10) + ".0", Update: model.UpdateNone, OnWordPressOrg: p%6 != 5}
		if p%5 == 4 {
			item.Status = "inactive"
		}
		if pending && p%4 == 0 {
			item.Update, item.UpdateVersion = model.UpdateAvailable, "3.0.0"
		}
		scan.Items = append(scan.Items, item)
	}
	for _, t := range []string{"twentytwentyfive", "twentytwentyfour", "astra"} {
		item := model.ScanItem{Kind: model.ItemTheme, Slug: t, Title: t, Status: "inactive", Version: "1.2", Update: model.UpdateNone, OnWordPressOrg: true}
		if t == "astra" {
			item.Status = "active"
		}
		scan.Items = append(scan.Items, item)
	}
	for p := 0; p < 5; p++ {
		scan.Items = append(scan.Items, model.ScanItem{Kind: model.ItemLanguage, Slug: fmt.Sprintf("plugin-%02d", (n*7+p*3)%80), Language: "fr_FR", Update: model.UpdateNone})
	}
	two := "on"
	if n%4 == 0 {
		two = "off"
	}
	scan.Findings = []model.Finding{
		{Kind: model.KindAdministrator, Subject: "user:admin", Level: model.LevelInfo, Facts: map[string]string{"two-factor": "on", "email": "admin@" + s.Name}},
		{Kind: model.KindAdministrator, Subject: "user:editor", Level: levelOf(two), Facts: map[string]string{"two-factor": two, "email": "editor@" + s.Name}},
		{Kind: model.KindCertificate, Subject: "site", Version: "2027-01-01", Level: model.LevelInfo, Facts: map[string]string{"expires": "2027-01-01T00:00:00Z"}},
	}
	if n%20 == 0 {
		scan.Findings = append(scan.Findings, model.Finding{Kind: model.KindVulnerability, Subject: "plugin:" + scan.Items[1].Slug, Identifier: "fictional-" + strconv.Itoa(n),
			Version: scan.Items[1].Version, Level: model.LevelUrgent, Facts: map[string]string{"name": "Fictional SQL injection", "severity": "high", "score": "8.8"}})
	}
	if n%9 == 0 {
		scan.Findings = append(scan.Findings, model.Finding{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention})
	}
	return scan
}

func levelOf(twoFactor string) model.FindingLevel {
	if twoFactor == "off" {
		return model.LevelAttention
	}
	return model.LevelInfo
}
