package app

import (
	"context"
	"slices"
	"strings"
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
	// Text is what the notification of a long run's end said (Kind "ended", ADR-0035).
	Text string `json:"text"`
}

// ActivityEntry is a line of the journal: a run with what it changed and what Vigie notified, a
// scan made before runs were recorded, an update or a rollback, an install of WP-CLI.
type ActivityEntry struct {
	Kind    string   `json:"kind"`    // "run", "scan", "update", "rollback", "wpcli"
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
	// WPCLI is an install of WP-CLI's (ADR-0039); its Path is its host's, or the name the host
	// had when it is not stored.
	WPCLI *WPCLIInstallView `json:"wpCli"`
}

// WPCLIInstallView is an install of WP-CLI on an account: its version, the host's command once
// done, or the step it stopped at and what was said there.
type WPCLIInstallView struct {
	HostID     string `json:"hostId"` // empty when its host is not stored
	Version    string `json:"version"`
	Command    string `json:"command"`
	FailedStep string `json:"failedStep"`
	Message    string `json:"message"`
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
	ScanStatuses(ctx context.Context, since time.Time, sites []model.ID) ([]model.Scan, error)
	JournalScans(ctx context.Context, since time.Time, sites []model.ID) ([]model.Scan, error)
	OperationsSince(ctx context.Context, since time.Time) ([]model.Operation, error)
	WPCLIInstalls(ctx context.Context, since time.Time) ([]model.WPCLIInstall, error)
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
	var sites []model.ID // nil: every site's scans are read
	if node != model.Root {
		under := map[model.ID]bool{}
		sites = []model.ID{}
		for _, s := range tree.SitesUnder(node) {
			under[s.ID] = true
			sites = append(sites, s.ID)
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
	// What each scan noted of itself when it was made (ADR-0030): its status and its changes, without
	// its details. A scan stored before and not noted yet has the journal computed as before.
	scans, err := a.repo.JournalScans(ctx, since, sites)
	if err != nil {
		return Journal{}, err
	}
	noted := !slices.ContainsFunc(scans, func(s model.Scan) bool { return !s.Recorded })
	if !noted {
		if scans, err = a.repo.ScanStatuses(ctx, since, sites); err != nil {
			return Journal{}, err
		}
	}
	status := model.Scan.Status
	if noted {
		status = func(s model.Scan) model.SiteStatus { return s.RecordedStatus }
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
		found := changesOf(tree, before, had, s, status)
		var site []SiteChangesView
		var c []model.Change
		switch {
		case noted:
			c = s.RecordedChanges
		case hadComplete:
			c = model.Changes(last, s, ops)
		}
		if len(c) > 0 {
			name := ""
			if st, ok := tree.Site(s.SiteID); ok {
				name = st.Name
			}
			site = []SiteChangesView{{SiteID: s.SiteID.String(), Name: name, Changes: changeViews(c)}}
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
				e.Notices = append(e.Notices, NoticeView{Kind: n.Kind, Names: n.Names, Text: n.Text})
			}
		}
		entries = append(entries, e)
	}
	for _, op := range ops {
		if !op.Started.Before(since) && shown(op.SiteID) {
			e := siteEntry(tree, string(op.Kind), op.SiteID, op.Started, op.Finished, string(op.Outcome))
			e.Summary = op.Summary
			// The end of an update is told under its last operation (ADR-0035).
			for _, n := range notices {
				if n.RunID == op.ID {
					e.Notices = append(e.Notices, NoticeView{Kind: n.Kind, Names: n.Names, Text: n.Text})
				}
			}
			entries = append(entries, e)
		}
	}
	installs, err := a.repo.WPCLIInstalls(ctx, since)
	if err != nil {
		return Journal{}, err
	}
	entries = append(entries, installEntries(tree, node, installs)...)
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
func changesOf(tree *model.Tree, before model.Scan, had bool, s model.Scan, status func(model.Scan) model.SiteStatus) []Change {
	previous := map[model.ID]model.Scan{}
	if had {
		previous[s.SiteID] = before
	}
	found := []Change{}
	for _, a := range alertsBy(tree, previous, []model.Scan{s}, status) {
		found = append(found, Change{Kind: a.Kind, NodeID: a.NodeID.String(), Name: a.Name})
	}
	bad := func(x model.Scan) bool { return status(x) == model.StatusUrgent || unreachable(x) }
	if had && bad(before) && !bad(s) && s.Outcome == model.ScanCompleted {
		if site, ok := tree.Site(s.SiteID); ok {
			found = append(found, Change{Kind: ChangeRecovered, NodeID: site.ID.String(), Name: site.Name})
		}
	}
	return found
}

// installEntries are the installs of WP-CLI, each under the stored host of its account, those of
// the node asked when it is not the root.
func installEntries(tree *model.Tree, node model.Node, installs []model.WPCLIInstall) []ActivityEntry {
	hosts := map[string]model.Host{}
	tree.Walk(func(n model.Node, _ int) {
		if n.Kind != model.KindHost {
			return
		}
		if h, ok := tree.Host(n.ID); ok {
			if _, taken := hosts[h.Account()]; !taken {
				hosts[h.Account()] = h
			}
		}
	})
	under := ""
	if node != model.Root {
		under = tree.PathOf(node)
	}
	entries := []ActivityEntry{}
	for _, i := range installs {
		h, stored := hosts[i.Account]
		path, hostID := i.HostName, ""
		if stored {
			path, hostID = tree.PathOf(model.Node{Kind: model.KindHost, ID: h.ID}), h.ID.String()
		}
		if under != "" && (!stored || (path != under && !strings.HasPrefix(path, under+"/"))) {
			continue
		}
		outcome := string(model.OperationCompleted)
		if !i.Completed {
			outcome = string(model.OperationFailed)
		}
		entries = append(entries, ActivityEntry{
			Kind: "wpcli", At: rfc3339(i.Started), Ended: rfc3339(i.Finished), Path: path, Outcome: outcome,
			Changes: []Change{}, SiteChanges: []SiteChangesView{}, Notices: []NoticeView{},
			WPCLI: &WPCLIInstallView{HostID: hostID, Version: i.Version, Command: i.Command, FailedStep: i.FailedStep, Message: i.Message},
		})
	}
	return entries
}
