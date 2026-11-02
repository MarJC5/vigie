package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"

	"github.com/MarJC5/vigie/internal/lookup"
	"github.com/MarJC5/vigie/internal/workspace"
)

// SelfUpdateEvent carries a SelfUpdateView to the window whenever the state of Vigie's own update
// changes (ADR-0038).
const SelfUpdateEvent = "vigie:selfupdate"

// selfUpdateCheckedSetting is when Vigie last looked for a new version, RFC 3339.
const selfUpdateCheckedSetting = "selfupdate.checked"

// InstallKind is how Vigie was installed, which decides who updates it.
type InstallKind string

const (
	InstallApp      InstallKind = "app"      // the app or the AppImage: it updates itself
	InstallHomebrew InstallKind = "homebrew" // brew upgrade --cask vigie
	InstallPackage  InstallKind = "package"  // the Linux package manager
	InstallDev      InstallKind = "dev"      // a build of the sources: no update
)

// SelfUpdater is Wails' updater, as Vigie uses it: a check, then on the user's word a download,
// verified against the embedded key, and a restart.
type SelfUpdater interface {
	Check(ctx context.Context) (*updater.Release, error)
	DownloadAndInstall(ctx context.Context) error
	Restart(ctx context.Context) error
}

// SelfUpdateView is what Settings, About shows of Vigie's own update.
type SelfUpdateView struct {
	Install string `json:"install"` // app, homebrew, package, dev
	Current string `json:"current"`
	// State: idle, checking, up-to-date, available, downloading, error.
	State     string   `json:"state"`
	Latest    string   `json:"latest"`
	Notes     string   `json:"notes"`     // Markdown
	Published string   `json:"published"` // RFC 3339
	Checked   string   `json:"checked"`   // the last check, RFC 3339
	Written   int64    `json:"written"`   // bytes downloaded
	Total     int64    `json:"total"`
	Problem   *Problem `json:"problem"`
}

// SelfUpdate serves Vigie's own update to the window (ADR-0038): never for Homebrew, a Linux
// package or a build of the sources; a daily check under the lookups' consent and the github.com
// switch, one when the user asks; a download only when the user asks, not while a scan or an update
// runs.
type SelfUpdate struct {
	ws      *workspace.Workspace
	up      SelfUpdater
	install InstallKind
	busy    func() bool
	emit    func(name string, data any)
	now     func() time.Time

	mu   sync.Mutex
	view SelfUpdateView
}

// NewSelfUpdate returns the service; busy tells whether scans or updates run.
func NewSelfUpdate(ws *workspace.Workspace, up SelfUpdater, install InstallKind, current string, busy func() bool,
	emit func(name string, data any), now func() time.Time) *SelfUpdate {
	s := &SelfUpdate{ws: ws, up: up, install: install, busy: busy, emit: emit, now: now,
		view: SelfUpdateView{Install: string(install), Current: current, State: "idle"}}
	if checked, ok, err := ws.DB().Setting(context.Background(), selfUpdateCheckedSetting); err == nil && ok {
		s.view.Checked = checked
	}
	return s
}

// State returns what About shows.
func (s *SelfUpdate) State() SelfUpdateView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

// Check looks for a new version now, because the user asked.
func (s *SelfUpdate) Check() *Problem {
	if s.install != InstallApp {
		return nil
	}
	return s.check(context.Background())
}

// Due looks for a new version once a day, when the lookups are agreed and github.com is on.
func (s *SelfUpdate) Due(ctx context.Context) {
	if s.install != InstallApp || !s.ws.LookupAllowed(lookup.GitHub) {
		return
	}
	s.mu.Lock()
	last, state := s.view.Checked, s.view.State
	s.mu.Unlock()
	if state == "checking" || state == "downloading" || state == "available" {
		return
	}
	if t, err := time.Parse(time.RFC3339, last); err == nil && s.now().Sub(t) < 24*time.Hour {
		return
	}
	_ = s.check(ctx)
}

func (s *SelfUpdate) check(ctx context.Context) *Problem {
	s.set(func(v *SelfUpdateView) { v.State, v.Problem = "checking", nil })
	rel, err := s.up.Check(ctx)
	checked := s.now().UTC().Format(time.RFC3339)
	_ = s.ws.DB().SetSetting(context.WithoutCancel(ctx), selfUpdateCheckedSetting, checked)
	var p *Problem
	s.set(func(v *SelfUpdateView) {
		v.Checked = checked
		switch {
		case err != nil:
			p = &Problem{Code: "problem.selfUpdateUnreachable", Detail: err.Error()}
			v.State, v.Problem = "error", p
		case rel == nil:
			v.State, v.Latest, v.Notes, v.Published = "up-to-date", "", "", ""
		default:
			v.State, v.Latest, v.Notes, v.Published = "available", rel.Version, rel.Notes, rfc3339(rel.PublishedAt)
		}
	})
	return p
}

// Install downloads the new version, verified against the key in the app, installs it and
// restarts Vigie; refused while scans or updates run.
func (s *SelfUpdate) Install() *Problem {
	if s.install != InstallApp {
		return &Problem{Code: "problem.selfUpdateNotHere"}
	}
	if s.busy() {
		return &Problem{Code: "problem.selfUpdateBusy"}
	}
	s.set(func(v *SelfUpdateView) { v.State, v.Written, v.Total, v.Problem = "downloading", 0, 0, nil })
	ctx := context.Background()
	if err := s.up.DownloadAndInstall(ctx); err != nil {
		p := &Problem{Code: "problem.selfUpdateFailed", Detail: err.Error()}
		if msg := err.Error(); strings.Contains(msg, "signature") || strings.Contains(msg, "digest") {
			p.Code = "problem.selfUpdateSignature"
		}
		s.set(func(v *SelfUpdateView) { v.State, v.Problem = "error", p })
		return p
	}
	if err := s.up.Restart(ctx); err != nil {
		p := &Problem{Code: "problem.selfUpdateFailed", Detail: err.Error()}
		s.set(func(v *SelfUpdateView) { v.State, v.Problem = "error", p })
		return p
	}
	return nil
}

// Progress follows the download, from Wails' events.
func (s *SelfUpdate) Progress(written, total int64) {
	s.set(func(v *SelfUpdateView) { v.Written, v.Total = written, total })
}

func (s *SelfUpdate) set(change func(v *SelfUpdateView)) {
	s.mu.Lock()
	change(&s.view)
	view := s.view
	s.mu.Unlock()
	s.emit(SelfUpdateEvent, view)
}
