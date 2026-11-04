package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"uuid"

	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sitecopy"
	"github.com/MarJC5/vigie/internal/workspace"
)

// CopiesEvent carries the CopiesState to the window while a copy runs and when it ends.
const CopiesEvent = "vigie:copies"

// EndCopy is the kind of a copy's end (ADR-0035, ADR-0041).
const EndCopy = "copy"

var (
	// errNoPreview refuses a copy whose sizes were not read first: the dialog shows them.
	errNoPreview = errors.New("the copy's sizes were not read")
	// errNotACopy refuses a folder that is not one of the site's copies.
	errNotACopy = errors.New("not a copy of this site")
	// errCopyRunning refuses a second copy of a site while one runs.
	errCopyRunning = errors.New("a copy of this site runs")
)

// CopyFolderView is a folder of wp-content and its size, in the copy dialog.
type CopyFolderView struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// CopyPreviewView is what the copy dialog shows (mockups 28a to 28c): the size of each content,
// the folders of wp-content, where the copy goes and how much room is there.
type CopyPreviewView struct {
	SiteID   string           `json:"siteId"`
	Database int64            `json:"database"`
	Content  int64            `json:"content"` // the database and wp-content
	Site     int64            `json:"site"`    // the database and the whole site folder
	Folders  []CopyFolderView `json:"folders"`
	// Split tells that the archives are split by size: the account has GNU find and tar.
	Split       bool   `json:"split"`
	Destination string `json:"destination"` // the site's folder of copies
	Free        int64  `json:"free"`        // bytes free there, -1 when unknown
	ArchiveSize int    `json:"archiveSize"` // MB
}

