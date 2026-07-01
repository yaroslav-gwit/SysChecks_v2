package cmd

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.2", "1.0.2", 0},
		{"v1.0.2", "1.0.2", 0},
		{"1.0.2", "1.0.3", -1},
		{"1.0.3", "1.0.2", 1},
		{"1.2.0", "1.10.0", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.0", "1.0.0", 0},
		// Dev/dirty builds carry a suffix; the base version is what matters.
		{"1.0.2-2-g51630fd-dirty", "1.0.2", 0},
		{"1.0.2-2-g51630fd", "1.0.3", -1},
		// Unparseable current version (e.g. "development") is treated as older.
		{"development", "1.0.3", -1},
	}

	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	if v, ok := parseSemver("1.2.3"); !ok || v != [3]int{1, 2, 3} {
		t.Errorf("parseSemver(1.2.3) = %v, %v", v, ok)
	}
	if _, ok := parseSemver("not-a-version"); ok {
		t.Errorf("parseSemver(not-a-version) should fail")
	}
	if _, ok := parseSemver("1.2.3.4"); ok {
		t.Errorf("parseSemver(1.2.3.4) should fail (too many parts)")
	}
}
