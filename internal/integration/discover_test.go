//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/MarJC5/vigie/internal/discover"
	"github.com/MarJC5/vigie/internal/model"
	"github.com/MarJC5/vigie/internal/sshx"
)

func TestDiscoverOverSSH(t *testing.T) {
	a := sshAccess(t)
	d := discover.Discoverer{Runner: a.runner(t, a.trusted(t)), Limiter: sshx.NewLimiter()}
	existing := []model.Site{{Name: "plain", Path: "~/sites/plain"}}
	found, truncated, err := d.Host(context.Background(), discover.Account{
		Target: a.target(a.key), HostKey: "bench", Max: 2, WPCLI: "wp",
	}, existing)
	if err != nil || truncated {
		t.Fatalf("Host() error = %v, truncated %v", err, truncated)
	}
	states := map[string]discover.State{}
	for _, f := range found {
		states[f.Path] = f.State
	}
	want := map[string]discover.State{
		"~/sites/plain":                discover.StateAlreadyAdded,
		"~/sites/parent-config/public": discover.StateNew,
		"~/sites/network":              discover.StateNew,
		"~/sites/outdated":             discover.StateNew,
		"~/sites/offline":              discover.StateNew,
		"~/sites/broken":               discover.StateCannotLoad,
		"~/sites/plugin-theme":         discover.StateNew,
		"~/sites/updates":              discover.StateNew,
		"~/sites/security":             discover.StateNew,
	}
	if len(states) != len(want) {
		t.Fatalf("found %v, want %v (nothing under node_modules)", states, want)
	}
	for path, w := range want {
		if states[path] != w {
			t.Errorf("%s: %q, want %q", path, states[path], w)
		}
	}
}
