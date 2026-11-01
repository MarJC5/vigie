package workspace_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
)

// tagsOf returns the stored tags of the bench's sites, by name.
func tagsOf(t *testing.T, ws *workspace.Workspace) map[string][]string {
	t.Helper()
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, s := range tree.SitesUnder(model.Root) {
		out[s.Name] = s.Tags
	}
	return out
}

func wantTags(t *testing.T, ws *workspace.Workspace, site string, want ...string) {
	t.Helper()
	if got := tagsOf(t, ws)[site]; !slices.Equal(got, want) && !(len(got) == 0 && len(want) == 0) {
		t.Errorf("tags of %s = %q, want %q", site, got, want)
	}
}

func TestSetTagsKeepsTheSpellingOfTheOtherSites(t *testing.T) {
	ctx := context.Background()
	ws, _ := bench(t)
	outdated, network := resolve(t, ws, "Clients/Bench/outdated.example.test"), resolve(t, ws, "Clients/Bench/network.example.test")
	if err := ws.SetTags(ctx, outdated, []string{"Contrat", "  Prioritaire "}); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "outdated.example.test", "Contrat", "Prioritaire")
	if err := ws.SetTags(ctx, network, []string{"CONTRAT"}); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "network.example.test", "Contrat")
	// A site alone with a tag may change how it is written.
	if err := ws.SetTags(ctx, outdated, []string{"contrat", "prioritaire"}); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "outdated.example.test", "Contrat", "prioritaire")

	if err := ws.SetTags(ctx, outdated, []string{"a,b"}); !errors.Is(err, model.ErrTagCharacter) {
		t.Errorf("a tag with a comma: %v, want ErrTagCharacter", err)
	}
	if err := ws.SetTags(ctx, resolve(t, ws, "Clients/Bench"), []string{"x"}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("tags on a host: %v, want ErrNotFound", err)
	}
}

func TestRenameATagEverywhereOrMergeIt(t *testing.T) {
	ctx := context.Background()
	ws, _ := bench(t)
	for site, tags := range map[string][]string{
		"outdated.example.test": {"Contrat", "Prioritaire"}, "network.example.test": {"Contrat"}, "broken": {"Urgent"},
	} {
		if err := ws.SetTags(ctx, resolve(t, ws, "Clients/Bench/"+site), tags); err != nil {
			t.Fatal(err)
		}
	}

	if err := ws.RenameTag(ctx, "contrat", "Maintenance"); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "outdated.example.test", "Maintenance", "Prioritaire")
	wantTags(t, ws, "network.example.test", "Maintenance")

	// Onto another tag: both become one, in the spelling it has; a site with both keeps one.
	if err := ws.RenameTag(ctx, "Urgent", "prioritaire"); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "broken", "Prioritaire")
	if err := ws.RenameTag(ctx, "Maintenance", "PRIORITAIRE"); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "outdated.example.test", "Prioritaire")
	wantTags(t, ws, "network.example.test", "Prioritaire")

	// Onto itself in another case: the new spelling everywhere.
	if err := ws.RenameTag(ctx, "prioritaire", "PRIORITAIRE"); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "broken", "PRIORITAIRE")

	if err := ws.RenameTag(ctx, "Nothing", "x"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("renaming a tag no site has: %v, want ErrNotFound", err)
	}
	if err := ws.RenameTag(ctx, "PRIORITAIRE", "a|b"); !errors.Is(err, model.ErrTagCharacter) {
		t.Errorf("renaming to a bad tag: %v, want ErrTagCharacter", err)
	}
}

func TestRemoveATagFromEverySite(t *testing.T) {
	ctx := context.Background()
	ws, _ := bench(t)
	if err := ws.SetTags(ctx, resolve(t, ws, "Clients/Bench/outdated.example.test"), []string{"Contrat", "Kept"}); err != nil {
		t.Fatal(err)
	}
	if err := ws.SetTags(ctx, resolve(t, ws, "Clients/Bench/broken"), []string{"contrat"}); err != nil {
		t.Fatal(err)
	}
	if err := ws.RemoveTag(ctx, "CONTRAT"); err != nil {
		t.Fatal(err)
	}
	wantTags(t, ws, "outdated.example.test", "Kept")
	wantTags(t, ws, "broken")
	if err := ws.RemoveTag(ctx, "Contrat"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("removing it again: %v, want ErrNotFound", err)
	}
}
