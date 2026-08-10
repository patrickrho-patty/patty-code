package agent

import "testing"

func TestPreviewProseDropsLeadingFileRefs(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"@src/App.tsx 이 컴포넌트를 분리해 줘", "이 컴포넌트를 분리해 줘"},
		{"@a.ts @b.ts fix the imports", "fix the imports"},
		{"  @src/App.tsx\tcheck this", "check this"},
		{"@__patty_external_folder/mock/notes/ summarise", "summarise"},
		{"no refs here", "no refs here"},
		{"look at @src/App.tsx", "look at @src/App.tsx"},
	} {
		if got := previewProse(tc.in); got != tc.want {
			t.Errorf("previewProse(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPreviewProseKeepsRefOnlyPrompts(t *testing.T) {
	for _, in := range []string{"@src/App.tsx", "@a.ts @b.ts", "@a.ts   "} {
		if got := previewProse(in); got != in {
			t.Errorf("previewProse(%q) = %q, want it unchanged", in, got)
		}
	}
}
