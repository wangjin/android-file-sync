// Package update handles checking for, downloading, and opening app updates
// from GitHub Releases. Network access goes through the ghproxy mirror first,
// falling back to a direct connection.
package update

import (
	"log"
	"strconv"
	"strings"
)

// Version is a parsed semantic version: [major, minor, patch].
type Version [3]int

// Parse turns a version string into a Version. Accepts an optional leading "v",
// missing segments (treated as 0), and a trailing "-N-gHASH" suffix produced by
// `git describe`. Returns ok=false for anything it cannot parse (e.g. "dev",
// non-numeric segments), without panicking.
func Parse(v string) (Version, bool) {
	v = strings.TrimPrefix(v, "v")
	// strip git-describe suffix: "1.0.0-3-gabc123" -> "1.0.0"
	if i := strings.Index(v, "-"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, false
	}
	var out Version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, false
		}
		if n < 0 {
			return Version{}, false
		}
		out[i] = n
	}
	return out, true
}

// Compare returns -1 if a is older than b, 0 if equal, 1 if a is newer.
// If either string cannot be parsed, it logs the fallback and compares the raw
// strings lexically (never panicking).
func Compare(a, b string) int {
	va, oka := Parse(a)
	vb, okb := Parse(b)
	if !oka || !okb {
		log.Printf("update: version parse fallback (a=%q ok=%v, b=%q ok=%v), using lexical compare", a, oka, b, okb)
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	}
	for i := 0; i < 3; i++ {
		if va[i] < vb[i] {
			return -1
		}
		if va[i] > vb[i] {
			return 1
		}
	}
	return 0
}
