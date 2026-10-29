package model

import (
	"cmp"
	"strconv"
	"strings"
)

// CompareVersions compares WordPress, plugin and theme versions: numbers part by part ("7.10"
// after "7.9", a missing part counts as 0), then a release after its pre-releases ("7.2" after
// "7.2-beta1"), pre-releases by their text. A part that is not a number compares as text.
func CompareVersions(a, b string) int {
	coreA, preA, hasPreA := strings.Cut(a, "-")
	coreB, preB, hasPreB := strings.Cut(b, "-")
	partsA, partsB := strings.Split(coreA, "."), strings.Split(coreB, ".")
	for i := range max(len(partsA), len(partsB)) {
		if c := comparePart(part(partsA, i), part(partsB, i)); c != 0 {
			return c
		}
	}
	switch {
	case hasPreA && hasPreB:
		return strings.Compare(preA, preB)
	case hasPreA:
		return -1
	case hasPreB:
		return 1
	}
	return 0
}

func part(parts []string, i int) string {
	if i < len(parts) {
		return parts[i]
	}
	return "0"
}

func comparePart(x, y string) int {
	nx, errX := strconv.Atoi(x)
	ny, errY := strconv.Atoi(y)
	if errX == nil && errY == nil {
		return cmp.Compare(nx, ny)
	}
	return strings.Compare(x, y)
}
