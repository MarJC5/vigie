package workspace_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
)

// manyBench is Clients/Shared (a.example, b.example) with Clients/Inner, Personal/VPS (c.example)
// and an empty folder Archives.
type manyBench struct {
	ws                                *workspace.Workspace
	db                                *store.DB
	fake                              *sshxtest.Fake
	clients, inner, personal, archive model.Node
	shared, vps                       model.Node
	a, b, c                           model.Node
}

func newManyBench(t *testing.T) manyBench {
	t.Helper()
	ctx := context.Background()
	db := emptyDB(t)
	clients, personal, archives := model.Folder{ID: model.NewID(), Name: "Clients"}, model.Folder{ID: model.NewID(), Name: "Personal", Position: 1},
		model.Folder{ID: model.NewID(), Name: "Archives", Position: 2}
	inner := model.Folder{ID: model.NewID(), ParentID: clients.ID, Name: "Inner"}
	shared := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Shared", Alias: "shared", Position: 1}
	vps := model.Host{ID: model.NewID(), FolderID: personal.ID, Name: "VPS", Alias: "vps"}
	a := model.Site{ID: model.NewID(), HostID: shared.ID, Name: "a.example", Tags: []string{"Client Dupont"}}
	b := model.Site{ID: model.NewID(), HostID: shared.ID, Name: "b.example", Position: 1}
	c := model.Site{ID: model.NewID(), HostID: vps.ID, Name: "c.example"}
	for _, err := range []error{db.SaveFolder(ctx, clients), db.SaveFolder(ctx, personal), db.SaveFolder(ctx, archives), db.SaveFolder(ctx, inner),
		db.SaveHost(ctx, shared), db.SaveHost(ctx, vps), db.SaveSite(ctx, a), db.SaveSite(ctx, b), db.SaveSite(ctx, c)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := sshxtest.NewFake()
	folder := func(f model.Folder) model.Node { return model.Node{Kind: model.KindFolder, ID: f.ID} }
	site := func(s model.Site) model.Node { return model.Node{Kind: model.KindSite, ID: s.ID} }
	return manyBench{
		ws: workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now}), db: db, fake: fake,
		clients: folder(clients), inner: folder(inner), personal: folder(personal), archive: folder(archives),
		shared: model.Node{Kind: model.KindHost, ID: shared.ID}, vps: model.Node{Kind: model.KindHost, ID: vps.ID},
		a: site(a), b: site(b), c: site(c),
	}
}

func (m manyBench) tree(t *testing.T) *model.Tree {
	t.Helper()
	tree, err := m.db.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// Moved together: the hosts and folders go, the sites stay under their hosts; a move that would
// put a folder into itself is refused as a whole (ADR-0034).
func TestMoveMany(t *testing.T) {
	ctx := context.Background()
	m := newManyBench(t)
	if err := m.ws.MoveMany(ctx, []model.Node{m.shared, m.vps, m.a}, m.archive.ID); err != nil {
		t.Fatal(err)
	}
	tree := m.tree(t)
	if tree.PathOf(m.a) != "Archives/Shared/a.example" || tree.PathOf(m.vps) != "Archives/VPS" {
		t.Errorf("paths %q, %q; want both hosts in Archives", tree.PathOf(m.a), tree.PathOf(m.vps))
	}
	if err := m.ws.MoveMany(ctx, []model.Node{m.personal, m.clients}, m.inner.ID); !errors.Is(err, store.ErrCycle) {
		t.Errorf("into its own folder: %v, want ErrCycle", err)
	}
	if tree := m.tree(t); tree.PathOf(m.personal) != "Personal" {
		t.Errorf("Personal moved to %q although the move was refused", tree.PathOf(m.personal))
	}
	if err := m.ws.MoveMany(ctx, []model.Node{m.a, m.b}, m.archive.ID); !errors.Is(err, workspace.ErrNothingToMove) {
		t.Errorf("sites only: %v, want ErrNothingToMove", err)
	}
}

func TestSetColorMany(t *testing.T) {
	ctx := context.Background()
	m := newManyBench(t)
	if err := m.ws.SetColorMany(ctx, []model.Node{m.clients, m.vps, m.c}, model.ColorGreen); err != nil {
		t.Fatal(err)
	}
	tree := m.tree(t)
	f, _ := tree.Folder(m.clients.ID)
	h, _ := tree.Host(m.vps.ID)
	s, _ := tree.Site(m.c.ID)
	if f.Color != model.ColorGreen || h.Color != model.ColorGreen || s.Color != model.ColorGreen {
		t.Errorf("colors %s %s %s, want green", f.Color, h.Color, s.Color)
	}
}

// A tag goes on every site the selection holds, in the spelling another site has; then off.
func TestTagMany(t *testing.T) {
	ctx := context.Background()
	m := newManyBench(t)
	if err := m.ws.TagMany(ctx, []model.Node{m.vps, m.b}, "client dupont", true); err != nil {
		t.Fatal(err)
	}
	tags := func() map[string][]string {
		got := map[string][]string{}
		for _, s := range m.tree(t).SitesUnder(model.Root) {
			got[s.Name] = s.Tags
		}
		return got
	}
	got := tags()
	if !slices.Equal(got["b.example"], []string{"Client Dupont"}) || !slices.Equal(got["c.example"], []string{"Client Dupont"}) {
		t.Errorf("tags after adding: %v", got)
	}
	if err := m.ws.TagMany(ctx, []model.Node{m.shared}, "Client Dupont", false); err != nil {
		t.Fatal(err)
	}
	got = tags()
	if len(got["a.example"]) != 0 || len(got["b.example"]) != 0 || len(got["c.example"]) != 1 {
		t.Errorf("tags after removing from Shared's sites: %v", got)
	}
}

// Deleted together: a folder emptied by the same delete goes; a folder that keeps something
// refuses the whole delete.
func TestDeleteMany(t *testing.T) {
	ctx := context.Background()
	m := newManyBench(t)
	if err := m.ws.DeleteMany(ctx, []model.Node{m.clients, m.a}); !errors.Is(err, store.ErrNotEmpty) {
		t.Fatalf("a folder still holding something: %v, want ErrNotEmpty", err)
	}
	if _, ok := m.tree(t).Site(m.a.ID); !ok {
		t.Error("a.example deleted although the delete was refused")
	}
	if err := m.ws.DeleteMany(ctx, []model.Node{m.personal, m.vps, m.archive}); err != nil {
		t.Fatal(err)
	}
	tree := m.tree(t)
	if _, ok := tree.Folder(m.personal.ID); ok {
		t.Error("Personal still there")
	}
	if len(tree.SitesUnder(model.Root)) != 2 {
		t.Errorf("%d sites left, want Shared's two", len(tree.SitesUnder(model.Root)))
	}
}

// With their backups: a command per host; the hosts that cannot be reached leave theirs, named.
func TestDeleteManyWithBackups(t *testing.T) {
	ctx := context.Background()
	m := newManyBench(t)
	line, err := remote.RemoveSiteBackups(m.a.ID, m.b.ID)
	if err != nil {
		t.Fatal(err)
	}
	m.fake.Set(line, sshx.Result{})
	left, err := m.ws.DeleteManyWithBackups(ctx, []model.Node{m.shared, m.c})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(left, []model.ID{m.c.ID}) {
		t.Errorf("left %v, want c.example's, its server unreachable", left)
	}
	if n := len(m.tree(t).SitesUnder(model.Root)); n != 0 {
		t.Errorf("%d sites left, want none", n)
	}
}
