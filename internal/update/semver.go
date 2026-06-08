package update

import (
	"strconv"
	"strings"
)

// Compare returns 1 if a > b, -1 if a < b, 0 if equal.
// Pre-release segments are ignored for simplicity.
func Compare(a, b string) int {
	pa := parseParts(a)
	pb := parseParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

// IsNewer reports whether candidate is newer than current.
func IsNewer(current, candidate string) bool {
	return Compare(candidate, current) > 0
}

func parseParts(v string) [3]int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}
