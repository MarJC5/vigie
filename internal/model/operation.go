package model

import "time"

// OperationKind is what an operation does on a server.
type OperationKind string

const (
	OperationUpdate   OperationKind = "update"
	OperationRollback OperationKind = "rollback"
)

// OperationOutcome is where an operation stands.
type OperationOutcome string

const (
	OperationRunning   OperationOutcome = "running"
	OperationCompleted OperationOutcome = "completed"
	OperationFailed    OperationOutcome = "failed" // stopped at its first failure
	OperationCancelled OperationOutcome = "cancelled"
	// OperationInterrupted: the process running it ended (Vigie quit or crashed) before it did.
	OperationInterrupted OperationOutcome = "interrupted"
)

// Operation is one run that writes on a site's server, recorded step by step: the log of what
// Vigie did there.
type Operation struct {
	ID        ID
	SiteID    ID
	Kind      OperationKind
	Started   time.Time
	Finished  time.Time // zero while it runs
	Outcome   OperationOutcome
	Summary   string
	BackupDir string // the folder of its backups on the server
	PID       int    // the process running it, to tell a running operation from an interrupted one
	// OfOperation is the update a rollback undoes, when known; zero otherwise.
	OfOperation ID
	// Automatic tells a rollback Vigie started on its own after a failed check (ADR-0042).
	Automatic bool
	// By is who started it, the window when empty; ByClient the assistant's client (ADR-0043).
	By       RunBy
	ByClient string
	Steps    []OperationStep
}

// StepKind is one kind of step of an operation.
type StepKind string

const (
	StepBackupDatabase  StepKind = "backup-database"
	StepBackupFiles     StepKind = "backup-files"
	StepMaintenanceOn   StepKind = "maintenance-on"
	StepUpdate          StepKind = "update"
	StepUpdateDatabase  StepKind = "update-database"
	StepCheck           StepKind = "check"
	StepMaintenanceOff  StepKind = "maintenance-off"
	StepRestoreFiles    StepKind = "restore-files"
	StepRestoreDatabase StepKind = "restore-database"
)

// StepOutcome is where a step stands.
type StepOutcome string

const (
	StepRunning StepOutcome = "running"
	StepDone    StepOutcome = "done"
	StepFailed  StepOutcome = "failed"
	StepSkipped StepOutcome = "skipped"
)

// OperationStep is one step of an operation; an item step names the core, plugin, theme or
// translation it is about.
type OperationStep struct {
	Position      int
	Step          StepKind
	ItemKind      ItemKind // empty for a step about the whole site
	Slug          string
	VersionBefore string
	VersionAfter  string
	Started       time.Time
	Finished      time.Time // zero while it runs
	Outcome       StepOutcome
	Message       string
}
