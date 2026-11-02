package app

import (
	"strings"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// LongRun is how long a run the user started lasts before its end is notified (ADR-0035).
const LongRun = 30 * time.Second

// The kinds of runs whose end is notified.
const (
	EndScan   = "scan"
	EndUpdate = "update"
)

// RunEnd is a scan or an update as it ended: what it covered, how long it took, whether it was
// stopped, and how it went (ADR-0035).
type RunEnd struct {
	Kind string
	By   model.RunBy
	// RunID is where the journal shows its notification: the run of a scan, the last operation of
	// an update.
	RunID model.ID
	// NodeIDs is what it covered, opened by a click: one node ("" for everything), or the items of
	// a selection.
	NodeIDs []string
	// Name is the node's name; empty for everything, unused for several items.
	Name              string
	Started, Finished time.Time
	Cancelled         bool // stopped by the user

	// A scan: the sites scanned, those urgent, the hosts that could not be reached.
	Sites, Urgent, Unreachable int

	// An update of the sites of a host, folder or selection: the sites updated and failed. An
	// update of one site that failed counts 1 in Failed.
	Several         bool
	Updated, Failed int

	// An update of one site: the items updated, the one that failed (its title, or its kind for
	// WordPress and the translations; neither for a step about the whole site, a backup for
	// example) and those left after it.
	Items      int
	FailedKind model.ItemKind
	FailedItem string
	NotStarted int
}

// Notifies tells whether the end is worth a notification: a run the user started, long enough,
// not stopped.
func (e RunEnd) Notifies() bool {
	return e.By == model.RunByUser && !e.Cancelled && e.Finished.Sub(e.Started) >= LongRun
}

// EndText is what the notification of a run's end says: its title and body, and the line the
// journal keeps under the run, which names what the run covered already.
type EndText struct {
	Title, Body, Line string
}

// EndNotice is the notification of a run's end: its title says what ended, its body how, and the
// alerts of the same run follow, so that one notification says it all.
func EndNotice(e RunEnd, alerts []Alert, label func(key string, vars map[string]any) string) EndText {
	var title string
	n := func(key string, count int) string { return label("notify.end."+key, map[string]any{"count": count}) }
	var details []string
	switch {
	case e.Kind == EndScan:
		title = label("notify.end.scanTitle", nil)
		details = append(details, n("scanned", e.Sites))
		if e.Urgent > 0 {
			details = append(details, n("urgent", e.Urgent))
		}
		if e.Unreachable > 0 {
			details = append(details, n("unreachable", e.Unreachable))
		}
	case e.Several:
		title = label("notify.end.updateTitle", nil)
		details = append(details, n("sitesUpdated", e.Updated))
		if e.Failed > 0 {
			details = append(details, n("sitesFailed", e.Failed))
		}
	case e.Failed > 0:
		title = label("notify.end.stoppedTitle", nil)
		item := e.FailedItem
		switch {
		case e.FailedKind == model.ItemCore:
			item = label("notify.end.core", nil)
		case e.FailedKind == model.ItemLanguage:
			item = label("notify.end.translations", nil)
		case item == "":
			item = label("notify.end.step", nil)
		}
		details = append(details, label("notify.end.itemFailed", map[string]any{"item": item}))
		if e.NotStarted > 0 {
			details = append(details, n("notStarted", e.NotStarted))
		}
	default:
		title = label("notify.end.updateTitle", nil)
		details = append(details, n("itemsUpdated", e.Items))
	}
	what := e.Name
	switch {
	case len(e.NodeIDs) > 1:
		what = n("items", len(e.NodeIDs))
	case len(e.NodeIDs) == 1 && e.NodeIDs[0] == "":
		what = label("notify.end.everything", nil)
	}
	var after []string
	for _, a := range groupAlerts(alerts) {
		after = append(after, label("notify."+a.kind, map[string]any{"count": len(a.names), "name": a.names[0], "names": strings.Join(a.names, ", ")})+".")
	}
	body := append([]string{label("notify.end.line", map[string]any{"what": what, "details": strings.Join(details, ", ")})}, after...)
	line := append([]string{title + ", " + strings.Join(details, ", ") + "."}, after...)
	return EndText{Title: title, Body: strings.Join(body, " "), Line: strings.Join(line, " ")}
}

// AlertKinds are the kinds of alerts, in the order they are told.
var AlertKinds = []string{AlertUrgent, AlertUnreachable, AlertAdmin, AlertInstalled}

type alertGroup struct {
	kind  string
	names []string
}

// groupAlerts gathers the names of the alerts by kind, in the order of AlertKinds.
func groupAlerts(alerts []Alert) []alertGroup {
	var groups []alertGroup
	for _, kind := range AlertKinds {
		var names []string
		for _, a := range alerts {
			if a.Kind == kind {
				names = append(names, a.Name)
			}
		}
		if len(names) > 0 {
			groups = append(groups, alertGroup{kind: kind, names: names})
		}
	}
	return groups
}
