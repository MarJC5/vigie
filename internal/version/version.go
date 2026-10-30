// Package version compares versions the way PHP's version_compare does, which is how WordPress and
// the vulnerability data (ADR-0008) order them: "1.0" before "1.0.0", "dev" before "alpha" before
// "beta" before "RC" before the release, and a patch level ("pl", "p") after it.
package version

import (
	"fmt"
	"strings"
)

// Compare returns -1, 0 or 1 as a is before, the same as, or after b.
func Compare(a, b string) int {
	if a == "" || b == "" {
		switch {
		case a == b:
			return 0
		case a == "":
			return -1
		}
		return 1
	}
	return compare(canon(a), canon(b))
}

// canon is the canonical form PHP compares, except that a version starting with "#" (the stand-in
// for a number) is kept as it is.
func canon(v string) string {
	if strings.HasPrefix(v, "#") {
		return v
	}
	return canonical(v)
}

// number stands for any number when a word is compared with one ("#N#" in PHP).
const number = "#N#"

// compare runs PHP's loop over two canonical versions: part by part while both have one, then
// the longer one decides by what its next part is (PHP compares that rest again from the start).
func compare(a, b string) int {
	pa, pb := a, b
	var restA, restB string
	hasA, hasB := true, true
	result := 0
	for pa != "" && pb != "" && hasA && hasB {
		var partA, partB string
		partA, restA, hasA = strings.Cut(pa, ".")
		partB, restB, hasB = strings.Cut(pb, ".")
		result = comparePart(partA, partB)
		if result != 0 {
			break
		}
		if hasA {
			pa = restA
		}
		if hasB {
			pb = restB
		}
	}
	if result != 0 {
		return result
	}
	switch {
	case hasA:
		if isDigit(first(pa)) {
			return 1
		}
		return Compare(pa, number)
	case hasB:
		if isDigit(first(pb)) {
			return -1
		}
		return Compare(number, pb)
	}
	return 0
}

func comparePart(a, b string) int {
	switch {
	case isDigit(first(a)) && isDigit(first(b)):
		return compareNumbers(a, b)
	case !isDigit(first(a)) && !isDigit(first(b)):
		return compareForms(a, b)
	case isDigit(first(a)):
		return compareForms(number, b)
	}
	return compareForms(a, number)
}

// compareNumbers compares two runs of digits by value, whatever their length.
func compareNumbers(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return sign(len(a) - len(b))
	}
	return strings.Compare(a, b)
}

// forms are PHP's special words in order; a word matches the first form it starts with, an
// unknown word comes before them all.
var forms = []struct {
	name  string
	order int
}{
	{"dev", 0}, {"alpha", 1}, {"a", 1}, {"beta", 2}, {"b", 2}, {"RC", 3}, {"rc", 3}, {"#", 4}, {"pl", 5}, {"p", 5},
}

func compareForms(a, b string) int {
	return sign(formOrder(a) - formOrder(b))
}

func formOrder(word string) int {
	for _, f := range forms {
		if strings.HasPrefix(word, f.name) {
			return f.order
		}
	}
	return -1
}

// canonical rewrites a version as PHP does before comparing: "-", "_" and "+" become dots, a dot
// goes between digits and letters, other signs become dots, never two dots in a row.
func canonical(v string) string {
	out := []byte{v[0]}
	last := v[0]
	dot := func() {
		if out[len(out)-1] != '.' {
			out = append(out, '.')
		}
	}
	for i := 1; i < len(v); i++ {
		c := v[i]
		switch {
		case c == '-' || c == '_' || c == '+':
			dot()
		case (notDigit(last) && digit(c)) || (digit(last) && notDigit(c)):
			dot()
			out = append(out, c)
		case !isAlnum(c):
			dot()
		default:
			out = append(out, c)
		}
		last = c
	}
	return string(out)
}

// digit and notDigit are PHP's isdig and isndig: a dot is neither.
func digit(c byte) bool    { return isDigit(c) && c != '.' }
func notDigit(c byte) bool { return !isDigit(c) && c != '.' }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlnum(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func first(s string) byte {
	if s == "" {
		return 0
	}
	return s[0]
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// Matches tells whether v op target holds, op being a WPVulnerability operator (lt, le, gt, ge,
// eq, ne) or one of PHP's symbols.
func Matches(v, op, target string) (bool, error) {
	c := Compare(v, target)
	switch op {
	case "lt", "<":
		return c < 0, nil
	case "le", "<=":
		return c <= 0, nil
	case "gt", ">":
		return c > 0, nil
	case "ge", ">=":
		return c >= 0, nil
	case "eq", "==", "=":
		return c == 0, nil
	case "ne", "!=", "<>":
		return c != 0, nil
	}
	return false, fmt.Errorf("version operator %q: unknown", op)
}
