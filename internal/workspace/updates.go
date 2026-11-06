package workspace

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MarJC5/vigie/internal/httpcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// Updates previews, runs and rolls back the update of one site (update.Updater).
type Updates interface {
	Preview(ctx context.Context, job update.Job) (update.Preview, error)
	Run(ctx context.Context, job update.Job, preview update.Preview, sel update.Selection) (model.Operation, error)
	Rollback(ctx context.Context, job update.Job, failed model.Operation) (model.Operation, error)
}

// Follow is what a caller hears of updates as they go: each site as it ends, each step as it
// starts and ends.
type Follow struct {
	OnSite func(SiteUpdate)
	OnStep func(op model.Operation, step model.OperationStep)
}

// checks is how Vigie watches a site after a change: OPcache checks the files every 2 s by
// default, so the first round comes after 3 s, then a round each second for 3 s.
var checks = httpcheck.Checker{Settle: 3 * time.Second, Window: 3 * time.Second, Interval: time.Second}

// updater is the deps' Updates, else the real one over the workspace's runner, limiter and
// database; onStep, when set, hears of its steps.
func (w *Workspace) updater(onStep func(model.Operation, model.OperationStep)) Updates {
	if w.deps.Updates != nil {
		return w.deps.Updates(onStep)
	}
	checker := w.deps.Checker
	if checker == nil {
		checker = checks
	}
	return update.Updater{Runner: w.deps.Runner, Limiter: w.limiter, Store: w.db, Checker: checker, Now: w.deps.Now,
		PID: os.Getpid(), OnStep: onStep}
}

// SitePreview is what an update of one site would change; Err when it could not be read.
type SitePreview struct {
	Site    model.Site
	Path    string // in the tree
	Job     update.Job
	Preview update.Preview
	Err     error
	// Ignored are the updates offered that the site's ignores cover (ADR-0026): a bulk run leaves
	// them out; the site's own update takes one chosen on purpose.
	Ignored []update.Item
	// SetAside are the updates offered whose version rolled back on its own before (ADR-0042): a
	// bulk run leaves them out too; the site's own update takes one chosen on purpose.
	SetAside []update.Item
}

// Bulk selects what a bulk run updates on the site: everything offered, the ignored updates and
// the versions set aside left out.
func (p SitePreview) Bulk() update.Selection {
	sel := update.All(p.Preview)
	for _, it := range slices.Concat(p.Ignored, p.SetAside) {
		switch it.Kind {
		case model.ItemCore:
			sel.Core = false
		case model.ItemPlugin:
			sel.Plugins = slices.DeleteFunc(sel.Plugins, func(slug string) bool { return slug == it.Slug })
		case model.ItemTheme:
			sel.Themes = slices.DeleteFunc(sel.Themes, func(slug string) bool { return slug == it.Slug })
		}
	}
	return sel
}

// IsIgnored tells whether an update of the preview is one the site's ignores cover.
func (p SitePreview) IsIgnored(it update.Item) bool {
	return slices.Contains(p.Ignored, it)
}

// IsSetAside tells whether an update of the preview is a version set aside on the site.
func (p SitePreview) IsSetAside(it update.Item) bool {
	return slices.Contains(p.SetAside, it)
}

// offered are the updates of a preview: WordPress, the plugins, the themes.
func offered(p update.Preview) []update.Item {
	var items []update.Item
	if p.Core != nil {
		items = append(items, *p.Core)
	}
	return append(append(items, p.Plugins...), p.Themes...)
}

// setAsideIn are the updates of a preview to the very version set aside on the site.
func setAsideIn(p update.Preview, asides []model.SetAside) []update.Item {
	var out []update.Item
	for _, it := range offered(p) {
		if slices.ContainsFunc(asides, func(s model.SetAside) bool { return s.Kind == it.Kind && s.Slug == it.Slug && s.Version == it.To }) {
			out = append(out, it)
		}
	}
	return out
}

// ignoredIn are the updates of a preview that the ignores cover.
func ignoredIn(p update.Preview, ignores []model.Ignore) []update.Item {
	var ignored []update.Item
	for _, it := range offered(p) {
		item := model.ScanItem{Kind: it.Kind, Slug: it.Slug, Update: model.UpdateAvailable, UpdateVersion: it.To}
		if slices.ContainsFunc(ignores, func(i model.Ignore) bool { return i.CoversUpdate(item) }) {
			ignored = append(ignored, it)
		}
	}
	return ignored
}

