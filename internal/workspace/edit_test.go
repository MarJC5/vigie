package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
)

// resolve finds a node of the stored tree by its path.
func resolve(t *testing.T, ws *workspace.Workspace, path string) model.Node {
	t.Helper()
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n, err := tree.Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func pathOf(t *testing.T, ws *workspace.Workspace, n model.Node) string {
	t.Helper()
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return tree.PathOf(n)
}

func TestCreateRenameAndMove(t *testing.T) {
	ctx := context.Background()
	ws, _ := bench(t)
	personal, err := ws.CreateFolder(ctx, model.NoID, "Personal")
	if err != nil {
		t.Fatal(err)
	}
	archives, err := ws.CreateFolder(ctx, personal.ID, "Archives")
	if err != nil {
		t.Fatal(err)
	}
	host := resolve(t, ws, "Clients/Bench")
	if err := ws.Move(ctx, host, archives.ID); err != nil {
		t.Fatal(err)
	}
	if err := ws.Rename(ctx, model.Node{Kind: model.KindFolder, ID: archives.ID}, "Old"); err != nil {
		t.Fatal(err)
	}
	site := resolve(t, ws, "Personal/Old/Bench/outdated.example.test")
	if err := ws.Rename(ctx, site, "outdated"); err != nil {
		t.Fatal(err)
	}
	if got := pathOf(t, ws, site); got != "Personal/Old/Bench/outdated" {
		t.Errorf("path after the edits = %q", got)
	}
	if _, err := ws.CreateFolder(ctx, model.NoID, "clients"); !errors.Is(err, model.ErrDuplicateName) {
		t.Errorf("a second Clients at the root: %v, want ErrDuplicateName", err)
	}
	if err := ws.Rename(ctx, host, "  "); !errors.Is(err, model.ErrEmptyName) {
		t.Errorf("empty name: %v, want ErrEmptyName", err)
	}
}

func TestMoveRules(t *testing.T) {
	ctx := context.Background()
	ws, _ := bench(t)
	clients := resolve(t, ws, "Clients")
	inner, err := ws.CreateFolder(ctx, clients.ID, "Inner")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Move(ctx, clients, inner.ID); !errors.Is(err, store.ErrCycle) {
		t.Errorf("folder into its own child: %v, want ErrCycle", err)
	}
	if err := ws.Move(ctx, resolve(t, ws, "Clients/Bench/broken"), inner.ID); !errors.Is(err, model.ErrSiteMoved) {
		t.Errorf("site into a folder: %v, want ErrSiteMoved", err)
	}
	if err := ws.Move(ctx, resolve(t, ws, "Clients/Bench"), model.NewID()); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("host into a missing folder: %v, want ErrNotFound", err)
	}
	if err := ws.Move(ctx, model.Node{Kind: model.KindHost, ID: model.NewID()}, model.NoID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("missing host: %v, want ErrNotFound", err)
	}
	if err := ws.Move(ctx, inner.Node(), model.NoID); err != nil {
		t.Fatal(err)
	}
	if got := pathOf(t, ws, inner.Node()); got != "Inner" {
		t.Errorf("folder moved to the root: path %q", got)
	}
}

func TestSetColor(t *testing.T) {
	ctx := context.Background()
	ws, db := bench(t)
	host := resolve(t, ws, "Clients/Bench")
	if err := ws.SetColor(ctx, host, model.ColorOrange); err != nil {
		t.Fatal(err)
	}
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := tree.Host(host.ID); h.Color != model.ColorOrange || h.Alias != "bench" {
		t.Errorf("host after SetColor = %+v", h)
	}
	if err := ws.SetColor(ctx, host, 9); !errors.Is(err, model.ErrUnknownColor) {
		t.Errorf("unknown color: %v, want ErrUnknownColor", err)
	}
}

func TestDeleteRules(t *testing.T) {
	ctx := context.Background()
	ws, db := bench(t)
	clients, host := resolve(t, ws, "Clients"), resolve(t, ws, "Clients/Bench")
	if err := ws.Delete(ctx, clients); !errors.Is(err, store.ErrNotEmpty) {
		t.Fatalf("folder holding a host: %v, want ErrNotEmpty", err)
	}
	if err := ws.Delete(ctx, host); err != nil {
		t.Fatal(err)
	}
	if err := ws.Delete(ctx, clients); err != nil {
		t.Fatal(err)
	}
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sites := tree.SitesUnder(model.Root); len(sites) != 0 {
		t.Errorf("sites left after deleting their host: %d", len(sites))
	}
}

