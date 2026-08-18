package textutil

import "testing"

func TestHangulLeadingJamo(t *testing.T) {
	cases := []struct {
		in   rune
		want rune
	}{
		{'가', 'ㄱ'},
		{'까', 'ㄲ'},
		{'한', 'ㅎ'},
		{'힣', 'ㅎ'},
		// Non-syllables: ASCII, bare jamo (outside the syllable block), and
		// compat jamo (U+1100 block) all return 0.
		{'A', 0},
		{'ㅎ', 0}, // U+314E: jamo, not a precomposed syllable
		{'ᄀ', 0}, // U+1100: compat jamo, not a precomposed syllable
	}
	for _, c := range cases {
		if got := HangulLeadingJamo(c.in); got != c.want {
			t.Errorf("HangulLeadingJamo(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestChoseongOf(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"모델변경", "ㅁㄷㅂㄱ"},
		{"/압축", "/ㅇㅊ"},
		{"한/글", "ㅎ/ㄱ"}, // path separator preserved, not decomposed
		{"README.md", "README.md"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ChoseongOf(c.in); got != c.want {
			t.Errorf("ChoseongOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHasJamo(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"ㅇㅊ", true},
		{"/ㅇㅊ", true},
		{"login", false},
		{"압축", false}, // syllables are not jamo
		{"", false},
	}
	for _, c := range cases {
		if got := HasJamo(c.in); got != c.want {
			t.Errorf("HasJamo(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestChoseongMatch(t *testing.T) {
	cases := []struct {
		name      string
		candidate string
		query     string
		want      bool
	}{
		{"empty query matches everything", "압축", "", true},
		{"pure-Latin query never matches (D3)", "ㅋㅋlogin", "login", false},
		{"pure-Latin uppercase never matches (D3)", "ㅋㅋlogin", "LOGIN", false},
		{"full chosung", "압축", "ㅇㅊ", true},
		{"slash-prefixed chosung (palette case)", "/압축", "/ㅇㅊ", true},
		{"label with parens", "/압축 (compact)", "ㅇㅊ", true},
		{"partial prefix", "모델변경", "ㅁㄷ", true},
		{"partial subsequence, not prefix", "모델변경", "ㄷㅂㄱ", true},
		{"mixed query", "ㅋㅋlogin", "ㅋㅋlogin", true},
		{"mixed query, missing jamo part", "login", "ㅋㅋlogin", false},
		{"mixed multi-run, in order", "ㅁbㄷ", "ㅁbㄷ", true},
		{"mixed multi-run, order violated", "bㅁㄷ", "ㅁbㄷ", false},
		{"mixed with uppercase Latin (query folded)", "ㅋㅋlogin", "ㅋㅋLOGIN", true},
		{"mixed with uppercase Latin (candidate folded)", "ㅋㅋLogin", "ㅋㅋlogin", true},
		{"double consonant: ㄱ must not match ㄲ (D2)", "까치", "ㄱ", false},
		{"double consonant: ㄲ matches ㄲ", "까치", "ㄲ", true},
		{"plain consonant matches", "가나다", "ㄱ", true},
		{"no Hangul in candidate", "apple", "ㅇㅊ", false},
		{"korean filename at top level", "한국어문서.md", "ㅎㄱ", true},
		{"korean filename, absent runes", "한국어문서.md", "ㄷㄷ", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ChoseongMatch(c.candidate, c.query); got != c.want {
				t.Errorf("ChoseongMatch(%q, %q) = %v, want %v", c.candidate, c.query, got, c.want)
			}
		})
	}
}