// PreviewNode reads the previews of a site, the sites of a host, or every site under a folder
// (model.Root for all), in tree order; several sites at once, each host within its limit.
func (w *Workspace) PreviewNode(ctx context.Context, node model.Node) ([]SitePreview, error) {
	return w.PreviewNodes(ctx, []model.Node{node})
}

// PreviewNodes reads the previews of the sites the nodes hold, each once, in tree order (ADR-0034).
func (w *Workspace) PreviewNodes(ctx context.Context, nodes []model.Node) ([]SitePreview, error) {
	return w.PreviewNodesFollowed(ctx, nodes, nil)
}

// PreviewNodesFollowed is PreviewNodes telling, when progress is set, how many sites are read out
// of how many, as each one is (the command line's progress, ADR-0037).
func (w *Workspace) PreviewNodesFollowed(ctx context.Context, nodes []model.Node, progress func(done, total int)) ([]SitePreview, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	sites := SitesOf(tree, nodes)
	keep, siblings, err := w.backupLimits(ctx, tree)
	if err != nil {
		return nil, err
	}
	previews := make([]SitePreview, len(sites))
	var read atomic.Int64
	u := w.updater(nil)
	slots := make(chan struct{}, parallelSites)
	var wg sync.WaitGroup
	for i, site := range sites {
		host, _ := tree.Host(site.HostID)
		previews[i] = SitePreview{
			Site: site, Path: tree.PathOf(model.Node{Kind: model.KindSite, ID: site.ID}),
			Job: update.Job{Site: site, Target: sshx.TargetFor(host), HostKey: host.ID.String(), Max: maxConcurrent(host),
				WPCLI: tree.EffectiveWPCLI(site.ID), KeepFor: keep, Siblings: siblings(site)},
		}
		ignores, err := w.db.Ignores(ctx, site.ID)
		if err != nil {
			return nil, err
		}
		asides, err := w.db.SetAsides(ctx, site.ID)
		if err != nil {
			return nil, err
		}
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			previews[i].Preview, previews[i].Err = u.Preview(ctx, previews[i].Job)
			previews[i].Ignored = ignoredIn(previews[i].Preview, ignores)
			previews[i].SetAside = setAsideIn(previews[i].Preview, asides)
			if progress != nil {
				progress(int(read.Add(1)), len(sites))
			}
		})
	}
	wg.Wait()
	return previews, nil
}

// Planned is a site to update with what to update on it.
type Planned struct {
	SitePreview
	Selection update.Selection
}

// Everything plans every item of every preview, as "Update all" does, the ignored updates aside.
func Everything(previews []SitePreview) []Planned {
	planned := make([]Planned, len(previews))
	for i, p := range previews {
		planned[i] = Planned{SitePreview: p, Selection: p.Bulk()}
	}
	return planned
}

// SiteOutcome is how the update of one site of a bulk run ended.
type SiteOutcome string

const (
	SiteUpdated    SiteOutcome = "updated"
	SiteFailed     SiteOutcome = "failed"      // an item failed; the host stopped there
	SiteRefused    SiteOutcome = "refused"     // nothing was written: no dump tool, busy, preview unreadable
	SiteNothing    SiteOutcome = "nothing"     // nothing to update
	SiteNotStarted SiteOutcome = "not-started" // after a failure on its host, or a cancel
	SiteCancelled  SiteOutcome = "cancelled"
)

// SiteUpdate is the result for one site of a bulk run.
type SiteUpdate struct {
	Site      model.Site
	Path      string
	Outcome   SiteOutcome
	Operation model.Operation // zero when nothing ran
	// Rollbacks are those Vigie ran on its own during the update (ADR-0042), passed or not.
	Rollbacks []model.Operation
	Err       error // why it was refused
}

