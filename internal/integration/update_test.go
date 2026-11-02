//go:build integration

package integration_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/httpcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// updateBench is the updates site over SSH, as the app updates it: a database for the
// operations, the account's web server for the checks.
func updateBench(t *testing.T, user string) (update.Updater, update.Job, *store.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "updates.example.test", Path: "~/sites/updates",
		URL: "http://updates.example.test"}
	if err := db.SaveHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSite(ctx, site); err != nil {
		t.Fatal(err)
	}

	a := sshAccess(t)
	target := a.target(a.key)
	target.User = user
	u := update.Updater{
		Runner: a.runner(t, a.trusted(t)), Limiter: sshx.NewLimiter(), Store: db, Checker: benchChecker(t),
		Now: time.Now, PID: os.Getpid(),
	}
	return u, update.Job{Site: site, Target: target, HostKey: "bench", Max: 2, WPCLI: "wp"}, db
}

// benchChecker asks the account's web server for every site name: the fixture names resolve
// nowhere else.
func benchChecker(t *testing.T) httpcheck.Checker {
	t.Helper()
	ctx := context.Background()
	host, err := account.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := account.MappedPort(ctx, "80/tcp")
	if err != nil {
		t.Fatal(err)
	}
	var dialer net.Dialer
	transport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port.Port()))
	}}
	return httpcheck.Checker{Client: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		// opcache.revalidate_freq is 2 s in the image: one second more to be past it.
		Settle: 3 * time.Second, Window: 3 * time.Second, Interval: time.Second}
}

// pluginVersion reads a plugin's version on the updates site.
func pluginVersion(t *testing.T, slug string) string {
	t.Helper()
	res, err := runAs(context.Background(), "wp", "plugin", "get", slug, "--field=version", "--skip-plugins", "--skip-themes", "--path=sites/updates")
	if err != nil || res.exit != 0 {
		t.Fatalf("version of %s: %v %s", slug, err, res.stderr)
	}
	return strings.TrimSpace(res.stdout)
}

