package workspace_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/MarJC5/vigie/internal/errorlog"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/remote"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// The site's real folder on the server, as PHP writes its paths.
const errorsRoot = "/home/vigie/sites/outdated"

var errorsNow = time.Date(2026, 11, 9, 10, 32, 0, 0, time.UTC)

type errorsBench struct {
	ws   *workspace.Workspace
	db   *store.DB
	fake *sshxtest.Fake
	site model.Site
	path string // the database's file
}

func newErrorsBench(t *testing.T) errorsBench {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vigie.db")
	db, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, store.Migrations(), "test"); err != nil {
		t.Fatal(err)
	}
	folder := model.Folder{ID: model.NewID(), Name: "Clients"}
	host := model.Host{ID: model.NewID(), FolderID: folder.ID, Name: "Bench", Alias: "bench"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "outdated.example.test", Path: "~/sites/outdated"}
	for _, err := range []error{db.SaveFolder(ctx, folder), db.SaveHost(ctx, host), db.SaveSite(ctx, site)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := sshxtest.NewFake()
	b := errorsBench{ws: workspace.New(db, workspace.Deps{Runner: fake, Now: func() time.Time { return errorsNow }}), db: db, fake: fake, site: site, path: path}
	line := lineOf(t)
	fake.Set(line(remote.RealRoot(site.Path)), sshx.Result{Stdout: []byte(errorsRoot + "\n")})
	return b
}

// constants answers WP_DEBUG and WP_DEBUG_LOG as WP-CLI does; "" is a constant not defined.
func (b errorsBench) constants(t *testing.T, debug, log string) {
	t.Helper()
	line := lineOf(t)
	for name, value := range map[string]string{"WP_DEBUG": debug, "WP_DEBUG_LOG": log} {
		res := sshx.Result{Stdout: []byte(value + "\n")}
		if value == "" {
			res = sshx.Result{ExitCode: 1, Stderr: []byte("Error: The constant '" + name + "' is not defined in the 'wp-config.php' file.\n")}
		}
		b.fake.Set(line(wpcli.ConfigConstant(name).Remote("wp", b.site.Path)), res)
	}
}

// log answers the read of the file where WordPress writes it by default.
func (b errorsBench) log(t *testing.T, out string) {
	t.Helper()
	b.fake.Set(lineOf(t)(remote.ErrorLines(errorsRoot+"/wp-content/debug.log")), sshx.Result{Stdout: []byte(out)})
}

const errorsLog = "38200000\n" +
	"[28-Oct-2026 11:02:00 UTC] PHP Warning:  mail(): Failed to connect to mailserver, sending to marie@example.test in " + errorsRoot + "/wp-content/plugins/contact-form-7/includes/mail.php on line 207\n" +
	"[03-Nov-2026 02:14:00 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_order() in " + errorsRoot + "/wp-content/themes/astra-child/functions.php:42\n" +
	"[09-Nov-2026 08:03:00 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_order() in " + errorsRoot + "/wp-content/themes/astra-child/functions.php:42\n"

func TestSiteErrorsStates(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name       string
		debug, log string
		out        string
		want       workspace.ErrorsState
		groups     int
	}{
		{"nothing written", "", "", "", workspace.ErrorsOff, 0},
		{"WP_DEBUG on without its log", "true", "false", "", workspace.ErrorsOff, 0},
		{"written, not created yet", "true", "true", "", workspace.ErrorsEmpty, 0},
		{"written, empty", "true", "true", "0\n", workspace.ErrorsEmpty, 0},
		{"left from before", "false", "true", errorsLog, workspace.ErrorsLeft, 2},
		{"written", "true", "true", errorsLog, workspace.ErrorsRead, 2},
		{"written, nothing an error", "true", "true", "2400000\n", workspace.ErrorsRead, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newErrorsBench(t)
			b.constants(t, tc.debug, tc.log)
			b.log(t, tc.out)
			got, err := b.ws.SiteErrors(ctx, b.site.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.want || len(got.Groups) != tc.groups {
				t.Errorf("state %q with %d groups, want %q with %d", got.State, len(got.Groups), tc.want, tc.groups)
			}
			if got.File != "wp-content/debug.log" || !got.Read.Equal(errorsNow) {
				t.Errorf("file %q read at %v", got.File, got.Read)
			}
		})
	}
}

