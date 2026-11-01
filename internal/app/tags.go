package app

import (
	"context"
	"errors"

	"github.com/MarJC5/vigie/internal/model"
)

// SetTags gives a site its tags (ADR-0025): trimmed, each once, a tag another site has taking
// the spelling it has there.
func (e *Editor) SetTags(ctx context.Context, id string, tags []string) *Problem {
	n, err := e.node(ctx, id)
	if err != nil {
		return ProblemOf(err)
	}
	if n.Kind != model.KindSite {
		return ProblemOf(errNodeNotFound) // only a site has tags
	}
	return ProblemOf(e.ws.SetTags(ctx, n, tags))
}

// RenameTag renames a tag on every site that has it; renamed to another tag, both become one.
func (e *Editor) RenameTag(ctx context.Context, from, to string) *Problem {
	return tagProblem(e.ws.RenameTag(ctx, from, to))
}

// RemoveTag takes a tag off every site that has it.
func (e *Editor) RemoveTag(ctx context.Context, tag string) *Problem {
	return tagProblem(e.ws.RemoveTag(ctx, tag))
}

// tagProblem says a tag no site has any more as such, not as a missing folder.
func tagProblem(err error) *Problem {
	if errors.Is(err, model.ErrNotFound) {
		return ProblemOf(errTagNotFound)
	}
	return ProblemOf(err)
}
