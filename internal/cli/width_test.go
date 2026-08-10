package cli

import (
	"strings"
	"testing"
)

func TestVisibleWidthGraphemeClusters(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want int
	}{
		{"ascii", "abc", 3},
		{"cjk", "기능", 4},
		{"emoji", "🔥", 2},
		// x/ansi counts the VS16 keycap as 2 (emoji presentation). Terminals
		// disagree on VS16 width, but the point is consistency: wrapAnsi /
		// clampWidth now measure via the same x/ansi, so box rails and wrapping
		// agree — which a mixed uniseg(1)/ansi(2) split would break.
		{"keycap", "1️⃣", 2},
		// occupying one emoji's width, not the rune-by-rune sum (which was 8).
		{"zwj-family", "👨‍👩‍👧‍👦", 2},
		{"ansi-stripped", "\x1b[31mab\x1b[0m", 2},
		{"mixed", "a한🔥", 5},
	}
	for _, c := range cases {
		if got := visibleWidth(c.s); got != c.want {
			t.Errorf("%s: visibleWidth(%q) = %d, want %d", c.name, c.s, got, c.want)
		}
	}
}

func TestClampWidthHardwrap(t *testing.T) {
	for line := range strings.SplitSeq(clampWidth("기능한", 4), "\n") {
		if visibleWidth(line) > 4 {
			t.Errorf("cjk line %q exceeds width 4 (got %d)", line, visibleWidth(line))
		}
	}

	if got := clampWidth("ab", 10); got != "ab" {
		t.Errorf("in-width line altered: %q", got)
	}

	styled := clampWidth("\x1b[31mab\x1b[0m", 2)
	if visibleWidth(styled) > 2 {
		t.Errorf("styled line exceeds width 2 (got %d): %q", visibleWidth(styled), styled)
	}

	for line := range strings.SplitSeq(clampWidth(strings.Repeat("한", 10), 6), "\n") {
		if visibleWidth(line) > 6 {
			t.Errorf("line %q exceeds width 6 (got %d)", line, visibleWidth(line))
		}
	}
}
