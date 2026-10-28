// Command seed prepares a data directory for the end-to-end test of the window: the interface
// language when -lang is given, and unless -empty a fictional site manager that mirrors the fixture of the interface
// stories (never real hosts or sites).
//
// Usage: seed [-lang en|fr] [-empty] <data-dir>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
