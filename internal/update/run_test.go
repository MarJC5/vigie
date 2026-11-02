package update_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/httpcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

var started = time.Date(2026, 10, 29, 13, 12, 3, 0, time.UTC)

const root = "/home/vigie/sites/updates"

// checks answers the HTTP checks in order; a healthy site after the last one given.
type checks struct {
	results []httpcheck.Result
	urls    [][]string
}

func (c *checks) Watch(_ context.Context, urls []string) httpcheck.Result {
	c.urls = append(c.urls, urls)
	if len(c.results) == 0 {
		return httpcheck.Result{}
	}
	r := c.results[0]
	c.results = c.results[1:]
	return r
}

// wp is the command line of a WP-CLI command on the updates site.
func wp(t *testing.T, c wpcli.Command) string {
	t.Helper()
	l, err := c.Remote("wp", "~/sites/updates")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

type runBench struct {
	t       *testing.T
	fake    *sshxtest.Fake
	db      *store.DB
	job     update.Job
	dir     string // the run's backups folder
	checks  *checks
	preview update.Preview
}

// newRun answers every command of an update of the updates site with success: the bench's
// recordings for WP-CLI, plain answers for the shell.
func newRun(t *testing.T) *runBench {
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
	j := job("updates")
	host := model.Host{ID: model.NewID(), Name: "Bench", Alias: "bench"}
	j.Site.HostID = host.ID
	if err := db.SaveHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSite(ctx, j.Site); err != nil {
		t.Fatal(err)
	}

	b := &runBench{t: t, fake: bench(t), db: db, job: j, dir: remote.BackupDir(j.Site.ID, started), checks: &checks{}}
	f := b.fake
	wpclitest.ReplayUpdates(t, f, "constant-debug", "constant-debug-log", "plugin-dir", "theme-dir",
		"maintenance-on", "maintenance-off", "update-plugin", "update-theme", "update-core", "update-database",
		"update-languages-core", "update-languages-plugins", "update-languages-themes")
	ok := sshx.Result{}
	f.Set(b.line(remote.RealRoot("~/sites/updates")), sshx.Result{Stdout: []byte(root + "\n")})
	f.Set(b.line(remote.MakeDir(b.dir)), ok)
	f.Set(wp(t, wpcli.DatabaseExport(b.dir+"/database.sql")), sshx.Result{Stdout: []byte(b.dir + "/database.sql\n")})
	f.Set(b.line(remote.Compress(b.dir+"/database.sql")), ok)
	f.Set(b.line(remote.Archive(b.dir+"/plugin-akismet.tar.gz", root+"/wp-content/plugins/akismet")), ok)
	f.Set(b.line(remote.Archive(b.dir+"/theme-twentytwentyfour.tar.gz", root+"/wp-content/themes/twentytwentyfour")), ok)
	f.Set(b.line(remote.ArchiveCore(b.dir+"/core.tar.gz", root)), ok)
	f.Set(b.line(remote.FileSize(root+"/wp-content/debug.log")), sshx.Result{Stdout: []byte("0\n")})
	f.Set(b.line(remote.FatalsSince(root+"/wp-content/debug.log", 0)), sshx.Result{ExitCode: 1})
	f.Set(b.line(remote.ListBackups(j.Site.ID)), sshx.Result{Stdout: []byte(filepath.Base(b.dir) + "\n")})

	p, err := updater(f).Preview(ctx, j)
	if err != nil {
		t.Fatal(err)
	}
	b.preview = p
	return b
}

// line is the command line of a remote builder, failing the test on a refused path.
func (b *runBench) line(cmd string, err error) string {
	b.t.Helper()
	if err != nil {
		b.t.Fatal(err)
	}
	return cmd
}

func (b *runBench) updater() update.Updater {
	return update.Updater{Runner: b.fake, Limiter: sshx.NewLimiter(), Store: b.db, Checker: b.checks,
		Now: func() time.Time { return started }, PID: os.Getpid()}
}

// index is the position of a command line among the calls, -1 when it was not run.
func index(calls []string, line string) int {
	return slices.Index(calls, line)
}

func TestRunUpdatesWithBackupsFirst(t *testing.T) {
	b := newRun(t)
	sel := update.Selection{Core: true, Plugins: []string{"akismet"}, Themes: []string{"twentytwentyfour"}, Languages: true}
	u := b.updater()
	var reported []model.OperationStep
	u.OnStep = func(_ model.Operation, st model.OperationStep) { reported = append(reported, st) }
	op, err := u.Run(context.Background(), b.job, b.preview, sel)
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationCompleted || op.BackupDir != b.dir {
		t.Fatalf("operation = %+v", op)
	}
	// Each step is reported as it starts and as it ends.
	if len(reported) != 2*len(op.Steps) || reported[0].Outcome != model.StepRunning || reported[1].Outcome != model.StepDone {
		t.Errorf("%d steps reported for %d steps", len(reported), len(op.Steps))
	}
	calls := b.fake.Calls()
	export := index(calls, wp(t, wpcli.DatabaseExport(b.dir+"/database.sql")))
	order := []string{
		wp(t, wpcli.DatabaseExport(b.dir+"/database.sql")),
		// The archives in the order of the updates: WordPress first.
		b.line(remote.ArchiveCore(b.dir+"/core.tar.gz", root)),
		b.line(remote.Archive(b.dir+"/plugin-akismet.tar.gz", root+"/wp-content/plugins/akismet")),
		b.line(remote.Archive(b.dir+"/theme-twentytwentyfour.tar.gz", root+"/wp-content/themes/twentytwentyfour")),
		wp(t, wpcli.Maintenance(true)),
		wp(t, wpcli.UpdateCore("7.1.4")),
		wp(t, wpcli.UpdateDatabase(false)),
		wp(t, wpcli.Maintenance(false)),
		wp(t, wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet")),
		wp(t, wpcli.UpdateOne(wpcli.ExtensionThemes, "twentytwentyfour")),
		wp(t, wpcli.UpdateLanguages(wpcli.LanguagesOfCore)),
	}
	last := -1
	for _, l := range order {
		i := index(calls, l)
		if i <= last {
			t.Fatalf("%s at %d, after %d: want this order\n%s", l, i, last, strings.Join(order, "\n"))
		}
		last = i
	}
	// WordPress removes the maintenance file of its own update: on again for the database.
	on := wp(t, wpcli.Maintenance(true))
	if again := slices.Index(calls[index(calls, wp(t, wpcli.UpdateCore("7.1.4"))):], on); again < 0 {
		t.Error("maintenance mode not turned on again after the update of WordPress's files")
	}
	if export < 0 || index(calls, b.line(remote.MakeDir(b.dir))) > export {
		t.Error("the backups folder is made before the dump")
	}
	// Each item is checked: the home and login pages after WordPress, the plugin and the theme.
	if len(b.checks.urls) < 3 || !slices.Equal(b.checks.urls[0], []string{"http://updates.example.test/", "http://updates.example.test/wp-login.php"}) {
		t.Errorf("checks = %v", b.checks.urls)
	}
	ops, err := b.db.Operations(context.Background(), b.job.Site.ID)
	if err != nil || len(ops) != 1 {
		t.Fatalf("operations = %+v, %v", ops, err)
	}
	for _, st := range ops[0].Steps {
		if st.Outcome != model.StepDone {
			t.Errorf("step %+v not done", st)
		}
	}
	// The archive steps keep the folder they saved, for a rollback.
	if i := slices.IndexFunc(ops[0].Steps, func(s model.OperationStep) bool { return s.Step == model.StepBackupFiles && s.Slug == "akismet" }); i < 0 ||
		ops[0].Steps[i].Message != root+"/wp-content/plugins/akismet" {
		t.Errorf("backup step of akismet = %+v", ops[0].Steps)
	}
}

// Every update starts with a database dump: without a dump tool, nothing starts.
func TestRunRefusedWithoutADumpTool(t *testing.T) {
	b := newRun(t)
	b.preview.DumpTool = false
	_, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}})
	if !errors.Is(err, wpcli.ErrNoDumpTool) {
		t.Fatalf("Run() = %v, want ErrNoDumpTool", err)
	}
	if ops, _ := b.db.Operations(context.Background(), b.job.Site.ID); len(ops) != 0 {
		t.Errorf("an operation was recorded: %+v", ops)
	}
}

