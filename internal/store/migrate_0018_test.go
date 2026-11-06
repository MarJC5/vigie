package store

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/MarJC5/vigie/internal/model"
)

// upTo returns the shipped migrations numbered up to n, to build a database as an older Vigie left it.
func upTo(t *testing.T, n int) fs.FS {
	t.Helper()
	list, err := loadMigrations(Migrations())
	if err != nil {
		t.Fatal(err)
	}
	sub := fstest.MapFS{}
	for _, m := range list[:n] {
		sub[m.name] = &fstest.MapFile{Data: []byte(m.sql)}
	}
	return sub
}

// A database of Vigie 0.2 keeps its runs, operations and copies through migration 0018: its runs
// keep who started them, its operations and copies read as started from the window, and a run can
// then be started by an assistant (ADR-0043).
func TestMigration0018KeepsTheRowsOfAnOlderVigie(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "vigie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx, upTo(t, 17), "0.2.0"); err != nil {
		t.Fatal(err)
	}
	host := model.Host{ID: model.NewID(), Name: "h", Alias: "h"}
	site := model.Site{ID: model.NewID(), HostID: host.ID, Name: "a"}
	if err := db.SaveHost(ctx, host); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	run, op, cp := model.NewID(), model.NewID(), model.NewID()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO runs (id, path, started_by, started_at, finished_at, outcome, sites) VALUES (?, '', 'cli', ?, ?, 'completed', 11)`,
			[]any{run.String(), timeText(at), timeText(at.Add(time.Minute))}},
		{`INSERT INTO operations (id, site_id, kind, started_at, finished_at, outcome) VALUES (?, ?, 'update', ?, ?, 'completed')`,
			[]any{op.String(), site.ID.String(), timeText(at), timeText(at.Add(time.Minute))}},
		{`INSERT INTO copies (id, site_id, site_name, host_name, folder, content, resumed, started_at, finished_at, outcome, archives, bytes)
			VALUES (?, ?, 'a', 'h', '/copies', 'database', 0, ?, ?, 'completed', 1, 10)`,
			[]any{cp.String(), site.ID.String(), timeText(at), timeText(at.Add(time.Minute))}},
	} {
		if _, err := db.sql.ExecContext(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Migrate(ctx, Migrations(), "0.3.0"); err != nil {
		t.Fatal(err)
	}
	runs, err := db.Runs(ctx, at)
	if err != nil || len(runs) != 1 || runs[0].ID != run || runs[0].By != model.RunByCLI || runs[0].ByClient != "" || runs[0].Sites != 11 {
		t.Errorf("Runs() = %+v, %v; want the command's run kept", runs, err)
	}
	ops, err := db.Operations(ctx, site.ID)
	if err != nil || len(ops) != 1 || ops[0].ID != op || ops[0].By != model.RunByUser {
		t.Errorf("Operations() = %+v, %v; want the update, from the window", ops, err)
	}
	copies, err := db.Copies(ctx, at)
	if err != nil || len(copies) != 1 || copies[0].ID != cp || copies[0].By != model.RunByUser {
		t.Errorf("Copies() = %+v, %v; want the copy, from the window", copies, err)
	}
	byAssistant := model.Run{ID: model.NewID(), By: model.RunByAssistant, ByClient: "Claude", Started: at.Add(time.Hour), Outcome: model.RunRunning}
	if err := db.StartRun(ctx, byAssistant); err != nil {
		t.Errorf("StartRun(by an assistant) after the migration: %v", err)
	}
}
