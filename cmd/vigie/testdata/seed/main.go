// Command seed prepares a data directory for the end-to-end test of the window: the interface
// language when -lang is given, and unless -empty a fictional site manager that mirrors the fixture of the interface
// stories, with fictional scans of two of its sites and the history of updates of one (never real
// hosts or sites).
//
// Usage: seed [-lang en|fr] [-empty] <data-dir>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

func main() {
	lang := flag.String("lang", "", "interface language; the system language when empty")
	empty := flag.Bool("empty", false, "no folders, hosts or sites")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: seed [-lang en|fr] [-empty] <data-dir>")
		os.Exit(2)
	}
	if err := seed(context.Background(), flag.Arg(0), *lang, *empty); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func seed(ctx context.Context, dir, lang string, empty bool) error {
	if err := store.EnsureDataDir(dir); err != nil {
		return err
	}
	db, err := store.Open(ctx, filepath.Join(dir, "vigie.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx, store.Migrations(), "seed"); err != nil {
		return err
	}
	// Stored as a choice, so the test does not depend on the language of the machine.
	if lang != "" {
		if err := app.NewPreferences(db, nil).SetLanguage(ctx, lang); err != nil {
			return err
		}
	}
	if empty {
		return nil
	}

	clients := model.Folder{ID: model.NewID(), Name: "Clients", Color: model.ColorBlue}
	shared := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Shared hosting", Alias: "shared"}
	personal := model.Folder{ID: model.NewID(), Name: "Personal", Position: 1}
	vps := model.Host{ID: model.NewID(), FolderID: personal.ID, Name: "Managed VPS", Alias: "vps"}
	sites := []model.Site{
		{ID: model.NewID(), HostID: shared.ID, Name: "atelier-exemple.ch"},
		{ID: model.NewID(), HostID: shared.ID, Name: "boulangerie-exemple.ch", Color: model.ColorOrange, Position: 1},
		{ID: model.NewID(), HostID: shared.ID, Name: "reseau-ecoles.ch", Position: 2, Multisite: true,
			Subsites: []model.Subsite{{BlogID: 1, URL: "https://reseau-ecoles.ch"}, {BlogID: 2, URL: "https://reseau-ecoles.ch/b"}}},
	}

	errs := []error{db.SaveFolder(ctx, clients), db.SaveHost(ctx, shared), db.SaveFolder(ctx, personal), db.SaveHost(ctx, vps)}
	for _, s := range sites {
		errs = append(errs, db.SaveSite(ctx, s))
	}
	for _, scan := range scans(sites[0].ID, sites[1].ID) {
		errs = append(errs, db.SaveScan(ctx, scan))
	}
	errs = append(errs, ignore(ctx, db, sites[1].ID))
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return history(ctx, db, sites[1].ID)
}

// history stores two updates of the site with updates pending: a week ago, one that went well;
// today, one stopped by the check after its second plugin, whose rollback is offered.
func history(ctx context.Context, db *store.DB, site model.ID) error {
	now := time.Now().Add(-10 * time.Minute)
	step := func(at time.Time, kind model.StepKind, item model.ItemKind, slug, before, after string) model.OperationStep {
		return model.OperationStep{Step: kind, ItemKind: item, Slug: slug, VersionBefore: before, VersionAfter: after,
			Started: at, Finished: at.Add(2 * time.Second), Outcome: model.StepDone}
	}
	week := now.Add(-7 * 24 * time.Hour)
	failing := step(now.Add(14*time.Second), model.StepCheck, model.ItemPlugin, "woocommerce", "", "")
	failing.Outcome, failing.Message = model.StepFailed, "http://boulangerie-exemple.ch/ answers 500 (critical-error)"
	runs := []struct {
		op    model.Operation
		steps []model.OperationStep
	}{
		{
			model.Operation{ID: model.NewID(), SiteID: site, Kind: model.OperationUpdate, Started: week, Finished: week.Add(31 * time.Second),
				Outcome: model.OperationCompleted, Summary: "1 of 1", BackupDir: "~/.vigie/backups/" + site.String() + "/" + week.UTC().Format("20060102-150405")},
			[]model.OperationStep{
				step(week, model.StepBackupDatabase, "", "", "", ""),
				step(week.Add(2*time.Second), model.StepBackupFiles, model.ItemPlugin, "akismet", "", ""),
				step(week.Add(4*time.Second), model.StepUpdate, model.ItemPlugin, "akismet", "5.2", "5.3"),
				step(week.Add(6*time.Second), model.StepCheck, model.ItemPlugin, "akismet", "", ""),
			},
		},
		{
			model.Operation{ID: model.NewID(), SiteID: site, Kind: model.OperationUpdate, Started: now, Finished: now.Add(16 * time.Second),
				Outcome: model.OperationFailed, Summary: "1 of 2", BackupDir: "~/.vigie/backups/" + site.String() + "/" + now.UTC().Format("20060102-150405")},
			[]model.OperationStep{
				step(now, model.StepBackupDatabase, "", "", "", ""),
				step(now.Add(2*time.Second), model.StepBackupFiles, model.ItemPlugin, "akismet", "", ""),
				step(now.Add(4*time.Second), model.StepBackupFiles, model.ItemPlugin, "woocommerce", "", ""),
				step(now.Add(6*time.Second), model.StepUpdate, model.ItemPlugin, "akismet", "5.3", "5.7.2"),
				step(now.Add(8*time.Second), model.StepCheck, model.ItemPlugin, "akismet", "", ""),
				step(now.Add(10*time.Second), model.StepUpdate, model.ItemPlugin, "woocommerce", "9.8.1", "10.2.0"),
				failing,
			},
		},
	}
	for _, r := range runs {
		r.op.PID = os.Getpid()
		if err := db.StartOperation(ctx, r.op); err != nil {
			return err
		}
		for i, st := range r.steps {
			st.Position = i
			if err := db.SaveOperationStep(ctx, r.op.ID, st); err != nil {
				return err
			}
		}
		if err := db.FinishOperation(ctx, r.op.ID, r.op.Outcome, r.op.Finished, r.op.Summary); err != nil {
			return err
		}
	}
	return nil
}

// scans are the latest scans of the first site (up to date) and the second (updates pending), with
// the complete one before of the second; the network is left never scanned.
func scans(upToDate, outdated model.ID) []model.Scan {
	finished := time.Now().Add(-3 * time.Minute)
	earlierID := model.NewID() // identifiers are ordered in time: the earlier scan's comes first
	base := model.Scan{
		Started: finished.Add(-4 * time.Second), Finished: finished, Outcome: model.ScanCompleted,
		WordPress: "7.1.3", PHPCLI: "8.4.27", WPCLI: "2.12.0",
		CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
	}
	ok := base
	ok.ID, ok.SiteID, ok.Home = model.NewID(), upToDate, "https://atelier-exemple.ch"
	ok.Items = []model.ScanItem{
		{Kind: model.ItemCore, Version: "7.1.3", Update: model.UpdateNone},
		{Kind: model.ItemPlugin, Slug: "akismet", Title: "Akismet Anti-spam", Status: "active", Version: "5.7.2", Update: model.UpdateNone, OnWordPressOrg: true},
	}
	warn := base
	warn.ID, warn.SiteID, warn.Home = model.NewID(), outdated, "https://boulangerie-exemple.ch"
	warn.Items = []model.ScanItem{
		{Kind: model.ItemCore, Status: "minor", Version: "7.1.3", Update: model.UpdateAvailable, UpdateVersion: "7.1.4"},
		{Kind: model.ItemPlugin, Slug: "akismet", Title: "Akismet Anti-spam", Status: "active", Version: "5.3",
			Update: model.UpdateAvailable, UpdateVersion: "5.7.2", OnWordPressOrg: true},
		{Kind: model.ItemLanguage, Slug: "akismet", Language: "fr_FR", Update: model.UpdateAvailable},
	}
	warn.FilesChecked = finished
	// Nothing urgent: the site keeps needing attention for its updates. Hidden from search engines is
	// ignored (see ignores).
	warn.Findings = []model.Finding{
		{Kind: model.KindVulnerability, Subject: "plugin:akismet", Identifier: "fictional-1", Version: "5.3", Level: model.LevelAttention,
			Facts: map[string]string{"name": "Fictional stored cross-site scripting", "severity": "medium", "score": "5.4", "fixed": "5.4"}},
		{Kind: model.KindFileEditing, Subject: "site", Level: model.LevelAttention},
		{Kind: model.KindHiddenFromSearch, Subject: "site", Level: model.LevelAttention},
		{Kind: model.KindCheck, Subject: "site", Identifier: "domain", Level: model.LevelNotChecked, Facts: map[string]string{"reason": "off"}},
		{Kind: model.KindCertificate, Subject: "site", Version: "2026-12-25", Level: model.LevelInfo,
			Facts: map[string]string{"expires": "2026-12-25T10:00:00Z", "issuer": "Fictional CA"}},
	}
	// A scan of the up-to-date site cancelled after its complete one: the complete one stays shown.
	cancelled := model.Scan{ID: model.NewID(), SiteID: upToDate, Started: finished.Add(time.Minute), Finished: finished.Add(time.Minute + time.Second),
		Outcome: model.ScanCancelled, Error: "cancelled"}
	// The complete scan of the site with updates pending two days before: since then, WordPress moved
	// outside Vigie, a plugin and an administrator were removed, file editing was allowed (ADR-0027).
	earlier := warn
	earlier.ID, earlier.Started, earlier.Finished = earlierID, warn.Started.Add(-48*time.Hour), warn.Finished.Add(-48*time.Hour)
	earlier.WordPress, earlier.FilesChecked = "7.1.2", earlier.Finished
	earlier.Items = append([]model.ScanItem{
		{Kind: model.ItemCore, Status: "minor", Version: "7.1.2", Update: model.UpdateAvailable, UpdateVersion: "7.1.4"},
		{Kind: model.ItemPlugin, Slug: "hello-dolly", Title: "Hello Dolly", Status: "inactive", Version: "1.7.2", Update: model.UpdateNone, OnWordPressOrg: true},
	}, warn.Items[1:]...)
	earlier.Findings = []model.Finding{warn.Findings[0], warn.Findings[2], warn.Findings[3], warn.Findings[4],
		{Kind: model.KindAdministrator, Subject: "user:stagiaire", Level: model.LevelInfo, Facts: map[string]string{"two-factor": "on"}}}
	return []model.Scan{earlier, ok, warn, cancelled}
}

// ignore sets aside the outdated site's "hidden from search engines", permanently.
func ignore(ctx context.Context, db *store.DB, site model.ID) error {
	return db.SaveIgnore(ctx, model.Ignore{SiteID: site, Key: model.Finding{Kind: model.KindHiddenFromSearch, Subject: "site"}.Key(),
		Reason: "staging copy, hidden on purpose", Created: time.Now().Add(-24 * time.Hour)})
}
