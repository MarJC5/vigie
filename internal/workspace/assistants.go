package workspace

import (
	"context"
	"fmt"

	"github.com/MarJC5/vigie/internal/model"
)

// assistantAccessSetting holds what an assistant may do through vigie mcp (ADR-0043, point 3).
const assistantAccessSetting = "mcp.access"

// AssistantAccess tells what an assistant may do: off by default, and off for a stored value this
// Vigie does not know. vigie mcp reads it at every call, so a change applies at once.
func (w *Workspace) AssistantAccess(ctx context.Context) (model.AssistantAccess, error) {
	value, _, err := w.db.Setting(ctx, assistantAccessSetting)
	if err != nil {
		return model.AssistantOff, err
	}
	if access := model.AssistantAccess(value); access.Valid() {
		return access, nil
	}
	return model.AssistantOff, nil
}

// SetAssistantAccess sets what an assistant may do.
func (w *Workspace) SetAssistantAccess(ctx context.Context, access model.AssistantAccess) error {
	if !access.Valid() {
		return fmt.Errorf("%q: %w", access, model.ErrInvalidAssistantAccess)
	}
	return w.db.SetSetting(ctx, assistantAccessSetting, string(access))
}

// assistantErrorsSetting lets an assistant read a site's errors with "on", on top of the level read.
const assistantErrorsSetting = "mcp.errors"

// AssistantErrors tells whether an assistant may read a site's errors: off by default, and off for a
// stored value this Vigie does not know. It stands apart from the level, which leaves it as set.
// vigie mcp reads it at every call, as the level.
func (w *Workspace) AssistantErrors(ctx context.Context) (bool, error) {
	value, _, err := w.db.Setting(ctx, assistantErrorsSetting)
	return value == "on", err
}

// SetAssistantErrors lets an assistant read a site's errors, or stops it.
func (w *Workspace) SetAssistantErrors(ctx context.Context, on bool) error {
	value := "off"
	if on {
		value = "on"
	}
	return w.db.SetSetting(ctx, assistantErrorsSetting, value)
}
