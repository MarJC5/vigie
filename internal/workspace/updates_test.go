package workspace_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
	"github.com/MarJC5/vigie/internal/wpcli/wpclitest"
)

// updates stands for update.Updater: each site answers as told, and the runs are recorded.
type updates struct {
	mu       sync.Mutex
	previews map[string]update.Preview // by site name
	errs     map[string]error          // a preview that cannot be read
	outcomes map[string]model.OperationOutcome
	refusals map[string]error
	runs     []string
	running  map[model.ID]int // runs at once per host
	overlap  bool
	hostOf   map[string]model.ID
	chosen   map[string]update.Selection // what each run was given, by site name
	auto     map[string]bool             // whether each run rolls back on its own, by site name
	// breaks names, by site name, the plugin whose check fails; with the rollback on, it goes back
	// on its own, stored as Vigie would (ADR-0042).
	breaks map[string]string
	db     *store.DB
	fake   *sshxtest.Fake
}

func (u *updates) Preview(_ context.Context, job update.Job) (update.Preview, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if err := u.errs[job.Site.Name]; err != nil {
		return update.Preview{}, err
	}
	return u.previews[job.Site.Name], nil
}

func (u *updates) Run(_ context.Context, job update.Job, _ update.Preview, sel update.Selection) (model.Operation, error) {
	u.mu.Lock()
	if u.chosen != nil {
		u.chosen[job.Site.Name] = sel
	}
	host := u.hostOf[job.Site.Name]
	u.running[host]++
	if u.running[host] > 1 {
		u.overlap = true
	}
	u.runs = append(u.runs, job.Site.Name)
	u.auto[job.Site.Name] = job.AutoRollback
	err := u.refusals[job.Site.Name]
	outcome := u.outcomes[job.Site.Name]
	broken := u.breaks[job.Site.Name]
	u.mu.Unlock()
	time.Sleep(10 * time.Millisecond) // long enough for two runs on a host to meet
	u.mu.Lock()
	u.running[host]--
	u.mu.Unlock()
	if err != nil {
		return model.Operation{}, err
	}
	if outcome == "" {
		outcome = model.OperationCompleted
	}
	op := model.Operation{ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationUpdate, Outcome: outcome}
	for _, slug := range sel.Plugins {
		check := model.OperationStep{Step: model.StepCheck, ItemKind: model.ItemPlugin, Slug: slug, Outcome: model.StepDone}
		if slug == broken {
			check.Outcome, check.Message = model.StepFailed, "http://a1.example.test/ shows the critical error page of WordPress (status 500)"
		}
		op.Steps = append(op.Steps, model.OperationStep{Step: model.StepUpdate, ItemKind: model.ItemPlugin, Slug: slug,
			VersionBefore: "6.8.10", VersionAfter: "6.9.0", Outcome: model.StepDone}, check)
		if slug == broken && job.AutoRollback {
			u.rollBack(job, op, slug)
		}
	}
	return op, nil
}

// rollBack stores the rollback on its own of a plugin of an update, as package update does.
func (u *updates) rollBack(job update.Job, of model.Operation, slug string) {
	ctx := context.Background()
	back := model.Operation{ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationRollback, Started: time.Now(),
		OfOperation: of.ID, Automatic: true}
	if err := u.db.StartOperation(ctx, back); err != nil {
		panic(err)
	}
	restore := model.OperationStep{Step: model.StepRestoreFiles, ItemKind: model.ItemPlugin, Slug: slug,
		VersionBefore: "6.9.0", VersionAfter: "6.8.10", Started: time.Now(), Finished: time.Now(), Outcome: model.StepDone}
	if err := u.db.SaveOperationStep(ctx, back.ID, restore); err != nil {
		panic(err)
	}
	if err := u.db.FinishOperation(ctx, back.ID, model.OperationCompleted, time.Now(), "plugin "+slug+" back to 6.8.10"); err != nil {
		panic(err)
	}
}

func (u *updates) Rollback(_ context.Context, job update.Job, failed model.Operation) (model.Operation, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.runs = append(u.runs, "rollback "+job.Site.Name+" "+failed.ID.String())
	return model.Operation{ID: model.NewID(), SiteID: job.Site.ID, Kind: model.OperationRollback, Outcome: model.OperationCompleted}, nil
}

var pending = update.Preview{DumpTool: true, Plugins: []update.Item{{Kind: model.ItemPlugin, Slug: "akismet", From: "5.3", To: "5.7.2"}}}