func TestRunRefusesWhatThePreviewDidNotOffer(t *testing.T) {
	b := newRun(t)
	_, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"hello"}})
	if !errors.Is(err, update.ErrNotOffered) {
		t.Fatalf("Run() = %v, want ErrNotOffered", err)
	}
}

// A failed backup stops everything: no update, no maintenance mode.
func TestBackupFailureStopsEverything(t *testing.T) {
	b := newRun(t)
	b.fake.Set(wp(t, wpcli.DatabaseExport(b.dir+"/database.sql")), sshx.Result{ExitCode: 1, Stderr: []byte("Error: Disk quota exceeded\n")})
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Core: true, Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationFailed {
		t.Fatalf("outcome %q, want failed", op.Outcome)
	}
	calls := b.fake.Calls()
	for _, l := range []string{wp(t, wpcli.Maintenance(true)), wp(t, wpcli.UpdateCore("7.1.4")), wp(t, wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet"))} {
		if index(calls, l) >= 0 {
			t.Errorf("%s ran after a failed backup", l)
		}
	}
	ops, _ := b.db.Operations(context.Background(), b.job.Site.ID)
	if last := ops[0].Steps[len(ops[0].Steps)-1]; last.Step != model.StepBackupDatabase || last.Outcome != model.StepFailed || !strings.Contains(last.Message, "Disk quota") {
		t.Errorf("last step = %+v", last)
	}
}

// A folder outside the site's own is never archived nor touched.
func TestFolderOutsideTheSiteStops(t *testing.T) {
	b := newRun(t)
	b.fake.Set(wp(t, wpcli.ExtensionDir(wpcli.ExtensionPlugins, "akismet")), sshx.Result{Stdout: []byte("/etc/akismet\n")})
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationFailed || index(b.fake.Calls(), wp(t, wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet"))) >= 0 {
		t.Fatalf("outcome %q, calls %v", op.Outcome, b.fake.Calls())
	}
}

// The first item that breaks the site stops the run: the next ones are not updated.
func TestFailedCheckStopsBeforeTheNextItem(t *testing.T) {
	b := newRun(t)
	b.checks.results = []httpcheck.Result{{URL: "http://updates.example.test/", Status: 500, Problem: httpcheck.ProblemCriticalError}}
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}, Themes: []string{"twentytwentyfour"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Outcome != model.OperationFailed {
		t.Fatalf("outcome %q", op.Outcome)
	}
	if index(b.fake.Calls(), wp(t, wpcli.UpdateOne(wpcli.ExtensionThemes, "twentytwentyfour"))) >= 0 {
		t.Error("the theme was updated after the plugin broke the site")
	}
	last := op.Steps[len(op.Steps)-1]
	if last.Step != model.StepCheck || last.Outcome != model.StepFailed || last.Slug != "akismet" || !strings.Contains(last.Message, "500") {
		t.Errorf("last step = %+v", last)
	}
}

// A new fatal error in debug.log fails the check, even when the pages answer.
func TestNewFatalErrorFailsTheCheck(t *testing.T) {
	b := newRun(t)
	fatal := "[29-Oct-2026 13:12:18 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function x()"
	b.fake.Set(b.line(remote.FatalsSince(root+"/wp-content/debug.log", 0)), sshx.Result{Stdout: []byte(fatal + "\n")})
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	if last := op.Steps[len(op.Steps)-1]; op.Outcome != model.OperationFailed || !strings.Contains(last.Message, "undefined function x()") {
		t.Errorf("outcome %q, last step %+v", op.Outcome, last)
	}
}

// Maintenance mode is turned off even when the update of WordPress fails.
func TestMaintenanceOffAfterAFailedCoreUpdate(t *testing.T) {
	b := newRun(t)
	b.fake.Set(wp(t, wpcli.UpdateCore("7.1.4")), sshx.Result{ExitCode: 1, Stderr: []byte("Error: Download failed.\n")})
	op, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Core: true})
	if err != nil {
		t.Fatal(err)
	}
	calls := b.fake.Calls()
	if op.Outcome != model.OperationFailed || index(calls, wp(t, wpcli.Maintenance(false))) < index(calls, wp(t, wpcli.UpdateCore("7.1.4"))) {
		t.Fatalf("outcome %q, calls %v", op.Outcome, calls)
	}
}

