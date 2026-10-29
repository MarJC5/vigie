package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/i18n"
)

// roles maps the standard items of app.MenuBar to the ones of Wails.
var roles = map[string]application.Role{
	"about":            application.About,
	"hide":             application.Hide,
	"hideOthers":       application.HideOthers,
	"showAll":          application.ShowAll,
	"quit":             application.Quit,
	"closeWindow":      application.CloseWindow,
	"undo":             application.Undo,
	"redo":             application.Redo,
	"cut":              application.Cut,
	"copy":             application.Copy,
	"paste":            application.Paste,
	"selectAll":        application.SelectAll,
	"toggleFullscreen": application.ToggleFullscreen,
	"minimise":         application.Minimise,
	"zoom":             application.Zoom,
	"bringAllToFront":  application.BringAllToFront,
}

// menuBar builds app.MenuBar in the interface language; the items with an action are sent to the
// window with emit.
func menuBar(texts *i18n.Catalogs, lang string, emit func(name string, data any)) (*application.Menu, error) {
	label := func(key string) string { return texts.T(lang, key, map[string]any{"app": "Vigie"}) }
	bar := application.NewMenu()
	for _, section := range app.MenuBar {
		title := "Vigie" // the application menu: macOS shows the application's name
		if section.Key != "" {
			title = label(section.Key)
		}
		menu := bar.AddSubmenu(title)
		for _, entry := range section.Items {
			switch {
			case entry.Separator:
				menu.AddSeparator()
			case entry.Role != "":
				role, ok := roles[entry.Role]
				if !ok {
					return nil, fmt.Errorf("menu: unknown role %q", entry.Role)
				}
				if item := menu.AddRole(role).FindByRole(role); item != nil {
					item.SetLabel(label(entry.Key))
				}
			default:
				action := entry.Action
				item := menu.Add(label(entry.Key)).OnClick(func(*application.Context) {
					emit(app.MenuEvent, app.MenuCommand{Action: action})
				})
				if entry.Accelerator != "" {
					item.SetAccelerator(entry.Accelerator)
				}
			}
		}
	}
	return bar, nil
}
