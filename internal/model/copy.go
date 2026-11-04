package model

import "time"

// CopyOutcome is how a copy of a site to the computer ended (ADR-0041).
type CopyOutcome string

const (
	CopyCompleted CopyOutcome = "completed"
	CopyFailed    CopyOutcome = "failed"
	CopyCancelled CopyOutcome = "cancelled"
)

// Copy is a copy of a site to the computer, or its resume, told in the journal. The site's and
// host's names are kept, so that the journal still names a site removed since.
type Copy struct {
	ID                ID
	SiteID            ID
	SiteName          string
	HostName          string
	Folder            string // the copy's folder on the computer
	Content           string // database, content or site
	Resumed           bool
	Started, Finished time.Time
	Outcome           CopyOutcome
	Archives          int   // archives written by this run
	Bytes             int64 // their size on the computer
	Message           string
}
