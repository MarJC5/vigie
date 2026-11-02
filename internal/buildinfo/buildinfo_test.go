package buildinfo_test

import (
	"testing"

	"github.com/MarJC5/vigie/internal/buildinfo"
)

func TestVersionDefaultsToDev(t *testing.T) {
	if got := buildinfo.Version(); got != "dev" {
		t.Fatalf("Version() = %q, want %q", got, "dev")
	}
}

func TestUserAgent(t *testing.T) {
	if got, want := buildinfo.UserAgent("0.6.0"), "Vigie/0.6.0 (+https://github.com/MarJC5/vigie)"; got != want {
		t.Fatalf("UserAgent() = %q, want %q", got, want)
	}
}