// PlanBulk plans an update of everything offered on the sites previewed, as Update all and vigie
// update do: the sites that can be updated, their ignored updates and the versions set aside left
// out, and for the others why not (refused, or nothing to update), in the previews' order.
func PlanBulk(previews []SitePreview) ([]Planned, []SiteUpdate) {
	var planned []Planned
	var not []SiteUpdate
	for _, p := range previews {
		switch {
		case p.Err != nil:
			not = append(not, SiteUpdate{Site: p.Site, Path: p.Path, Outcome: SiteRefused, Err: p.Err})
		// Nothing offered, or only updates the site ignores or set aside.
		case p.Preview.Pending() == len(p.Ignored)+len(p.SetAside):
			not = append(not, SiteUpdate{Site: p.Site, Path: p.Path, Outcome: SiteNothing})
		case !p.Preview.DumpTool:
			not = append(not, SiteUpdate{Site: p.Site, Path: p.Path, Outcome: SiteRefused, Err: wpcli.ErrNoDumpTool})
		default:
			planned = append(planned, Planned{SitePreview: p, Selection: p.Bulk()})
		}
	}
	return planned, not
}

// UpdateSites updates the sites planned: hosts at once, the sites of a host one after another,
// each host stopping at its first failure; a cancel lets the current site end. Every site updated
// is scanned again.
func (w *Workspace) UpdateSites(ctx context.Context, planned []Planned, follow Follow) []SiteUpdate {
	results := make([]SiteUpdate, len(planned))
	byHost := map[string][]int{}
	var hosts []string
	for i, p := range planned {
		results[i] = SiteUpdate{Site: p.Site, Path: p.Path, Outcome: SiteNotStarted}
		if _, ok := byHost[p.Job.HostKey]; !ok {
			hosts = append(hosts, p.Job.HostKey)
		}
		byHost[p.Job.HostKey] = append(byHost[p.Job.HostKey], i)
	}
	// The setting is read once per run; unreadable, plugins and themes roll back on their own.
	auto, err := w.AutoRollback(ctx)
	if err != nil {
		auto = true
	}
	u := w.updater(follow.OnStep)
	var mu sync.Mutex // OnSite hears of the hosts one at a time
	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Go(func() {
			for _, i := range byHost[host] {
				if ctx.Err() != nil {
					return
				}
				p := planned[i]
				p.Job.AutoRollback = auto
				results[i] = w.updateSite(ctx, u, p)
				if follow.OnSite != nil {
					mu.Lock()
					follow.OnSite(results[i])
					mu.Unlock()
				}
				if results[i].Outcome == SiteFailed {
					return
				}
			}
		})
	}
	wg.Wait()
	return results
}

// updateSite runs the update of one planned site, then scans it again.
func (w *Workspace) updateSite(ctx context.Context, u Updates, p Planned) SiteUpdate {
	r := SiteUpdate{Site: p.Site, Path: p.Path}
	switch {
	case p.Err != nil:
		r.Outcome, r.Err = SiteRefused, p.Err
		return r
	case empty(p.Selection):
		r.Outcome = SiteNothing
		return r
	}
	op, err := u.Run(ctx, p.Job, p.Preview, p.Selection)
	if err != nil {
		r.Outcome, r.Err = SiteRefused, err
		return r
	}
	r.Operation = op
	r.Rollbacks = w.setAside(context.WithoutCancel(ctx), op)
	switch op.Outcome {
	case model.OperationCompleted:
		r.Outcome = SiteUpdated
	case model.OperationCancelled:
		r.Outcome = SiteCancelled
	default:
		r.Outcome = SiteFailed
	}
	// The scan shows the new versions, or what the failure left; kept even after a cancel.
	w.rescan(context.WithoutCancel(ctx), p.Job)
	return r
}

// Rollback undoes the item that failed in an update of a site, from the operation stored, then
// scans the site again.
func (w *Workspace) Rollback(ctx context.Context, siteID, operationID model.ID, onStep func(model.Operation, model.OperationStep)) (model.Operation, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return model.Operation{}, err
	}
	site, ok := tree.Site(siteID)
	if !ok {
		return model.Operation{}, fmt.Errorf("site %s: %w", siteID, model.ErrNotFound)
	}
	ops, err := w.db.Operations(ctx, siteID)
	if err != nil {
		return model.Operation{}, err
	}
	i := slices.IndexFunc(ops, func(op model.Operation) bool { return op.ID == operationID })
	if i < 0 {
		return model.Operation{}, fmt.Errorf("operation %s: %w", operationID, model.ErrNotFound)
	}
	host, _ := tree.Host(site.HostID)
	job := update.Job{Site: site, Target: sshx.TargetFor(host), HostKey: host.ID.String(), Max: maxConcurrent(host),
		WPCLI: tree.EffectiveWPCLI(site.ID)}
	op, err := w.updater(onStep).Rollback(ctx, job, ops[i])
	if err != nil {
		return op, err
	}
	w.rescan(context.WithoutCancel(ctx), job)
	return op, nil
}

