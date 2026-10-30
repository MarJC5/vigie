package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/store"
	"github.com/MarJC5/vigie/internal/update"
	"github.com/MarJC5/vigie/internal/workspace"
	"github.com/MarJC5/vigie/internal/wpcli"
)

// UpdatesEvent carries an UpdatesState to the interface whenever an update starts, takes a step
// or ends.
const UpdatesEvent = "vigie:updates"

// errPreviewFirst is returned for an update of a site whose preview the window has not read.
var errPreviewFirst = errors.New("preview the update first")

// UpdateItemView is one item an update changes.
type UpdateItemView struct {
	Kind     string `json:"kind"` // core, plugin, theme, language
	Slug     string `json:"slug"`
	Language string `json:"language"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// SitePreviewView is what an update of a site would change.
type SitePreviewView struct {
	SiteID    string           `json:"siteId"`
	Name      string           `json:"name"`
	Path      string           `json:"path"`
	Core      *UpdateItemView  `json:"core"`
	Plugins   []UpdateItemView `json:"plugins"`
	Themes    []UpdateItemView `json:"themes"`
	Languages []UpdateItemView `json:"languages"`
	DumpTool  bool             `json:"dumpTool"`
	Problem   *Problem         `json:"problem"` // the preview could not be read
	Backups   string           `json:"backups"` // the folder of the site's backups on the server
}

// SelectionView is what the user chose to update on a site.
type SelectionView struct {
	Core      bool     `json:"core"`
	Plugins   []string `json:"plugins"`
	Themes    []string `json:"themes"`
	Languages bool     `json:"languages"`
}

// StepView is one step of an operation.
type StepView struct {
	Position      int    `json:"position"`
	Step          string `json:"step"`
	ItemKind      string `json:"itemKind"`
	Slug          string `json:"slug"`
	VersionBefore string `json:"versionBefore"`
	VersionAfter  string `json:"versionAfter"`
	Started       string `json:"started"`  // RFC 3339
	Finished      string `json:"finished"` // empty while it runs
	Outcome       string `json:"outcome"`
	Message       string `json:"message"`
}

// OperationView is an update or a rollback of a site, with its steps.
type OperationView struct {
	ID        string     `json:"id"`
	SiteID    string     `json:"siteId"`
	Kind      string     `json:"kind"` // update, rollback
	Started   string     `json:"started"`
	Finished  string     `json:"finished"`
	Outcome   string     `json:"outcome"`
	Summary   string     `json:"summary"`
	BackupDir string     `json:"backupDir"`
	Steps     []StepView `json:"steps"`
	// Rollback is what a rollback would undo, on the latest operation only; nil otherwise.
	Rollback *UpdateItemView `json:"rollback"`
}

// SiteResultView is how the update of one site of a host or folder ended.
type SiteResultView struct {
	SiteID      string `json:"siteId"`
	Path        string `json:"path"`
	Outcome     string `json:"outcome"` // updated, failed, refused, nothing, not-started, cancelled
	Detail      string `json:"detail"`  // the step that failed, or why it was refused
	OperationID string `json:"operationId"`
}

// UpdatesState is what runs now and how the last update of several sites ended; Completed counts
// the operations ended since Vigie started, so the window reloads what changed when it grows.
type UpdatesState struct {
	Running []OperationView `json:"running"`
	// Active counts the runs not over yet: between two sites of a host, nothing runs but the run
	// goes on.
	Active    int              `json:"active"`
	Bulk      []SiteResultView `json:"bulk"`
	Completed int              `json:"completed"`
	// Sites lists the sites of the runs not over yet, waiting or updating: none can be scanned or
	// updated again until it leaves the list.
	Sites []string `json:"sites"`
}

// Updates serves the updates of sites to the window: previews, runs in the background, rollbacks
// and their history.
type Updates struct {
	ws   *workspace.Workspace
	emit func(name string, data any)

	mu        sync.Mutex
	previews  map[model.ID]workspace.SitePreview // the last preview read of each site
	running   map[model.ID]*OperationView        // by operation
	order     []model.ID
	bulk      []SiteResultView
	completed int
	cancels   map[int]context.CancelFunc
	next      int
	busy      map[model.ID]bool // the sites of the runs not over yet
	scanner   *Scanner          // a site being scanned is not updated
}

// errScanning refuses to update a site while it is being scanned: the scan would read it halfway.
var errScanning = errors.New("the site is being scanned")

// NewUpdates returns the service; emit sends an event to the interface. A site the scanner is
// scanning is not updated nor rolled back; without a scanner, nothing is refused for that.
func NewUpdates(ws *workspace.Workspace, emit func(name string, data any), scanner *Scanner) *Updates {
	return &Updates{ws: ws, emit: emit, previews: map[model.ID]workspace.SitePreview{},
		running: map[model.ID]*OperationView{}, cancels: map[int]context.CancelFunc{}, busy: map[model.ID]bool{}, scanner: scanner}
}

// scanning tells whether one of the sites is being scanned.
func (u *Updates) scanning(sites ...model.ID) bool {
	if u.scanner == nil {
		return false
	}
	for _, id := range sites {
		if u.scanner.scanning(id) {
			return true
		}
	}
	return false
}

// Preview reads what an update of a site, or of the sites of a host or folder, would change.
func (u *Updates) Preview(ctx context.Context, nodeID string) ([]SitePreviewView, error) {
	tree, err := u.ws.DB().LoadTree(ctx)
	if err != nil {
		return nil, err
	}
	node, err := nodeByID(tree, nodeID)
	if err != nil {
		return nil, err
	}
	previews, err := u.ws.PreviewNode(ctx, node)
	if err != nil {
		return nil, err
	}
	views := make([]SitePreviewView, len(previews))
	u.mu.Lock()
	defer u.mu.Unlock()
	for i, p := range previews {
		u.previews[p.Site.ID] = p
		views[i] = previewView(p)
	}
	return views, nil
}

func previewView(p workspace.SitePreview) SitePreviewView {
	v := SitePreviewView{
		SiteID: p.Site.ID.String(), Name: p.Site.Name, Path: p.Path, DumpTool: p.Preview.DumpTool,
		Plugins: items(p.Preview.Plugins), Themes: items(p.Preview.Themes), Languages: items(p.Preview.Languages),
		Problem: ProblemOf(p.Err), Backups: "~/.vigie/backups/" + p.Site.ID.String() + "/",
	}
	if p.Preview.Core != nil {
		core := itemView(*p.Preview.Core)
		v.Core = &core
	}
	return v
}

func items(list []update.Item) []UpdateItemView {
	views := make([]UpdateItemView, len(list))
	for i, it := range list {
		views[i] = itemView(it)
	}
	return views
}

func itemView(it update.Item) UpdateItemView {
	return UpdateItemView{Kind: string(it.Kind), Slug: it.Slug, Language: it.Language, From: it.From, To: it.To}
}

// Update starts updating a site with the items chosen among its last preview, and returns at once.
func (u *Updates) Update(siteID string, sel SelectionView) *Problem {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return ProblemOf(errNodeNotFound)
	}
	u.mu.Lock()
	p, ok := u.previews[id]
	u.mu.Unlock()
	switch {
	case u.scanning(id):
		return ProblemOf(errScanning)
	case !ok:
		return ProblemOf(errPreviewFirst)
	case p.Err != nil:
		return ProblemOf(p.Err)
	case !p.Preview.DumpTool:
		return ProblemOf(wpcli.ErrNoDumpTool)
	}
	planned := []workspace.Planned{{SitePreview: p, Selection: update.Selection{
		Core: sel.Core, Plugins: sel.Plugins, Themes: sel.Themes, Languages: sel.Languages,
	}}}
	return u.start(planned, false)
}

// UpdateAll starts updating every site previewed under a host or folder with all its items, and
// returns at once; the summary shows in the state's Bulk when it ends.
func (u *Updates) UpdateAll(nodeID string) *Problem {
	tree, err := u.ws.DB().LoadTree(context.Background())
	if err != nil {
		return ProblemOf(err)
	}
	node, err := nodeByID(tree, nodeID)
	if err != nil {
		return ProblemOf(errNodeNotFound)
	}
	for _, site := range tree.SitesUnder(node) {
		if u.scanning(site.ID) {
			return ProblemOf(errScanning)
		}
	}
	var previews []workspace.SitePreview
	u.mu.Lock()
	for _, site := range tree.SitesUnder(node) {
		if p, ok := u.previews[site.ID]; ok {
			previews = append(previews, p)
		}
	}
	u.mu.Unlock()
	if len(previews) == 0 {
		return ProblemOf(errPreviewFirst)
	}
	return u.start(workspace.Everything(previews), true)
}

// start runs the planned updates in the background. A site already in a run refuses the update of
// one site; in a bulk run, it is refused when its turn comes.
func (u *Updates) start(planned []workspace.Planned, bulk bool) *Problem {
	ctx, cancel := context.WithCancel(context.Background())
	u.mu.Lock()
	if !bulk && u.busy[planned[0].Site.ID] {
		u.mu.Unlock()
		cancel()
		return ProblemOf(store.ErrBusy)
	}
	mine := map[model.ID]bool{}
	for _, p := range planned {
		if !u.busy[p.Site.ID] {
			u.busy[p.Site.ID], mine[p.Site.ID] = true, true
		}
	}
	key := u.next
	u.next++
	u.cancels[key] = cancel
	u.mu.Unlock()
	u.send()
	// A site leaves the busy ones as soon as it ends, the others when the run does.
	release := func(id model.ID) {
		if mine[id] {
			delete(u.busy, id)
			delete(mine, id)
		}
	}
	go func() {
		defer cancel()
		results := u.ws.UpdateSites(ctx, planned, workspace.Follow{OnStep: u.step, OnSite: func(r workspace.SiteUpdate) {
			u.mu.Lock()
			release(r.Site.ID)
			u.mu.Unlock()
			u.ended(r)
		}})
		u.mu.Lock()
		for id := range mine {
			release(id)
		}
		delete(u.cancels, key)
		if bulk {
			u.bulk = make([]SiteResultView, len(results))
			for i, r := range results {
				u.bulk[i] = resultView(r)
			}
		}
		u.mu.Unlock()
		u.send()
	}()
	return nil
}

// Rollback starts undoing the item that failed in an operation of a site, and returns at once.
func (u *Updates) Rollback(siteID, operationID string) *Problem {
	site, err := uuid.Parse(siteID)
	if err != nil {
		return ProblemOf(errNodeNotFound)
	}
	op, err := uuid.Parse(operationID)
	if err != nil {
		return ProblemOf(update.ErrNothingToRollBack)
	}
	if u.scanning(site) {
		return ProblemOf(errScanning)
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.mu.Lock()
	if u.busy[site] {
		u.mu.Unlock()
		cancel()
		return ProblemOf(store.ErrBusy)
	}
	u.busy[site] = true
	key := u.next
	u.next++
	u.cancels[key] = cancel
	u.mu.Unlock()
	u.send()
	go func() {
		defer cancel()
		done, err := u.ws.Rollback(ctx, site, op, u.step)
		u.mu.Lock()
		delete(u.cancels, key)
		delete(u.busy, site)
		u.mu.Unlock()
		if err == nil || done.ID != model.NoID {
			u.finish(done.ID)
		}
		u.send()
	}()
	return nil
}

// CancelAll stops every update after its current item; a rollback is never left halfway.
func (u *Updates) CancelAll() {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, cancel := range u.cancels {
		cancel()
	}
}

// State returns what runs now.
func (u *Updates) State() UpdatesState {
	u.mu.Lock()
	defer u.mu.Unlock()
	state := UpdatesState{Running: []OperationView{}, Active: len(u.cancels), Bulk: append([]SiteResultView{}, u.bulk...),
		Completed: u.completed, Sites: []string{}}
	for id := range u.busy {
		state.Sites = append(state.Sites, id.String())
	}
	slices.Sort(state.Sites)
	for _, id := range u.order {
		op := *u.running[id]
		op.Steps = append([]StepView{}, op.Steps...)
		state.Running = append(state.Running, op)
	}
	return state
}

// History lists the updates and rollbacks of a site, the newest first; the newest says what a
// rollback would undo, when it can.
func (u *Updates) History(ctx context.Context, siteID string) ([]OperationView, error) {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return nil, errNodeNotFound
	}
	ops, err := u.ws.DB().Operations(ctx, id)
	if err != nil {
		return nil, err
	}
	views := make([]OperationView, len(ops))
	for i, op := range ops {
		views[i] = operationView(op)
	}
	if len(ops) > 0 {
		if item, ok := update.RollbackOf(ops[0]); ok {
			v := itemView(item)
			views[0].Rollback = &v
		}
	}
	return views, nil
}

// step follows an operation in progress.
func (u *Updates) step(op model.Operation, st model.OperationStep) {
	u.mu.Lock()
	view, ok := u.running[op.ID]
	if !ok {
		v := operationView(op)
		v.Steps = nil
		view = &v
		u.running[op.ID] = view
		u.order = append(u.order, op.ID)
	}
	s := stepView(st)
	if st.Position < len(view.Steps) {
		view.Steps[st.Position] = s
	} else {
		view.Steps = append(view.Steps, s)
	}
	u.mu.Unlock()
	u.send()
}

// ended takes a site's operation out of the running ones.
func (u *Updates) ended(r workspace.SiteUpdate) {
	if r.Operation.ID != model.NoID {
		u.finish(r.Operation.ID)
	}
	u.send()
}

func (u *Updates) finish(id model.ID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.running, id)
	if i := slices.Index(u.order, id); i >= 0 {
		u.order = slices.Delete(u.order, i, i+1)
	}
	u.completed++
}

func (u *Updates) send() {
	u.emit(UpdatesEvent, u.State())
}

func operationView(op model.Operation) OperationView {
	v := OperationView{
		ID: op.ID.String(), SiteID: op.SiteID.String(), Kind: string(op.Kind), Started: timeText(op.Started),
		Finished: timeText(op.Finished), Outcome: string(op.Outcome), Summary: op.Summary, BackupDir: op.BackupDir,
		Steps: make([]StepView, len(op.Steps)),
	}
	for i, st := range op.Steps {
		v.Steps[i] = stepView(st)
	}
	return v
}

func stepView(st model.OperationStep) StepView {
	return StepView{
		Position: st.Position, Step: string(st.Step), ItemKind: string(st.ItemKind), Slug: st.Slug,
		VersionBefore: st.VersionBefore, VersionAfter: st.VersionAfter, Started: timeText(st.Started),
		Finished: timeText(st.Finished), Outcome: string(st.Outcome), Message: st.Message,
	}
}

func resultView(r workspace.SiteUpdate) SiteResultView {
	v := SiteResultView{SiteID: r.Site.ID.String(), Path: r.Path, Outcome: string(r.Outcome)}
	if r.Err != nil {
		if p := ProblemOf(r.Err); p != nil {
			v.Detail = p.Code
		}
	}
	for i := len(r.Operation.Steps) - 1; i >= 0; i-- {
		if r.Operation.Steps[i].Outcome == model.StepFailed {
			v.Detail = r.Operation.Steps[i].Message
			break
		}
	}
	if r.Operation.ID != model.NoID {
		v.OperationID = r.Operation.ID.String()
	}
	return v
}

// timeText is a time for the window, empty when zero.
func timeText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
