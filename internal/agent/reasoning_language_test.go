package agent

import (
	"strings"
	"testing"
)

func TestWithResponseLanguageOnlySkipsLeadingInjectedBlock(t *testing.T) {
	userMention := "explain why <response-language> appears in this file"
	got := WithResponseLanguage(userMention, "en")
	if !strings.HasPrefix(got, "<response-language>") || !strings.Contains(got, "use English") || !strings.HasSuffix(got, userMention) {
		t.Fatalf("WithResponseLanguage should prefix user-authored tag mentions, got %q", got)
	}

	alreadyPrefixed := ResponseLanguageBlock("en") + "\n\n" + userMention
	if got := WithResponseLanguage(alreadyPrefixed, "en"); got != alreadyPrefixed {
		t.Fatalf("WithResponseLanguage duplicated a leading injected block:\n got %q\nwant %q", got, alreadyPrefixed)
	}

	withLeadingMemory := "<memory-update>\nRemember this.\n</memory-update>\n\n" + alreadyPrefixed
	if got := WithResponseLanguage(withLeadingMemory, "en"); got != withLeadingMemory {
		t.Fatalf("WithResponseLanguage duplicated a response block after leading transient context:\n got %q\nwant %q", got, withLeadingMemory)
	}
}

func TestWithReasoningLanguageOnlySkipsLeadingInjectedBlock(t *testing.T) {
	userMention := "explain why <reasoning-language> appears in this file"
	got := WithReasoningLanguage(userMention, "ko-KR")
	if !strings.HasPrefix(got, "<reasoning-language>") || !strings.Contains(got, "한국어") || !strings.HasSuffix(got, userMention) {
		t.Fatalf("WithReasoningLanguage should prefix user-authored tag mentions, got %q", got)
	}

	alreadyPrefixed := ReasoningLanguageBlock("ko-KR") + "\n\n" + userMention
	if got := WithReasoningLanguage(alreadyPrefixed, "ko-KR"); got != alreadyPrefixed {
		t.Fatalf("WithReasoningLanguage duplicated a leading injected block:\n got %q\nwant %q", got, alreadyPrefixed)
	}

	withLeadingMemory := "<memory-update>\nRemember this.\n</memory-update>\n\n" + alreadyPrefixed
	if got := WithReasoningLanguage(withLeadingMemory, "ko-KR"); got != withLeadingMemory {
		t.Fatalf("WithReasoningLanguage duplicated a reasoning block after leading transient context:\n got %q\nwant %q", got, withLeadingMemory)
	}
}

func TestReasoningLanguageBlockKoreanStaysImperative(t *testing.T) {
	// The imperative form measurably outperforms soft "선호" phrasing on
	// Korean prompts that embed English logs/code; keep it from regressing.
	block := ReasoningLanguageBlock("ko-KR")
	for _, want := range []string{"반드시 한국어로 작성해야 합니다", "전체 턴", "사용자가 최종 답변 언어에 대해 명시적으로 요청한 내용을 덮어쓰지 않습니다"} {
		if !strings.Contains(block, want) {
			t.Fatalf("zh reasoning block lost required anchor %q:\n%s", want, block)
		}
	}
}

func TestWithReasoningLanguageAutoInfersFromSource(t *testing.T) {
	korean := WithReasoningLanguage("AuthHandler의 panic을 설명해 주세요", "auto")
	if !strings.HasPrefix(korean, "<reasoning-language>") || !strings.Contains(korean, "한국어로 작성해야 합니다") || !strings.HasSuffix(korean, "AuthHandler의 panic을 설명해 주세요") {
		t.Fatalf("auto reasoning language should infer Korean from a Hangul prompt, got %q", korean)
	}

	english := WithReasoningLanguage("explain this module", "auto")
	if english != "explain this module" {
		t.Fatalf("auto reasoning language should keep English prompts unwrapped, got %q", english)
	}

	short := WithReasoningLanguage("hi", "auto")
	if short != "hi" {
		t.Fatalf("short ambiguous auto prompt should not be wrapped, got %q", short)
	}
}

func TestWithReasoningLanguageAutoUsesRawSourceOverReferencedContext(t *testing.T) {
	expanded := "Referenced context:\n\n<file path=\"auth.go\">\npackage main\nfunc AuthHandler() error { return errors.New(\"not authorized\") }\n</file>\n\n@auth.go의 오류를 설명해 주세요"

	got := WithReasoningLanguageForSource(expanded, "auto", "@auth.go의 오류를 설명해 주세요")
	if !strings.HasPrefix(got, "<reasoning-language>") || !strings.Contains(got, "한국어로 작성해야 합니다") || !strings.HasSuffix(got, expanded) {
		t.Fatalf("auto reasoning language should infer Korean from the raw Hangul prompt, got %q", got)
	}
	if strings.Contains(got, "use English") {
		t.Fatalf("referenced English code should not make auto prefer English:\n%s", got)
	}
}
