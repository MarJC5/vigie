package model

import "time"

// WPCLIInstall is an install of WP-CLI on an account by Vigie (ADR-0039), as the journal tells it.
// It may come before its host is stored (the add-host wizard): Account links it to its host.
type WPCLIInstall struct {
	ID                ID
	Account           string // Host.Account
	HostName          string
	Started, Finished time.Time
	Completed         bool
	Version           string // the WP-CLI installed
	Command           string // the host's command once installed
	// FailedStep is where a failed install stopped: download, check, send or test; Message what
	// was said there.
	FailedStep string
	Message    string
}
