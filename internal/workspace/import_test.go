package workspace_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
)

// emptyVigie is a workspace without data, with its own known_hosts.
func emptyVigie(t *testing.T) (*workspace.Workspace, *store.DB, string) {
	t.Helper()
	db := emptyDB(t)
	known := filepath.Join(t.TempDir(), "known_hosts")
	return workspace.New(db, workspace.Deps{Now: time.Now, KnownHosts: known}), db, known
}

func plan(t *testing.T, ws *workspace.Workspace, f export.File, into string) workspace.ImportPlan {
	t.Helper()
	p, err := ws.PlanImport(context.Background(), f, into)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// statuses lists the entries as "path status reason".
func statuses(p workspace.ImportPlan) []string {
	var out []string
	for _, e := range p.Entries {
		line := e.Path + " " + string(e.Status)
		if e.Reason != "" {
			line += " " + string(e.Reason)
		}
		out = append(out, line)
	}
	return out
}

func TestAnExportImportedIntoAnEmptyVigieGivesTheSameData(t *testing.T) {
	ctx := context.Background()
	from, key := exportBench(t)
	want, err := from.Export(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}
	ws, _, known := emptyVigie(t)
	p := plan(t, ws, want, "")
	if p.Added() != 7 || p.Ignores != 1 || p.Scans != 3 || p.Operations != 0 {
		t.Fatalf("plan adds %d nodes, %d ignores, %d scans, %d operations\n%q", p.Added(), p.Ignores, p.Scans, p.Operations, statuses(p))
	}
	fp, _ := key.Fingerprint()
	if len(p.Keys) != 1 || p.Keys[0].Host != "Clients/Shared hosting" || p.Keys[0].Fingerprint != fp || p.Keys[0].Refused != "" {
		t.Fatalf("keys offered = %+v", p.Keys)
	}
	if err := ws.Import(ctx, p, []model.ID{p.Keys[0].HostID}); err != nil {
		t.Fatal(err)
	}
	got, err := ws.Export(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}
	got.Exported = want.Exported
	if !reflect.DeepEqual(got, want) {
		t.Errorf("export after the import =\n%+v\nwant\n%+v", got, want)
	}
	if keys, err := sshx.TrustedKeys(known, "[ssh.example.test]:2222"); err != nil || !slices.Equal(keys, []sshx.HostKey{key}) {
		t.Errorf("known_hosts after the import = %v, %v", keys, err)
	}
}

func TestAnImportAddsOnlyWhatIsNew(t *testing.T) {
	ctx := context.Background()
	ws, _ := exportBench(t)
	f, err := ws.Export(ctx, "", false)
	if err != nil {
		t.Fatal(err)
	}
	// The same file again: everything is there by its ID.
	p := plan(t, ws, f, "")
	if p.Added() != 0 || p.Ignores != 0 || len(p.Keys) != 0 {
		t.Fatalf("a second import adds %d\n%q", p.Added(), statuses(p))
	}
	if err := ws.Import(ctx, p, nil); err != nil {
		t.Fatal(err)
	}

	// The same data from another Vigie (other IDs), a site more on the bench, and another
	// account named like it.
	for i := range f.Folders {
		f.Folders[i].ID = model.NewID().String()
	}
	ids := map[string]string{}
	for i := range f.Hosts {
		ids[f.Hosts[i].ID] = model.NewID().String()
		f.Hosts[i].ID, f.Hosts[i].Folder = ids[f.Hosts[i].ID], f.Folders[0].ID
	}
	for i := range f.Sites {
		f.Sites[i].ID, f.Sites[i].Host = model.NewID().String(), ids[f.Sites[i].Host]
	}
	f.Ignores = nil
	bench := f.Hosts[slices.IndexFunc(f.Hosts, func(h export.Host) bool { return h.Name == "Bench" })]
	f.Sites = append(f.Sites, export.Site{ID: model.NewID().String(), Host: bench.ID, Name: "new.example.test", Path: "~/sites/new", Color: "none"})
	f.Hosts = append(f.Hosts, export.Host{ID: model.NewID().String(), Folder: f.Folders[0].ID, Name: "Bench", Alias: "other", Color: "none"})
	p = plan(t, ws, f, "")
	want := []string{
		"Clients present same-name",
		"Clients/Bench present same-connection",
		"Clients/Shared hosting present same-connection",
		"Clients/Bench (2) added renamed",
		"Clients/Bench/outdated.example.test present same-path",
		"Clients/Bench/network.example.test present same-path",
		"Clients/Bench/broken present same-path",
		"Clients/Shared hosting/atelier-exemple.ch present same-path",
		"Clients/Bench/new.example.test added",
	}
	if got := statuses(p); !sameItems(got, want) {
		t.Fatalf("plan =\n%q\nwant\n%q", got, want)
	}
	if err := ws.Import(ctx, p, nil); err != nil {
		t.Fatal(err)
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n, err := tree.Resolve("Clients/Bench/new.example.test")
	if err != nil {
		t.Fatal(err)
	}
	// After the bench's three sites.
	if s, _ := tree.Site(n.ID); s.Position != 3 {
		t.Errorf("new site position = %d, want 3", s.Position)
	}
	ignores, err := ws.DB().Ignores(ctx, tree.SitesUnder(mustResolve(t, tree, "Clients/Shared hosting"))[0].ID)
	if err != nil || len(ignores) != 1 {
		t.Errorf("the ignore stored before = %v, %v; want it kept", ignores, err)
	}
}

func sameItems(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func mustResolve(t *testing.T, tree *model.Tree, path string) model.Node {
	t.Helper()
	n, err := tree.Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// A file to import: a folder, and a host with a key for the name given.
func keyFile(t *testing.T, name string, key sshx.HostKey) export.File {
	t.Helper()
	folder, host := model.NewID().String(), model.NewID().String()
	return export.File{
		Folders: []export.Folder{{ID: folder, Name: "Imported", Color: "none"}},
		Hosts: []export.Host{{ID: host, Folder: folder, Name: "Server", Hostname: "ssh.example.test", Port: 2222, User: "u1",
			Color: "none", TrustedKeys: []export.HostKey{{Name: name, Type: key.Type, Key: key.Key}}}},
	}
}

func TestAKeyComesBackOnlyOnceConfirmed(t *testing.T) {
	ctx := context.Background()
	ws, db, known := emptyVigie(t)
	key := testKey(t)
	p := plan(t, ws, keyFile(t, "[ssh.example.test]:2222", key), "")
	if len(p.Keys) != 1 || p.Keys[0].Refused != "" {
		t.Fatalf("keys = %+v", p.Keys)
	}
	if err := ws.Import(ctx, p, nil); err != nil {
		t.Fatal(err)
	}
	if keys, _ := sshx.TrustedKeys(known, "[ssh.example.test]:2222"); len(keys) != 0 {
		t.Errorf("a key not confirmed was trusted: %v", keys)
	}
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := tree.Host(mustResolve(t, tree, "Imported/Server").ID); h.HostKeyFingerprint != "" {
		t.Errorf("fingerprint = %q, want none: the host asks at its first connection", h.HostKeyFingerprint)
	}
}

func TestKeysThatCannotBeTrusted(t *testing.T) {
	ws, _, known := emptyVigie(t)
	key, other := testKey(t), testKey(t)
	for _, tc := range []struct {
		name string
		file export.File
		want workspace.KeyRefusal
	}{
		{"for another server", keyFile(t, "ssh.example.test", key), workspace.KeyOtherServer},
		{"not a key", keyFile(t, "[ssh.example.test]:2222", sshx.HostKey{Type: "ssh-ed25519", Key: "bm90IGEga2V5"}), workspace.KeyInvalid},
	} {
		p := plan(t, ws, tc.file, "")
		if len(p.Keys) != 1 || p.Keys[0].Refused != tc.want {
			t.Errorf("%s: keys = %+v, want refused %q", tc.name, p.Keys, tc.want)
		}
	}
	// Vigie trusts another key for that server: it is never replaced by an import.
	if err := sshx.Trust(known, "[ssh.example.test]:2222", other); err != nil {
		t.Fatal(err)
	}
	p := plan(t, ws, keyFile(t, "[ssh.example.test]:2222", key), "")
	if len(p.Keys) != 1 || p.Keys[0].Refused != workspace.KeyOtherTrusted {
		t.Fatalf("keys = %+v, want refused %q", p.Keys, workspace.KeyOtherTrusted)
	}
	if err := ws.Import(context.Background(), p, []model.ID{p.Keys[0].HostID}); err != nil {
		t.Fatal(err)
	}
	if keys, _ := sshx.TrustedKeys(known, "[ssh.example.test]:2222"); !slices.Equal(keys, []sshx.HostKey{other}) {
		t.Errorf("known_hosts = %v, want the key trusted before", keys)
	}
}

func TestWhatCannotBeAddedIsRefusedWithWhatItHolds(t *testing.T) {
	ws, _, _ := emptyVigie(t)
	folder, host, bad := model.NewID().String(), model.NewID().String(), model.NewID().String()
	f := export.File{
		Folders: []export.Folder{{ID: folder, Name: "A/B", Color: "none"}},
		Hosts: []export.Host{
			{ID: host, Folder: folder, Name: "Server", Alias: "server", Color: "none"},
			{ID: bad, Name: "No address", Color: "none"},
		},
		Sites: []export.Site{
			{ID: model.NewID().String(), Host: host, Name: "a.example.test", Path: "~/a", Color: "none"},
			{ID: model.NewID().String(), Host: bad, Name: "b.example.test", Path: "~/b", Color: "none"},
		},
	}
	p := plan(t, ws, f, "")
	want := []string{
		"A/B refused invalid",
		"A/B/Server refused parent-refused",
		"No address refused invalid",
		"A/B/Server/a.example.test refused parent-refused",
		"No address/b.example.test refused parent-refused",
	}
	if got := statuses(p); !slices.Equal(got, want) || p.Added() != 0 {
		t.Errorf("plan =\n%q\nwant\n%q", got, want)
	}
}

func TestAnImportGoesIntoTheFolderChosen(t *testing.T) {
	ctx := context.Background()
	from, _ := exportBench(t)
	f, err := from.Export(ctx, "Clients/Shared hosting", false)
	if err != nil {
		t.Fatal(err)
	}
	ws, _ := bench(t)
	p := plan(t, ws, f, "Clients")
	if got, want := statuses(p), []string{"Clients/Shared hosting added", "Clients/Shared hosting/atelier-exemple.ch added"}; !slices.Equal(got, want) {
		t.Errorf("plan = %q, want %q", got, want)
	}
	if _, err := ws.PlanImport(ctx, f, "Clients/Bench"); !errors.Is(err, workspace.ErrNotAFolder) {
		t.Errorf("PlanImport(into a host) = %v, want ErrNotAFolder", err)
	}
}

// A preview made before another change that clashes with it adds nothing.
func TestAPreviewOverTakenByAChangeAddsNothing(t *testing.T) {
	ctx := context.Background()
	ws, db, _ := emptyVigie(t)
	p := plan(t, ws, keyFile(t, "[ssh.example.test]:2222", testKey(t)), "")
	if err := db.SaveFolder(ctx, model.Folder{ID: model.NewID(), Name: "Imported"}); err != nil {
		t.Fatal(err)
	}
	if err := ws.Import(ctx, p, nil); !errors.Is(err, model.ErrDuplicateName) {
		t.Fatalf("Import(an old preview) = %v, want ErrDuplicateName", err)
	}
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Resolve("Imported/Server"); err == nil {
		t.Error("the host was added without its folder")
	}
}