// cancelling cancels the run when a command line is about to run.
type cancelling struct {
	sshx.Runner
	at     string
	cancel context.CancelFunc
}

func (c cancelling) Run(ctx context.Context, t sshx.Target, line string) (sshx.Result, error) {
	res, err := c.Runner.Run(ctx, t, line)
	if line == c.at {
		c.cancel()
	}
	return res, err
}

func (c cancelling) RunInput(ctx context.Context, target sshx.Target, remote string, _ []byte) (sshx.Result, error) {
	return c.Run(ctx, target, remote)
}

// A cancel stops after the current item and never leaves maintenance mode on.
func TestCancelTurnsMaintenanceOff(t *testing.T) {
	b := newRun(t)
	ctx, cancel := context.WithCancel(context.Background())
	u := b.updater()
	u.Runner = cancelling{Runner: b.fake, at: wp(t, wpcli.UpdateCore("7.1.4")), cancel: cancel}
	op, err := u.Run(ctx, b.job, b.preview, update.Selection{Core: true, Plugins: []string{"akismet"}})
	if err != nil {
		t.Fatal(err)
	}
	calls := b.fake.Calls()
	if op.Outcome != model.OperationCancelled || index(calls, wp(t, wpcli.Maintenance(false))) < 0 ||
		index(calls, wp(t, wpcli.UpdateOne(wpcli.ExtensionPlugins, "akismet"))) >= 0 {
		t.Fatalf("outcome %q, calls %v", op.Outcome, calls)
	}
}

