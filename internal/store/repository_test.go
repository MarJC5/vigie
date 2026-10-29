package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

// migrated opens a temporary database with the shipped migrations applied.
func migrated(t *testing.T) *store.DB {
	t.Helper()
	db, _ := openTemp(t)
	if err := db.Migrate(context.Background(), store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	return db
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSaveAndLoadTree(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)

	clients := model.Folder{ID: model.NewID(), Name: "Clients", Color: model.ColorBlue, Notes: "Paying customers"}
	valais := model.Folder{ID: model.NewID(), ParentID: clients.ID, Name: "Valais", Position: 2}
	host := model.Host{
		ID: model.NewID(), FolderID: valais.ID, Name: "Infomaniak", Hostname: "example.org", Port: 2222,
		User: "deploy", KeyPath: "~/.ssh/id_ed25519", WPCLI: "wp-cli", MaxConcurrent: 3,
		HostKeyFingerprint: "SHA256:abc", Color: model.ColorGreen, Notes: "Shared hosting",
	}
	site := model.Site{
		ID: model.NewID(), HostID: host.ID, Name: "network.example", Path: "~/sites/network", URL: "https://network.example",
		WPCLI: "/opt/wp", Color: model.ColorRed, Notes: "Multisite", Tags: []string{"client-a", "priority"},
		Multisite: true,
		Subsites: []model.Subsite{
			{BlogID: 1, URL: "https://network.example", Public: true, Pinned: true},
			{BlogID: 2, URL: "https://network.example/old", Archived: true},
		},
	}
	must(t, db.SaveFolder(ctx, clients))
	must(t, db.SaveFolder(ctx, valais))
	must(t, db.SaveHost(ctx, host))
	must(t, db.SaveSite(ctx, site))

	tree, err := db.LoadTree(ctx)
	must(t, err)
	gotFolder, _ := tree.Folder(valais.ID)
	gotHost, _ := tree.Host(host.ID)
	gotSite, _ := tree.Site(site.ID)
	if gotFolder != valais {
		t.Errorf("folder = %+v, want %+v", gotFolder, valais)
	}
	if gotHost != host {
		t.Errorf("host = %+v, want %+v", gotHost, host)
	}
	if !siteEqual(gotSite, site) {
		t.Errorf("site = %+v, want %+v", gotSite, site)
	}
	node, err := tree.Resolve("Clients/Valais/Infomaniak/network.example")
	if err != nil || node.ID != site.ID {
		t.Errorf("Resolve after load = %+v, %v", node, err)
	}
}

func siteEqual(a, b model.Site) bool {
	return a.ID == b.ID && a.HostID == b.HostID && a.Name == b.Name && a.Path == b.Path && a.URL == b.URL &&
		a.WPCLI == b.WPCLI && a.Color == b.Color && a.Notes == b.Notes && a.Position == b.Position &&
		a.Multisite == b.Multisite && slices.Equal(a.Tags, b.Tags) && slices.Equal(a.Subsites, b.Subsites)
}

func TestSaveUpdatesInPlace(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	a := model.Folder{ID: model.NewID(), Name: "A"}
	b := model.Folder{ID: model.NewID(), Name: "B"}
	host := model.Host{ID: model.NewID(), FolderID: a.ID, Name: "h", Alias: "h"}
	must(t, db.SaveFolder(ctx, a))
	must(t, db.SaveFolder(ctx, b))
	must(t, db.SaveHost(ctx, host))

	host.Name, host.FolderID = "renamed", b.ID // rename and move
	must(t, db.SaveHost(ctx, host))
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "s", Tags: []string{"x"}}
	must(t, db.SaveSite(ctx, site))
	site.Tags = []string{"y"}
	must(t, db.SaveSite(ctx, site))

	tree, err := db.LoadTree(ctx)
	must(t, err)
	if node, err := tree.Resolve("B/renamed/s"); err != nil || node.ID != site.ID {
		t.Fatalf("Resolve(B/renamed/s) = %+v, %v", node, err)
	}
	if s, _ := tree.Site(site.ID); !slices.Equal(s.Tags, []string{"y"}) {
		t.Fatalf("tags after update = %v, want [y]", s.Tags)
	}
}

