package app_test

import (
	"context"
	"testing"

	"github.com/MarJC5/vigie/internal/app"
)

func TestLanguageDefaultsToTheSystem(t *testing.T) {
	prefs := app.NewPreferences(repository(t), func() string { return "fr" })
	if got, err := prefs.Language(context.Background()); err != nil || got != "fr" {
		t.Fatalf("Language() = %q, %v; want the system language", got, err)
	}
}

func TestLanguageChoiceIsStored(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	prefs := app.NewPreferences(db, func() string { return "fr" })
	if err := prefs.SetLanguage(ctx, "en"); err != nil {
		t.Fatal(err)
	}
	if got, _ := app.NewPreferences(db, func() string { return "fr" }).Language(ctx); got != "en" {
		t.Fatalf("Language() after SetLanguage(en) = %q", got)
	}
	if err := prefs.SetLanguage(ctx, "de"); err == nil {
		t.Fatal("SetLanguage(de): want an error, the language is not supported")
	}
}
