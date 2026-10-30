package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MarJC5/vigie/addon/addontest"
	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/workspace"
)

// addonsOf serves the Add-ons page over a folder holding the fake add-on under the names given.
func addonsOf(t *testing.T, names ...string) (*app.Addons, *app.Data) {
	t.Helper()
	program := addontest.Build(t, "../addons/testdata/fake")
	data, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	d, _, _ := emptyData(t, &picker{}, workspace.Deps{})
	return app.NewAddons(d, &picker{}, func() string { return dir }), d
}

func TestThePageListsTheAddOnsAndTheLibrary(t *testing.T) {
	a, _ := addonsOf(t, "vigie-good", "vigie-broken")
	v := a.List(context.Background())
	if v.Problem != nil || len(v.Installed) != 2 || len(v.Library) == 0 {
		t.Fatalf("List() = %+v", v)
	}
	broken, good := v.Installed[0], v.Installed[1]
	if good.Name != "good" || good.Problem != nil || good.Title["en"] != "Fake good" || len(good.Settings) != 1 || !good.Installed {
		t.Errorf("good = %+v", good)
	}
	if broken.Problem == nil || broken.Problem.Code != "problem.addonInvalid" {
		t.Errorf("broken = %+v", broken)
	}
	// The Library's add-ons are not installed here.
	for _, k := range v.Library {
		if k.Installed || k.Install == "" {
			t.Errorf("library %s = %+v", k.Name, k)
		}
	}
}

func TestARunOpensThePreviewOfWhatTheAddOnWrote(t *testing.T) {
	ctx := context.Background()
	a, d := addonsOf(t, "vigie-good", "vigie-fails")
	p := a.Run(ctx, "good", map[string]string{"names": "shop,blog"})
	if p.Problem != nil || p.Source != app.SourceAddOn || p.AddOn != "good" || p.AddOnTitle != "Fake good" || p.Report != "2 hosts.\n" || p.Added != 2 {
		t.Fatalf("Run() = %+v", p)
	}
	if got := d.ApplyImport(ctx, nil); got.Problem != nil || got.Added != 2 {
		t.Errorf("ApplyImport() = %+v", got)
	}
	failed := a.Run(ctx, "fails", nil)
	if failed.Problem == nil || failed.Problem.Code != "problem.addonFailed" || failed.Report != "vigie-fails: the list cannot be read\n" {
		t.Errorf("Run(fails) = %+v", failed)
	}
	if missing := a.Run(ctx, "gone", nil); missing.Problem == nil || missing.Problem.Code != "problem.addonMissing" {
		t.Errorf("Run(gone) = %+v", missing)
	}
}