func TestSiblingNamesMustBeUnique(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	clients := model.Folder{ID: model.NewID(), Name: "Clients"}
	must(t, db.SaveFolder(ctx, clients))
	host := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Infomaniak", Alias: "i"}
	must(t, db.SaveHost(ctx, host))
	must(t, db.SaveSite(ctx, model.Site{ID: model.NewID(), HostID: host.ID, Name: "misits.ch"}))

	cases := map[string]error{
		"folder at the root, other case":  db.SaveFolder(ctx, model.Folder{ID: model.NewID(), Name: "CLIENTS"}),
		"host at the root, like a folder": db.SaveHost(ctx, model.Host{ID: model.NewID(), Name: "clients", Alias: "c"}),
		"folder next to a host":           db.SaveFolder(ctx, model.Folder{ID: model.NewID(), ParentID: clients.ID, Name: "infomaniak"}),
		"site, other case":                db.SaveSite(ctx, model.Site{ID: model.NewID(), HostID: host.ID, Name: "MISITS.CH"}),
	}
	for name, err := range cases {
		if !errors.Is(err, model.ErrDuplicateName) {
			t.Errorf("%s: %v, want ErrDuplicateName", name, err)
		}
	}
	// Renaming a record to its own name is not a duplicate.
	must(t, db.SaveFolder(ctx, clients))
}

func TestSaveValidatesRecords(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	if err := db.SaveFolder(ctx, model.Folder{ID: model.NewID(), Name: "a/b"}); !errors.Is(err, model.ErrSlashInName) {
		t.Errorf("folder with a slash: %v", err)
	}
	if err := db.SaveHost(ctx, model.Host{ID: model.NewID(), Name: "h"}); !errors.Is(err, model.ErrNoConnection) {
		t.Errorf("host without connection: %v", err)
	}
	if err := db.SaveFolder(ctx, model.Folder{ID: model.NewID(), ParentID: model.NewID(), Name: "x"}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("folder with unknown parent: %v", err)
	}
	if err := db.SaveSite(ctx, model.Site{ID: model.NewID(), HostID: model.NewID(), Name: "s"}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("site with unknown host: %v", err)
	}
}

func TestFolderCannotMoveIntoItself(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	top := model.Folder{ID: model.NewID(), Name: "Top"}
	child := model.Folder{ID: model.NewID(), ParentID: top.ID, Name: "Child"}
	must(t, db.SaveFolder(ctx, top))
	must(t, db.SaveFolder(ctx, child))
	for _, parent := range []model.ID{top.ID, child.ID} {
		top.ParentID = parent
		if err := db.SaveFolder(ctx, top); !errors.Is(err, store.ErrCycle) {
			t.Errorf("move Top under %s: %v, want ErrCycle", parent, err)
		}
	}
}

func TestDeleteFolderOnlyWhenEmpty(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	folder := model.Folder{ID: model.NewID(), Name: "F"}
	host := model.Host{ID: model.NewID(), FolderID: folder.ID, Name: "h", Alias: "h"}
	must(t, db.SaveFolder(ctx, folder))
	must(t, db.SaveHost(ctx, host))

	if err := db.DeleteFolder(ctx, folder.ID); !errors.Is(err, store.ErrNotEmpty) {
		t.Errorf("delete folder with a host: %v, want ErrNotEmpty", err)
	}
	must(t, db.DeleteHost(ctx, host.ID))
	must(t, db.DeleteFolder(ctx, folder.ID))
	if err := db.DeleteFolder(ctx, folder.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete a missing folder: %v, want ErrNotFound", err)
	}
}

