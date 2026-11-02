package app

import (
	"context"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
)

// BackupView is the backups of one update on the server, as the latest scan read them.
type BackupView struct {
	At    string `json:"at"` // RFC 3339, the update's time
	Bytes int64  `json:"bytes"`
	// Expired: the limits remove it at the next update (ADR-0033); ByAge when it is older than the
	// days kept, else it is beyond the last updates kept.
	Expired bool `json:"expired"`
	ByAge   bool `json:"byAge"`
}

// BackupsView is what a site keeps on its server, for its History tab.
type BackupsView struct {
	Read  bool         `json:"read"`  // the latest scan read them
	Days  int          `json:"days"`  // the days kept, 0 for no limit
	Kept  int          `json:"kept"`  // the updates of a site kept at most
	Items []BackupView `json:"items"` // oldest first
}

// Backups returns a site's backups on the server, as its latest scan read them.
func (u *Updates) Backups(ctx context.Context, siteID string) (BackupsView, error) {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return BackupsView{}, errNodeNotFound
	}
	days, err := u.ws.BackupDays(ctx)
	if err != nil {
		return BackupsView{}, err
	}
	view := BackupsView{Days: days, Kept: update.KeptBackups, Items: []BackupView{}}
	scan, ok, err := u.ws.DB().LatestScan(ctx, id)
	if err != nil || !ok || scan.Backups == nil {
		return view, err
	}
	view.Read = true
	keep := time.Duration(days) * 24 * time.Hour
	times := make([]time.Time, len(scan.Backups))
	for i, b := range scan.Backups {
		times[i] = b.At
	}
	now := time.Now()
	for i, past := range update.Expired(times, now, keep) {
		b := scan.Backups[i]
		view.Items = append(view.Items, BackupView{At: b.At.UTC().Format(time.RFC3339), Bytes: b.Bytes, Expired: past,
			ByAge: past && keep > 0 && now.Sub(b.At) > keep})
	}
	return view, nil
}

// RemoveOldBackups removes a site's backups past the limits, on its server; refused while the
// site is updated or scanned.
func (u *Updates) RemoveOldBackups(ctx context.Context, siteID string) *Problem {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return ProblemOf(errNodeNotFound)
	}
	u.mu.Lock()
	busy := u.busy[id]
	u.mu.Unlock()
	if busy {
		return ProblemOf(store.ErrBusy)
	}
	if u.scanning(id) {
		return ProblemOf(errScanning)
	}
	return ProblemOf(u.ws.RemoveOldBackups(ctx, id))
}

// SiteBackupsView is what one site keeps on its server, for the delete dialog.
type SiteBackupsView struct {
	SiteID string `json:"siteId"`
	Read   bool   `json:"read"` // its latest scan read them
	Count  int    `json:"count"`
	Bytes  int64  `json:"bytes"`
}

// Backups returns what the sites of a site or host keep on their server, as their latest scans
// read them; none for a folder, which is only deleted empty.
func (e *Editor) Backups(ctx context.Context, id string) ([]SiteBackupsView, error) {
	n, err := e.node(ctx, id)
	if err != nil {
		return nil, err
	}
	views := []SiteBackupsView{}
	if n.Kind != model.KindSite && n.Kind != model.KindHost {
		return views, nil
	}
	tree, err := e.ws.DB().LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	for _, site := range tree.SitesUnder(n) {
		v := SiteBackupsView{SiteID: site.ID.String()}
		scan, ok, err := e.ws.DB().LatestScan(ctx, site.ID)
		if err != nil {
			return nil, err
		}
		if ok && scan.Backups != nil {
			v.Read, v.Count = true, len(scan.Backups)
			for _, b := range scan.Backups {
				v.Bytes += b.Bytes
			}
		}
		views = append(views, v)
	}
	return views, nil
}

// DeleteResult is what a delete did: the sites whose backups stayed on a server that could not be
// reached, and the command that removes them there.
type DeleteResult struct {
	Problem *Problem `json:"problem"`
	Left    []string `json:"left"`
	Command string   `json:"command"`
}

// Delete removes a site, a host with its sites, or an empty folder from Vigie; with backups, the
// sites' backups on their server first (ADR-0033).
func (e *Editor) Delete(ctx context.Context, id string, backups bool) DeleteResult {
	n, err := e.node(ctx, id)
	if err != nil {
		return DeleteResult{Problem: ProblemOf(err)}
	}
	if !backups {
		return DeleteResult{Problem: ProblemOf(e.ws.Delete(ctx, n))}
	}
	left, err := e.ws.DeleteWithBackups(ctx, n)
	if err != nil {
		return DeleteResult{Problem: ProblemOf(err)}
	}
	result := DeleteResult{Left: []string{}}
	if len(left) > 0 {
		dirs := make([]string, len(left))
		for i, site := range left {
			result.Left = append(result.Left, site.String())
			dirs[i] = "~/.vigie/backups/" + site.String()
		}
		result.Command = "rm -rf " + strings.Join(dirs, " ")
	}
	return result
}

// BackupDays returns the days the backups of an update stay on the server, 0 for no limit.
func (d *Data) BackupDays(ctx context.Context) (int, error) {
	return d.ws.BackupDays(ctx)
}

// SetBackupDays sets the days the backups of an update stay on the server: 7, 30 or 90, 0 for no
// limit.
func (d *Data) SetBackupDays(ctx context.Context, days int) *Problem {
	return ProblemOf(d.ws.SetBackupDays(ctx, days))
}
