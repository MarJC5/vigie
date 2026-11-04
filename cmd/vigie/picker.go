//go:build !server

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

// ChooseFolder asks for the folder of the copies (ADR-0041); "" when cancelled.
func (p picker) ChooseFolder() (string, error) {
	return p.app().Dialog.OpenFile().CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true).
		PromptForSingleSelection()
}

// OpenFile asks for an export, a CSV site list, or any file an add-on reads; "" when cancelled.
func (p picker) OpenFile(kind string) (string, error) {
	dialog := p.app().Dialog.OpenFile().CanChooseFiles(true).CanChooseDirectories(false)
	switch kind {
	case app.FileCSV:
		dialog.AddFilter("CSV", "*.csv")
	case app.FileVigie:
		dialog.AddFilter("JSON", "*.json")
	}
	return dialog.PromptForSingleSelection()
}
