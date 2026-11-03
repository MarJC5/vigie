package app

import (
	"context"
	"errors"

	"uuid"

	"github.com/MarJC5/vigie/internal/wpcliinstall"
)

// WPCLIOffer is the WP-CLI Vigie offers to install on an account that has none (ADR-0039): its
// version, its size, and where it goes.
type WPCLIOffer struct {
	Version string `json:"version"`
	Size    int64  `json:"size"`
	Path    string `json:"path"`
}

// WPCLIInstallEvent carries a WPCLIInstallProgress to the window while WP-CLI is installed.
const WPCLIInstallEvent = "vigie:wpcli-install"

// WPCLIInstallProgress is where an install stands: its step (download, check, send, test), and
// for a download or a send the bytes so far.
type WPCLIInstallProgress struct {
	Step  string `json:"step"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// WPCLIInstallResult is the answer of InstallWPCLI: the command that answered and its version, or
// the step it stopped at and why.
type WPCLIInstallResult struct {
	Command    string   `json:"command"`
	Version    string   `json:"version"`
	FailedStep string   `json:"failedStep"`
	Problem    *Problem `json:"problem"`
}

// InstallWPCLI installs WP-CLI on the account of the details (ADR-0039), after the user confirmed
// it, sending WPCLIInstallEvent as it goes. hostID names a stored host, whose command then
// becomes the one that answered; empty for the wizard, or for a host being edited, whose form
// takes the command.
func (w *HostWizard) InstallWPCLI(ctx context.Context, d HostDetails, hostID string) WPCLIInstallResult {
	h, err := d.host()
	if err != nil {
		return WPCLIInstallResult{Problem: ProblemOf(err)}
	}
	stored := hostID != ""
	if stored {
		if h.ID, err = uuid.Parse(hostID); err != nil {
			return WPCLIInstallResult{Problem: ProblemOf(errNodeNotFound)}
		}
	}
	answer, failed, err := w.ws.InstallWPCLI(ctx, h, stored, progressEvents(func(p WPCLIInstallProgress) { w.emit(WPCLIInstallEvent, p) }))
	if err != nil {
		p := ProblemOf(err)
		if errors.Is(err, wpcliinstall.ErrExists) {
			p.Detail = wpcliinstall.Path
		}
		return WPCLIInstallResult{FailedStep: string(failed), Problem: p}
	}
	return WPCLIInstallResult{Command: answer.Command, Version: answer.Version}
}

// progressEvents tells each step once, and the bytes of a download or a send every fiftieth of
// the way, rather than at each read.
func progressEvents(send func(WPCLIInstallProgress)) func(wpcliinstall.Progress) {
	var last wpcliinstall.Progress
	return func(p wpcliinstall.Progress) {
		changed := p.Step != last.Step
		far := p.Total > 0 && (p.Done == p.Total || p.Done-last.Done >= p.Total/50)
		if !changed && !far {
			return
		}
		last = p
		send(WPCLIInstallProgress{Step: string(p.Step), Done: p.Done, Total: p.Total})
	}
}
