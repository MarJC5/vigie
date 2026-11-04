//go:build linux && !server

package main

import (
	"os"
	"os/exec"
	"time"

	"github.com/MarJC5/vigie/internal/trash"
)

// toTrash moves a copy to the trash of the Linux desktop, as the freedesktop.org Trash
// specification says (ADR-0041).
func toTrash(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return trash.Home{DataHome: trash.DataHome(home, os.Getenv("XDG_DATA_HOME")), Now: time.Now}.Put(path)
}

// reveal opens a copy's folder in the desktop's file manager.
func reveal(path string) error {
	return exec.Command("xdg-open", path).Start()
}
