package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

func repository(t *testing.T) *store.DB {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTreeIsEmptyForANewDatabase(t *testing.T) {
	tree, err := app.NewSiteManager(repository(t)).Tree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 0 {
		t.Fatalf("Tree() = %+v, want no nodes", tree.Nodes)
	}
}

func TestTreeListsNodesParentsFirstWithEffectiveColors(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	clients := model.Folder{ID: model.NewID(), Name: "Clients", Color: model.ColorBlue}
	host := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Shared hosting", Alias: "shared"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "atelier-exemple.ch", Multisite: true,
		Subsites: []model.Subsite{{BlogID: 1, URL: "https://atelier-exemple.ch"}, {BlogID: 2, URL: "https://atelier-exemple.ch/b"}}}
	for _, err := range []error{db.SaveFolder(ctx, clients), db.SaveHost(ctx, host), db.SaveSite(ctx, site)} {
		if err != nil {
			t.Fatal(err)
		}
	}

	tree, err := app.NewSiteManager(db).Tree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []app.Node{
		{ID: clients.ID.String(), Kind: "folder", Name: "Clients", Color: "blue", Path: "Clients"},
		{ID: host.ID.String(), ParentID: clients.ID.String(), Kind: "host", Name: "Shared hosting", Color: "blue", Path: "Clients/Shared hosting"},
		{ID: site.ID.String(), ParentID: host.ID.String(), Kind: "site", Name: "atelier-exemple.ch", Color: "blue",
			Path: "Clients/Shared hosting/atelier-exemple.ch", Multisite: true, Subsites: 2},
	}
	if len(tree.Nodes) != len(want) {
		t.Fatalf("Tree() has %d nodes, want %d: %+v", len(tree.Nodes), len(want), tree.Nodes)
	}
	for i := range want {
		if tree.Nodes[i] != want[i] {
			t.Errorf("node %d = %+v\nwant %+v", i, tree.Nodes[i], want[i])
		}
	}
}
