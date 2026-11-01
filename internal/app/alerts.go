package app

import (
	"slices"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
)

// The kinds of alerts, each a notification.
const (
	AlertUrgent      = "urgent"      // a site turned urgent
	AlertUnreachable = "unreachable" // a host Vigie could not connect to any more
	AlertAdmin       = "admin"       // a site got a new administrator (ADR-0027)
	AlertInstalled   = "installed"   // a plugin or theme was installed on a site outside Vigie
)

// Alert is something a scan made worse: a site turned urgent (Name and NodeID are the site's), or a
// host turned unreachable (the host's).
type Alert struct {
	Kind   string
	Name   string
	NodeID model.ID
}

// Alerts compares the latest scans before a scan with the ones it made, in their order, and tells
// what got worse: never what was already known, never what got better.
func Alerts(tree *model.Tree, before map[model.ID]model.Scan, after []model.Scan) []Alert {
	var alerts []Alert
	var hosts []model.ID
	for _, s := range after {
		site, ok := tree.Site(s.SiteID)
		if !ok {
			continue
		}
		previous, scanned := before[s.SiteID]
		switch {
		case s.Status() == model.StatusUrgent && (!scanned || previous.Status() != model.StatusUrgent):
			alerts = append(alerts, Alert{Kind: AlertUrgent, Name: site.Name, NodeID: site.ID})
		case unreachable(s) && !(scanned && unreachable(previous)) && !slices.Contains(hosts, site.HostID):
			hosts = append(hosts, site.HostID)
			if host, ok := tree.Host(site.HostID); ok {
				alerts = append(alerts, Alert{Kind: AlertUnreachable, Name: host.Name, NodeID: host.ID})
			}
		}
	}
	return alerts
}

// unreachable tells whether a scan could not connect to its host.
func unreachable(s model.Scan) bool {
	return s.Outcome == model.ScanFailed && s.Failure != "" && s.Failure != scan.FailureUnreadable
}

// ChangeAlerts tells, once per site, the changes that notify (ADR-0027): a new administrator, and
// a plugin or theme installed outside Vigie (Vigie never installs one).
func ChangeAlerts(tree *model.Tree, changes map[model.ID][]model.Change) []Alert {
	var alerts []Alert
	for _, site := range tree.SitesUnder(model.Root) {
		for _, kind := range []struct {
			change model.ChangeKind
			alert  string
		}{{model.ChangeAdminAdded, AlertAdmin}, {model.ChangeInstalled, AlertInstalled}} {
			if slices.ContainsFunc(changes[site.ID], func(c model.Change) bool { return c.Kind == kind.change }) {
				alerts = append(alerts, Alert{Kind: kind.alert, Name: site.Name, NodeID: site.ID})
			}
		}
	}
	return alerts
}
