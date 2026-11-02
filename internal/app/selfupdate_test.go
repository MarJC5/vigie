package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"

	"github.com/MarJC5/vigie/internal/app"
	"github.com/MarJC5/vigie/internal/sshx/sshxtest"
)

// fakeSelfUpdater stands for Wails' updater: the release it finds, and what was asked of it.
type fakeSelfUpdater struct {
	mu       sync.Mutex
	release  *updater.Release
	checks   int
	installs int
	restarts int
	fail     error // DownloadAndInstall's
}

func (f *fakeSelfUpdater) Check(context.Context) (*updater.Release, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	return f.release, nil
}

func (f *fakeSelfUpdater) DownloadAndInstall(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.installs++
	return f.fail
}

func (f *fakeSelfUpdater) Restart(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	return nil
}

var newVersion = &updater.Release{Version: "0.6.0", Notes: "- Several items selected", PublishedAt: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)}

// A check the user asks finds the new version; nothing is downloaded before Install (ADR-0038).
func TestCheckFindsANewVersion(t *testing.T) {
	ws, _, _ := benchSites(t, sshxtest.NewFake())
	up := &fakeSelfUpdater{release: newVersion}
	s := app.NewSelfUpdate(ws, up, app.InstallApp, "0.5.0", func() bool { return false }, func(string, any) {}, time.Now)
	if p := s.Check(); p != nil {
		t.Fatal(p)
	}
	v := s.State()
	if v.State != "available" || v.Latest != "0.6.0" || v.Notes != "- Several items selected" || v.Published == "" || v.Checked == "" || up.installs != 0 {
		t.Fatalf("State() = %+v, installs %d", v, up.installs)
	}
}

// The daily check runs only with the lookups agreed and github.com on, once a day at most.
func TestTheDailyCheck(t *testing.T) {
	ws, _, _ := benchSites(t, sshxtest.NewFake())
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	up := &fakeSelfUpdater{}
	s := app.NewSelfUpdate(ws, up, app.InstallApp, "0.5.0", func() bool { return false }, func(string, any) {}, func() time.Time { return now })
	security := app.NewSecurity(ws, func() time.Time { return now })
	ctx := context.Background()

	s.Due(ctx)
	if up.checks != 0 {
		t.Fatal("checked before the lookups were agreed")
	}
	if p := security.Agree(ctx, []string{"wpvulnerability", "github"}); p != nil {
		t.Fatal(p)
	}
	s.Due(ctx)
	if up.checks != 1 {
		t.Fatalf("checks = %d after agreeing, want 1", up.checks)
	}
	now = now.Add(23 * time.Hour)
	s.Due(ctx)
	if up.checks != 1 {
		t.Fatal("checked twice within a day")
	}
	now = now.Add(2 * time.Hour)
	s.Due(ctx)
	if up.checks != 2 {
		t.Fatalf("checks = %d a day later, want 2", up.checks)
	}
	if p := security.SetDestination(ctx, "github", false); p != nil {
		t.Fatal(p)
	}
	now = now.Add(48 * time.Hour)
	s.Due(ctx)
	if up.checks != 2 {
		t.Fatal("checked with github.com turned off")
	}
}

// Homebrew, a Linux package and a build of the sources never check nor download.
func TestOtherInstallsNeverUpdateThemselves(t *testing.T) {
	ws, _, _ := benchSites(t, sshxtest.NewFake())
	for _, install := range []app.InstallKind{app.InstallHomebrew, app.InstallPackage, app.InstallDev} {
		up := &fakeSelfUpdater{release: newVersion}
		s := app.NewSelfUpdate(ws, up, install, "0.5.0", func() bool { return false }, func(string, any) {}, time.Now)
		s.Check()
		s.Due(context.Background())
		if p := s.Install(); p == nil || up.checks+up.installs != 0 || s.State().Install != string(install) {
			t.Errorf("%s: checks %d, installs %d, Install() = %v", install, up.checks, up.installs, p)
		}
	}
}

// Installing waits for the scans and updates to end, then restarts; a signature refused says so.
func TestInstall(t *testing.T) {
	ws, _, _ := benchSites(t, sshxtest.NewFake())
	busy := true
	up := &fakeSelfUpdater{release: newVersion}
	s := app.NewSelfUpdate(ws, up, app.InstallApp, "0.5.0", func() bool { return busy }, func(string, any) {}, time.Now)
	s.Check()
	if p := s.Install(); code(p) != "problem.selfUpdateBusy" || up.installs != 0 {
		t.Fatalf("Install() while busy = %+v, installs %d", p, up.installs)
	}
	busy = false
	if p := s.Install(); p != nil || up.installs != 1 || up.restarts != 1 {
		t.Fatalf("Install() = %+v, installs %d, restarts %d", p, up.installs, up.restarts)
	}

	up = &fakeSelfUpdater{release: newVersion, fail: errors.New("updater: ed25519 signature did not verify")}
	s = app.NewSelfUpdate(ws, up, app.InstallApp, "0.5.0", func() bool { return false }, func(string, any) {}, time.Now)
	s.Check()
	if p := s.Install(); code(p) != "problem.selfUpdateSignature" || up.restarts != 0 || s.State().State != "error" {
		t.Fatalf("Install() with a bad signature = %+v, restarts %d, state %+v", p, up.restarts, s.State())
	}
}
