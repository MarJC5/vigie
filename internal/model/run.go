package model

import "time"

// RunBy says who started a run.
type RunBy string

// Who starts a run.
const (
	RunByUser      RunBy = "user"      // from the window
	RunBySchedule  RunBy = "schedule"  // the automatic scans
	RunByCLI       RunBy = "cli"       // vigie scan
	RunByAssistant RunBy = "assistant" // an assistant, through vigie mcp (ADR-0043)
)

// RunOutcome says how a run ended.
type RunOutcome string

// How a run ends.
const (
	RunRunning   RunOutcome = "running"
	RunCompleted RunOutcome = "completed"
	RunCancelled RunOutcome = "cancelled"
)

// Run is one scan of a node: of everything, a folder, a host or a site. Each scan of a site it made
// points to it (Scan.RunID).
type Run struct {
	ID       ID
	Path     string // the node, by its path when the run started; "" for everything
	By       RunBy
	ByClient string // the assistant's client, when it gave its name; "" otherwise
	Started  time.Time
	Finished time.Time // zero while running
	Outcome  RunOutcome
	Sites    int
}
