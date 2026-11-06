package model_test

import (
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

// Three levels, each allowing what the one before allows (ADR-0043, point 3).
func TestAssistantLevels(t *testing.T) {
	for _, c := range []struct {
		access    model.AssistantAccess
		read, act bool
		valid     bool
	}{
		{model.AssistantOff, false, false, true},
		{model.AssistantRead, true, false, true},
		{model.AssistantAct, true, true, true},
		{"", false, false, false},
		{"write", false, false, false},
	} {
		if got := c.access.Valid(); got != c.valid {
			t.Errorf("%q.Valid() = %v, want %v", c.access, got, c.valid)
		}
		if got := c.access.Allows(model.AssistantRead); got != c.read {
			t.Errorf("%q.Allows(read) = %v, want %v", c.access, got, c.read)
		}
		if got := c.access.Allows(model.AssistantAct); got != c.act {
			t.Errorf("%q.Allows(act) = %v, want %v", c.access, got, c.act)
		}
	}
}