// twoHosts stores host A with sites a1, a2, a3 and host B with b1, all on the plain fixture's
// recordings for the scans that follow an update.
func twoHosts(t *testing.T) (*workspace.Workspace, *updates) {
	t.Helper()
	ctx := context.Background()
	db := emptyDB(t)
	fake := sshxtest.NewFake()
	u := &updates{previews: map[string]update.Preview{}, errs: map[string]error{}, outcomes: map[string]model.OperationOutcome{},
		refusals: map[string]error{}, running: map[model.ID]int{}, hostOf: map[string]model.ID{}, auto: map[string]bool{},
		breaks: map[string]string{}, db: db, fake: fake}
	for _, h := range []struct {
		name  string
		sites []string
	}{{"A", []string{"a1", "a2", "a3"}}, {"B", []string{"b1"}}} {
		host := model.Host{ID: model.NewID(), Name: h.name, Alias: h.name}
		if err := db.SaveHost(ctx, host); err != nil {
			t.Fatal(err)
		}
		for i, name := range h.sites {
			path := "~/sites/" + name
			site := model.Site{ID: model.NewID(), HostID: host.ID, Name: name, Path: path, Position: i}
			if err := db.SaveSite(ctx, site); err != nil {
				t.Fatal(err)
			}
			wpclitest.Replay(t, fake, "plain", path, "wp")
			u.previews[name] = pending
			u.hostOf[name] = host.ID
		}
	}
	ws := workspace.New(db, workspace.Deps{Runner: fake, Now: time.Now, Updates: func(func(model.Operation, model.OperationStep)) workspace.Updates { return u }})
	return ws, u
}

func outcomes(results []workspace.SiteUpdate) map[string]workspace.SiteOutcome {
	got := map[string]workspace.SiteOutcome{}
	for _, r := range results {
		got[r.Site.Name] = r.Outcome
	}
	return got
}

func previewAll(t *testing.T, ws *workspace.Workspace) []workspace.SitePreview {
	t.Helper()
	previews, err := ws.PreviewNode(context.Background(), model.Root)
	if err != nil {
		t.Fatal(err)
	}
	return previews
}

func TestUpdateSitesHostByHost(t *testing.T) {
	ws, u := twoHosts(t)
	previews := previewAll(t, ws)
	if len(previews) != 4 || previews[0].Path != "A/a1" {
		t.Fatalf("previews = %+v", previews)
	}
	results := ws.UpdateSites(context.Background(), workspace.Everything(previews), workspace.Follow{})
	want := map[string]workspace.SiteOutcome{"a1": workspace.SiteUpdated, "a2": workspace.SiteUpdated, "a3": workspace.SiteUpdated, "b1": workspace.SiteUpdated}
	if got := outcomes(results); !maps.Equal(got, want) {
		t.Errorf("outcomes = %v", got)
	}
	if u.overlap {
		t.Error("two sites of a host updated at once")
	}
	// Each site updated is scanned again.
	for _, r := range results {
		if n, err := ws.DB().ScanCount(context.Background(), r.Site.ID); err != nil || n != 1 {
			t.Errorf("%s scanned %d times, %v", r.Site.Name, n, err)
		}
	}
}

// The first failure on a host stops it: its next sites are not started; another host goes on.
func TestFirstFailureStopsTheHost(t *testing.T) {
	ws, u := twoHosts(t)
	u.outcomes["a1"] = model.OperationFailed
	results := ws.UpdateSites(context.Background(), workspace.Everything(previewAll(t, ws)), workspace.Follow{})
	want := map[string]workspace.SiteOutcome{"a1": workspace.SiteFailed, "a2": workspace.SiteNotStarted, "a3": workspace.SiteNotStarted, "b1": workspace.SiteUpdated}
	if got := outcomes(results); !maps.Equal(got, want) {
		t.Errorf("outcomes = %v", got)
	}
}

// A refusal (no dump tool, a busy site) writes nothing: the host goes on.
func TestRefusalDoesNotStopTheHost(t *testing.T) {
	ws, u := twoHosts(t)
	u.refusals["a1"] = wpcli.ErrNoDumpTool
	results := ws.UpdateSites(context.Background(), workspace.Everything(previewAll(t, ws)), workspace.Follow{})
	if got := outcomes(results); got["a1"] != workspace.SiteRefused || got["a2"] != workspace.SiteUpdated {
		t.Errorf("outcomes = %v", got)
	}
	if !errors.Is(results[0].Err, wpcli.ErrNoDumpTool) {
		t.Errorf("a1: %v, want the reason", results[0].Err)
	}
}

