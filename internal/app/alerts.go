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
)

// Alert is something a scan made worse: a site turned urgent (Name is the site's), or a host turned
// unreachable (Name is the host's).
type Alert struct {
	Kind string
	Name string
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
			alerts = append(alerts, Alert{Kind: AlertUrgent, Name: site.Name})
		case unreachable(s) && !(scanned && unreachable(previous)) && !slices.Contains(hosts, site.HostID):
			hosts = append(hosts, site.HostID)
			if host, ok := tree.Host(site.HostID); ok {
				alerts = append(alerts, Alert{Kind: AlertUnreachable, Name: host.Name})
			}
		}
	}
	return alerts
}

// unreachable tells whether a scan could not connect to its host.
func unreachable(s model.Scan) bool {
	return s.Outcome == model.ScanFailed && s.Failure != "" && s.Failure != scan.FailureUnreadable
}
