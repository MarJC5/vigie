package workspace

import (
	"context"
	"fmt"

	"github.com/MarJC5/vigie/internal/model"
)

// SetTags gives a site its tags (ADR-0025), by model.SiteTags: a tag another site has takes the
// spelling it has there.
func (w *Workspace) SetTags(ctx context.Context, n model.Node, tags []string) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	if _, ok := tree.Site(n.ID); n.Kind != model.KindSite || !ok {
		return fmt.Errorf("%s %s: %w", n.Kind, n.ID, model.ErrNotFound)
	}
	var used []string
	for _, s := range tree.SitesUnder(model.Root) {
		if s.ID != n.ID {
			used = append(used, s.Tags...)
		}
	}
	tags, err = model.SiteTags(tags, used)
	if err != nil {
		return err
	}
	return w.db.SaveTags(ctx, map[model.ID][]string{n.ID: tags})
}

// RenameTag renames a tag on every site that has it. Renamed to another tag, both become one, in
// the spelling that one has; renamed to itself in another case, it takes the new spelling.
func (w *Workspace) RenameTag(ctx context.Context, from, to string) error {
	to, err := model.NormalizeTag(to)
	if err != nil {
		return err
	}
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	for _, c := range tree.Tags() {
		if model.TagKey(c.Name) == model.TagKey(to) && model.TagKey(c.Name) != model.TagKey(from) {
			to = c.Name
		}
	}
	return w.retag(ctx, tree, from, func(tags []string) []string {
		out, seen := []string{}, map[string]bool{}
		for _, tag := range tags {
			if model.TagKey(tag) == model.TagKey(from) {
				tag = to
			}
			if !seen[model.TagKey(tag)] {
				seen[model.TagKey(tag)] = true
				out = append(out, tag)
			}
		}
		return out
	})
}

// RemoveTag takes a tag off every site that has it; the sites themselves do not change.
func (w *Workspace) RemoveTag(ctx context.Context, tag string) error {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return err
	}
	return w.retag(ctx, tree, tag, func(tags []string) []string {
		out := []string{}
		for _, t := range tags {
			if model.TagKey(t) != model.TagKey(tag) {
				out = append(out, t)
			}
		}
		return out
	})
}

// retag changes the tags of the sites with a tag, all of them or none.
func (w *Workspace) retag(ctx context.Context, tree *model.Tree, tag string, change func([]string) []string) error {
	sites := tree.SitesUnder(model.TagNode(tag))
	if len(sites) == 0 {
		return fmt.Errorf("tag %q: %w", tag, model.ErrNotFound)
	}
	changes := map[model.ID][]string{}
	for _, s := range sites {
		changes[s.ID] = change(s.Tags)
	}
	return w.db.SaveTags(ctx, changes)
}