// A site with nothing to update is left alone; one whose preview failed is reported, not run.
func TestNothingToUpdateAndUnreadablePreviews(t *testing.T) {
	ws, u := twoHosts(t)
	u.previews["a1"] = update.Preview{DumpTool: true}
	u.errs["a2"] = wpcli.ErrCannotLoad
	previews := previewAll(t, ws)
	if !errors.Is(previews[1].Err, wpcli.ErrCannotLoad) {
		t.Fatalf("a2 preview = %+v", previews[1])
	}
	results := ws.UpdateSites(context.Background(), workspace.Everything(previews), workspace.Follow{})
	if got := outcomes(results); got["a1"] != workspace.SiteNothing || got["a2"] != workspace.SiteRefused || got["a3"] != workspace.SiteUpdated {
		t.Errorf("outcomes = %v", got)
	}
	for _, name := range u.runs {
		if name == "a1" || name == "a2" {
			t.Errorf("%s was run", name)
		}
	}
}

// A cancel lets the current site finish; the sites not reached are not started.
func TestCancelLeavesTheRestNotStarted(t *testing.T) {
	ws, _ := twoHosts(t)
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	results := ws.UpdateSites(ctx, workspace.Everything(previewAll(t, ws)), workspace.Follow{OnSite: func(workspace.SiteUpdate) {
		once.Do(cancel)
	}})
	notStarted := 0
	for _, r := range results {
		if r.Outcome == workspace.SiteNotStarted {
			notStarted++
		}
	}
	if notStarted == 0 {
		t.Errorf("outcomes = %v, want some not started", outcomes(results))
	}
}

// A rollback finds the site and its failed operation in the database.
func TestRollbackOfAStoredOperation(t *testing.T) {
	ctx := context.Background()
	ws, u := twoHosts(t)
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	node, err := tree.Resolve("A/a1")
	if err != nil {
		t.Fatal(err)
	}
	failed := model.Operation{ID: model.NewID(), SiteID: node.ID, Kind: model.OperationUpdate, Started: time.Now(), PID: 1}
	if err := ws.DB().StartOperation(ctx, failed); err != nil {
		t.Fatal(err)
	}
	if err := ws.DB().FinishOperation(ctx, failed.ID, model.OperationFailed, time.Now(), ""); err != nil {
		t.Fatal(err)
	}
	op, err := ws.Rollback(ctx, node.ID, failed.ID, nil)
	if err != nil || op.Kind != model.OperationRollback {
		t.Fatalf("Rollback() = %+v, %v", op, err)
	}
	if len(u.runs) != 1 || u.runs[0] != "rollback a1 "+failed.ID.String() {
		t.Errorf("runs = %v", u.runs)
	}
	if _, err := ws.Rollback(ctx, node.ID, model.NewID(), nil); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("an unknown operation: %v, want ErrNotFound", err)
	}
}

