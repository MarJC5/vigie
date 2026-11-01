package app_test

import (
	"context"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/app"
)

func TestTagsAreServedWithTheTree(t *testing.T) {
	ctx := context.Background()
	e, ws, ids := editor(t)
	if p := e.SetTags(ctx, ids["site"], []string{"Contrat", " Prioritaire "}); p != nil {
		t.Fatal(p)
	}
	sm := app.NewSiteManager(ws.DB())
	tree, err := sm.Tree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tree.Nodes {
		if n.ID == ids["site"] && !slices.Equal(n.Tags, []string{"Contrat", "Prioritaire"}) {
			t.Errorf("site node tags = %q", n.Tags)
		}
		if n.ID == ids["host"] && len(n.Tags) != 0 {
			t.Errorf("host node tags = %q, want none", n.Tags)
		}
	}
	if !slices.Equal(tree.Tags, []app.TagView{{Name: "Contrat", Sites: 1}, {Name: "Prioritaire", Sites: 1}}) {
		t.Errorf("tree tags = %+v", tree.Tags)
	}

	// A tag's page is addressed as "tag:" and the tag, like a node.
	rows, err := sm.Sites(ctx, "tag:contrat")
	if err != nil || len(rows) != 1 || rows[0].ID != ids["site"] {
		t.Fatalf("Sites(tag:contrat) = %+v, %v", rows, err)
	}
	if _, err := sm.Sites(ctx, "tag:nothing"); err == nil {
		t.Error("Sites of a tag no site has: want an error")
	}
}

func TestEditTags(t *testing.T) {
	ctx := context.Background()
	e, ws, ids := editor(t)
	if p := e.SetTags(ctx, ids["site"], []string{"Contrat"}); p != nil {
		t.Fatal(p)
	}
	if p := e.RenameTag(ctx, "contrat", "Maintenance"); p != nil {
		t.Fatal(p)
	}
	tree, _ := app.NewSiteManager(ws.DB()).Tree(ctx)
	if !slices.Equal(tree.Tags, []app.TagView{{Name: "Maintenance", Sites: 1}}) {
		t.Errorf("tags after the rename = %+v", tree.Tags)
	}
	if p := e.RemoveTag(ctx, "maintenance"); p != nil {
		t.Fatal(p)
	}
	if tree, _ := app.NewSiteManager(ws.DB()).Tree(ctx); len(tree.Tags) != 0 {
		t.Errorf("tags after the removal = %+v, want none", tree.Tags)
	}

	for name, tc := range map[string]struct {
		got  *app.Problem
		want string
	}{
		"empty tag":         {e.SetTags(ctx, ids["site"], []string{" "}), "problem.emptyTag"},
		"tag too long":      {e.SetTags(ctx, ids["site"], []string{"0123456789012345678901234567890123456789x"}), "problem.tagTooLong"},
		"comma in a tag":    {e.SetTags(ctx, ids["site"], []string{"a,b"}), "problem.tagCharacter"},
		"tags on a host":    {e.SetTags(ctx, ids["host"], []string{"x"}), "problem.notFound"},
		"rename to nothing": {e.RenameTag(ctx, "x", ""), "problem.emptyTag"},
		"rename a lost tag": {e.RenameTag(ctx, "gone", "x"), "problem.tagNotFound"},
		"remove a lost tag": {e.RemoveTag(ctx, "gone"), "problem.tagNotFound"},
	} {
		if code(tc.got) != tc.want {
			t.Errorf("%s: %+v, want %s", name, tc.got, tc.want)
		}
	}
	many := make([]string, 21)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	if p := e.SetTags(ctx, ids["site"], many); code(p) != "problem.tooManyTags" {
		t.Errorf("21 tags: %+v, want problem.tooManyTags", p)
	}
}
