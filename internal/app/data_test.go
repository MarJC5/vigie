package app_test

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
	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/workspace"
)

// picker answers the file dialogs with the files given; "" stands for a cancel.
type picker struct {
	save, open string
	suggested  string
	kind       string
}

func (p *picker) SaveFile(name string) (string, error) {
	p.suggested = name
	return p.save, nil
}

func (p *picker) ChooseFolder() (string, error) {
	return p.open, nil
}

func (p *picker) OpenFile(kind string) (string, error) {
	p.kind = kind
	return p.open, nil
}

var day = time.Date(2026, 10, 30, 14, 12, 0, 0, time.UTC)

// emptyData is the service over a workspace without data, with its own known_hosts.
func emptyData(t *testing.T, pick *picker, deps workspace.Deps) (*app.Data, *workspace.Workspace, string) {
	t.Helper()
	deps.Now = time.Now
	deps.KnownHosts = filepath.Join(t.TempDir(), "known_hosts")
	ws := workspace.New(repository(t), deps)
	return app.NewData(ws, pick, func() time.Time { return day }, nil), ws, deps.KnownHosts
}

func TestExportFromTheWindow(t *testing.T) {
	ctx := context.Background()
	_, ws, ids := editor(t)
	pick := &picker{save: filepath.Join(t.TempDir(), "backup.json")}
	d := app.NewData(ws, pick, func() time.Time { return day }, nil)

	sum := d.ExportSummary(ctx, "")
	if sum.Problem != nil || sum.Folders != 1 || sum.Hosts != 1 || sum.Sites != 1 || sum.Ignores != 0 || sum.HistoryBytes <= 0 {
		t.Errorf("ExportSummary() = %+v", sum)
	}
	if got := d.ExportSummary(ctx, ids["site"]); got.Problem == nil || got.Problem.Code != "problem.notExportable" {
		t.Errorf("ExportSummary(a site) = %+v", got)
	}
	got := d.Export(ctx, ids["host"], false)
	if got.Problem != nil || got.File != pick.save || pick.suggested != "vigie-bench-2026-10-30.json" {
		t.Fatalf("Export() = %+v, suggested %q", got, pick.suggested)
	}
	if info, err := os.Stat(pick.save); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("export file: %v, %v", info, err)
	}
	// The user cancels the dialog: nothing written, nothing wrong.
	pick.save = ""
	if got := d.Export(ctx, "", true); got.Problem != nil || got.File != "" {
		t.Errorf("Export(cancelled) = %+v", got)
	}
}

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