// removedBackups runs an update and returns the folders of backups it removed.
func (b *runBench) removedBackups() []string {
	b.t.Helper()
	if _, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}}); err != nil {
		b.t.Fatal(err)
	}
	var removed []string
	for _, c := range b.fake.Calls() {
		if dir, ok := strings.CutPrefix(c, "rm -rf '"); ok {
			removed = append(removed, strings.TrimSuffix(dir, "'"))
		}
	}
	return removed
}

// A run's backups older than the limit are removed, the run's own kept (ADR-0033); no limit keeps
// the last three runs only.
func TestBackupsPastTheAgeLimitRemoved(t *testing.T) {
	for _, c := range []struct {
		keep time.Duration
		want []string
	}{
		{30 * 24 * time.Hour, []string{"20260901-080000"}},
		{0, nil},
	} {
		b := newRun(t)
		b.job.KeepFor = c.keep
		base := ".vigie/backups/" + b.job.Site.ID.String() + "/"
		b.fake.Set(b.line(remote.ListBackups(b.job.Site.ID)), sshx.Result{Stdout: []byte("20260901-080000\n20261020-080000\n" + filepath.Base(b.dir) + "\n")})
		var want []string
		for _, old := range c.want {
			b.fake.Set(b.line(remote.RemoveBackup(base+old)), sshx.Result{})
			want = append(want, base+old)
		}
		if got := b.removedBackups(); !slices.Equal(got, want) {
			t.Errorf("keep %s: removed %v, want %v", c.keep, got, want)
		}
	}
}

// The limits hold for every site of this Vigie on the account, not only the one updated; a site
// whose backups cannot be listed is left for the next update.
func TestBackupsOfTheAccountsOtherSitesPruned(t *testing.T) {
	b := newRun(t)
	other, unread := model.NewID(), model.NewID()
	b.job.Siblings, b.job.KeepFor = []model.ID{unread, other}, 30*24*time.Hour
	b.fake.Set(b.line(remote.ListBackups(unread)), sshx.Result{ExitCode: 2, Stderr: []byte("ls: cannot access")})
	b.fake.Set(b.line(remote.ListBackups(other)), sshx.Result{Stdout: []byte("20260915-080000\n20261020-080000\n20261021-080000\n20261022-080000\n20261023-080000\n")})
	base := ".vigie/backups/" + other.String() + "/"
	for _, old := range []string{"20260915-080000", "20261020-080000"} {
		b.fake.Set(b.line(remote.RemoveBackup(base+old)), sshx.Result{})
	}
	want := []string{base + "20260915-080000", base + "20261020-080000"}
	if got := b.removedBackups(); !slices.Equal(got, want) {
		t.Errorf("removed %v, want the other site's run past the limit and beyond the last three", got)
	}
}

// One operation at a time on a site, the window and the command line together.
func TestRunOnABusySite(t *testing.T) {
	b := newRun(t)
	other := model.Operation{ID: model.NewID(), SiteID: b.job.Site.ID, Kind: model.OperationUpdate, Started: started, PID: os.Getpid()}
	if err := b.db.StartOperation(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if _, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}}); !errors.Is(err, store.ErrBusy) {
		t.Fatalf("Run() = %v, want ErrBusy", err)
	}
}

// Only the last three runs keep their backups.
func TestOldBackupsRemoved(t *testing.T) {
	b := newRun(t)
	listed := "20261001-080000\n20261008-080000\n20261015-080000\n20261022-080000\n" + filepath.Base(b.dir) + "\n"
	b.fake.Set(b.line(remote.ListBackups(b.job.Site.ID)), sshx.Result{Stdout: []byte(listed)})
	base := ".vigie/backups/" + b.job.Site.ID.String() + "/"
	for _, old := range []string{"20261001-080000", "20261008-080000"} {
		b.fake.Set(b.line(remote.RemoveBackup(base+old)), sshx.Result{})
	}
	if _, err := b.updater().Run(context.Background(), b.job, b.preview, update.Selection{Plugins: []string{"akismet"}}); err != nil {
		t.Fatal(err)
	}
	calls := b.fake.Calls()
	removed := 0
	for _, c := range calls {
		if strings.HasPrefix(c, "rm -rf '"+base) {
			removed++
		}
	}
	if removed != 2 || index(calls, b.line(remote.RemoveBackup(base+"20261008-080000"))) < 0 {
		t.Errorf("%d backups removed, calls %v", removed, calls)
	}
}
