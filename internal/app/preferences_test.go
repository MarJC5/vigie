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

// A change of the background settings is told at once: the menu bar icon follows it.
func TestAChangeOfTheBackgroundSettingsIsTold(t *testing.T) {
	ctx := context.Background()
	prefs := app.NewPreferences(repository(t), nil)
	var told []app.BackgroundSettings
	prefs.OnBackground = func(b app.BackgroundSettings) { told = append(told, b) }
	want := app.BackgroundSettings{On: true, AutoScan: 1}
	if err := prefs.SetBackground(ctx, want); err != nil {
		t.Fatal(err)
	}
	if err := prefs.SetBackground(ctx, app.BackgroundSettings{AutoScan: 5}); err == nil {
		t.Fatal("SetBackground(every 5 hours): want an error")
	}
	if len(told) != 1 || told[0] != want {
		t.Fatalf("told %+v, want only the stored change", told)
	}
}

func TestTheSiteListIsLeftAsTheUserLeftIt(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	prefs := app.NewPreferences(db, func() string { return "fr" })
	if got, err := prefs.TreeView(ctx); err != nil || got.Hidden || len(got.Folded) != 0 {
		t.Fatalf("TreeView() = %+v, %v; want everything unfolded and shown", got, err)
	}
	if err := prefs.SetTreeView(ctx, app.TreeView{Folded: []string{"b", "a", "b", ""}, Hidden: true}); err != nil {
		t.Fatal(err)
	}
	got, err := app.NewPreferences(db, func() string { return "fr" }).TreeView(ctx)
	if err != nil || !got.Hidden || len(got.Folded) != 2 || got.Folded[0] != "a" || got.Folded[1] != "b" {
		t.Fatalf("TreeView() after SetTreeView = %+v, %v; want a and b folded, once each, and the list hidden", got, err)
	}
}

func TestAnUnreadableSiteListSettingIsForgotten(t *testing.T) {
	ctx := context.Background()
	db := repository(t)
	if err := db.SetSetting(ctx, "tree", "{not json"); err != nil {
		t.Fatal(err)
	}
	if got, err := app.NewPreferences(db, func() string { return "fr" }).TreeView(ctx); err != nil || got.Hidden || len(got.Folded) != 0 {
		t.Fatalf("TreeView() = %+v, %v; want the default", got, err)
	}
}
