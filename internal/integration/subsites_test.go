//go:build integration

package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/MarJC5/vigie/internal/wpcli"
)

// The subsites script, sent over SSH to WP-CLI on the bench's network (ADR-0029): every subsite
// but the archived one, each with its theme and administrators, and the super administrators.
func TestSubsitesScriptOnTheNetwork(t *testing.T) {
	a := sshAccess(t)
	line, err := wpcli.Subsites().Remote("wp", "~/sites/network")
	if err != nil {
		t.Fatal(err)
	}
	res, err := a.runner(t, a.trusted(t)).RunInput(context.Background(), a.target(a.key), line, wpcli.SubsitesScript())
	if err != nil {
		t.Fatal(err)
	}
	network, err := wpcli.ParseSubsites(res)
	if err != nil {
		t.Fatalf("%v\nstdout %s\nstderr %s", err, res.Stdout, res.Stderr)
	}
	var ids []int
	for _, s := range network.Sites {
		ids = append(ids, s.ID)
		if s.Theme == "" || s.ThemeName == "" || s.Email == "" {
			t.Errorf("subsite %d: theme %q (%q), email %q", s.ID, s.Theme, s.ThemeName, s.Email)
		}
	}
	if !slices.Equal(ids, []int{1, 2, 3}) {
		t.Errorf("subsites %v, want 1, 2 and 3 (4 is archived)", ids)
	}
	if !slices.Contains(network.Supers, "admin") {
		t.Errorf("super administrators %v, want admin", network.Supers)
	}
	if len(network.Sites) > 0 && !slices.ContainsFunc(network.Sites[0].Admins, func(a wpcli.SubsiteAdmin) bool { return a.Login == "admin" }) {
		t.Errorf("main site's administrators %+v, want admin", network.Sites[0].Admins)
	}
	if len(res.Stderr) > 0 {
		t.Errorf("the script wrote on stderr: %s", res.Stderr)
	}
}
