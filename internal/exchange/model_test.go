package exchange_test

import (
	"slices"
	"testing"
	"time"

	"github.com/MarJC5/vigie/addon/export"
	"github.com/MarJC5/vigie/internal/exchange"
	"github.com/MarJC5/vigie/internal/model"
)

// An operation still running where it was exported runs nowhere on the machine that imports it.
func TestAnOperationStillRunningComesBackInterrupted(t *testing.T) {
	at := time.Date(2026, 10, 30, 14, 12, 0, 0, time.UTC)
	op := export.Operation{ID: model.NewID().String(), Kind: "update", Started: at, Outcome: "running",
		Steps: []export.Step{{Step: "update", ItemKind: "plugin", Slug: "akismet", Started: at, Outcome: "running"}}}
	if got := exchange.ToOperation(op, model.NewID()); got.Outcome != model.OperationInterrupted || !got.Finished.IsZero() || len(got.Steps) != 1 {
		t.Errorf("ToOperation() = %+v", got)
	}
}

// The values the format lists are the model's: the format is defined in the shared module, which
// cannot import the model.
func TestTheFormatListsTheModelsValues(t *testing.T) {
	var colors []string
	for _, c := range model.Colors() {
		colors = append(colors, c.String())
	}
	if !slices.Equal(export.Colors, colors) {
		t.Errorf("format colors %q, model %q", export.Colors, colors)
	}
	levels := []string{string(model.LevelUrgent), string(model.LevelAttention), string(model.LevelInfo), string(model.LevelNotChecked)}
	if !slices.Equal(export.Levels, levels) {
		t.Errorf("format levels %q, model %q", export.Levels, levels)
	}
}
