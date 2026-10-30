package cli_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/cli"
	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
)

// dataVigie is a workspace without data, with its own known_hosts and home.
type dataVigie struct {
	ws    *workspace.Workspace
	db    *store.DB
	known string
	home  string
}

func newDataVigie(t *testing.T, resolver workspace.Resolver) dataVigie {
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
	v := dataVigie{db: db, known: filepath.Join(t.TempDir(), "known_hosts"), home: t.TempDir()}
	v.ws = workspace.New(db, workspace.Deps{Now: time.Now, KnownHosts: v.known, Home: v.home, Resolver: resolver})
	return v
}

func (v dataVigie) run(t *testing.T, lang, input string, args ...string) (int, string, string) {
	t.Helper()
	opts, err := cli.Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), opts, cli.Env{
		In: strings.NewReader(input), Out: &out, Err: &errOut, Lang: lang, Texts: texts(t),
		Open: func(context.Context) (*workspace.Workspace, error) { return v.ws, nil },
	})
	return code, out.String(), errOut.String()
}

func (v dataVigie) tree(t *testing.T) *model.Tree {
	t.Helper()
	tree, err := v.db.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// hostKey is an ed25519 public key in the form known_hosts stores, made for the test.
func hostKey(t *testing.T) sshx.HostKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var blob bytes.Buffer
	for _, part := range [][]byte{[]byte("ssh-ed25519"), pub} {
		binary.Write(&blob, binary.BigEndian, uint32(len(part)))
		blob.Write(part)
	}
	return sshx.HostKey{Type: "ssh-ed25519", Key: base64.StdEncoding.EncodeToString(blob.Bytes())}
}

