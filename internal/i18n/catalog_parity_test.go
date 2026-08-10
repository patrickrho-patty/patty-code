package i18n

import (
	"reflect"
	"regexp"
	"sort"
	"testing"
)

// translated message dropping (or inventing) a "code token" that its English
//
// - `backtick-quoted spans` — commands the user is told to run
// - leading-slash command tokens — /compact, /language, /init, ...
// - key hints — PgUp/PgDn/Ctrl+Home/End, Shift+Tab, Ctrl+C/Y/D, Esc, arrows
//
// "PgUp/PgDn 스크롤" while en/zh also carry Ctrl+Home/End. Because the check is
//
// The test enumerates fields of the baseline catalogue (Korean) — the same
// Messages type all catalogues use — exactly like TestCatalogsComplete,
// and verifies every other catalogue keeps the same code tokens.
func TestCatalogsAgreeOnCodeTokens(t *testing.T) {
	excluded := map[string]bool{
		"UsageBody":       true,
		"ReportUsageBody": true,
	}

	ko := reflect.ValueOf(Korean)
	typ := ko.Type()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if excluded[name] {
			continue
		}
		want := extractCodeTokens(ko.Field(i).String())
		for _, cat := range []struct {
			tag string
			v   reflect.Value
		}{
			{"en", reflect.ValueOf(English)},
		} {
			got := extractCodeTokens(cat.v.Field(i).String())
			if !equalTokenSets(want, got) {
				t.Errorf("%s (%s): code tokens differ from ko\n  ko: %v\n  %-5s: %v",
					name, cat.tag, sortedTokens(want), cat.tag, sortedTokens(got))
			}
		}
	}
}


var (
	reBacktick = regexp.MustCompile("`([^`]+)`")
// A slash token is a leading-slash word (/init, /resume <n>). Requiring a
// non-word boundary before it keeps enumerations such as "y/a/p/n",
// "drag-select/scrollbar" or "auth/quota" from being misread as commands.
	reSlashCmd = regexp.MustCompile("(?:^|[ \\t\\n\\r(（)·\\[：“\"'`,，;；:：、])(/[A-Za-z][A-Za-z0-9_-]*)")
	reKeyToken = regexp.MustCompile(`\b(?:PgUp|PgDn|Home|End|Esc|Shift\+Tab|Ctrl[-+][A-Za-z]+)\b`)
	reArrow    = regexp.MustCompile(`[↑↓←→]`)
// `patcode run "your task"`   vs   `patcode run "당신의 작업"`
// `patcode remote add <name>` vs   `patcode remote add <이름>`
	reSpanQuoted = regexp.MustCompile(`"[^"]*"`)
	reSpanAngle  = regexp.MustCompile(`<[^>]*>`)
)

func extractCodeTokens(s string) map[string]bool {
	toks := make(map[string]bool)
	for _, m := range reBacktick.FindAllStringSubmatch(s, -1) {
		span := reSpanQuoted.ReplaceAllString(m[1], `"x"`)
		span = reSpanAngle.ReplaceAllString(span, "<x>")
		toks["`"+span+"`"] = true
	}
	for _, m := range reSlashCmd.FindAllStringSubmatch(s, -1) {
		toks[m[1]] = true
	}
	for _, m := range reKeyToken.FindAllString(s, -1) {
		toks[m] = true
	}
	for _, m := range reArrow.FindAllString(s, -1) {
		toks[m] = true
	}
	return toks
}

func equalTokenSets(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func sortedTokens(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
