package agent

import "testing"

func TestReasoningLanguageDirectiveIsNotUserAuthored(t *testing.T) {
	for _, injected := range []string{
		"<reasoning-language>\nUse Standard Korean",
		"<reasoning-language>\nUse Standard Korean\n</reasoning-language>",
	} {
		if IsUserAuthoredTurn(injected) {
			t.Fatalf("reasoning-language directive %q must not count as user-authored", injected)
		}
	}
}

func TestStripPasteDisplayLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "standard Korean", in: "[붙여넣은 텍스트 #1 · 100 줄]\npackage main", want: "package main"},
		{name: "traditional Korean", in: "[붙여넣은 텍스트 #2 · 5 줄]\r\n이 코드 좀 봐 줘", want: "이 코드 좀 봐 줘"},
		{name: "English", in: "[Pasted text #3 · 42 lines]\nfunc foo() {}", want: "func foo() {}"},
		{name: "inline mention", in: "Explain [Pasted text #1 · 2 lines] handling", want: "Explain [Pasted text #1 · 2 lines] handling"},
		{name: "later standalone line", in: "Keep this\n[Pasted text #1 · 2 lines]\nverbatim", want: "Keep this\n[Pasted text #1 · 2 lines]\nverbatim"},
		{name: "no label", in: "debug the login issue", want: "debug the login issue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripPasteDisplayLabel(tt.in); got != tt.want {
				t.Fatalf("StripPasteDisplayLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