// An update ignored on a site is left out of a bulk run (ADR-0026); a site with nothing else has
// nothing to update.
func TestABulkRunLeavesTheIgnoredUpdatesOut(t *testing.T) {
	ctx := context.Background()
	ws, u := twoHosts(t)
	u.chosen = map[string]update.Selection{}
	builder := update.Item{Kind: model.ItemPlugin, Slug: "page-builder-kit", From: "4.2.0", To: "5.0.0"}
	u.previews["a1"] = update.Preview{DumpTool: true, Plugins: []update.Item{pending.Plugins[0], builder}}
	u.previews["a2"] = update.Preview{DumpTool: true, Plugins: []update.Item{builder}}
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	key := model.UpdateKey(model.ScanItem{Kind: model.ItemPlugin, Slug: "page-builder-kit"})
	for _, name := range []string{"A/a1", "A/a2"} {
		n, err := tree.Resolve(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.DB().SaveIgnore(ctx, model.Ignore{SiteID: n.ID, Key: key, Reason: "the theme uses its API", Created: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	previews := previewAll(t, ws)
	if len(previews[0].Ignored) != 1 || previews[0].Ignored[0].Slug != "page-builder-kit" || len(previews[2].Ignored) != 0 {
		t.Fatalf("ignored = %+v, %+v", previews[0].Ignored, previews[2].Ignored)
	}
	results := ws.UpdateSites(ctx, workspace.Everything(previews), workspace.Follow{})
	if got := outcomes(results); got["a1"] != workspace.SiteUpdated || got["a2"] != workspace.SiteNothing {
		t.Errorf("outcomes = %v", got)
	}
	if sel := u.chosen["a1"]; len(sel.Plugins) != 1 || sel.Plugins[0] != "akismet" {
		t.Errorf("a1 was given %+v, want akismet only", sel)
	}
}

// An update keeps the backups 30 days by default, as set; its job names the other sites of its
// host, whose backups the limits apply to too (ADR-0033).
func TestUpdateJobsCarryTheBackupLimits(t *testing.T) {
	ctx := context.Background()
	ws, _ := twoHosts(t)
	if days, err := ws.BackupDays(ctx); err != nil || days != 30 {
		t.Fatalf("BackupDays() = %d, %v; want 30 by default", days, err)
	}
	if err := ws.SetBackupDays(ctx, 12); !errors.Is(err, workspace.ErrBackupDays) {
		t.Errorf("SetBackupDays(12) = %v, want ErrBackupDays", err)
	}
	if err := ws.SetBackupDays(ctx, 7); err != nil {
		t.Fatal(err)
	}
	names := map[model.ID]string{}
	previews := previewAll(t, ws)
	for _, p := range previews {
		names[p.Site.ID] = p.Site.Name
	}
	for _, p := range previews {
		var siblings []string
		for _, id := range p.Job.Siblings {
			siblings = append(siblings, names[id])
		}
		want := map[string][]string{"a1": {"a2", "a3"}, "a2": {"a1", "a3"}, "a3": {"a1", "a2"}, "b1": nil}[p.Site.Name]
		if p.Job.KeepFor != 7*24*time.Hour || !slices.Equal(siblings, want) {
			t.Errorf("%s: keep %s, siblings %v; want 7 days and %v", p.Site.Name, p.Job.KeepFor, siblings, want)
		}
	}
	if err := ws.SetBackupDays(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if p := previewAll(t, ws)[0]; p.Job.KeepFor != 0 {
		t.Errorf("no limit: keep %s, want 0", p.Job.KeepFor)
	}
}

// A selection's sites are previewed once each, in tree order (ADR-0034).
func TestPreviewASelection(t *testing.T) {
	ctx := context.Background()
	ws, u := twoHosts(t)
	tree, err := ws.DB().LoadTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var a1, hostA model.Node
	for _, s := range tree.SitesUnder(model.Root) {
		if s.Name == "a1" {
			a1 = model.Node{Kind: model.KindSite, ID: s.ID}
		}
	}
	hostA = model.Node{Kind: model.KindHost, ID: u.hostOf["a1"]}
	previews, err := ws.PreviewNodes(ctx, []model.Node{a1, hostA})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range previews {
		names = append(names, p.Site.Name)
	}
	if !slices.Equal(names, []string{"a1", "a2", "a3"}) {
		t.Errorf("previewed %v, want host A's sites once each", names)
	}
}

var acf = update.Item{Kind: model.ItemPlugin, Slug: "acf-pro", From: "6.8.10", To: "6.9.0"}

// offers makes the scans of a site read its plugins with acf-pro at 6.8.10, offered version to.
func (u *updates) offers(t *testing.T, site, to string) {
	t.Helper()
	line, err := wpcli.PluginList().Remote("wp", "~/sites/"+site)
	if err != nil {
		t.Fatal(err)
	}
	list := `[{"name":"akismet","title":"Akismet Anti-spam","status":"inactive","version":"5.7.2","update":"none","update_version":"","auto_update":"off"},` +
		`{"name":"acf-pro","title":"Advanced Custom Fields PRO","status":"active","version":"6.8.10","update":"available","update_version":"` + to + `","auto_update":"off"}]`
	u.fake.Set(line, sshx.Result{Stdout: []byte(list)})
}

func siteNamed(t *testing.T, ws *workspace.Workspace, path string) model.ID {
	t.Helper()
	tree, err := ws.DB().LoadTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n, err := tree.Resolve(path)
	if err != nil {
		t.Fatal(err)
	}
	return n.ID
}

// Updates roll back on their own unless the user turned it off (ADR-0042, point 4).
func TestUpdatesFollowTheRollbackSetting(t *testing.T) {
	ctx := context.Background()
	ws, u := twoHosts(t)
	if on, err := ws.AutoRollback(ctx); err != nil || !on {
		t.Fatalf("AutoRollback() = %v, %v; want on by default", on, err)
	}
	ws.UpdateSites(ctx, workspace.Everything(previewAll(t, ws)), workspace.Follow{})
	if !u.auto["a1"] || !u.auto["b1"] {
		t.Errorf("runs given AutoRollback %v, want on", u.auto)
	}
	if err := ws.SetAutoRollback(ctx, false); err != nil {
		t.Fatal(err)
	}
	if on, err := ws.AutoRollback(ctx); err != nil || on {
		t.Fatalf("AutoRollback() = %v, %v; want off", on, err)
	}
	ws.UpdateSites(ctx, workspace.Everything(previewAll(t, ws)), workspace.Follow{})
	if u.auto["a1"] || u.auto["b1"] {
		t.Errorf("runs given AutoRollback %v, want off", u.auto)
	}
}

// A version rolled back on its own is set aside: the next Update all leaves it out, the update of
// the site with it chosen on purpose takes it, and ends the set-aside when it passes (point 5).
func TestRolledBackVersionSetAside(t *testing.T) {
	ctx := context.Background()
	ws, u := twoHosts(t)
	u.chosen = map[string]update.Selection{}
	u.previews["a1"] = update.Preview{DumpTool: true, Plugins: []update.Item{acf, pending.Plugins[0]}}
	u.breaks["a1"] = "acf-pro"
	u.offers(t, "a1", "6.9.0") // the scan after the update offers again the version rolled back
	a1 := siteNamed(t, ws, "A/a1")

	results := ws.UpdateSites(ctx, workspace.Everything(previewAll(t, ws)), workspace.Follow{})
	r := results[0]
	if r.Site.Name != "a1" || r.Outcome != workspace.SiteUpdated || len(r.Rollbacks) != 1 || !r.Rollbacks[0].Automatic {
		t.Fatalf("a1 = %s, %d rollbacks", r.Outcome, len(r.Rollbacks))
	}
	asides, err := ws.DB().SetAsides(ctx, a1)
	if err != nil {
		t.Fatal(err)
	}
	if len(asides) != 1 || asides[0].Slug != "acf-pro" || asides[0].Version != "6.9.0" || asides[0].OperationID != r.Operation.ID ||
		!strings.Contains(asides[0].Reason, "status 500") {
		t.Fatalf("set-asides = %+v", asides)
	}

	previews := previewAll(t, ws)
	if !previews[0].IsSetAside(acf) || len(previews[0].SetAside) != 1 {
		t.Fatalf("set aside in the preview = %+v", previews[0].SetAside)
	}
	u.breaks = map[string]string{}
	ws.UpdateSites(ctx, workspace.Everything(previews), workspace.Follow{})
	if sel := u.chosen["a1"]; slices.Contains(sel.Plugins, "acf-pro") {
		t.Errorf("Update all gave a1 %+v: the version set aside is left out", sel)
	}
	if asides, _ := ws.DB().SetAsides(ctx, a1); len(asides) != 1 {
		t.Errorf("set-asides after Update all = %+v, want kept", asides)
	}

	// The user ticks it in the site's Updates tab: it is updated, passes, and is set aside no more.
	alone := []workspace.Planned{{SitePreview: previews[0], Selection: update.Selection{Plugins: []string{"acf-pro"}}}}
	ws.UpdateSites(ctx, alone, workspace.Follow{})
	if sel := u.chosen["a1"]; !slices.Equal(sel.Plugins, []string{"acf-pro"}) {
		t.Errorf("a1 was given %+v, want acf-pro", sel)
	}
	if asides, _ := ws.DB().SetAsides(ctx, a1); len(asides) != 0 {
		t.Errorf("set-asides after its update passed = %+v, want none", asides)
	}
}

// A scan that offers a newer version ends the set-aside: the next Update all tries that one.
func TestNewerVersionEndsTheSetAside(t *testing.T) {
	ctx := context.Background()
	ws, u := twoHosts(t)
	a1 := siteNamed(t, ws, "A/a1")
	if err := ws.DB().SaveSetAside(ctx, model.SetAside{SiteID: a1, Kind: model.ItemPlugin, Slug: "acf-pro", Version: "6.9.0",
		OperationID: model.NewID(), Created: time.Now()}); err != nil {
		t.Fatal(err)
	}
	u.offers(t, "a1", "6.9.0")
	if _, err := ws.ScanNode(ctx, model.Node{Kind: model.KindSite, ID: a1}, workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if asides, _ := ws.DB().SetAsides(ctx, a1); len(asides) != 1 {
		t.Fatalf("set-asides after a scan offering 6.9.0 = %+v, want kept", asides)
	}
	u.offers(t, "a1", "6.9.1")
	if _, err := ws.ScanNode(ctx, model.Node{Kind: model.KindSite, ID: a1}, workspace.ScanOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if asides, _ := ws.DB().SetAsides(ctx, a1); len(asides) != 0 {
		t.Errorf("set-asides after a scan offering 6.9.1 = %+v, want none", asides)
	}
}
