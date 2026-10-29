// Command seed prepares a data directory for the end-to-end test of the window: the interface
// language when -lang is given, and unless -empty a fictional site manager that mirrors the fixture of the interface
// stories, with fictional scans of two of its sites (never real hosts or sites).
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
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// scans are the latest scans of the first site (up to date) and the second (updates pending);
// the network is left never scanned.
func scans(upToDate, outdated model.ID) []model.Scan {
	finished := time.Now().Add(-3 * time.Minute)
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
	return []model.Scan{ok, warn}
}