// exportFile writes an export of a folder, a host with its key and a site, and returns its path.
func exportFile(t *testing.T, key sshx.HostKey) string {
	t.Helper()
	folder, host := model.NewID().String(), model.NewID().String()
	f := exchange.File{
		Folders: []exchange.Folder{{ID: folder, Name: "Clients", Color: "blue"}},
		Hosts: []exchange.Host{{ID: host, Folder: folder, Name: "Shared", Hostname: "ssh.example.test", Port: 2222, User: "u12ab",
			Color: "none", TrustedKeys: []exchange.HostKey{{Name: "[ssh.example.test]:2222", Type: key.Type, Key: key.Key}}}},
		Sites:   []exchange.Site{{ID: model.NewID().String(), Host: host, Name: "atelier.example.test", Path: "~/www", Color: "none"}},
		Ignores: []exchange.Ignore{},
	}
	path := filepath.Join(t.TempDir(), "vigie.json")
	if err := exchange.WriteFile(path, f); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportShowsThePreviewAndAsksForEachKey(t *testing.T) {
	v := newDataVigie(t, nil)
	key := hostKey(t)
	fp, _ := key.Fingerprint()
	code, out, errOut := v.run(t, "en", "y\ny\n", "import", exportFile(t, key))
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	for _, want := range []string{"Into the top of the site manager:", "new  Clients/Shared/atelier.example.test\n",
		"Key of Clients/Shared: ssh-ed25519 " + fp, "Import 3 folders, hosts and sites? [y/N]",
		"Trust ssh-ed25519 " + fp + " for Clients/Shared?", "3 folders, hosts and sites imported."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if keys, _ := sshx.TrustedKeys(v.known, "[ssh.example.test]:2222"); len(keys) != 1 {
		t.Errorf("known_hosts = %v, want the key confirmed", keys)
	}
	// Again: everything is there.
	code, out, _ = v.run(t, "en", "", "import", exportFile(t, key))
	if code != 0 || !strings.Contains(out, "Nothing to import.") {
		t.Errorf("a second import: exit %d\n%s", code, out)
	}
}

func TestImportDeclinedAddsNothing(t *testing.T) {
	v := newDataVigie(t, nil)
	code, out, _ := v.run(t, "fr", "n\n", "import", exportFile(t, hostKey(t)))
	if code != 1 || !strings.Contains(out, "Importer 3 dossiers, hébergements et sites ? [o/N]") || !strings.Contains(out, "Rien n'a été importé.") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if _, err := v.tree(t).Resolve("Clients"); err == nil {
		t.Error("something was imported after a no")
	}
}

// A script answers with --yes: the keys stay out unless --trust-keys says otherwise.
func TestImportJSON(t *testing.T) {
	v := newDataVigie(t, nil)
	file := exportFile(t, hostKey(t))
	if code, _, errOut := v.run(t, "en", "", "import", file, "--json"); code != 3 || !strings.Contains(errOut, "--json needs --yes") {
		t.Errorf("--json without --yes: exit %d, %s", code, errOut)
	}
	code, out, _ := v.run(t, "en", "", "import", file, "--yes", "--json")
	var got struct {
		Entries  []struct{ Path, Status string }
		Keys     []struct{ Trusted bool }
		Imported bool
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if code != 0 || !got.Imported || len(got.Entries) != 3 || len(got.Keys) != 1 || got.Keys[0].Trusted {
		t.Errorf("exit %d, %+v", code, got)
	}
	if keys, _ := sshx.TrustedKeys(v.known, "[ssh.example.test]:2222"); len(keys) != 0 {
		t.Errorf("a key was trusted without --trust-keys: %v", keys)
	}

	w := newDataVigie(t, nil)
	if code, _, _ := w.run(t, "en", "", "import", file, "--yes", "--trust-keys", "--into=Nowhere"); code != 3 {
		t.Errorf("--into a missing folder: exit %d", code)
	}
	if code, out, _ := w.run(t, "en", "", "import", file, "--yes", "--trust-keys"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if keys, _ := sshx.TrustedKeys(w.known, "[ssh.example.test]:2222"); len(keys) != 1 {
		t.Errorf("--trust-keys: known_hosts = %v", keys)
	}
}

func TestImportACSVListIntoAFolder(t *testing.T) {
	v := newDataVigie(t, nil)
	if err := v.db.SaveFolder(context.Background(), model.Folder{ID: model.NewID(), Name: "Clients"}); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(t.TempDir(), "sites.csv")
	data := "folder;host;user;path;url\nValais;ssh.example.test;u12ab;~/www;https://atelier.example.test\nValais;;;~/b;\n"
	if err := os.WriteFile(list, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	// A line refused: imported, and the exit status says to look.
	code, out, _ := v.run(t, "en", "", "import", list, "--into", "Clients", "--yes")
	if code != 1 || !strings.Contains(out, "Line 3 refused: neither an alias nor a host") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if _, err := v.tree(t).Resolve("Clients/Valais/ssh.example.test/atelier.example.test"); err != nil {
		t.Errorf("site not imported: %v\n%s", err, out)
	}
	notes := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notes, []byte("not a list"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := v.run(t, "en", "", "import", notes); code != 3 || !strings.Contains(errOut, "neither a Vigie export nor a CSV site list") {
		t.Errorf("a text file: exit %d, %s", code, errOut)
	}
}

type oneAlias struct{}

func (oneAlias) Resolve(_ context.Context, alias string) (sshx.Resolved, error) {
	if alias != "shared" {
		return sshx.Resolved{}, errors.New("exit status 255")
	}
	return sshx.Resolved{Hostname: "ssh.example.test", Port: 22, User: "u12ab"}, nil
}

func TestImportTheSSHConfig(t *testing.T) {
	v := newDataVigie(t, oneAlias{})
	if err := os.MkdirAll(filepath.Join(v.home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v.home, ".ssh", "config"), []byte("Host shared broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := v.run(t, "en", "y\n", "import", "--ssh-config")
	if code != 1 || !strings.Contains(out, "ssh cannot read it: exit status 255") || !strings.Contains(out, "1 folder, host or site imported.") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if h, _ := v.tree(t).Host(mustNode(t, v.tree(t), "shared").ID); h.Alias != "shared" {
		t.Errorf("host = %+v", h)
	}
	if code, _, errOut := v.run(t, "en", "", "import", "--ssh-config", "file.json"); code != 3 || !strings.Contains(errOut, "import needs one file") {
		t.Errorf("a file and --ssh-config: exit %d, %s", code, errOut)
	}
}

func mustNode(t *testing.T, tree *model.Tree, path string) model.Node {
	t.Helper()
	n, err := tree.Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestExport(t *testing.T) {
	v := newDataVigie(t, nil)
	if code, _, _ := v.run(t, "en", "", "import", exportFile(t, hostKey(t)), "--yes"); code != 0 {
		t.Fatal("import")
	}
	// To the standard output: the export itself, for a script or an add-on.
	code, out, _ := v.run(t, "en", "", "export", "Clients", "--json")
	if f, err := exchange.Read(strings.NewReader(out)); code != 0 || err != nil || f.Scope != "Clients" || len(f.Sites) != 1 {
		t.Fatalf("exit %d, %v\n%s", code, err, out)
	}
	file := filepath.Join(t.TempDir(), "backup.json")
	code, out, _ = v.run(t, "en", "", "export", "--history", "-o", file)
	if code != 0 || !strings.Contains(out, "Exported to "+file+" (folders: 1, hosts: 1, sites: 1).") || !strings.Contains(out, "keep it somewhere safe") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("export file: %v, %v", info, err)
	}
	if code, _, errOut := v.run(t, "en", "", "export", "Clients/Shared/atelier.example.test"); code != 3 || !strings.Contains(errOut, "exported with its host") {
		t.Errorf("export of a site: exit %d, %s", code, errOut)
	}
	if code, _, errOut := v.run(t, "en", "", "export", "-o"); code != 3 || !strings.Contains(errOut, "-o needs a value") {
		t.Errorf("-o alone: exit %d, %s", code, errOut)
	}
	if code, _, errOut := v.run(t, "en", "", "export", "--yes"); code != 3 || !strings.Contains(errOut, `unknown option "--yes"`) {
		t.Errorf("--yes on export: exit %d, %s", code, errOut)
	}
}
