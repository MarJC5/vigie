package model

import "time"

// Notice is a notification Vigie sent: what it told (Kind, as app.Alert's) about which sites or
// hosts, after which run.
type Notice struct {
	ID    ID
	RunID ID // NoID when no run made it
	Kind  string
	Names []string
	Sent  time.Time
}
