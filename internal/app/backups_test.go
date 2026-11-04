package app_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/workspace"
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
	if b, err := e.Backups(ctx, []string{ids["host"]}); err != nil || len(b) != 1 || b[0].SiteID != ids["site"] || b[0].Read {
		t.Errorf("Backups(host) = %+v, %v; want its one site, not read", b, err)
	}
	r := e.Delete(ctx, []string{ids["site"]}, true)
	if r.Problem != nil || len(r.Left) != 1 || r.Command != "rm -rf ~/.vigie/backups/"+ids["site"] {
		t.Errorf("Delete() = %+v", r)
	}
	if tree, _ := ws.DB().LoadTree(ctx); len(tree.SitesUnder(model.Root)) != 0 {
		t.Error("the site is still there")
	}
}

// Several items at once (ADR-0034): moved, colored and tagged together; sites alone are not moved.
func TestEditSeveral(t *testing.T) {
	ctx := context.Background()
	e, ws, ids := editor(t)
	if p := e.MoveMany(ctx, []string{ids["site"]}, ""); code(p) != "problem.nothingToMove" {
		t.Errorf("MoveMany(site) = %+v, want problem.nothingToMove", p)
	}
	if p := e.MoveMany(ctx, []string{ids["host"], ids["site"]}, ""); p != nil {
		t.Fatal(p)
	}
	if p := e.SetColorMany(ctx, []string{ids["host"], ids["site"]}, "green"); p != nil {
		t.Fatal(p)
	}
	if p := e.TagMany(ctx, []string{ids["host"]}, "Client Dupont", true); p != nil {
		t.Fatal(p)
	}
	tree, _ := ws.DB().LoadTree(ctx)
	h, _ := tree.Host(uuid.MustParse(ids["host"]))
	s, _ := tree.Site(uuid.MustParse(ids["site"]))
	if h.FolderID != model.NoID || h.Color != model.ColorGreen || s.Color != model.ColorGreen || len(s.Tags) != 1 {
		t.Errorf("host %+v, site %+v; want the host at the root, both green, the site tagged", h, s)
	}
}

// The settings of the copies: their defaults, the folder chosen in the dialog, a cancel, the size
// refused (ADR-0041).
func TestTheSettingsOfTheCopies(t *testing.T) {
	ctx := context.Background()
	pick := &picker{}
	d, _, _ := emptyData(t, pick, workspace.Deps{Home: "/Users/u"})
	s, err := d.CopySettings(ctx)
	if err != nil || s.Folder != "/Users/u/Documents/Vigie" || s.ArchiveSize != 500 || len(s.Sizes) != 6 {
		t.Fatalf("CopySettings() = %+v, %v", s, err)
	}
	pick.open = "/Volumes/Disk/Copies"
	if s, p := d.ChooseCopyFolder(ctx); p != nil || s.Folder != "/Volumes/Disk/Copies" {
		t.Fatalf("ChooseCopyFolder() = %+v, %+v", s, p)
	}
	pick.open = ""
	if s, p := d.ChooseCopyFolder(ctx); p != nil || s.Folder != "/Volumes/Disk/Copies" {
		t.Errorf("cancelled: %+v, %+v; want the folder kept", s, p)
	}
	if p := d.SetArchiveSize(ctx, 3); code(p) != "problem.archiveSize" {
		t.Errorf("SetArchiveSize(3) = %+v", p)
	}
}
