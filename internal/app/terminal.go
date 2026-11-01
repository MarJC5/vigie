package app

import (
	"os/exec"
	"runtime"
	"strings"
)

// CanOpenSSH tells whether the system opens ssh:// addresses in a terminal (ADR-0023): macOS's
// Terminal declares them; on Linux, a program set for x-scheme-handler/ssh does.
func (s *SiteManager) CanOpenSSH() bool {
	switch runtime.GOOS {
	case "darwin":
		return true
	case "linux":
		out, err := exec.Command("xdg-mime", "query", "default", "x-scheme-handler/ssh").Output()
		return err == nil && strings.TrimSpace(string(out)) != ""
	}
	return false
}