// setAside sets aside on the site the versions an update rolled back on its own, and ends the
// set-aside of an item it updated and checked (the user chose it on purpose); it returns the
// rollbacks. A failure leaves the set-asides as they were: Update all may try a version again.
func (w *Workspace) setAside(ctx context.Context, op model.Operation) []model.Operation {
	for _, st := range op.Steps {
		if st.Step == model.StepCheck && st.Outcome == model.StepDone && (st.ItemKind == model.ItemPlugin || st.ItemKind == model.ItemTheme) {
			_ = w.db.DeleteSetAside(ctx, op.SiteID, st.ItemKind, st.Slug)
		}
	}
	ops, err := w.db.Operations(ctx, op.SiteID)
	if err != nil {
		return nil
	}
	var backs []model.Operation
	for _, back := range slices.Backward(ops) {
		if back.Kind != model.OperationRollback || !back.Automatic || back.OfOperation != op.ID {
			continue
		}
		backs = append(backs, back)
		i := slices.IndexFunc(back.Steps, func(st model.OperationStep) bool { return st.Step == model.StepRestoreFiles })
		if back.Outcome != model.OperationCompleted || i < 0 {
			continue
		}
		restored := back.Steps[i]
		reason := ""
		for _, st := range op.Steps {
			if st.Step == model.StepCheck && st.Outcome == model.StepFailed && st.ItemKind == restored.ItemKind && st.Slug == restored.Slug {
				reason = st.Message
			}
		}
		_ = w.db.SaveSetAside(ctx, model.SetAside{SiteID: op.SiteID, Kind: restored.ItemKind, Slug: restored.Slug,
			Version: restored.VersionBefore, OperationID: op.ID, Reason: reason, Created: w.deps.Now()})
	}
	return backs
}

func empty(sel update.Selection) bool {
	return !sel.Core && !sel.Languages && len(sel.Plugins) == 0 && len(sel.Themes) == 0
}

// rescan scans a site after its update and stores the scan; a failure only shows in the history.
func (w *Workspace) rescan(ctx context.Context, job update.Job) {
	latest, err := w.db.LatestScans(ctx)
	if err != nil {
		return
	}
	s, subsites := w.scanner().Site(ctx, scan.Job{Site: job.Site, Target: job.Target, HostKey: job.HostKey, Max: job.Max,
		WPCLI: job.WPCLI, Previous: latest[job.Site.ID]}, nil)
	scanned := Scanned{Scan: s}
	previous, ok := latest[job.Site.ID]
	if err := w.prepare(ctx, &scanned, previous, ok); err != nil {
		return
	}
	s = scanned.Scan
	if err := w.db.SaveScan(ctx, s); err != nil {
		return
	}
	if job.Site.Multisite && s.Outcome == model.ScanCompleted {
		if err := w.db.SaveSubsites(ctx, job.Site.ID, subsites); err != nil {
			return
		}
	}
}

// autoRollbackSetting turns the rollback on its own off with "off" (ADR-0042, point 4).
const autoRollbackSetting = "updates.autorollback"

// AutoRollback tells whether plugins and themes that break a site roll back on their own; on by
// default.
func (w *Workspace) AutoRollback(ctx context.Context) (bool, error) {
	value, _, err := w.db.Setting(ctx, autoRollbackSetting)
	return value != "off", err
}

// SetAutoRollback turns the rollback on its own on or off.
func (w *Workspace) SetAutoRollback(ctx context.Context, on bool) error {
	value := "on"
	if !on {
		value = "off"
	}
	return w.db.SetSetting(ctx, autoRollbackSetting, value)
}

// endSetAsides ends the set-asides of a site that a completed scan no longer offers the version of:
// a newer one is offered, the item was updated elsewhere or removed.
func (w *Workspace) endSetAsides(ctx context.Context, s model.Scan) error {
	asides, err := w.db.SetAsides(ctx, s.SiteID)
	if err != nil {
		return err
	}
	for _, a := range asides {
		if !a.Holds(s.Items) {
			if err := w.db.DeleteSetAside(ctx, a.SiteID, a.Kind, a.Slug); err != nil {
				return err
			}
		}
	}
	return nil
}
