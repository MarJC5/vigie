package model

import "time"

// NoticeEnded is the kind of the notification of a long run's end (ADR-0035).
const NoticeEnded = "ended"

// Notice is a notification Vigie sent: what it told (Kind, as app.Alert's, or NoticeEnded) about
// which sites or hosts, after which run.
type Notice struct {
	ID    ID
	RunID ID // NoID when no run made it; for the end of an update, its last operation
	Kind  string
	Names []string
	Text  string // NoticeEnded: what the notification said, its title then its body
	Sent  time.Time
}
