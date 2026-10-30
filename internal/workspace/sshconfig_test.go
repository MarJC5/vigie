package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/workspace"
)

// aliases resolves the names it knows, as ssh -G would.
type aliases map[string]sshx.Resolved

func (a aliases) Resolve(_ context.Context, alias string) (sshx.Resolved, error) {
	r, ok := a[alias]
	if !ok {
		return sshx.Resolved{}, errors.New("ssh -G: exit status 255")
	}
	return r, nil
}

func TestTheHostsOfTheSSHConfigAndTheOnesAlreadyThere(t *testing.T) {
	ctx := context.Background()
	_, db, _ := setup(t) // Clients/Bench reaches the alias "bench"
	shared := model.Host{ID: model.NewID(), Name: "Shared", Hostname: "ssh.example.test", Port: 2222, User: "u12ab"}
	if err := db.SaveHost(ctx, shared); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "Host bench shared new broken *.example.test\n"
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := workspace.New(db, workspace.Deps{Home: home, Now: time.Now, Resolver: aliases{
		"bench":  {Hostname: "bench.example.test", Port: 22, User: "wp"},
		"shared": {Hostname: "SSH.example.test", Port: 2222, User: "u12ab"},
		"new":    {Hostname: "new.example.test", Port: 22, User: "deploy"},
	}})
	got, err := ws.SSHConfigHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []workspace.SSHConfigHost{
		{Alias: "bench", Hostname: "bench.example.test", Port: 22, User: "wp", Present: "Clients/Bench"},
		{Alias: "shared", Hostname: "SSH.example.test", Port: 2222, User: "u12ab", Present: "Shared"},
		{Alias: "new", Hostname: "new.example.test", Port: 22, User: "deploy"},
		{Alias: "broken", Error: "ssh -G: exit status 255"},
	}
	if !slices.Equal(got.Hosts, want) || got.Patterns != 1 {
		t.Fatalf("SSHConfigHosts() =\n%+v\nwant\n%+v and 1 pattern", got, want)
	}

	// The aliases chosen become hosts in the folder chosen, as an import.
	p, err := ws.PlanImport(ctx, workspace.SSHConfigFile([]string{"new"}), "Clients")
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses(p); !slices.Equal(got, []string{"Clients/new added"}) {
		t.Fatalf("plan = %q", got)
	}
	if err := ws.Import(ctx, p, nil); err != nil {
		t.Fatal(err)
	}
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := tree.Host(mustResolve(t, tree, "Clients/new").ID); h.Alias != "new" || h.Hostname != "" {
		t.Errorf("host added = %+v, want the alias only: ssh keeps its settings", h)
	}
}