// A plugin whose new version breaks the site: the run stops on its check, and the rollback brings
// the old version back.
func TestBreakingUpdateRolledBackOverSSH(t *testing.T) {
	ctx := context.Background()
	u, job, _ := updateBench(t, "vigie")
	preview, err := u.Preview(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := u.Run(ctx, job, preview, update.Selection{Plugins: []string{"fixture-breaks"}})
	if err != nil {
		t.Fatal(err)
	}
	last := failed.Steps[len(failed.Steps)-1]
	if failed.Outcome != model.OperationFailed || last.Step != model.StepCheck || !strings.Contains(last.Message, "critical error") {
		t.Fatalf("run = %q, last step %+v", failed.Outcome, last)
	}
	if v := pluginVersion(t, "fixture-breaks"); v != "2.0.0" {
		t.Fatalf("fixture-breaks %s after the run, want 2.0.0 in place", v)
	}

	back, err := u.Rollback(ctx, job, failed)
	if err != nil {
		t.Fatal(err)
	}
	if back.Outcome != model.OperationCompleted {
		t.Fatalf("rollback = %q, steps %+v", back.Outcome, back.Steps)
	}
	if v := pluginVersion(t, "fixture-breaks"); v != "1.0.0" {
		t.Errorf("fixture-breaks %s after the rollback, want 1.0.0", v)
	}
	if status, _ := getSite(t, "updates.example.test", "/"); status != http.StatusOK {
		t.Errorf("home page after the rollback: %d", status)
	}
}

// WordPress, a plugin from wordpress.org, one offering its own update, a theme and the
// translations, with their backups on the server.
func TestUpdateOverSSH(t *testing.T) {
	ctx := context.Background()
	u, job, db := updateBench(t, "vigie")
	// Older runs, of this site and of another site of the account (ADR-0033): kept 30 days, and the
	// last three runs of a site at most, whatever their age.
	backups := ".vigie/backups/" + job.Site.ID.String()
	other := model.NewID()
	otherBackups := ".vigie/backups/" + other.String()
	recent := time.Now().UTC().Add(-24 * time.Hour).Format("20060102-150405")
	older := []string{"20250101-080000", "20250102-080000", recent}
	job.KeepFor, job.Siblings = 30*24*time.Hour, []model.ID{other}
	for _, dir := range []string{backups + "/" + older[0], backups + "/" + older[1], backups + "/" + older[2],
		otherBackups + "/20250101-080000", otherBackups + "/" + recent} {
		if res, err := runAs(ctx, "sh", "-c", "mkdir -p "+dir+" && echo old > "+dir+"/database.sql.gz"); err != nil || res.exit != 0 {
			t.Fatalf("older backups: %v %s", err, res.stderr)
		}
	}
	preview, err := u.Preview(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	op, err := u.Run(ctx, job, preview, update.Selection{
		Core: true, Plugins: []string{"akismet", "fixture-pro"}, Themes: []string{"twentytwentyfour"}, Languages: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationCompleted {
		t.Fatalf("run = %q, steps %+v", op.Outcome, op.Steps)
	}
	if v := pluginVersion(t, "fixture-pro"); v != "1.1.0" {
		t.Errorf("fixture-pro %s, want 1.1.0", v)
	}
	res, err := runAs(ctx, "wp", "core", "version", "--path=sites/updates")
	if err != nil || strings.TrimSpace(res.stdout) != "7.1.4" {
		t.Errorf("WordPress %q, %v; want 7.1.4", res.stdout, err)
	}
	res, err = runAs(ctx, "sh", "-c", "ls "+op.BackupDir+" && stat -c %a "+op.BackupDir)
	if err != nil || res.exit != 0 {
		t.Fatalf("backups: %v %s", err, res.stderr)
	}
	for _, file := range []string{"database.sql.gz", "core.tar.gz", "plugin-akismet.tar.gz", "plugin-fixture-pro.tar.gz", "theme-twentytwentyfour.tar.gz", "700"} {
		if !strings.Contains(res.stdout, file) {
			t.Errorf("backups: %q has no %s", res.stdout, file)
		}
	}
	// The backups are whole: the dump is a database's, the archives open.
	res, err = runAs(ctx, "sh", "-c", "gzip -dc "+op.BackupDir+"/database.sql.gz | grep -c 'CREATE TABLE' && "+
		"tar -tzf "+op.BackupDir+"/plugin-akismet.tar.gz | grep -c akismet.php")
	if lines := strings.Fields(res.stdout); err != nil || res.exit != 0 || len(lines) != 2 || lines[0] == "0" || lines[1] == "0" {
		t.Errorf("backups not whole: %v %q %s", err, res.stdout, res.stderr)
	}
	res, err = runAs(ctx, "sh", "-c", "ls -1 "+backups+" && ls -1 "+otherBackups)
	if want := strings.Join([]string{recent, filepath.Base(op.BackupDir), recent}, "\n"); err != nil || strings.TrimSpace(res.stdout) != want {
		t.Errorf("backups kept: %q, want the runs of the last 30 days, of both sites: %q", res.stdout, want)
	}
	ops, err := db.Operations(ctx, job.Site.ID)
	if err != nil || len(ops) != 1 || len(ops[0].Steps) == 0 {
		t.Errorf("operations recorded: %+v, %v", ops, err)
	}
	// A scan of the site reads its backups left, with their sizes.
	s := scan.Scanner{Runner: u.Runner, Limiter: sshx.NewLimiter(), Now: time.Now}
	scanned, _ := s.Site(ctx, scan.Job{Site: job.Site, Target: job.Target, HostKey: "bench", Max: 2, WPCLI: "wp"}, nil)
	if b := scanned.Backups; len(b) != 2 || b[1].At.Format("20060102-150405") != filepath.Base(op.BackupDir) || b[1].Bytes < 1024 {
		t.Errorf("backups read by a scan: %+v, want yesterday's and the update's, with their sizes", b)
	}
}

// Every update starts with a database dump: an account without a dump tool is refused.
func TestUpdateWithoutADumpToolOverSSH(t *testing.T) {
	ctx := context.Background()
	u, job, _ := updateBench(t, "nodump")
	preview, err := u.Preview(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	if preview.DumpTool {
		t.Fatal("the preview found a dump tool")
	}
	if _, err := u.Run(ctx, job, preview, update.Selection{Plugins: []string{"akismet"}}); !errors.Is(err, wpcli.ErrNoDumpTool) {
		t.Fatalf("Run() = %v, want ErrNoDumpTool", err)
	}
}
