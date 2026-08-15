package dariproto

import "testing"

func TestVersionBelow(t *testing.T) {
	cases := []struct {
		v, min string
		want   bool
	}{
		{"1.2.3", "1.2.4", true},
		{"v1.2.4", "1.2.4", false},
		{"1.10.0", "1.9.9", false},
		{"dev", "1.0.0", true},
		{"1.0.0", "dev", false},
		{"2.0.0", "2.0.0", false},
		{"1.0", "1.0.1", false},
	}
	for _, c := range cases {
		if got := VersionBelow(c.v, c.min); got != c.want {
			t.Errorf("VersionBelow(%q, %q) = %v, want %v", c.v, c.min, got, c.want)
		}
	}
}
