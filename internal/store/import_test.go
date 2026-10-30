package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
)

// An import never changes what is stored: a node under a stored ID refuses the whole batch.
func TestAnImportUnderAStoredIDAddsNothing(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	stored := model.Folder{ID: model.NewID(), Name: "Clients"}
	must(t, db.SaveFolder(ctx, stored))
	batch := store.ImportBatch{Folders: []model.Folder{
		{ID: model.NewID(), Name: "New"},
		{ID: stored.ID, Name: "Renamed"},
	}}
	if err := db.Import(ctx, batch); !errors.Is(err, store.ErrIDTaken) {
		t.Fatalf("Import() = %v, want ErrIDTaken", err)
	}
	tree, err := db.LoadTree(ctx)
	must(t, err)
	if f, _ := tree.Folder(stored.ID); f.Name != "Clients" {
		t.Errorf("stored folder = %q, want it unchanged", f.Name)
	}
	if _, err := tree.Resolve("New"); err == nil {
		t.Error("part of the batch was added")
	}
}
