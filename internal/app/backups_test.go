package app_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
)

// A site's History tab shows its backups as the latest scan read them, those the limits remove
// marked, by age or beyond the last updates kept (ADR-0033).
func TestBackupsOfASite(t *testing.T) {
	ctx := context.Background()
	_, ws, ids := editor(t)
	site := uuid.MustParse(ids["site"])
	u := app.NewUpdates(ws, func(string, any) {}, app.NewScanner(ws, func(string, any) {}))
	if v, err := u.Backups(ctx, ids["site"]); err != nil || v.Read || v.Days != 30 || v.Kept != 3 {
		t.Fatalf("never scanned: %+v, %v; want not read, 30 days, 3 kept", v, err)
	}
	now := time.Now().UTC()
	scan := model.Scan{ID: model.NewID(), SiteID: site, Started: now, Finished: now, Outcome: model.ScanCompleted,
		Backups: []model.Backup{{At: now.AddDate(0, 0, -45), Bytes: 1}, {At: now.AddDate(0, 0, -3), Bytes: 2}, {At: now, Bytes: 3}}}
	if err := ws.DB().SaveScan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	v, err := u.Backups(ctx, ids["site"])
	if err != nil || !v.Read || len(v.Items) != 3 || !v.Items[0].Expired || !v.Items[0].ByAge || v.Items[1].Expired || v.Items[2].Expired {
		t.Errorf("Backups() = %+v, %v; want the first one expired by age", v, err)
	}
	d := app.NewData(ws, nil, time.Now, nil)
	if p := d.SetBackupDays(ctx, 12); code(p) != "problem.backupDays" {
		t.Errorf("SetBackupDays(12) = %+v, want problem.backupDays", p)
	}
}

// Deleting with the backups, when the server cannot be reached, deletes the site from Vigie and
// gives the command that removes its backups there.
func TestDeleteWithBackupsUnreachable(t *testing.T) {
	ctx := context.Background()
	e, ws, ids := editor(t)
	if b, err := e.Backups(ctx, ids["host"]); err != nil || len(b) != 1 || b[0].SiteID != ids["site"] || b[0].Read {
		t.Errorf("Backups(host) = %+v, %v; want its one site, not read", b, err)
	}
	r := e.Delete(ctx, ids["site"], true)
	if r.Problem != nil || len(r.Left) != 1 || r.Command != "rm -rf ~/.vigie/backups/"+ids["site"] {
		t.Errorf("Delete() = %+v", r)
	}
	if tree, _ := ws.DB().LoadTree(ctx); len(tree.SitesUnder(model.Root)) != 0 {
		t.Error("the site is still there")
	}
}
