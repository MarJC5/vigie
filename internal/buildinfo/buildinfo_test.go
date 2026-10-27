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
