package store_test

import (
	"context"
	"testing"
)

func TestSettings(t *testing.T) {
	ctx := context.Background()
	db := migrated(t)
	if _, ok, err := db.Setting(ctx, "language"); err != nil || ok {
		t.Fatalf("unset setting = ok %v, err %v; want not found", ok, err)
	}
	must(t, db.SetSetting(ctx, "language", "fr"))
	must(t, db.SetSetting(ctx, "language", "en")) // replaces
	value, ok, err := db.Setting(ctx, "language")
	if err != nil || !ok || value != "en" {
		t.Fatalf("Setting(language) = %q, %v, %v", value, ok, err)
	}
}
