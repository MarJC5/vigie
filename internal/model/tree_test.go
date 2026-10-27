package model_test

import (
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

// sampleTree builds: Clients (blue) / Infomaniak (host) / misits.ch, ok-jordane.ch (red),
// and a root host "solo" with one site.
func sampleTree(t *testing.T) (*model.Tree, map[string]model.ID) {
	t.Helper()
	ids := map[string]model.ID{}
	tr := model.NewTree()
	mustAdd := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	clients := model.Folder{ID: model.NewID(), Name: "Clients", Color: model.ColorBlue}
	mustAdd(tr.AddFolder(clients))
	ids["Clients"] = clients.ID

	host := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Infomaniak", Alias: "infomaniak", WPCLI: "wp-cli"}
	mustAdd(tr.AddHost(host))
	ids["Infomaniak"] = host.ID

	misits := model.Site{ID: model.NewID(), HostID: host.ID, Name: "misits.ch", Path: "~/sites/misits.ch"}
	mustAdd(tr.AddSite(misits))
	ids["misits.ch"] = misits.ID

	jordane := model.Site{ID: model.NewID(), HostID: host.ID, Name: "ok-jordane.ch", Color: model.ColorRed, WPCLI: "/opt/wp"}
	mustAdd(tr.AddSite(jordane))
	ids["ok-jordane.ch"] = jordane.ID

	solo := model.Host{ID: model.NewID(), Name: "solo", Hostname: "example.org", User: "u"}
	mustAdd(tr.AddHost(solo))
	ids["solo"] = solo.ID

	soloSite := model.Site{ID: model.NewID(), HostID: solo.ID, Name: "example.org"}
	mustAdd(tr.AddSite(soloSite))
	ids["example.org"] = soloSite.ID

	return tr, ids
}

func TestEffectiveColorInheritance(t *testing.T) {
	tr, ids := sampleTree(t)
	tests := []struct {
		site string
		want model.Color
	}{
		{"ok-jordane.ch", model.ColorRed}, // own color
		{"misits.ch", model.ColorBlue},    // host has none, folder is blue
		{"example.org", model.ColorNone},  // root host, no folder
	}
	for _, tt := range tests {
		if got := tr.EffectiveSiteColor(ids[tt.site]); got != tt.want {
			t.Errorf("EffectiveSiteColor(%s) = %s, want %s", tt.site, got, tt.want)
		}
	}
}

func TestNestedFolderColor(t *testing.T) {
	tr, ids := sampleTree(t)
	sub := model.Folder{ID: model.NewID(), ParentID: ids["Clients"], Name: "Valais"}
	if err := tr.AddFolder(sub); err != nil {
		t.Fatal(err)
	}
	h := model.Host{ID: model.NewID(), FolderID: sub.ID, Name: "vs", Alias: "vs"}
	if err := tr.AddHost(h); err != nil {
		t.Fatal(err)
	}
	if got := tr.EffectiveHostColor(h.ID); got != model.ColorBlue {
		t.Fatalf("EffectiveHostColor = %s, want blue from the parent folder", got)
	}
}

func TestEffectiveWPCLI(t *testing.T) {
	tr, ids := sampleTree(t)
	tests := map[string]string{
		"ok-jordane.ch": "/opt/wp", // site override
		"misits.ch":     "wp-cli",  // host setting
		"example.org":   "wp",      // default
	}
	for site, want := range tests {
		if got := tr.EffectiveWPCLI(ids[site]); got != want {
			t.Errorf("EffectiveWPCLI(%s) = %q, want %q", site, got, want)
		}
	}
}

func TestSiblingNamesAreUnique(t *testing.T) {
	tr, ids := sampleTree(t)
	// A folder and a host under the same folder share one namespace (paths must be unambiguous).
	err := tr.AddFolder(model.Folder{ID: model.NewID(), ParentID: ids["Clients"], Name: "infomaniak"})
	if !errors.Is(err, model.ErrDuplicateName) {
		t.Fatalf("folder named like a sibling host (any case): %v, want model.ErrDuplicateName", err)
	}
	err = tr.AddSite(model.Site{ID: model.NewID(), HostID: ids["Infomaniak"], Name: "MISITS.CH"})
	if !errors.Is(err, model.ErrDuplicateName) {
		t.Fatalf("duplicate site name: %v, want model.ErrDuplicateName", err)
	}
	// The same name is fine under another parent.
	if err := tr.AddSite(model.Site{ID: model.NewID(), HostID: ids["solo"], Name: "misits.ch"}); err != nil {
		t.Fatalf("same name under another host: %v", err)
	}
}

func TestAddRejectsUnknownParents(t *testing.T) {
	tr := model.NewTree()
	if err := tr.AddFolder(model.Folder{ID: model.NewID(), ParentID: model.NewID(), Name: "x"}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("folder with unknown parent: %v, want model.ErrNotFound", err)
	}
	if err := tr.AddHost(model.Host{ID: model.NewID(), FolderID: model.NewID(), Name: "h", Alias: "a"}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("host with unknown folder: %v, want model.ErrNotFound", err)
	}
	if err := tr.AddSite(model.Site{ID: model.NewID(), HostID: model.NewID(), Name: "s"}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("site with unknown host: %v, want model.ErrNotFound", err)
	}
}

func TestPathsRoundTrip(t *testing.T) {
	tr, ids := sampleTree(t)
	tests := []struct {
		path string
		kind model.NodeKind
		id   string
	}{
		{"Clients", model.KindFolder, "Clients"},
		{"Clients/Infomaniak", model.KindHost, "Infomaniak"},
		{"Clients/Infomaniak/misits.ch", model.KindSite, "misits.ch"},
		{"solo/example.org", model.KindSite, "example.org"},
	}
	for _, tt := range tests {
		node, err := tr.Resolve(tt.path)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", tt.path, err)
		}
		if node.Kind != tt.kind || node.ID != ids[tt.id] {
			t.Errorf("Resolve(%q) = %v %s, want %v %s", tt.path, node.Kind, node.ID, tt.kind, ids[tt.id])
		}
		if back := tr.PathOf(node); back != tt.path {
			t.Errorf("PathOf(Resolve(%q)) = %q", tt.path, back)
		}
	}
}

func TestResolveErrors(t *testing.T) {
	tr, _ := sampleTree(t)
	for _, p := range []string{"", "Nope", "Clients/Nope", "Clients/Infomaniak/misits.ch/extra"} {
		if _, err := tr.Resolve(p); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("Resolve(%q) error = %v, want model.ErrNotFound", p, err)
		}
	}
}
