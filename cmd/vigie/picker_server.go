//go:build server

package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/MarJC5/vigie/internal/app"
)

// The build in server mode serves the window to a browser for the end-to-end test, which has no
// system file dialogs: the files go to and come from the folder VIGIE_E2E_FILES names, an export
// under its suggested name, an import from import.json or import.csv.
type picker struct {
	app func() *application.App
}

var errNoTestFiles = errors.New("VIGIE_E2E_FILES names no folder")

func testFiles() (string, error) {
	dir := os.Getenv("VIGIE_E2E_FILES")
	if dir == "" {
		return "", errNoTestFiles
	}
	return dir, nil
}

func (p picker) SaveFile(name string) (string, error) {
	dir, err := testFiles()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func (p picker) OpenFile(kind string) (string, error) {
	dir, err := testFiles()
	if err != nil {
		return "", err
	}
	if kind == app.FileCSV {
		return filepath.Join(dir, "import.csv"), nil
	}
	return filepath.Join(dir, "import.json"), nil
}
