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

func TestThemeFollowsTheSystemUntilChosen(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	prefs := app.NewPreferences(db, func() string { return "fr" })
	if got, err := prefs.Theme(ctx); err != nil || got != app.ThemeSystem {
		t.Fatalf("Theme() = %q, %v; want the system's", got, err)
	}
	if err := prefs.SetTheme(ctx, app.ThemeLight); err != nil {
		t.Fatal(err)
	}
	if got, _ := app.NewPreferences(db, nil).Theme(ctx); got != app.ThemeLight {
		t.Fatalf("Theme() after SetTheme(light) = %q", got)
	}
	if err := prefs.SetTheme(ctx, "sepia"); err == nil {
		t.Fatal("SetTheme(sepia): want an error, the theme does not exist")
	}
}

func TestVersionForTheAboutTab(t *testing.T) {
	if got := app.NewPreferences(repository(t), func() string { return "fr" }).Version(); got != "dev" {
		t.Errorf("Version() = %q, want dev in tests", got)
	}
}

// The background mode, automatic scans and notifications are off until the user turns them on.
func TestTheBackgroundSettingsAreOffUntilChosen(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	prefs := app.NewPreferences(db, nil)
	if got, err := prefs.Background(ctx); err != nil || got.On || got.AutoScan != 0 || got.Notify {
		t.Fatalf("Background() = %+v, %v; want everything off", got, err)
	}
	want := app.BackgroundSettings{On: true, AutoScan: 6, Notify: true}
	if err := prefs.SetBackground(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got, _ := app.NewPreferences(db, nil).Background(ctx); got != want {
		t.Fatalf("Background() after SetBackground = %+v, want %+v", got, want)
	}
	for _, hours := range []int{2, -1, 48} {
		if err := prefs.SetBackground(ctx, app.BackgroundSettings{AutoScan: hours}); err == nil {
			t.Errorf("SetBackground(every %d hours): want an error, not an interval offered", hours)
		}
	}
}
