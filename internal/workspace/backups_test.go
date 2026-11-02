package workspace_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
)

// lineOf fails the test on a refused command line.
func lineOf(t *testing.T) func(string, error) string {
	return func(line string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return line
	}
}

// Removing a site's old backups removes, on its server, those past the limits only, then reads
// what is left into its latest scan (ADR-0033).
func TestRemoveOldBackups(t *testing.T) {
	ctx := context.Background()
	ws, db, fake := setup(t)
	line := lineOf(t)
	site := resolve(t, ws, "Clients/Bench/outdated.example.test")
	now := time.Now().UTC()
	recent := now.Add(-24 * time.Hour).Format(remote.BackupTimeLayout)
	if err := db.SaveScan(ctx, model.Scan{ID: model.NewID(), SiteID: site.ID, Started: now, Finished: now, Outcome: model.ScanCompleted,
		Backups: []model.Backup{{At: now.AddDate(-1, 0, 0), Bytes: 1}}}); err != nil {
		t.Fatal(err)
	}
	dir := ".vigie/backups/" + site.ID.String() + "/"
	fake.Set(line(remote.ListBackups(site.ID)), sshx.Result{Stdout: []byte("20250101-080000\n" + recent + "\n")})
	fake.Set(line(remote.RemoveBackup(dir+"20250101-080000")), sshx.Result{})
	fake.Set(line(remote.BackupSizes(site.ID)), sshx.Result{Stdout: []byte("48000\t" + recent + "/\n")})
	if err := ws.RemoveOldBackups(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	for _, call := range fake.Calls() {
		if strings.HasPrefix(call, "rm -rf") && !strings.Contains(call, "20250101-080000") {
			t.Errorf("removed %s, want only the run past 30 days", call)
		}
	}
	latest, _, err := db.LatestScan(ctx, site.ID)
	if err != nil || len(latest.Backups) != 1 || latest.Backups[0].At.Format(remote.BackupTimeLayout) != recent || latest.Backups[0].Bytes != 48000*1024 {
		t.Errorf("latest scan's backups = %+v, %v; want the one left", latest.Backups, err)
	}
}
