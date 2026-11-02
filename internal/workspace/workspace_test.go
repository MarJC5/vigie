package workspace_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

// bench stores Clients/Bench (the container account) with three of its sites, and a fake runner
// replaying their recordings.
func bench(t *testing.T) (*workspace.Workspace, *store.DB) {
	t.Helper()
	ws, db, _ := setup(t)
	return ws, db
}

func benchFake(t *testing.T) (*workspace.Workspace, *sshxtest.Fake) {
	t.Helper()
	ws, _, fake := setup(t)
	return ws, fake
}

// emptyDB is a migrated database in a temporary directory.
func emptyDB(t *testing.T) *store.DB {
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
	return db
}

func setup(t *testing.T) (*workspace.Workspace, *store.DB, *sshxtest.Fake) {
	t.Helper()
	ctx := context.Background()
	db := emptyDB(t)
	clients := model.Folder{ID: model.NewID(), Name: "Clients"}
	host := model.Host{ID: model.NewID(), FolderID: clients.ID, Name: "Bench", Alias: "bench"}
	sites := []model.Site{
		{ID: model.NewID(), HostID: host.ID, Name: "outdated.example.test", Path: "~/sites/outdated"},
		{ID: model.NewID(), HostID: host.ID, Name: "network.example.test", Path: "~/sites/network", Multisite: true, Position: 1},
		{ID: model.NewID(), HostID: host.ID, Name: "broken", Path: "~/sites/broken", Position: 2},
	}
	for _, err := range []error{db.SaveFolder(ctx, clients), db.SaveHost(ctx, host)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	fake := sshxtest.NewFake()
	for _, s := range sites {
		if err := db.SaveSite(ctx, s); err != nil {
			t.Fatal(err)
		}
		wpclitest.Replay(t, fake, filepath.Base(s.Path), s.Path, "wp")
	}
	return workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now}), db, fake
}

func TestScanAFolderStoresEveryScan(t *testing.T) {
	ws, db := bench(t)
	var reports []scan.Progress
	results, err := ws.Scan(context.Background(), "Clients", workspace.ScanOptions{}, func(p scan.Progress) { reports = append(reports, p) })
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(results))
	for i, r := range results {
		paths[i] = r.Path
	}
	want := []string{"Clients/Bench/outdated.example.test", "Clients/Bench/network.example.test", "Clients/Bench/broken"}
	if !slices.Equal(paths, want) {
		t.Fatalf("scanned %q, want %q in tree order", paths, want)
	}
	// Urgent: the bench's image adds wp-config-docker.php to WordPress's files.
	if results[0].Scan.Status() != model.StatusUrgent || results[2].Scan.Outcome != model.ScanCannotLoad {
		t.Errorf("results = %q, %q", results[0].Scan.Status(), results[2].Scan.Outcome)
	}
	if len(reports) == 0 {
		t.Error("no progress reported")
	}

	latest, err := db.LatestScans(context.Background())
	if err != nil || len(latest) != 3 {
		t.Fatalf("stored scans = %d, %v; want 3", len(latest), err)
	}
	tree, err := db.LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	network, _ := tree.Site(results[1].Scan.SiteID)
	if len(network.Subsites) != 4 {
		t.Errorf("network subsites stored = %d, want 4", len(network.Subsites))
	}
}

