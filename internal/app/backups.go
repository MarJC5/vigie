package app

import (
	"context"
	"strings"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
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

// Backups returns what the sites of the sites and hosts given keep on their server, as their latest
// scans read them; none for a folder, which is only deleted empty.
func (e *Editor) Backups(ctx context.Context, ids []string) ([]SiteBackupsView, error) {
	nodes, err := e.nodes(ctx, ids)
	if err != nil {
		return nil, err
	}
	tree, err := e.ws.DB().LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	var held []model.Node
	for _, n := range nodes {
		if n.Kind == model.KindSite || n.Kind == model.KindHost {
			held = append(held, n)
		}
	}
	views := []SiteBackupsView{}
	for _, site := range workspace.SitesOf(tree, held) {
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

// Delete removes sites, hosts with their sites, and empty folders from Vigie, one or several, all
// or none (ADR-0034); with backups, the sites' backups on their servers first (ADR-0033).
func (e *Editor) Delete(ctx context.Context, ids []string, backups bool) DeleteResult {
	nodes, err := e.nodes(ctx, ids)
	if err != nil {
		return DeleteResult{Problem: ProblemOf(err)}
	}
	if !backups {
		return DeleteResult{Problem: ProblemOf(e.ws.DeleteMany(ctx, nodes))}
	}
	left, err := e.ws.DeleteManyWithBackups(ctx, nodes)
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

// MoveMany puts the folders and hosts among the items in a folder ("" for the root), all or none;
// sites stay under their hosts (ADR-0034).
func (e *Editor) MoveMany(ctx context.Context, ids []string, folderID string) *Problem {
	nodes, err := e.nodes(ctx, ids)
	if err != nil {
		return ProblemOf(err)
	}
	folder, err := e.folderID(ctx, folderID)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(e.ws.MoveMany(ctx, nodes, folder))
}

// SetColorMany gives every item its own color ("none": inherit it).
func (e *Editor) SetColorMany(ctx context.Context, ids []string, color string) *Problem {
	nodes, err := e.nodes(ctx, ids)
	if err != nil {
		return ProblemOf(err)
	}
	c, err := model.ParseColor(color)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(e.ws.SetColorMany(ctx, nodes, c))
}

// TagMany adds a tag to every site the items hold (add), or takes it off those that have it.
func (e *Editor) TagMany(ctx context.Context, ids []string, tag string, add bool) *Problem {
	nodes, err := e.nodes(ctx, ids)
	if err != nil {
		return ProblemOf(err)
	}
	return ProblemOf(e.ws.TagMany(ctx, nodes, tag, add))
}

// nodes resolves the items of a selection; an unknown one refuses them all.
func (e *Editor) nodes(ctx context.Context, ids []string) ([]model.Node, error) {
	nodes := make([]model.Node, 0, len(ids))
	for _, id := range ids {
		n, err := e.node(ctx, id)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
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
