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
