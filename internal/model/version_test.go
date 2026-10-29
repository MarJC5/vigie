package model_test

import (
	"testing"

	"github.com/MarJC5/vigie/internal/model"
)

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"7.1.4", "7.1.3", 1},
		{"7.10", "7.9", 1}, // part by part, not as text
		{"7.1", "7.1.0", 0},
		{"7.1", "7.1.1", -1},
		{"6.9.4", "7.0", -1},
		{"7.2-beta2", "7.2-beta1", 1},
		{"7.2", "7.2-beta1", 1}, // a release comes after its betas
		{"7.2-RC1", "7.2", -1},
		{"5.7.2", "5.7.2", 0},
	} {
		if got := model.CompareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
