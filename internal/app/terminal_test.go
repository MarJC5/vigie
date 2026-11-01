package app_test

import (
	"runtime"
	"testing"

	"github.com/MarJC5/vigie/internal/app"
)

// On macOS, Terminal opens ssh:// addresses; elsewhere it depends on the system, never an error.
func TestCanOpenSSH(t *testing.T) {
	got := app.NewSiteManager(nil).CanOpenSSH()
	if runtime.GOOS == "darwin" && !got {
		t.Error("CanOpenSSH() = false on macOS")
	}
}
