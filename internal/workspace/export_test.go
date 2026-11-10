package workspace_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/workspace"
)

// testKey is an ed25519 public key in the form known_hosts stores, made for the test.
func testKey(t *testing.T) sshx.HostKey {
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

// exportBench is the bench workspace with a host trusted by its host name, an ignore, and the
// scans of a first scan of everything.
func exportBench(t *testing.T) (*workspace.Workspace, sshx.HostKey) {
	t.Helper()
	ctx := context.Background()
	ws, db, fake := setup(t)
	known := filepath.Join(t.TempDir(), "known_hosts")
	key := testKey(t)
	if err := sshx.Trust(known, "[ssh.example.test]:2222", key); err != nil {
		t.Fatal(err)
	}
	fp, err := key.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := tree.Resolve("Clients")
	if err != nil {
		t.Fatal(err)
	}
	host := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Shared hosting", Hostname: "ssh.example.test", Port: 2222,
		User: "u12ab", HostKeyFingerprint: fp, Notes: "contrat **2026**", Position: 2}
	if err := db.SaveHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "atelier-exemple.ch", Path: "~/www/atelier", Tags: []string{"client"}}
	if err := db.SaveSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveIgnore(ctx, model.Ignore{SiteID: site.ID, Key: "file-editing|site|", Reason: "staging", Created: time.Now()}); err != nil {
		t.Fatal(err)
	}
	ws = workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now, KnownHosts: known})
	if _, err := ws.Scan(ctx, "Clients/Bench", workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	return ws, key
}

func TestExportEverythingReadsBack(t *testing.T) {
	ws, key := exportBench(t)
	f, err := ws.Export(context.Background(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := export.Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	got, err := export.Read(&buf)
	if err != nil {
		t.Fatalf("Read(Export()) = %v", err)
	}
	if got.Scope != "" || len(got.Folders) != 1 || len(got.Hosts) != 2 || len(got.Sites) != 4 || len(got.Ignores) != 1 {
		t.Fatalf("export = %d folders, %d hosts, %d sites, %d ignores", len(got.Folders), len(got.Hosts), len(got.Sites), len(got.Ignores))
	}
	// Three bench sites scanned once each, nothing updated yet.
	if len(got.History.Scans) != 3 || len(got.History.Operations) != 0 {
		t.Errorf("history = %d scans, %d operations", len(got.History.Scans), len(got.History.Operations))
	}
	for _, h := range got.Hosts {
		switch h.Name {
		case "Shared hosting":
			if len(h.TrustedKeys) != 1 || h.TrustedKeys[0] != (export.HostKey{Name: "[ssh.example.test]:2222", Type: key.Type, Key: key.Key}) {
				t.Errorf("trusted keys = %+v", h.TrustedKeys)
			}
			if h.Notes != "contrat **2026**" {
				t.Errorf("notes = %q", h.Notes)
			}
		case "Bench":
			// An alias ssh is not asked about here: no key, asked again after an import.
			if len(h.TrustedKeys) != 0 {
				t.Errorf("bench keys = %+v", h.TrustedKeys)
			}
		}
	}
}

func TestExportAHostAlone(t *testing.T) {
	ws, _ := exportBench(t)
	f, err := ws.Export(context.Background(), "Clients/Shared hosting", false)
	if err != nil {
		t.Fatal(err)
	}
	if f.Scope != "Clients/Shared hosting" || len(f.Folders) != 0 || len(f.Hosts) != 1 || f.Hosts[0].Folder != "" ||
		len(f.Sites) != 1 || len(f.Ignores) != 1 || f.History != nil {
		t.Fatalf("export of a host = %+v", f)
	}
	if _, err := ws.Export(context.Background(), "Clients/Shared hosting/atelier-exemple.ch", false); !errors.Is(err, workspace.ErrNotExportable) {
		t.Errorf("Export(a site) = %v, want ErrNotExportable", err)
	}
}

// An export leaves out the assistants' reads of errors and the setting that allows them: they are
// this computer's journal and choice, not the sites'.
func TestAnExportLeavesOutTheReadsOfErrors(t *testing.T) {
	ctx := context.Background()
	ws, _ := exportBench(t)
	if err := ws.SetAssistantErrors(ctx, true); err != nil {
		t.Fatal(err)
	}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	site := mustResolve(t, tree, "Clients/Shared hosting/atelier-exemple.ch")
	read := model.ErrorRead{ID: model.NewID(), SiteID: site.ID, Path: "Clients/Shared hosting/atelier-exemple.ch", Client: "Assistant de test", At: time.Now()}
	if err := ws.DB().SaveErrorRead(ctx, read); err != nil {
		t.Fatal(err)
	}
	f, err := ws.Export(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := export.Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{read.ID.String(), read.Client, "mcp.errors", "error_reads"} {
		if bytes.Contains(buf.Bytes(), []byte(absent)) {
			t.Errorf("the export holds %q", absent)
		}
	}
}

// An export is readable by its owner only, also over a file that was readable by others.
func TestAnExportFileIsTheOwnersOnly(t *testing.T) {
	ws, _ := exportBench(t)
	f, err := ws.Export(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "vigie.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := export.WriteFile(path, f); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("export file mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := export.Read(file); err != nil {
		t.Errorf("Read(the file) = %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Errorf("folder holds %d files, want the export only", len(entries))
	}
}
