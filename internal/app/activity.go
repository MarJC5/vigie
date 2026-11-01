package app

import (
	"context"
	"slices"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// ChangeRecovered is a site back from urgent or unreachable.
const ChangeRecovered = "recovered"

// Change is what a scan changed for a site: turned urgent, its host turned unreachable (as
// notifications tell them, Alerts), or back.
type Change struct {
	Kind   string `json:"kind"`   // AlertUrgent, AlertUnreachable, ChangeRecovered
	NodeID string `json:"nodeId"` // the site, or the host for AlertUnreachable
	Name   string `json:"name"`
}

// NoticeView is a notification Vigie sent.
type NoticeView struct {
	Kind  string   `json:"kind"`
	Names []string `json:"names"`
}

// ActivityEntry is a line of the journal: a run with what it changed and what Vigie notified, a
// scan made before runs were recorded, an update or a rollback.
type ActivityEntry struct {
	Kind    string   `json:"kind"`    // "run", "scan", "update", "rollback"
	At      string   `json:"at"`      // when it started, RFC 3339
	Ended   string   `json:"ended"`   // "" while it runs
	By      string   `json:"by"`      // a run's: "user", "schedule", "cli"
	Path    string   `json:"path"`    // a run's node ("" for everything), else its site's path
	SiteID  string   `json:"siteId"`  // a scan's or an operation's site
	Sites   int      `json:"sites"`   // a run's
	Outcome string   `json:"outcome"` // as stored: a run's, a scan's or an operation's
	Summary string   `json:"summary"` // an operation's
	Changes []Change `json:"changes"`
	// SiteChanges are what changed on each site since its previous complete scan (ADR-0027): a
	// run's, or a scan's without a run.
	SiteChanges []SiteChangesView `json:"siteChanges"`
	Notices     []NoticeView      `json:"notices"`
}

// Journal is the journal of the days asked, newest first.
type Journal struct {
	Since   string          `json:"since"` // RFC 3339
	Entries []ActivityEntry `json:"entries"`
}

// activityData is the part of the storage the journal reads.
type activityData interface {
	LoadTree(ctx context.Context) (*model.Tree, error)
	Runs(ctx context.Context, since time.Time) ([]model.Run, error)
	Notices(ctx context.Context, since time.Time) ([]model.Notice, error)
	ScanStatuses(ctx context.Context, since time.Time) ([]model.Scan, error)
	OperationsSince(ctx context.Context, since time.Time) ([]model.Operation, error)
}

// Activity serves the journal to the Activity page: it reads what is stored, and keeps nothing.
type Activity struct {
	repo activityData
	now  func() time.Time
}

// NewActivity returns the service.
func NewActivity(repo activityData, now func() time.Time) *Activity {
	return &Activity{repo: repo, now: now}
}

// Journal returns what happened in the last days (30 when not given), of everything, or of the
// sites under a folder, host or site.
func (a *Activity) Journal(ctx context.Context, days int, nodeID string) (Journal, error) {
	if days < 1 {
		days = 30
	}
	since := a.now().Add(-time.Duration(days) * 24 * time.Hour)
	tree, err := a.repo.LoadTree(ctx)
	if err != nil {
		return Journal{}, err
	}
	node, err := nodeByID(tree, nodeID)
	if err != nil {
		return Journal{}, err
	}
	shown := func(site model.ID) bool { return true }
	if node != model.Root {
		under := map[model.ID]bool{}
		for _, s := range tree.SitesUnder(node) {
			under[s.ID] = true
		}
		shown = func(site model.ID) bool { return under[site] }
	}
	runs, err := a.repo.Runs(ctx, since)
	if err != nil {
		return Journal{}, err
	}
	notices, err := a.repo.Notices(ctx, since)
	if err != nil {
		return Journal{}, err
	}
	scans, err := a.repo.ScanStatuses(ctx, since)
	if err != nil {
		return Journal{}, err
	}
	// Vigie's operations from the oldest scan read, to tell the versions it changed.
	from := since
	for _, s := range scans {
		if s.Started.Before(from) {
			from = s.Started
		}
	}
	ops, err := a.repo.OperationsSince(ctx, from)
	if err != nil {
		return Journal{}, err
	}

	entries := []ActivityEntry{}
	changes := map[model.ID][]Change{}              // by run
	siteChanges := map[model.ID][]SiteChangesView{} // by run
	touched := map[model.ID]bool{}                  // runs with a site shown
	previous := map[model.ID]model.Scan{}
	complete := map[model.ID]model.Scan{} // the latest complete scan of each site so far
	for _, s := range scans {             // oldest first: the scan before each one is the previous of its site
		before, had := previous[s.SiteID]
		previous[s.SiteID] = s
		last, hadComplete := complete[s.SiteID]
		if s.Outcome == model.ScanCompleted {
			complete[s.SiteID] = s
		}
		if s.Started.Before(since) || !shown(s.SiteID) {
			continue
		}
		found := changesOf(tree, before, had, s)
		var site []SiteChangesView
		if hadComplete {
			if c := model.Changes(last, s, ops); len(c) > 0 {
				name := ""
				if st, ok := tree.Site(s.SiteID); ok {
					name = st.Name
				}
				site = []SiteChangesView{{SiteID: s.SiteID.String(), Name: name, Changes: changeViews(c, s)}}
			}
		}
		if s.RunID == model.NoID {
			e := siteEntry(tree, "scan", s.SiteID, s.Started, s.Finished, string(s.Outcome))
			e.Changes = found
			e.SiteChanges = append(e.SiteChanges, site...)
			entries = append(entries, e)
			continue
		}
		siteChanges[s.RunID] = append(siteChanges[s.RunID], site...)
		touched[s.RunID] = true
		for _, c := range found {
			if !slices.Contains(changes[s.RunID], c) {
				changes[s.RunID] = append(changes[s.RunID], c)
			}
		}
	}
	for _, r := range runs {
		if node != model.Root && !touched[r.ID] {
			continue
		}
		e := ActivityEntry{
			Kind: "run", At: rfc3339(r.Started), Ended: rfc3339(r.Finished), By: string(r.By), Path: r.Path,
			Sites: r.Sites, Outcome: string(r.Outcome), Changes: changes[r.ID], SiteChanges: siteChanges[r.ID], Notices: []NoticeView{},
		}
		if e.SiteChanges == nil {
			e.SiteChanges = []SiteChangesView{}
		}
		if e.Changes == nil {
			e.Changes = []Change{}
		}
		for _, n := range notices {
			if n.RunID == r.ID {
				e.Notices = append(e.Notices, NoticeView{Kind: n.Kind, Names: n.Names})
			}
		}
		entries = append(entries, e)
	}
	for _, op := range ops {
		if !op.Started.Before(since) && shown(op.SiteID) {
			e := siteEntry(tree, string(op.Kind), op.SiteID, op.Started, op.Finished, string(op.Outcome))
			e.Summary = op.Summary
			entries = append(entries, e)
		}
	}
	// Newest first; RFC 3339 in UTC sorts as time does.
	slices.SortStableFunc(entries, func(x, y ActivityEntry) int {
		switch {
		case x.At > y.At:
			return -1
		case x.At < y.At:
			return 1
		}
		return 0
	})
	return Journal{Since: rfc3339(since), Entries: entries}, nil
}

// siteEntry is an entry of one site: a scan without a run, an update, a rollback.
func siteEntry(tree *model.Tree, kind string, site model.ID, started, finished time.Time, outcome string) ActivityEntry {
	return ActivityEntry{
		Kind: kind, At: rfc3339(started), Ended: rfc3339(finished), SiteID: site.String(),
		Path: tree.PathOf(model.Node{Kind: model.KindSite, ID: site}), Outcome: outcome,
		Changes: []Change{}, SiteChanges: []SiteChangesView{}, Notices: []NoticeView{},
	}
}

// changesOf tells what a scan changed from the one before it on its site: what Alerts tells, and
// a site back from urgent or unreachable.
func changesOf(tree *model.Tree, before model.Scan, had bool, s model.Scan) []Change {
	previous := map[model.ID]model.Scan{}
	if had {
		previous[s.SiteID] = before
	}
	found := []Change{}
	for _, a := range Alerts(tree, previous, []model.Scan{s}) {
		found = append(found, Change{Kind: a.Kind, NodeID: a.NodeID.String(), Name: a.Name})
	}
	bad := func(x model.Scan) bool { return x.Status() == model.StatusUrgent || unreachable(x) }
	if had && bad(before) && !bad(s) && s.Outcome == model.ScanCompleted {
		if site, ok := tree.Site(s.SiteID); ok {
			found = append(found, Change{Kind: ChangeRecovered, NodeID: site.ID.String(), Name: site.Name})
		}
	}
	return found
}
