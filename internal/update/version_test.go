package update

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
		ok   bool
	}{
		{"v1.0.0", [3]int{1, 0, 0}, true},
		{"1.0.0", [3]int{1, 0, 0}, true},        // no v prefix
		{"1.0", [3]int{1, 0, 0}, true},          // missing patch -> 0
		{"2", [3]int{2, 0, 0}, true},            // only major
		{"v1.0.0-3-gabc123", [3]int{1, 0, 0}, true}, // git describe format
		{"dev", [3]int{}, false},                // dev build
		{"garbage", [3]int{}, false},            // non-numeric
		{"", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("Parse(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1.0", "v1.0.9", 1},
		{"1.0.0", "v1.0.0", 0},     // equal ignoring prefix
		{"v2.0.0", "v1.9.9", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v1.0.0", "v1.0.0", 0},
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		// normalize: only sign matters
		sign := 0
		if got < 0 {
			sign = -1
		} else if got > 0 {
			sign = 1
		}
		if sign != c.want {
			t.Errorf("Compare(%q,%q) sign = %d; want %d", c.a, c.b, sign, c.want)
		}
	}
}

// Compare must not panic on non-semver input — it falls back to string compare.
func TestCompareNonSemver(t *testing.T) {
	// "dev" vs "v1.0.0": dev not parseable, falls back to lexical.
	// Must not panic; returns some int.
	got := Compare("dev", "v1.0.0")
	if got != -1 && got != 0 && got != 1 {
		t.Errorf("Compare(dev, v1.0.0) = %d, want one of -1/0/1", got)
	}
}
