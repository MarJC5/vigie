package workspace

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/MarJC5/vigie/internal/httpcheck"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/sshx"
	"github.com/MarJC5/vigie/internal/update"
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
}

// PreviewNode reads the previews of a site, the sites of a host, or every site under a folder
// (model.Root for all), in tree order; several sites at once, each host within its limit.
func (w *Workspace) PreviewNode(ctx context.Context, node model.Node) ([]SitePreview, error) {
	tree, err := w.db.LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	sites := tree.SitesUnder(node)
	previews := make([]SitePreview, len(sites))
	u := w.updater(nil)
	slots := make(chan struct{}, parallelSites)
	var wg sync.WaitGroup
	for i, site := range sites {
		host, _ := tree.Host(site.HostID)
		previews[i] = SitePreview{
			Site: site, Path: tree.PathOf(model.Node{Kind: model.KindSite, ID: site.ID}),
			Job: update.Job{Site: site, Target: sshx.TargetFor(host), HostKey: host.ID.String(), Max: maxConcurrent(host),
				WPCLI: tree.EffectiveWPCLI(site.ID)},
		}
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			previews[i].Preview, previews[i].Err = u.Preview(ctx, previews[i].Job)
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

// Everything plans every item of every preview, as "Update all" does.
func Everything(previews []SitePreview) []Planned {
	planned := make([]Planned, len(previews))
	for i, p := range previews {
		planned[i] = Planned{SitePreview: p, Selection: update.All(p.Preview)}
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
	Err       error           // why it was refused
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
	u := w.updater(follow.OnStep)
	var mu sync.Mutex // OnSite hears of the hosts one at a time
	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Go(func() {
			for _, i := range byHost[host] {
				if ctx.Err() != nil {
					return
				}
				results[i] = w.updateSite(ctx, u, planned[i])
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

func empty(sel update.Selection) bool {
	return !sel.Core && !sel.Languages && len(sel.Plugins) == 0 && len(sel.Themes) == 0
}

// rescan scans a site after its update and stores the scan; a failure only shows in the history.
func (w *Workspace) rescan(ctx context.Context, job update.Job) {
	scanner := scan.Scanner{Runner: w.deps.Runner, Limiter: w.limiter, Now: w.deps.Now}
	s, subsites := scanner.Site(ctx, scan.Job{Site: job.Site, Target: job.Target, HostKey: job.HostKey, Max: job.Max, WPCLI: job.WPCLI}, nil)
	if err := w.db.SaveScan(ctx, s); err != nil {
		return
	}
	if job.Site.Multisite && s.Outcome == model.ScanCompleted {
		if err := w.db.SaveSubsites(ctx, job.Site.ID, subsites); err != nil {
			return
		}
	}
}