// A complete scan of everything records when it ended, for the schedule; a scan of a part of the
// tree, or one cancelled, does not.
func TestACompleteScanOfEverythingIsRecorded(t *testing.T) {
	ctx := context.Background()
	ws, _ := bench(t)
	if _, ok, err := ws.EverythingScanned(ctx); ok || err != nil {
		t.Fatalf("EverythingScanned() before any scan = %v, %v; want none", ok, err)
	}
	if _, err := ws.Scan(ctx, "Clients", workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := ws.EverythingScanned(ctx); ok {
		t.Fatal("a scan of a folder recorded a scan of everything")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := ws.Scan(cancelled, "", workspace.ScanOptions{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Scan() cancelled = %v, want context.Canceled", err)
	}
	if _, ok, _ := ws.EverythingScanned(ctx); ok {
		t.Fatal("a cancelled scan of everything was recorded")
	}
	before := time.Now().Truncate(time.Second)
	if _, err := ws.Scan(ctx, "", workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if at, ok, err := ws.EverythingScanned(ctx); !ok || err != nil || at.Before(before) {
		t.Fatalf("EverythingScanned() = %v, %v, %v; want the end of the scan", at, ok, err)
	}
}

// Each scan of a node is a run: who started it, on what, how it ended; its scans point to it.
func TestAScanIsARun(t *testing.T) {
	ctx := context.Background()
	ws, db := bench(t)
	before := time.Now().Add(-time.Minute)
	results, err := ws.Scan(ctx, "Clients", workspace.ScanOptions{By: model.RunBySchedule}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := db.Runs(ctx, before)
	if err != nil || len(runs) != 1 {
		t.Fatalf("Runs() = %+v, %v; want one", runs, err)
	}
	run := runs[0]
	if run.Path != "Clients" || run.By != model.RunBySchedule || run.Outcome != model.RunCompleted || run.Sites != 3 || run.Finished.IsZero() {
		t.Errorf("run = %+v", run)
	}
	for _, r := range results {
		if r.Scan.RunID != run.ID {
			t.Errorf("scan of %s in run %s, want %s", r.Path, r.Scan.RunID, run.ID)
		}
	}
	// Without a starter given, the user started it.
	if _, err := ws.Scan(ctx, "Clients/Bench/outdated.example.test", workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if runs, _ := db.Runs(ctx, before); len(runs) != 2 || runs[1].By != model.RunByUser {
		t.Errorf("runs = %+v, want the second started by the user", runs)
	}
}

func TestScanOneSite(t *testing.T) {
	ws, _ := bench(t)
	results, err := ws.Scan(context.Background(), "Clients/Bench/outdated.example.test", workspace.ScanOptions{}, nil)
	if err != nil || len(results) != 1 || results[0].Scan.Outcome != model.ScanCompleted {
		t.Fatalf("Scan() = %+v, %v", results, err)
	}
}

// The next scan carries over the comparison of the files of the latest one, stored with it.
func TestTheFilesOfTheLatestScanAreCarriedOver(t *testing.T) {
	ws, fake := benchFake(t)
	ctx := context.Background()
	checksums := func() int {
		n := 0
		for _, c := range fake.Calls() {
			if strings.Contains(c, "'verify-checksums'") {
				n++
			}
		}
		return n
	}
	first, err := ws.Scan(ctx, "Clients/Bench/outdated.example.test", workspace.ScanOptions{}, nil)
	if err != nil || checksums() != 2 {
		t.Fatalf("first scan: %v, %d checksum commands", err, checksums())
	}
	second, err := ws.Scan(ctx, "Clients/Bench/outdated.example.test", workspace.ScanOptions{}, nil)
	if err != nil || checksums() != 2 {
		t.Fatalf("second scan: %v, %d checksum commands; want none more", err, checksums())
	}
	if !second[0].Scan.FilesChecked.Equal(first[0].Scan.FilesChecked) {
		t.Errorf("files checked %v, then %v", first[0].Scan.FilesChecked, second[0].Scan.FilesChecked)
	}
	if _, err := ws.Scan(ctx, "Clients/Bench/outdated.example.test", workspace.ScanOptions{CheckFiles: true}, nil); err != nil || checksums() != 4 {
		t.Fatalf("scan asked to check the files: %v, %d checksum commands; want 2 more", err, checksums())
	}
}

// A fresh scan comes back with the ignores of its site applied.
func TestAScanAppliesTheIgnoresOfItsSite(t *testing.T) {
	ws, db := bench(t)
	ctx := context.Background()
	tree, err := db.LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	node, err := tree.Resolve("Clients/Bench/outdated.example.test")
	if err != nil {
		t.Fatal(err)
	}
	key := "core-file|core|"
	if err := db.SaveIgnore(ctx, model.Ignore{SiteID: node.ID, Key: key, Reason: "the container image's", Created: time.Now()}); err != nil {
		t.Fatal(err)
	}
	results, err := ws.Scan(ctx, "Clients/Bench/outdated.example.test", workspace.ScanOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range results[0].Scan.Findings {
		if f.Key() == key && f.Ignore == nil {
			t.Errorf("finding not marked ignored: %+v", f)
		}
	}
	if results[0].Scan.Status() != model.StatusAttention {
		t.Errorf("Status() = %q, want needs attention once the urgent finding is ignored", results[0].Scan.Status())
	}
	// Stored with what it noted of itself for the journal (ADR-0030): its status with the ignores
	// of then; a first scan has no changes.
	stored, ok, err := db.LatestScan(ctx, node.ID)
	if err != nil || !ok || !stored.Recorded || stored.RecordedStatus != model.StatusAttention || len(stored.RecordedChanges) != 0 {
		t.Fatalf("stored scan noted %v, %q, %+v (%v)", stored.Recorded, stored.RecordedStatus, stored.RecordedChanges, err)
	}
	// Ignoring nothing more, a second scan notes the same status, and no change from the first.
	if _, err := ws.Scan(ctx, "Clients/Bench/outdated.example.test", workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if stored, _, err = db.LatestScan(ctx, node.ID); err != nil || !stored.Recorded || stored.RecordedStatus != model.StatusAttention ||
		stored.RecordedChanges == nil || len(stored.RecordedChanges) != 0 {
		t.Fatalf("second scan noted %v, %q, %+v (%v)", stored.Recorded, stored.RecordedStatus, stored.RecordedChanges, err)
	}
}

// Nothing is asked of an outside service before the user agreed, nor of one turned off.
func TestLookupsNeedTheUsersAgreement(t *testing.T) {
	ws, _ := bench(t)
	ctx := context.Background()
	if ws.LookupAllowed(lookup.WPVulnerability) {
		t.Fatal("allowed before the user agreed")
	}
	asked := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	if err := ws.SetLookupSettings(ctx, workspace.LookupSettings{Asked: asked}); err != nil {
		t.Fatal(err)
	}
	if ws.LookupAllowed(lookup.WPVulnerability) {
		t.Fatal("allowed after \"not now\"")
	}
	agreed := asked.Add(time.Hour)
	if err := ws.SetLookupSettings(ctx, workspace.LookupSettings{Agreed: agreed, Asked: asked,
		Off: []lookup.Destination{lookup.Site, lookup.Registry}}); err != nil {
		t.Fatal(err)
	}
	if !ws.LookupAllowed(lookup.WPVulnerability) || !ws.LookupAllowed(lookup.WordPressOrg) ||
		ws.LookupAllowed(lookup.Site) || ws.LookupAllowed(lookup.Registry) {
		t.Error("a destination turned off is still allowed, or another is not")
	}
	got, err := ws.LookupSettings(ctx)
	if err != nil || !got.Agreed.Equal(agreed) || !got.Asked.Equal(asked) ||
		!slices.Equal(got.Off, []lookup.Destination{lookup.Registry, lookup.Site}) {
		t.Errorf("LookupSettings() = %+v, %v", got, err)
	}
	if err := ws.SetLookupSettings(ctx, workspace.LookupSettings{Off: []lookup.Destination{"example.com"}}); !errors.Is(err, workspace.ErrUnknownDestination) {
		t.Errorf("an unknown destination = %v, want ErrUnknownDestination", err)
	}
}

func TestScanUnknownPath(t *testing.T) {
	ws, _ := bench(t)
	if _, err := ws.Scan(context.Background(), "Clients/Nope", workspace.ScanOptions{}, nil); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Scan(unknown) = %v, want ErrNotFound", err)
	}
}

func TestDiscoverAHost(t *testing.T) {
	ws, _ := benchWithSearch(t)
	if _, _, err := ws.Discover(context.Background(), "Clients", nil); !errors.Is(err, workspace.ErrNotAHost) {
		t.Fatalf("Discover(folder) = %v, want ErrNotAHost", err)
	}
	found, _, err := ws.Discover(context.Background(), "Clients/Bench", nil)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]discover.State{}
	for _, f := range found {
		states[f.Path] = f.State
	}
	if len(found) != 9 || states["~/sites/outdated"] != discover.StateAlreadyAdded ||
		states["~/sites/plain"] != discover.StateNew || states["~/sites/broken"] != discover.StateAlreadyAdded {
		t.Fatalf("Discover() = %v, want nine installs, those in Vigie marked", states)
	}
}

// benchWithSearch also answers the search of the account and every install it finds.
func benchWithSearch(t *testing.T) (*workspace.Workspace, *sshxtest.Fake) {
	t.Helper()
	ws, fake := benchFake(t)
	data, err := os.ReadFile(filepath.Join("..", "discover", "testdata", "recorded", "find.json"))
	if err != nil {
		t.Fatal(err)
	}
	var find struct {
		Exit   int
		Stdout string
	}
	if err := json.Unmarshal(data, &find); err != nil {
		t.Fatal(err)
	}
	fake.Set(discover.FindCommand(), sshx.Result{Stdout: []byte(find.Stdout), ExitCode: find.Exit})
	for _, fixture := range wpclitest.Fixtures {
		root := "~/sites/" + fixture
		if fixture == "parent-config" {
			root += "/public"
		}
		wpclitest.Replay(t, fake, fixture, root, "wp")
	}
	return ws, fake
}

func TestOpenCreatesTheDataDirectory(t *testing.T) {
	// The cache directory of ssh connection sharing goes under HOME; short, for the length limit
	// of Unix socket paths (the default temporary directory of macOS is too long).
	home, err := os.MkdirTemp("/tmp", "vh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")
	dir := filepath.Join(home, "data")
	ws, err := workspace.Open(context.Background(), dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("data directory = %v, %v; want 0700", info, err)
	}
	if v, err := ws.DB().SchemaVersion(context.Background()); err != nil || v < 3 {
		t.Fatalf("schema version = %d, %v; want the scans migration applied", v, err)
	}
}

// At the start, the scans stored before ADR-0030 are noted, then the history pruned (ADR-0031):
// a base that kept a year of scans is put in order once, before any run.
func TestTidyHistoryNotesThenPrunes(t *testing.T) {
	ctx := context.Background()
	ws, db := bench(t)
	site := resolve(t, ws, "Clients/Bench/outdated.example.test")
	day := 24 * time.Hour
	var ids []model.ID
	for _, age := range []time.Duration{400 * day, 200 * day, 199 * day, 198 * day, 10 * day} {
		at := time.Now().Add(-age)
		s := model.Scan{ID: model.NewID(), SiteID: site.ID, Started: at, Finished: at, Outcome: model.ScanCompleted, WordPress: "7.1.3",
			CoreChecked: true, PluginsChecked: true, ThemesChecked: true, LanguagesChecked: true,
			Items: []model.ScanItem{{Kind: model.ItemCore, Version: "7.1.3", Update: model.UpdateNone}}}
		if err := db.SaveScan(ctx, s); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, s.ID)
	}
	if err := ws.TidyHistory(ctx); err != nil {
		t.Fatal(err)
	}
	scans, err := db.Scans(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	var left []model.ID
	for _, s := range scans {
		left = append(left, s.ID)
		if !s.Recorded {
			t.Errorf("scan %s not noted", s.ID)
		}
	}
	// Older than a year: gone; the first within it: kept; the same the next day: gone; the complete
	// scan before the latest, and the latest: kept.
	if want := []model.ID{ids[1], ids[3], ids[4]}; !slices.Equal(left, want) {
		t.Errorf("scans left %v, want %v", left, want)
	}
}
