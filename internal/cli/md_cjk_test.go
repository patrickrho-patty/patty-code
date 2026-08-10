package cli

import (
	"strings"
	"testing"
)

func TestFixCJKEmphasis(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "cjk punctuation bold",
			input: "**테스트，**더보기",
			want:  "**테스트，** 더보기",
		},
		{
			name:  "cjk punctuation bold with period",
			input: "**테스트。**더보기",
			want:  "**테스트。** 더보기",
		},
		{
			name:  "cjk punctuation bold with exclamation",
			input: "**좋아！**그리고",
			want:  "**좋아！** 그리고",
		},
		{
			name:  "non-punctuation cjk unchanged",
			input: "**기능정리검토**단어",
			want:  "**기능정리검토**단어",
		},
		{
			name:  "english unchanged",
			input: "**bold** text",
			want:  "**bold** text",
		},
		{
			name:  "cjk after opening unchanged",
			input: "앞**굵게**뒤",
			want:  "앞**굵게**뒤",
		},
		{
			name:  "inline code untouched",
			input: "`a**기능정리검토**b`",
			want:  "`a**기능정리검토**b`",
		},
		{
			name:  "fenced code untouched",
			input: "```\n**테스트，**더보기\n```",
			want:  "```\n**테스트，**더보기\n```",
		},
		{
			name:  "code span with cjk punctuation",
			input: "`**안녕，**세계` and **정말，**좋아",
			want:  "`**안녕，**세계` and **정말，** 좋아",
		},
		{
			name:  "multiple emphasis",
			input: "**첫째，**그리고**둘째，**모두",
			want:  "**첫째，** 그리고**둘째，** 모두",
		},
		{
			name:  "cjk punct before opener stays untouched (colon)",
			input: "주의：**중요**사항",
			want:  "주의：**중요**사항",
		},
		{
			name:  "cjk punct before opener stays untouched (comma)",
			input: "그는 말했다，**핵심**은",
			want:  "그는 말했다，**핵심**은",
		},
		{
			name:  "opener after punct, closer after punct",
			input: "그는 말했다：**주의，**그리고",
			want:  "그는 말했다：**주의，** 그리고",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixCJKEmphasis(tt.input)
			if got != tt.want {
				t.Errorf("fixCJKEmphasis(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFixCJKEmphasisRenderIntegration(t *testing.T) {
	r := newMarkdownRenderer(80)

	tests := []struct {
		name     string
		input    string
		wantText string // rendered output must contain this text
	}{
		{
			name:     "cjk punctuation bold renders",
			input:    "**테스트，**더보기",
			wantText: "테스트，",
		},
		{
			name:     "non-punctuation cjk already renders",
			input:    "**기능정리검토**단어",
			wantText: "기능정리검토",
		},
		{
			name:     "inline code preserved",
			input:    "`a**기능정리검토**b`",
			wantText: "a**기능정리검토**b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := r.Render(tt.input)
			if rendered == "" {
				t.Fatal("Render returned empty string")
			}
			if !strings.Contains(rendered, tt.wantText) {
				t.Errorf("rendered output missing %q:\n%s", tt.wantText, rendered)
			}
		})
	}
}

func TestFixCJKEmphasisPunctBeforeOpenerRendersBold(t *testing.T) {
	r := newMarkdownRenderer(80)
	for _, in := range []string{"주의：**중요**사항", "그는 말했다，**핵심**은"} {
		if rendered := r.Render(in); strings.Contains(rendered, "**") {
			t.Errorf("punct before opener left literal ** (not bold):\n%s", rendered)
		}
	}
}

func TestIsCJKPunct(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{',', false}, // ASCII comma
		{'。', true},  // CJK period
		{'，', true},  // CJK comma
		{'！', true},  // CJK exclamation
		{'？', true},  // CJK question
		{'한', false}, // CJK letter
		{'글', false}, // CJK letter
		{'a', false}, // ASCII letter
		{'「', true},  // CJK bracket
		{'」', true},  // CJK bracket
		{'、', true},  // CJK ideographic comma
		{'·', true},  // middle dot
	}
	for _, tt := range tests {
		if got := isCJKPunct(tt.r); got != tt.want {
			t.Errorf("isCJKPunct(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}
