package app_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/workspace"
)

// editor serves a tree holding Clients/Bench with one site.
func editor(t *testing.T) (*app.Editor, *workspace.Workspace, map[string]string) {
	t.Helper()
	ctx := context.Background()
	db := repository(t)
	clients := model.Folder{ID: model.NewID(), Name: "Clients"}
	host := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Bench", Hostname: "ssh.example.test", Port: 2222,
		User: "u12ab", WPCLI: "wp-cli", HostKeyFingerprint: "SHA256:abc"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "plain", Path: "~/sites/plain"}
	for _, err := range []error{db.SaveFolder(ctx, clients), db.SaveHost(ctx, host), db.SaveSite(ctx, site)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	ws := workspace.New(db, workspace.Deps{Runner: sshxtest.NewFake(), Now: time.Now})
	ids := map[string]string{"clients": clients.ID.String(), "host": host.ID.String(), "site": site.ID.String()}
	return app.NewEditor(ws), ws, ids
}

func sitePath(t *testing.T, ws *workspace.Workspace, id string) string {
	t.Helper()
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return tree.PathOf(model.Node{Kind: model.KindSite, ID: uuid.MustParse(id)})
}

func TestEditTheTree(t *testing.T) {
	ctx := context.Background()
	e, ws, ids := editor(t)
	created := e.CreateFolder(ctx, "", "Personal")
	if created.Problem != nil || created.ID == "" {
		t.Fatalf("CreateFolder() = %+v", created)
	}
	if p := e.Move(ctx, ids["host"], created.ID); p != nil {
		t.Fatal(p)
	}
	if p := e.Rename(ctx, ids["site"], "plain.example.test"); p != nil {
		t.Fatal(p)
	}
	if p := e.SetColor(ctx, ids["host"], "orange"); p != nil {
		t.Fatal(p)
	}
	if p := e.SetNotes(ctx, ids["site"], "**Client**: Plain"); p != nil {
		t.Fatal(p)
	}
	if p := e.SetAdminURL(ctx, ids["site"], " https://plain.example.test/connexion "); p != nil {
		t.Fatal(p)
	}
	if got := sitePath(t, ws, ids["site"]); got != "Personal/Bench/plain.example.test" {
		t.Errorf("site path = %q", got)
	}
	tree, _ := ws.DB().LoadTree(ctx)
	if h, _ := tree.Host(uuid.MustParse(ids["host"])); h.Color != model.ColorOrange {
		t.Errorf("host color = %s", h.Color)
	}
	if s, _ := tree.Site(uuid.MustParse(ids["site"])); s.Notes != "**Client**: Plain" || s.AdminURL != "https://plain.example.test/connexion" {
		t.Errorf("site notes = %q, admin address %q", s.Notes, s.AdminURL)
	}
	if p := e.Delete(ctx, ids["clients"], false).Problem; p != nil {
		t.Fatal(p)
	}
}

func TestEditProblems(t *testing.T) {
	ctx := context.Background()
	e, _, ids := editor(t)
	inner := e.CreateFolder(ctx, ids["clients"], "Inner")
	for name, tc := range map[string]struct {
		got  *app.Problem
		want string
	}{
		"folder holding a host":   {e.Delete(ctx, ids["clients"], false).Problem, "problem.folderNotEmpty"},
		"folder into its child":   {e.Move(ctx, ids["clients"], inner.ID), "problem.moveIntoItself"},
		"site into a folder":      {e.Move(ctx, ids["site"], inner.ID), "problem.siteMoved"},
		"unknown color":           {e.SetColor(ctx, ids["host"], "purple"), "problem.unknownColor"},
		"notes too long":          {e.SetNotes(ctx, ids["host"], strings.Repeat("x", model.MaxNotes+1)), "problem.notesTooLong"},
		"admin address not web":   {e.SetAdminURL(ctx, ids["site"], "ftp://plain.example.test"), "problem.invalidAddress"},
		"admin address no host":   {e.SetAdminURL(ctx, ids["site"], "https:///wp-admin"), "problem.invalidAddress"},
		"admin address of a host": {e.SetAdminURL(ctx, ids["host"], "https://plain.example.test"), "problem.notFound"},
		"name taken":              {e.Rename(ctx, inner.ID, "bench"), "problem.duplicateName"},
		"empty name":              {e.CreateFolder(ctx, "", " ").Problem, "problem.emptyName"},
		"missing node":            {e.Rename(ctx, model.NewID().String(), "x"), "problem.notFound"},
		"not an id":               {e.Delete(ctx, "nope", false).Problem, "problem.notFound"},
		"into a missing folder":   {e.Move(ctx, ids["host"], model.NewID().String()), "problem.folderNotFound"},
		"folder under a site":     {e.CreateFolder(ctx, ids["site"], "x").Problem, "problem.folderNotFound"},
	} {
		if code(tc.got) != tc.want {
			t.Errorf("%s: %+v, want %s", name, tc.got, tc.want)
		}
	}
}

func TestEditAHost(t *testing.T) {
	ctx := context.Background()
	e, ws, ids := editor(t)
	d, err := e.Host(ctx, ids["host"])
	if err != nil {
		t.Fatal(err)
	}
	want := app.HostDetails{Name: "Bench", Hostname: "ssh.example.test", Port: 2222, User: "u12ab", WPCLI: "wp-cli",
		FolderID: ids["clients"]}
	if d != want {
		t.Fatalf("Host() = %+v, want %+v", d, want)
	}
	d.WPCLI, d.MaxConcurrent = "wp", 3
	if got := e.EditHost(ctx, ids["host"], d); got.Problem != nil || got.KeyCleared {
		t.Fatalf("EditHost(command) = %+v, want the key kept", got)
	}
	d.Hostname = "ssh2.example.test"
	if got := e.EditHost(ctx, ids["host"], d); got.Problem != nil || !got.KeyCleared {
		t.Fatalf("EditHost(new host name) = %+v, want the key cleared", got)
	}
	tree, _ := ws.DB().LoadTree(ctx)
	if h, _ := tree.Host(uuid.MustParse(ids["host"])); h.Hostname != "ssh2.example.test" || h.MaxConcurrent != 3 || h.HostKeyFingerprint != "" {
		t.Errorf("stored host = %+v", h)
	}
	d.User = ""
	if got := e.EditHost(ctx, ids["host"], d); code(got.Problem) != "problem.noUser" {
		t.Errorf("EditHost(no user) = %+v, want noUser", got)
	}
	if _, err := e.Host(ctx, ids["site"]); err == nil {
		t.Error("Host(a site) gave no error")
	}
}
