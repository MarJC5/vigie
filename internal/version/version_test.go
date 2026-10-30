package version_test

import (
	"regexp"
	"testing"

	"github.com/MarJC5/vigie/internal/version"
)

// Expected results are PHP's version_compare (ext/standard/versioning.c), which WordPress and the
// vulnerability data use.
func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "1.0.0", -1}, // a missing part is not a zero
		{"5.2.0", "5.2", 1},
		{"1.10", "1.9", 1},
		{"8.4.27", "8.4.5", 1},
		{"6.0.4", "6.0.6", -1},
		{"1.0-dev", "1.0alpha", -1},
		{"1.0alpha", "1.0a", 0}, // a and alpha are the same form
		{"1.0a1", "1.0b1", -1},
		{"1.0beta2", "1.0RC1", -1},
		{"1.0rc1", "1.0", -1},
		{"1.0-beta", "1.0", -1},
		{"1.0", "1.0pl1", -1}, // a patch level comes after the release
		{"1.0-p1", "1.0", 1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1},
		{"7.1.3", "7.1.3", 0},
		{"1_0", "1.0", 0}, // -, _ and + read as dots
		{"1.0+2", "1.0.2", 0},
		{"1.0.x", "1.0.0", -1}, // an unknown word comes before any number
		{"1.0.", "1.0", -1},    // an empty rest is before anything
		{"", "1.0", -1},
		{"", "", 0},
	} {
		if got := version.Compare(tc.a, tc.b); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
		if got := version.Compare(tc.b, tc.a); got != -tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.b, tc.a, got, -tc.want)
		}
	}
}

func TestMatches(t *testing.T) {
	for _, tc := range []struct {
		v, op, target string
		want          bool
	}{
		{"9.8.1", "lt", "9.8.3", true},
		{"9.8.3", "lt", "9.8.3", false},
		{"6.2.4", "le", "6.2.4", true},
		{"5.2.3", "ge", "5.2.3", true},
		{"5.2.2", "ge", "5.2.3", false},
		{"1.0", "eq", "1.0", true},
		{"1.0", "ne", "1.0.0", true},
		{"2.0", "gt", "1.9", true},
		{"2.0", "<=", "2.0", true}, // PHP's symbols too
	} {
		got, err := version.Matches(tc.v, tc.op, tc.target)
		if err != nil || got != tc.want {
			t.Errorf("Matches(%q, %q, %q) = %v, %v; want %v", tc.v, tc.op, tc.target, got, err, tc.want)
		}
	}
	if _, err := version.Matches("1.0", "about", "1.0"); err == nil {
		t.Error("Matches() with an unknown operator: want an error")
	}
}

// wellFormed is a version without empty parts. PHP's algorithm, which Compare follows, is not
// antisymmetric otherwise: it compares ".." and "-_+" as smaller than each other.
var wellFormed = regexp.MustCompile(`^[0-9A-Za-z]+([.\-_+][0-9A-Za-z]+)*$`)

// Versions come from servers and from a third party: never a panic, and Compare is antisymmetric
// on well-formed versions.
func FuzzCompare(f *testing.F) {
	for _, seed := range [][2]string{{"1.0", "1.0.0"}, {"1.0-beta", "1.0rc1"}, {"1.0.", ""}, {"#N#", "pl"}, {"..", "-_+"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		ab, ba := version.Compare(a, b), version.Compare(b, a)
		if !wellFormed.MatchString(a) || !wellFormed.MatchString(b) {
			return
		}
		if ab != -ba {
			t.Fatalf("Compare(%q, %q) = %d but Compare(%q, %q) = %d", a, b, ab, b, a, ba)
		}
	})
}
