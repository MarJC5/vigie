package buildinfo

import "testing"

func TestVersionDefaultsToDev(t *testing.T) {
	if got := Version(); got != "dev" {
		t.Fatalf("Version() = %q, want %q", got, "dev")
	}
}
