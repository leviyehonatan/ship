package detect

import (
	"testing"
)

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"1.49.0", "1.49.1", -1},
		{"1.50.0", "1.49.0", 1},
		{"1.67.0", "1.49.0", 1},
		{"0.9.0", "1.0.0", -1},
	}
	for _, tt := range tests {
		got := compareSemver(parseSemver(tt.a), parseSemver(tt.b))
		if got != tt.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		installed, min, max string
		wantWarning          bool
	}{
		{"1.49.0", "1.0.0", "1.49.0", false},  // exact match
		{"1.50.0", "1.0.0", "1.49.0", true},   // newer than tested
		{"0.9.0", "1.0.0", "1.49.0", true},    // too old
		{"1.49.0", "1.0.0", "1.50.0", false},  // within range
	}
	for _, tt := range tests {
		warn := checkVersion("test", tt.installed, tt.min, tt.max)
		hasWarning := warn != ""
		if hasWarning != tt.wantWarning {
			t.Errorf("checkVersion(%q, %q, %q) warning=%v, wantWarning=%v",
				tt.installed, tt.min, tt.max, hasWarning, tt.wantWarning)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		in   string
		want semver
	}{
		{"v1.49.0", [3]int{1, 49, 0}},
		{"1.67.0", [3]int{1, 67, 0}},
		{"version 2.3.4 extra", [3]int{2, 3, 4}},
		{"no version here", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		got := parseSemver(tt.in)
		if got != semver(tt.want) {
			t.Errorf("parseSemver(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
