package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

// The assistants are off until the user lets them read, then act; a level Vigie does not know is
// refused, and a stored value it does not know reads as off (ADR-0043, point 3).
func TestAssistantsAreOffUntilTurnedOn(t *testing.T) {
	ctx := context.Background()
	ws, db, _ := emptyVigie(t)
	if got, err := ws.AssistantAccess(ctx); err != nil || got != model.AssistantOff {
		t.Fatalf("AssistantAccess() = %q, %v; want off by default", got, err)
	}
	for _, level := range []model.AssistantAccess{model.AssistantRead, model.AssistantAct, model.AssistantOff} {
		if err := ws.SetAssistantAccess(ctx, level); err != nil {
			t.Fatal(err)
		}
		if got, err := ws.AssistantAccess(ctx); err != nil || got != level {
			t.Errorf("AssistantAccess() = %q, %v; want %q", got, err, level)
		}
	}
	if err := ws.SetAssistantAccess(ctx, "write"); !errors.Is(err, model.ErrInvalidAssistantAccess) {
		t.Errorf("SetAssistantAccess(write) = %v, want ErrInvalidAssistantAccess", err)
	}
	if err := db.SetSetting(ctx, "mcp.access", "everything"); err != nil {
		t.Fatal(err)
	}
	if got, err := ws.AssistantAccess(ctx); err != nil || got != model.AssistantOff {
		t.Errorf("AssistantAccess() with an unknown stored value = %q, %v; want off", got, err)
	}
}