func TestEditHostClearsTheKeyWhenTheAddressChanges(t *testing.T) {
	ctx := context.Background()
	ws, db := bench(t)
	node := resolve(t, ws, "Clients/Bench")
	tree, _ := db.LoadTree(ctx)
	stored, _ := tree.Host(node.ID)
	stored.HostKeyFingerprint, stored.Color, stored.Notes = "SHA256:abc", model.ColorGreen, "notes"
	if err := db.SaveHost(ctx, stored); err != nil {
		t.Fatal(err)
	}

	edited := stored
	edited.WPCLI, edited.MaxConcurrent, edited.HostKeyFingerprint, edited.Notes = "wp-cli", 3, "", ""
	cleared, err := ws.EditHost(ctx, edited)
	if err != nil || cleared {
		t.Fatalf("EditHost(command only) = %v, %v; want the key kept", cleared, err)
	}
	tree, _ = db.LoadTree(ctx)
	if h, _ := tree.Host(node.ID); h.WPCLI != "wp-cli" || h.MaxConcurrent != 3 || h.HostKeyFingerprint != "SHA256:abc" || h.Notes != "notes" || h.Color != model.ColorGreen {
		t.Errorf("host after editing its command = %+v", h)
	}

	edited.Alias, edited.Hostname, edited.User = "", "ssh2.example.test", "u12ab"
	if cleared, err = ws.EditHost(ctx, edited); err != nil || !cleared {
		t.Fatalf("EditHost(new address) = %v, %v; want the key cleared", cleared, err)
	}
	tree, _ = db.LoadTree(ctx)
	if h, _ := tree.Host(node.ID); h.HostKeyFingerprint != "" || h.Hostname != "ssh2.example.test" {
		t.Errorf("host after a new address = %+v", h)
	}
	if err := ws.SetHostKey(ctx, node.ID, "SHA256:new"); err != nil {
		t.Fatal(err)
	}
	tree, _ = db.LoadTree(ctx)
	if h, _ := tree.Host(node.ID); h.HostKeyFingerprint != "SHA256:new" {
		t.Errorf("fingerprint after SetHostKey = %q", h.HostKeyFingerprint)
	}

	edited.User = ""
	if _, err := ws.EditHost(ctx, edited); !errors.Is(err, model.ErrNoUser) {
		t.Errorf("hostname without user: %v, want ErrNoUser", err)
	}
}

func TestFindNewSitesOnAHost(t *testing.T) {
	ctx := context.Background()
	ws, _ := benchWithSearch(t)
	host := resolve(t, ws, "Clients/Bench")
	found, _, err := ws.DiscoverHost(ctx, host.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]discover.State{}
	for _, f := range found {
		states[f.Path] = f.State
	}
	if states["~/sites/outdated"] != discover.StateAlreadyAdded || states["~/sites/plain"] != discover.StateNew {
		t.Fatalf("DiscoverHost() = %v", states)
	}

	plain := model.Site{ID: model.NewID(), HostID: host.ID, Name: "plain.example.test", Path: "~/sites/plain"}
	clash := model.Site{ID: model.NewID(), HostID: host.ID, Name: "broken", Path: "~/sites/plugin-theme"}
	if err := ws.AddSites(ctx, host.ID, []model.Site{plain, clash}); !errors.Is(err, model.ErrDuplicateName) {
		t.Fatalf("a name already used: %v, want ErrDuplicateName", err)
	}
	other := plain
	other.HostID = model.NewID()
	if err := ws.AddSites(ctx, host.ID, []model.Site{other}); !errors.Is(err, workspace.ErrSiteOfAnotherHost) {
		t.Fatalf("a site of another host: %v, want ErrSiteOfAnotherHost", err)
	}
	if err := ws.AddSites(ctx, host.ID, []model.Site{plain}); err != nil {
		t.Fatal(err)
	}
	if got := pathOf(t, ws, model.Node{Kind: model.KindSite, ID: plain.ID}); got != "Clients/Bench/plain.example.test" {
		t.Errorf("added site at %q", got)
	}
	tree, _ := ws.DB().LoadTree(ctx)
	if _, ok := tree.Site(clash.ID); ok {
		t.Error("the refused batch left a site behind")
	}
}