// CopyRunView is a copy that runs: the archive being read and how far it went.
type CopyRunView struct {
	SiteID   string `json:"siteId"`
	Folder   string `json:"folder"`
	Archive  string `json:"archive"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Read     int64  `json:"read"`
	Expected int64  `json:"expected"`
	Resumed  bool   `json:"resumed"`
}

// CopyEndView is how the last copy of a site ended: its folder, if one was made, and its problem,
// nil when it completed.
type CopyEndView struct {
	SiteID    string   `json:"siteId"`
	Folder    string   `json:"folder"`
	Cancelled bool     `json:"cancelled"`
	Problem   *Problem `json:"problem"`
}

// CopiesState is what copies run now, and how the last copy of each site ended; Completed counts
// the copies ended since Vigie started, so the window reloads a site's copies when it grows.
type CopiesState struct {
	Running   []CopyRunView `json:"running"`
	Ends      []CopyEndView `json:"ends"`
	Completed int           `json:"completed"`
}

// CopyView is a copy of a site on this computer, as its History tab lists it (mockup 28e).
type CopyView struct {
	Folder   string   `json:"folder"`
	Started  string   `json:"started"` // RFC 3339
	Content  string   `json:"content"`
	LeftOut  []string `json:"leftOut"`
	Archives int      `json:"archives"` // written
	Planned  int      `json:"planned"`
	Bytes    int64    `json:"bytes"`
	Complete bool     `json:"complete"`
}

// Copies serves the copies of sites to the computer (ADR-0041): their sizes, a copy in the
// background, its resume, and the copies of a site.
type Copies struct {
	ws   *workspace.Workspace
	emit func(name string, data any)
	// OnEnded, when set, hears how each copy ended (ADR-0035).
	OnEnded func(RunEnd)

	mu        sync.Mutex
	previews  map[model.ID]sitecopy.Preview
	running   map[model.ID]*CopyRunView
	cancels   map[model.ID]context.CancelFunc
	ends      map[model.ID]CopyEndView
	completed int
}

// NewCopies returns the service; emit sends an event to the window.
func NewCopies(ws *workspace.Workspace, emit func(name string, data any)) *Copies {
	return &Copies{ws: ws, emit: emit, previews: map[model.ID]sitecopy.Preview{},
		running: map[model.ID]*CopyRunView{}, cancels: map[model.ID]context.CancelFunc{}, ends: map[model.ID]CopyEndView{}}
}

// siteFolder is the folder of a site's copies, in the folder of the copies set now.
func (c *Copies) siteFolder(ctx context.Context, site model.ID) (string, model.Site, error) {
	tree, err := c.ws.DB().LoadTree(ctx)
	if err != nil {
		return "", model.Site{}, err
	}
	s, ok := tree.Site(site)
	if !ok {
		return "", model.Site{}, errNodeNotFound
	}
	h, _ := tree.Host(s.HostID)
	root, err := c.ws.CopyFolder(ctx)
	if err != nil {
		return "", model.Site{}, err
	}
	return sitecopy.SiteFolder(root, h.Name, s.Name), s, nil
}

// Prepare reads, read-only, the sizes the copy dialog shows; Start copies from them.
func (c *Copies) Prepare(ctx context.Context, siteID string) (CopyPreviewView, *Problem) {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return CopyPreviewView{}, ProblemOf(errNodeNotFound)
	}
	dest, _, err := c.siteFolder(ctx, id)
	if err != nil {
		return CopyPreviewView{}, ProblemOf(err)
	}
	p, err := c.ws.PrepareCopy(ctx, id)
	if err != nil {
		return CopyPreviewView{}, ProblemOf(err)
	}
	size, err := c.ws.ArchiveSize(ctx)
	if err != nil {
		return CopyPreviewView{}, ProblemOf(err)
	}
	c.mu.Lock()
	c.previews[id] = p
	c.mu.Unlock()
	v := CopyPreviewView{
		SiteID: siteID, Database: p.Database, Content: p.Size(sitecopy.WithContent, nil), Site: p.Size(sitecopy.WholeSite, nil),
		Folders: []CopyFolderView{}, Split: p.Tools.GNUFind && p.Tools.GNUTar, Destination: dest, Free: freeAt(dest), ArchiveSize: size,
	}
	var inFolders int64
	for _, f := range p.Content.Folders {
		v.Folders = append(v.Folders, CopyFolderView{Name: f.Name, Bytes: f.Bytes})
		inFolders += f.Bytes
	}
	// What lies in wp-content itself, outside its folders, goes with the "others" archive.
	if rest := p.Content.Total - inFolders; rest > 0 {
		v.Folders = append(v.Folders, CopyFolderView{Name: "", Bytes: rest})
	}
	return v, nil
}

// freeAt is the free space of the disk a folder is on, measured at its nearest parent that exists;
// -1 when it cannot be read.
func freeAt(dir string) int64 {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			free, err := sitecopy.DiskFree(d)
			if err != nil {
				return -1
			}
			return int64(min(free, 1<<62))
		}
		if d == filepath.Dir(d) {
			return -1
		}
	}
}

// Start copies a site in the background, the content chosen and the folders of wp-content left
// out, from the sizes Prepare read; CopiesEvent tells how it goes.
func (c *Copies) Start(siteID, content string, leftOut []string) *Problem {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return ProblemOf(errNodeNotFound)
	}
	ctx := context.Background()
	root, err := c.ws.CopyFolder(ctx)
	if err != nil {
		return ProblemOf(err)
	}
	size, err := c.ws.ArchiveSize(ctx)
	if err != nil {
		return ProblemOf(err)
	}
	c.mu.Lock()
	p, ok := c.previews[id]
	c.mu.Unlock()
	if !ok {
		return ProblemOf(errNoPreview)
	}
	// Refused here when the disk is too small, so that the dialog says it.
	if free := freeAt(root); free >= 0 && p.Size(sitecopy.Content(content), leftOut) > free {
		return ProblemOf(sitecopy.ErrNoSpace)
	}
	c.mu.Lock()
	if _, busy := c.running[id]; busy {
		c.mu.Unlock()
		return ProblemOf(errCopyRunning)
	}
	run, ctx := c.begin(id, "", false)
	c.mu.Unlock()
	req := workspace.CopyRequest{Root: root, Content: sitecopy.Content(content), LeftOut: leftOut, Limit: int64(size) << 20}
	go c.run(ctx, id, run, func(ctx context.Context, progress func(sitecopy.Progress)) (model.Copy, error) {
		return c.ws.CopySite(ctx, id, req, p, progress)
	})
	return nil
}

// Resume reads the archives missing of a cut copy of a site, in the background.
func (c *Copies) Resume(siteID, folder string) *Problem {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return ProblemOf(errNodeNotFound)
	}
	site, _, err := c.siteFolder(context.Background(), id)
	if err != nil {
		return ProblemOf(err)
	}
	if filepath.Dir(folder) != site || !strings.HasPrefix(folder, site+string(filepath.Separator)) {
		return ProblemOf(errNotACopy)
	}
	c.mu.Lock()
	if _, busy := c.running[id]; busy {
		c.mu.Unlock()
		return ProblemOf(errCopyRunning)
	}
	run, ctx := c.begin(id, folder, true)
	c.mu.Unlock()
	go c.run(ctx, id, run, func(ctx context.Context, progress func(sitecopy.Progress)) (model.Copy, error) {
		return c.ws.ResumeCopy(ctx, id, folder, progress)
	})
	return nil
}

// begin marks a site's copy as running, Stop can cancel it; the caller holds the lock.
func (c *Copies) begin(id model.ID, folder string, resumed bool) (*CopyRunView, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	run := &CopyRunView{SiteID: id.String(), Folder: folder, Resumed: resumed}
	c.running[id] = run
	c.cancels[id] = cancel
	return run, ctx
}

// run runs a copy or a resume, telling the window how it goes, then how it ended.
func (c *Copies) run(ctx context.Context, id model.ID, run *CopyRunView, do func(context.Context, func(sitecopy.Progress)) (model.Copy, error)) {
	c.send()
	started := time.Now()
	var last time.Time
	kept, err := do(ctx, func(p sitecopy.Progress) {
		c.mu.Lock()
		run.Archive, run.Done, run.Total, run.Read, run.Expected = p.Archive, p.Done, p.Total, p.Read, p.Expected
		c.mu.Unlock()
		// Told at most ten times a second: a stream reports each block it writes.
		if now := time.Now(); now.Sub(last) >= 100*time.Millisecond || p.Archive == "" {
			last = now
			c.send()
		}
	})
	c.mu.Lock()
	if cancel := c.cancels[id]; cancel != nil {
		cancel()
	}
	delete(c.running, id)
	delete(c.cancels, id)
	c.completed++
	end := CopyEndView{SiteID: id.String(), Folder: kept.Folder, Cancelled: errors.Is(err, context.Canceled)}
	if err != nil && !end.Cancelled {
		end.Problem = ProblemOf(err)
	}
	c.ends[id] = end
	c.mu.Unlock()
	if c.OnEnded != nil && kept.ID != model.NoID {
		c.OnEnded(RunEnd{
			Kind: EndCopy, By: model.RunByUser, RunID: kept.ID, NodeIDs: []string{id.String()}, Name: kept.SiteName,
			Started: started, Finished: time.Now(), Cancelled: errors.Is(err, context.Canceled),
			Archives: kept.Archives, Failed: failedCount(err),
		})
	}
	c.send()
}

func failedCount(err error) int {
	if err != nil && !errors.Is(err, context.Canceled) {
		return 1
	}
	return 0
}

// Stop stops the copy of a site after the bytes in flight: the archives complete stay.
func (c *Copies) Stop(siteID string) {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return
	}
	c.mu.Lock()
	cancel := c.cancels[id]
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// State returns the copies that run now.
func (c *Copies) State() CopiesState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := CopiesState{Running: []CopyRunView{}, Ends: []CopyEndView{}, Completed: c.completed}
	for _, r := range c.running {
		s.Running = append(s.Running, *r)
	}
	for _, e := range c.ends {
		s.Ends = append(s.Ends, e)
	}
	slices.SortFunc(s.Running, func(a, b CopyRunView) int { return strings.Compare(a.SiteID, b.SiteID) })
	slices.SortFunc(s.Ends, func(a, b CopyEndView) int { return strings.Compare(a.SiteID, b.SiteID) })
	return s
}

func (c *Copies) send() {
	if c.emit != nil {
		c.emit(CopiesEvent, c.State())
	}
}

// List returns the copies of a site on this computer, the newest first (mockup 28e).
func (c *Copies) List(ctx context.Context, siteID string) ([]CopyView, error) {
	id, err := uuid.Parse(siteID)
	if err != nil {
		return nil, errNodeNotFound
	}
	folder, _, err := c.siteFolder(ctx, id)
	if err != nil {
		return nil, err
	}
	listed, err := sitecopy.List(folder)
	if err != nil {
		return nil, err
	}
	views := make([]CopyView, 0, len(listed))
	for _, l := range listed {
		v := CopyView{Folder: l.Dir, Started: rfc3339(l.Manifest.Started), Content: string(l.Manifest.Content),
			LeftOut: l.Manifest.LeftOut, Archives: len(l.Manifest.Archives), Planned: l.Planned, Complete: l.Complete}
		if v.LeftOut == nil {
			v.LeftOut = []string{}
		}
		for _, r := range l.Manifest.Archives {
			v.Bytes += r.Bytes
		}
		views = append(views, v)
	}
	return views, nil
}
