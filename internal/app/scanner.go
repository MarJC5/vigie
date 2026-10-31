package app

import (
	"context"
	"fmt"
	"sync"
	"uuid"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/scan"
	"github.com/MarJC5/vigie/internal/workspace"
)

// ScansEvent carries a ScansState to the interface whenever a scan starts, progresses or ends.
const ScansEvent = "vigie:scans"

// RunningScan is a scan of a node in progress.
type RunningScan struct {
	NodeID string `json:"nodeId"` // empty: the whole tree
	Sites  int    `json:"sites"`  // sites to scan
	Done   int    `json:"done"`   // sites finished
}

// ScansState lists the scans in progress; Completed counts the site scans stored since Vigie
// started, so the interface reloads what changed when it grows.
type ScansState struct {
	Running   []RunningScan `json:"running"`
	Completed int           `json:"completed"`
}

// Scanner runs the scans the window asks for, in the background, one at a time per node.
type Scanner struct {
	ws   *workspace.Workspace
	emit func(name string, data any)
	// OnScanned, when set, hears the end of each scan: the latest scans before it, and the ones it
	// made, before the window is told the scan ended.
	OnScanned func(tree *model.Tree, before map[model.ID]model.Scan, after []model.Scan)

	mu        sync.Mutex
	order     []string // node IDs, in the order they started
	running   map[string]*running
	completed int
}

type running struct {
	cancel context.CancelFunc
	sites  int
	ids    map[model.ID]bool // the sites of the scan
	done   map[model.ID]bool
}

// scanning tells whether a scan running now has a site not done yet.
func (s *Scanner) scanning(site model.ID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.running {
		if r.ids[site] && !r.done[site] {
			return true
		}
	}
	return false
}

// NewScanner returns the service; emit sends an event to the interface.
func NewScanner(ws *workspace.Workspace, emit func(name string, data any)) *Scanner {
	return &Scanner{ws: ws, emit: emit, running: map[string]*running{}}
}

// Scan starts scanning a folder, host or site ("" for everything) and returns at once; starting
// a node that is already being scanned does nothing.
func (s *Scanner) Scan(nodeID string) *Problem {
	return s.start(nodeID, workspace.ScanOptions{})
}

// ScanScheduled is Scan of everything for the automatic scans: its run says the schedule started it.
func (s *Scanner) ScanScheduled() *Problem {
	return s.start("", workspace.ScanOptions{By: model.RunBySchedule})
}

// CheckFiles is Scan, comparing the files with wordpress.org's even when it was done less than a
// day ago.
func (s *Scanner) CheckFiles(nodeID string) *Problem {
	return s.start(nodeID, workspace.ScanOptions{CheckFiles: true})
}

func (s *Scanner) start(nodeID string, opts workspace.ScanOptions) *Problem {
	tree, err := s.ws.DB().LoadTree(context.Background())
	if err != nil {
		return ProblemOf(err)
	}
	node, err := nodeByID(tree, nodeID)
	if err != nil {
		return ProblemOf(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if _, busy := s.running[nodeID]; busy {
		s.mu.Unlock()
		cancel()
		return nil
	}
	sites := tree.SitesUnder(node)
	ids := map[model.ID]bool{}
	for _, site := range sites {
		ids[site.ID] = true
	}
	s.running[nodeID] = &running{cancel: cancel, sites: len(sites), ids: ids, done: map[model.ID]bool{}}
	s.order = append(s.order, nodeID)
	s.mu.Unlock()
	s.send()

	before, err := s.ws.DB().LatestScans(ctx)
	if err != nil {
		before = nil // unread: this scan alerts on nothing
	}
	go func() {
		defer cancel()
		results, _ := s.ws.ScanNode(ctx, node, opts, func(p scan.Progress) { s.progress(nodeID, p) })
		if s.OnScanned != nil && before != nil {
			after := make([]model.Scan, len(results))
			for i, r := range results {
				after[i] = r.Scan
			}
			s.OnScanned(tree, before, after)
		}
		s.mu.Lock()
		delete(s.running, nodeID)
		for i, id := range s.order {
			if id == nodeID {
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
		}
		s.completed += len(results)
		s.mu.Unlock()
		s.send()
	}()
	return nil
}

// Cancel stops the scan of a node; what was scanned so far is kept.
func (s *Scanner) Cancel(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.running[nodeID]; ok {
		r.cancel()
	}
}

// CancelAll stops every scan.
func (s *Scanner) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.running {
		r.cancel()
	}
}

// State returns the scans in progress.
func (s *Scanner) State() ScansState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := ScansState{Running: []RunningScan{}, Completed: s.completed}
	for _, id := range s.order {
		r := s.running[id]
		state.Running = append(state.Running, RunningScan{NodeID: id, Sites: r.sites, Done: len(r.done)})
	}
	return state
}

// progress counts a site as done once its scan reports its last step.
func (s *Scanner) progress(nodeID string, p scan.Progress) {
	if p.Done < p.Total {
		return
	}
	s.mu.Lock()
	r, ok := s.running[nodeID]
	changed := ok && !r.done[p.SiteID]
	if changed {
		r.done[p.SiteID] = true
	}
	s.mu.Unlock()
	if changed {
		s.send()
	}
}

func (s *Scanner) send() {
	s.emit(ScansEvent, s.State())
}

// nodeByID finds a folder, host or site by its ID; "" is the whole tree.
func nodeByID(tree *model.Tree, id string) (model.Node, error) {
	if id == "" {
		return model.Root, nil
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return model.Node{}, fmt.Errorf("node %q: %w", id, err)
	}
	if _, ok := tree.Folder(parsed); ok {
		return model.Node{Kind: model.KindFolder, ID: parsed}, nil
	}
	if _, ok := tree.Host(parsed); ok {
		return model.Node{Kind: model.KindHost, ID: parsed}, nil
	}
	if _, ok := tree.Site(parsed); ok {
		return model.Node{Kind: model.KindSite, ID: parsed}, nil
	}
	return model.Node{}, fmt.Errorf("node %s: not found", id)
}