// anExport writes an export with a folder, a host with its key, and a site with a scan.
func anExport(t *testing.T, key sshx.HostKey) string {
	t.Helper()
	folder, host, site := model.NewID().String(), model.NewID().String(), model.NewID().String()
	f := export.File{
		Exported: day, App: "0.6.0",
		Folders: []export.Folder{{ID: folder, Name: "Valais", Color: "none"}},
		Hosts: []export.Host{{ID: host, Folder: folder, Name: "Atelier", Hostname: "ssh.example.test", Port: 2222, User: "atelier",
			Color: "none", TrustedKeys: []export.HostKey{{Name: "[ssh.example.test]:2222", Type: key.Type, Key: key.Key}}}},
		Sites:   []export.Site{{ID: site, Host: host, Name: "atelier.example.test", Path: "~/www", Color: "none"}},
		Ignores: []export.Ignore{},
		History: &export.History{Scans: []export.Scan{{ID: model.NewID().String(), Site: site, Started: day, Finished: day,
			Outcome: "completed", Items: []export.Item{}, Findings: []export.Finding{}}}, Operations: []export.Operation{}},
	}
	path := filepath.Join(t.TempDir(), "vigie-2026-10-30.json")
	if err := export.WriteFile(path, f); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportAnExportFromTheWindow(t *testing.T) {
	ctx := context.Background()
	key := testKey(t)
	pick := &picker{open: anExport(t, key)}
	d, ws, known := emptyData(t, pick, workspace.Deps{})
	clients := app.NewEditor(ws).CreateFolder(ctx, "", "Clients")
	if clients.Problem != nil {
		t.Fatal(clients.Problem)
	}

	p := d.ChooseImport(ctx, app.FileVigie)
	if p.Problem != nil || pick.kind != app.FileVigie || p.File != "vigie-2026-10-30.json" || p.App != "0.6.0" || !p.History ||
		p.Exported != "2026-10-30T14:12:00Z" || p.Added != 3 {
		t.Fatalf("ChooseImport() = %+v", p)
	}
	if p.Items[1].Account != "atelier@ssh.example.test:2222" || p.Items[2].SitePath != "~/www" || p.Items[2].Scans != 1 {
		t.Errorf("items = %+v", p.Items)
	}
	// Into Clients: the paths follow.
	p = d.PreviewImport(ctx, clients.ID)
	if p.Problem != nil || p.Into != clients.ID || p.Items[0].Path != "Clients/Valais" || len(p.Keys) != 1 || p.Keys[0].Refused != "" {
		t.Fatalf("PreviewImport(Clients) = %+v", p)
	}
	if got := d.ApplyImport(ctx, []string{p.Keys[0].HostID}); got.Problem != nil || got.Added != 3 {
		t.Fatalf("ApplyImport() = %+v", got)
	}
	if keys, _ := sshx.TrustedKeys(known, "[ssh.example.test]:2222"); len(keys) != 1 {
		t.Errorf("known_hosts = %v, want the key kept", keys)
	}
	// Once done, there is nothing left to apply.
	if got := d.ApplyImport(ctx, nil); got.Problem == nil || got.Problem.Code != "problem.noImport" {
		t.Errorf("ApplyImport() again = %+v", got)
	}
}

func TestImportACSVListFromTheWindow(t *testing.T) {
	ctx := context.Background()
	list := filepath.Join(t.TempDir(), "sites.csv")
	if err := os.WriteFile(list, []byte("folder;host;user;path\nValais;ssh.example.test;atelier;~/www\nValais;ssh.example.test;atelier;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pick := &picker{open: list}
	d, _, _ := emptyData(t, pick, workspace.Deps{})
	p := d.ChooseImport(ctx, app.FileCSV)
	if p.Problem != nil || p.Lines != 2 || p.Separator != ";" || p.Added != 3 {
		t.Fatalf("ChooseImport(csv) = %+v", p)
	}
	last := p.Items[len(p.Items)-1]
	if last.Kind != "line" || last.Line != 3 || last.Reason != "no-path" || last.Detail != "Valais;ssh.example.test;atelier;" {
		t.Errorf("refused line = %+v", last)
	}
	d.CancelImport()
	if got := d.ApplyImport(ctx, nil); got.Problem == nil {
		t.Error("an import cancelled was applied")
	}
}

func TestAFileVigieCannotRead(t *testing.T) {
	ctx := context.Background()
	notes := filepath.Join(t.TempDir(), "notes.json")
	if err := os.WriteFile(notes, []byte(`{"format":"other"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	pick := &picker{open: notes}
	d, _, _ := emptyData(t, pick, workspace.Deps{})
	if p := d.ChooseImport(ctx, app.FileVigie); p.Problem == nil || p.Problem.Code != "problem.notAnExport" {
		t.Errorf("ChooseImport(not an export) = %+v", p)
	}
	pick.open = ""
	if p := d.ChooseImport(ctx, app.FileVigie); !p.Cancelled || p.Problem != nil {
		t.Errorf("ChooseImport(cancelled) = %+v", p)
	}
}

type twoAliases struct{}

func (twoAliases) Resolve(_ context.Context, alias string) (sshx.Resolved, error) {
	if alias == "broken" {
		return sshx.Resolved{}, errors.New("exit status 255")
	}
	return sshx.Resolved{Hostname: alias + ".example.test", Port: 22, User: "deploy"}, nil
}

func TestImportHostsOfTheSSHConfigFromTheWindow(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte("Host shop broken *.test !jump\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, ws, _ := emptyData(t, &picker{}, workspace.Deps{Home: home, Resolver: twoAliases{}})
	v := d.SSHConfig(ctx)
	if v.Problem != nil || v.Patterns != 2 || len(v.Hosts) != 2 || v.Hosts[0].Account != "deploy@shop.example.test:22" ||
		v.Hosts[1].Error == "" {
		t.Fatalf("SSHConfig() = %+v", v)
	}
	if got := d.ImportSSHConfig(ctx, []string{"shop"}, ""); got.Problem != nil || got.Added != 1 {
		t.Fatalf("ImportSSHConfig() = %+v", got)
	}
	if v := d.SSHConfig(ctx); v.Hosts[0].Present != "shop" {
		t.Errorf("after the import: %+v", v.Hosts[0])
	}
	if tree, err := ws.DB().LoadTree(ctx); err != nil {
		t.Fatal(err)
	} else if _, err := tree.Resolve("shop"); err != nil {
		t.Errorf("host not added: %v", err)
	}
}

// The database's size is shown, and compacting it is refused while scans or updates run.
func TestCompactFromTheWindow(t *testing.T) {
	ctx := context.Background()
	_, ws, _ := editor(t)
	running := true
	d := app.NewData(ws, &picker{}, func() time.Time { return day }, func() bool { return running })

	if got := d.Database(ctx); got.Problem != nil || got.Bytes == 0 {
		t.Errorf("Database() = %+v, want a size", got)
	}
	if got := d.Compact(ctx); got.Problem == nil || got.Problem.Code != "problem.running" {
		t.Errorf("Compact() while running = %+v, want problem.running", got)
	}
	running = false
	// Both sizes reach the window; that compacting gives the free space back is the store's test
	// (a database with none, as here, may come out a page longer).
	got := d.Compact(ctx)
	if got.Problem != nil || got.Before == 0 || got.After == 0 {
		t.Errorf("Compact() = %+v, want the sizes before and after", got)
	}
}
