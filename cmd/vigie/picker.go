package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/MarJC5/vigie/internal/app"
)

// picker opens the system's file dialogs for the Data service, once the application exists.
type picker struct {
	app func() *application.App
}

// SaveFile asks where to save an export, suggesting its name; "" when cancelled.
func (p picker) SaveFile(name string) (string, error) {
	return p.app().Dialog.SaveFile().SetFilename(name).AddFilter("JSON", "*.json").CanCreateDirectories(true).
		PromptForSingleSelection()
}

// OpenFile asks for an export or a CSV site list; "" when cancelled.
func (p picker) OpenFile(kind string) (string, error) {
	dialog := p.app().Dialog.OpenFile().CanChooseFiles(true).CanChooseDirectories(false)
	if kind == app.FileCSV {
		dialog.AddFilter("CSV", "*.csv")
	} else {
		dialog.AddFilter("JSON", "*.json")
	}
	return dialog.PromptForSingleSelection()
}
