//go:build server

package main

import (
	"os"
	"path/filepath"
)

// The build in server mode, for the end-to-end test, has no desktop: a copy removed goes to the
// folder trash under VIGIE_E2E_FILES, and showing a folder does nothing.
func toTrash(path string) error {
	dir, err := testFiles()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "trash"), 0o700); err != nil {
		return err
	}
	return os.Rename(path, filepath.Join(dir, "trash", filepath.Base(path)))
}

func reveal(string) error { return nil }
