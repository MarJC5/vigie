package app

import (
	"context"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// Watch tells what Vigie watches in the background, to the menu bar icon and to the window: when
// the next automatic scan is due, and how many sites are urgent now.
type Watch struct {
	sites *SiteManager
	next  func(ctx context.Context) (time.Time, bool, error)
}

// WatchState is the state the menu bar icon and the window show.
type WatchState struct {
	Next   string `json:"next"`   // RFC 3339; "" while automatic scans are off
	Urgent int    `json:"urgent"` // sites whose latest scan is urgent
}

// NewWatch returns the service; next is the schedule's next scan (schedule.Schedule.Next).
func NewWatch(sites *SiteManager, next func(ctx context.Context) (time.Time, bool, error)) *Watch {
	return &Watch{sites: sites, next: next}
}

// State returns the next automatic scan and the urgent sites, as the site list shows them.
func (w *Watch) State(ctx context.Context) (WatchState, error) {
	state := WatchState{}
	next, on, err := w.next(ctx)
	if err != nil {
		return WatchState{}, err
	}
	if on {
		state.Next = next.UTC().Format(time.RFC3339)
	}
	tree, err := w.sites.Tree(ctx)
	if err != nil {
		return WatchState{}, err
	}
	for _, n := range tree.Nodes {
		if n.Status == string(model.StatusUrgent) {
			state.Urgent++
		}
	}
	return state, nil
}