func TestSiteErrorsRead(t *testing.T) {
	b := newErrorsBench(t)
	b.constants(t, "true", "true")
	b.log(t, errorsLog)
	got, err := b.ws.SiteErrors(context.Background(), b.site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Size != 38200000 || got.Whole || got.More || got.Lines != 3 || !got.Since.Equal(time.Date(2026, 10, 28, 11, 2, 0, 0, time.UTC)) {
		t.Errorf("size %d, whole %v, more %v, %d lines since %v", got.Size, got.Whole, got.More, got.Lines, got.Since)
	}
	fatal := got.Groups[0]
	if fatal.Type != errorlog.Fatal || fatal.Kind != errorlog.Theme || fatal.Slug != "astra-child" || fatal.Count != 2 {
		t.Errorf("first group %+v", fatal)
	}
	if w := got.Groups[1]; w.Message != "mail(): Failed to connect to mailserver, sending to [email]" {
		t.Errorf("warning %q", w.Message)
	}
}

// A WordPress that writes elsewhere: the file WP_DEBUG_LOG names, outside the site, said as it is.
func TestSiteErrorsOfALogElsewhere(t *testing.T) {
	b := newErrorsBench(t)
	b.constants(t, "true", `"/home/vigie/logs/php.log"`)
	b.fake.Set(lineOf(t)(remote.ErrorLines("/home/vigie/logs/php.log")), sshx.Result{Stdout: []byte("0\n")})
	got, err := b.ws.SiteErrors(context.Background(), b.site.ID)
	if err != nil || got.File != "/home/vigie/logs/php.log" || got.State != workspace.ErrorsEmpty {
		t.Errorf("SiteErrors() = file %q, state %q, %v", got.File, got.State, err)
	}
}

// What changed in the 7 days before each group's first time: Vigie's updates and rollbacks, and the
// changes seen between two scans, the versions Vigie changed told by its operations only. Newest
// first; nothing after a group's first time, nothing older than its 7 days.
func TestSiteErrorsWhatChangedJustBefore(t *testing.T) {
	ctx := context.Background()
	b := newErrorsBench(t)
	b.constants(t, "true", "true")
	b.log(t, errorsLog)
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, time.Month(10+day/100), day%100, hour, minute, 0, 0, time.UTC)
	}
	update := func(kind model.OperationKind, started time.Time, slug string) model.Operation {
		op := model.Operation{ID: model.NewID(), SiteID: b.site.ID, Kind: kind, Started: started, Finished: started.Add(time.Minute), Outcome: model.OperationCompleted,
			Steps: []model.OperationStep{{Position: 0, Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: slug, VersionBefore: "9.8.1", VersionAfter: "9.8.5", Started: started, Finished: started, Outcome: model.StepDone}}}
		if err := b.db.StartOperation(ctx, op); err != nil {
			t.Fatal(err)
		}
		if err := b.db.FinishOperation(ctx, op.ID, model.OperationCompleted, op.Finished, ""); err != nil {
			t.Fatal(err)
		}
		return op
	}
	inUpdate := update(model.OperationUpdate, at(102, 23, 41), "woocommerce")     // 2 Nov, before the fatal error
	inRollback := update(model.OperationRollback, at(102, 23, 50), "woocommerce") // 2 Nov
	update(model.OperationUpdate, at(105, 9, 0), "akismet")                       // 5 Nov, after both first times
	scanAt := func(finished time.Time, changes ...model.Change) {
		s := model.Scan{ID: model.NewID(), SiteID: b.site.ID, Started: finished.Add(-time.Minute), Finished: finished, Outcome: model.ScanCompleted,
			Recorded: true, RecordedStatus: model.StatusUpToDate, RecordedChanges: changes}
		if err := b.db.SaveScan(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	theme := model.Change{Kind: model.ChangeThemeSwitched, Subject: "theme:astra-child", From: "Astra", To: "Astra Child"}
	installed := model.Change{Kind: model.ChangeInstalled, Subject: "plugin:host-cache", Title: "Host Cache", To: "1.2"}
	scanAt(at(10, 3, 0), model.Change{Kind: model.ChangeInstalled, Subject: "plugin:old"})                                         // 10 Oct, too old
	scanAt(at(101, 3, 0), theme, model.Change{Kind: model.ChangeVersion, Subject: "plugin:woocommerce", ByVigie: true}, installed) // 1 Nov
	scanAt(at(104, 3, 0), model.Change{Kind: model.ChangeInstalled, Subject: "plugin:late"})                                       // 4 Nov, after

	got, err := b.ws.SiteErrors(ctx, b.site.ID)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, before := range got.Before {
		switch {
		case before.Operation != nil:
			lines = append(lines, fmt.Sprintf("%s %s %s", before.At.Format("02.01 15:04"), before.Operation.Kind, before.Operation.ID))
		case before.Change != nil:
			lines = append(lines, fmt.Sprintf("%s %s %s", before.At.Format("02.01 15:04"), before.Change.Kind, before.Change.Subject))
		}
	}
	want := []string{
		"02.11 23:50 rollback " + inRollback.ID.String(),
		"02.11 23:41 update " + inUpdate.ID.String(),
		"01.11 03:00 theme-switched theme:astra-child",
		"01.11 03:00 installed plugin:host-cache",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("before:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

// Nothing of a read is kept: no text of the log in any table after it.
func TestSiteErrorsStoresNothing(t *testing.T) {
	b := newErrorsBench(t)
	b.constants(t, "true", "true")
	b.log(t, errorsLog)
	if _, err := b.ws.SiteErrors(context.Background(), b.site.ID); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("sqlite", b.path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	tables, err := conn.Query("SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	tables.Close()
	if len(names) < 10 {
		t.Fatalf("only %d tables read", len(names))
	}
	witness := false
	for _, name := range names {
		rows, err := conn.Query(`SELECT * FROM "` + name + `"`)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			for i, v := range values {
				if b, ok := v.([]byte); ok {
					values[i] = string(b)
				}
			}
			text := fmt.Sprint(values...)
			// The witness: the reading finds what the database does keep.
			witness = witness || strings.Contains(text, b.site.Name)
			for _, s := range []string{"wc_get_order", "mailserver", "marie@", "functions.php", "debug.log"} {
				if strings.Contains(text, s) {
					t.Errorf("table %s keeps %q: %s", name, s, text)
				}
			}
		}
		rows.Close()
	}
	if !witness {
		t.Error("the site's name not found in any table: the tables were not read")
	}
}

// A server that does not answer: an error, and nothing half read.
func TestSiteErrorsFailed(t *testing.T) {
	ctx := context.Background()
	b := newErrorsBench(t)
	b.constants(t, "true", "true")
	if _, err := b.ws.SiteErrors(ctx, b.site.ID); !errors.Is(err, sshxtest.ErrUnexpected) {
		t.Errorf("SiteErrors() without an answer = %v", err)
	}
	if _, err := b.ws.SiteErrors(ctx, model.NewID()); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("SiteErrors() of no site = %v, want model.ErrNotFound", err)
	}
	b.fake.Set(lineOf(t)(remote.ErrorLines(errorsRoot+"/wp-content/debug.log")), sshx.Result{ExitCode: 1, Stderr: []byte("tail: cannot open: Permission denied\n")})
	if _, err := b.ws.SiteErrors(ctx, b.site.ID); err == nil {
		t.Error("SiteErrors() of a log that cannot be read: no error")
	}
}
