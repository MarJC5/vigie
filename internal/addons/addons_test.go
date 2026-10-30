package addons_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/addon/addontest"
	"github.com/MarJC5/vigie/internal/addons"
)

// fakes puts the fake add-on under each name given in a folder of its own, and returns it.
func fakes(t *testing.T, program string, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestTheAddOnsOfThePATH(t *testing.T) {
	program := addontest.Build(t, "./testdata/fake")
	first := fakes(t, program, "vigie-good", "vigie-broken", "vigie-other", "vigie", "vigie-Bad", "unrelated")
	second := fakes(t, program, "vigie-good", "vigie-fails")
	notExec := filepath.Join(second, "vigie-plain")
	if err := os.WriteFile(notExec, []byte("not a program"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := strings.Join([]string{"relative/bin", first, second, filepath.Join(first, "missing")}, string(os.PathListSeparator))
	found := addons.Find(context.Background(), path)
	var names []string
	for _, a := range found {
		names = append(names, a.Name)
	}
	if !slices.Equal(names, []string{"broken", "fails", "good", "other"}) {
		t.Fatalf("found %q", names)
	}
	byName := map[string]addons.Installed{}
	for _, a := range found {
		byName[a.Name] = a
	}
	// The first of a name, in the order of the PATH.
	if good := byName["good"]; good.Err != nil || good.Path != filepath.Join(first, "vigie-good") || good.Info.Title["en"] != "Fake good" {
		t.Errorf("good = %+v", good)
	}
	if byName["broken"].Err == nil || byName["other"].Err == nil {
		t.Errorf("a description that is not one, or names another program, is refused: %v, %v", byName["broken"].Err, byName["other"].Err)
	}
}

func TestARunAsTheContractSays(t *testing.T) {
	program := addontest.Build(t, "./testdata/fake")
	dir := fakes(t, program, "vigie-good", "vigie-fails", "vigie-nothing", "vigie-slow")
	found := map[string]addons.Installed{}
	for _, a := range addons.Find(context.Background(), dir) {
		found[a.Name] = a
	}
	ctx := context.Background()
	f, report, err := addons.Run(ctx, found["good"], map[string]string{"names": "shop,blog"})
	if err != nil || len(f.Hosts) != 2 || f.Hosts[1].Alias != "blog" || report != "2 hosts.\n" {
		t.Fatalf("Run() = %+v, %q, %v", f, report, err)
	}
	// The default of a setting not given.
	if f, _, err := addons.Run(ctx, found["good"], nil); err != nil || len(f.Hosts) != 1 || f.Hosts[0].Alias != "shop" {
		t.Errorf("Run(defaults) = %+v, %v", f, err)
	}
	var failed *addons.Failed
	if _, _, err := addons.Run(ctx, found["fails"], nil); !errors.As(err, &failed) || failed.Report != "vigie-fails: the list cannot be read\n" {
		t.Errorf("Run(fails) = %v", err)
	}
	if _, _, err := addons.Run(ctx, found["nothing"], nil); !errors.Is(err, addons.ErrNoExport) {
		t.Errorf("Run(nothing) = %v, want ErrNoExport", err)
	}
	short, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, _, err := addons.Run(short, found["slow"], nil); !errors.Is(err, addons.ErrTimedOut) || time.Since(start) > 10*time.Second {
		t.Errorf("Run(slow) = %v after %v, want ErrTimedOut at once", err, time.Since(start))
	}
}

// The Library is data: each entry is checked like an add-on's own description.
func TestTheLibrary(t *testing.T) {
	known, err := addons.Library()
	if err != nil || len(known) == 0 {
		t.Fatalf("Library() = %v, %v", known, err)
	}
	for _, k := range known {
		if !strings.HasPrefix(k.Install, "https://") || len(k.Info.Settings) != 0 {
			t.Errorf("%s: install %q, %d settings", k.Info.Name, k.Install, len(k.Info.Settings))
		}
	}
}

// The core app's code names no add-on (ADR): only the Library's data does.
func TestTheCodeNamesNoAddOn(t *testing.T) {
	known, err := addons.Library()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "dist" || d.Name() == "conception" || d.Name() == ".git" || d.Name() == "bin") {
			return filepath.SkipDir
		}
		ext := filepath.Ext(path)
		if d.IsDir() || filepath.Base(path) == "library.json" || !slices.Contains([]string{".go", ".ts", ".tsx", ".json", ".css"}, ext) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, k := range known {
			if strings.Contains(string(data), addons.Prefix+k.Info.Name) {
				t.Errorf("%s names the add-on %s%s", path, addons.Prefix, k.Info.Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