// Deleting a host removes its sites, with their tags, subsites and scans.
func TestDeleteHostTakesItsSites(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	host := model.Host{ID: model.NewID(), Name: "h", Alias: "h"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "s", Tags: []string{"t"},
		Subsites: []model.Subsite{{BlogID: 1, URL: "https://s"}}}
	other := model.Host{ID: model.NewID(), Name: "other", Alias: "other"}
	kept := model.Site{ID: model.NewID(), HostID: other.ID, Name: "kept"}
	must(t, db.SaveHost(ctx, host))
	must(t, db.SaveSite(ctx, site))
	must(t, db.SaveHost(ctx, other))
	must(t, db.SaveSite(ctx, kept))
	must(t, db.SaveScan(ctx, model.Scan{ID: model.NewID(), SiteID: site.ID, Outcome: model.ScanCompleted}))

	must(t, db.DeleteHost(ctx, host.ID))
	tree, err := db.LoadTree(ctx)
	must(t, err)
	if _, ok := tree.Site(site.ID); ok {
		t.Error("site still in the tree after its host was deleted")
	}
	if _, ok := tree.Site(kept.ID); !ok {
		t.Error("the site of another host was deleted")
	}
	if n, err := db.ScanCount(ctx, site.ID); err != nil || n != 0 {
		t.Errorf("scans of the deleted site = %d, %v; want 0", n, err)
	}
	if err := db.DeleteHost(ctx, host.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete a missing host: %v, want ErrNotFound", err)
	}
	must(t, db.DeleteSite(ctx, kept.ID))
	if err := db.DeleteSite(ctx, kept.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete a missing site: %v, want ErrNotFound", err)
	}
}

// A site moved to another account is removed and found again: its path and scans belong to its host.
func TestSiteStaysUnderItsHost(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	a := model.Host{ID: model.NewID(), Name: "a", Alias: "a"}
	b := model.Host{ID: model.NewID(), Name: "b", Alias: "b"}
	site := model.Site{ID: model.NewID(), HostID: a.ID, Name: "s"}
	must(t, db.SaveHost(ctx, a))
	must(t, db.SaveHost(ctx, b))
	must(t, db.SaveSite(ctx, site))

	site.HostID = b.ID
	if err := db.SaveSite(ctx, site); !errors.Is(err, model.ErrSiteMoved) {
		t.Fatalf("save the site under another host: %v, want ErrSiteMoved", err)
	}
	site.HostID, site.Name = a.ID, "renamed"
	must(t, db.SaveSite(ctx, site))
}

func TestSaveRejectsUnknownColors(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	host := model.Host{ID: model.NewID(), Name: "h", Alias: "h"}
	must(t, db.SaveHost(ctx, host))
	for name, err := range map[string]error{
		"folder": db.SaveFolder(ctx, model.Folder{ID: model.NewID(), Name: "f", Color: 8}),
		"host":   db.SaveHost(ctx, model.Host{ID: model.NewID(), Name: "g", Alias: "g", Color: -1}),
		"site":   db.SaveSite(ctx, model.Site{ID: model.NewID(), HostID: host.ID, Name: "s", Color: 9}),
	} {
		if !errors.Is(err, model.ErrUnknownColor) {
			t.Errorf("%s with an unknown color: %v, want ErrUnknownColor", name, err)
		}
	}
}

func TestDBImplementsRepository(t *testing.T) {
	var _ store.Repository = migrated(t)
}

func TestAddHostWithSitesIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	sites := []model.Site{
		{ID: model.NewID(), HostID: host.ID, Name: "a.example.test", Path: "~/sites/a"},
		{ID: model.NewID(), HostID: host.ID, Name: "A.example.test", Path: "~/sites/b"}, // same name, other case
	}
	if err := db.AddHostWithSites(ctx, host, sites); !errors.Is(err, model.ErrDuplicateName) {
		t.Fatalf("AddHostWithSites() = %v, want ErrDuplicateName", err)
	}
	tree, err := db.LoadTree(ctx)
	must(t, err)
	if _, ok := tree.Host(host.ID); ok {
		t.Fatal("the host was stored although a site was refused")
	}

	sites[1].Name = "b.example.test"
	must(t, db.AddHostWithSites(ctx, host, sites))
	tree, err = db.LoadTree(ctx)
	must(t, err)
	if got := tree.SitesUnder(model.Node{Kind: model.KindHost, ID: host.ID}); len(got) != 2 {
		t.Fatalf("stored sites = %d, want 2", len(got))
	}
}
