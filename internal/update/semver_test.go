package update

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.2.1", -1},
		{"v1.3.0", "1.2.9", 1},
		{"1.2.0", "1.2.0", 0},
		{"1.2.0-beta", "1.2.0", 0},
	}
	for _, tt := range tests {
		if got := Compare(tt.a, tt.b); got != tt.want {
			t.Fatalf("Compare(%q,%q)=%d want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("1.2.0", "v1.2.1") {
		t.Fatal("expected newer")
	}
	if IsNewer("1.2.0", "1.2.0") {
		t.Fatal("same version should not be newer")
	}
}
