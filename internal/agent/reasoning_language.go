package agent

import (
	"context"
	"strings"
	"unicode"
)

type reasoningLanguageContextKey struct{}
type responseLanguageContextKey struct{}

func NormalizeReasoningLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ko-kr", "ko":
		return "ko-KR"
	case "en", "english":
		return "en"
	default:
		return "auto"
	}
}

func NormalizeResponseLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "ko-kr", "ko":
		return "ko-KR"
	case "en", "english":
		return "en"
	default:
		return "auto"
	}
}

func ResponseLanguageBlock(lang string) string {
	switch NormalizeResponseLanguage(lang) {
	case "ko-KR":
		return "<response-language>\nFinal answer language preference: use Standard Korean for user-facing replies unless the user explicitly asks for another language. Keep code, identifiers, file paths, shell commands, and untranslated technical terms in their original form.\n</response-language>"
	case "en":
		return "<response-language>\nFinal answer language preference: use English for user-facing replies unless the user explicitly asks for another language. Keep code, identifiers, file paths, shell commands, and untranslated technical terms in their original form.\n</response-language>"
	default:
		return ""
	}
}

func ReasoningLanguageBlock(lang string) string {
	switch NormalizeReasoningLanguage(lang) {
	case "ko-KR":
		return "<reasoning-language>\n모든 보이는 사고/추론 텍스트는 반드시 한국어로 작성해야 합니다: 첫 글자부터 한국어를 사용하고, 전체 턴 동안 한국어를 유지해야 합니다. 시스템 프롬프트, 도구 설명, 도구 출력 또는 참조된 코드가 영어인 경우에도 마찬가지입니다. 코드, 식별자, 파일 경로, 셸 명령 및 번역되지 않은 기술 용어는 원문을 유지합니다. 이 요구 사항은 보이는 사고 텍스트에만 적용되며, 사용자가 최종 답변 언어에 대해 명시적으로 요청한 내용을 덮어쓰지 않습니다.\n</reasoning-language>"
	case "en":
		return "<reasoning-language>\nVisible reasoning/thinking text preference: use English when the provider exposes reasoning text. Keep code, identifiers, file paths, shell commands, and untranslated technical terms in their original form. This preference does not override an explicit user request for the final answer language.\n</reasoning-language>"
	default:
		return ""
	}
}

func ResolveReasoningLanguage(lang, source string) string {
	mode := NormalizeReasoningLanguage(lang)
	if mode != "auto" {
		return mode
	}
	return InferReasoningLanguageFromText(source)
}

func InferReasoningLanguageFromText(source string) string {
	source = reasoningLanguageSourceText(source)
	if source == "" {
		return "auto"
	}
	han, cjkPunct := reasoningLanguageScriptCounts(source)
	switch {
	case han >= 4:
		return "ko-KR"
	case han >= 2 && (cjkPunct > 0 || hasChineseReasoningCue(source)):
		return "ko-KR"
	default:
		return "auto"
	}
}

func reasoningLanguageSourceText(source string) string {
	s := strings.TrimSpace(StripTransientUserBlocks(source))
	const preamble = "Referenced context:"
	if !strings.HasPrefix(s, preamble) {
		return s
	}
	s = strings.TrimSpace(s[len(preamble):])
	for {
		s = strings.TrimSpace(s)
		if s == "" || !strings.HasPrefix(s, "<") {
			return s
		}
		tagEnd := strings.IndexAny(s, " >\t\r\n")
		if tagEnd <= 1 {
			return s
		}
		tag := s[1:tagEnd]
		switch tag {
		case "file", "dir", "resource", "image":
			closeTag := "</" + tag + ">"
			i := strings.Index(s, closeTag)
			if i < 0 {
				return s
			}
			s = strings.TrimSpace(s[i+len(closeTag):])
		default:
			return s
		}
	}
}

func reasoningLanguageScriptCounts(source string) (han, cjkPunct int) {
	for _, r := range source {
		switch {
		case unicode.In(r, unicode.Han):
			han++
		case isCJKPunctuation(r):
			cjkPunct++
		}
	}
	return han, cjkPunct
}

func isCJKPunctuation(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F:
		return true
	case r >= 0xFF00 && r <= 0xFFEF:
		return true
	default:
		return false
	}
}

func hasChineseReasoningCue(source string) bool {
	for _, cue := range chineseReasoningLanguageCues {
		if strings.Contains(source, cue) {
			return true
		}
	}
	return false
}

var chineseReasoningLanguageCues = []string{
	"안녕", "부탁", "도와줘", "도움", "봐 줘", "확인해 줘", "설명", "요약", "분석",
	"복구", "구현", "최적화", "추적", "처리", "계속", "왜", "어떻게",
	"여부", "가능한지", "지원", "설정", "사고", "문제", "오류 보고",
	"코드", "파일", "이것", "저것",
}

func reasoningLanguageBlockForSource(lang, source string) string {
	return ReasoningLanguageBlock(ResolveReasoningLanguage(lang, source))
}

func WithResponseLanguage(content, lang string) string {
	block := ResponseLanguageBlock(lang)
	if block == "" || hasLeadingInjectedBlock(content, "response-language") {
		return content
	}
	return block + "\n\n" + content
}

func WithReasoningLanguage(content, lang string) string {
	return WithReasoningLanguageForSource(content, lang, content)
}

func WithReasoningLanguageForSource(content, lang, source string) string {
	block := reasoningLanguageBlockForSource(lang, source)
	if block == "" || hasLeadingInjectedBlock(content, "reasoning-language") {
		return content
	}
	return block + "\n\n" + content
}

func hasLeadingInjectedBlock(content, target string) bool {
	s := strings.TrimLeft(content, " \t\r\n")
	for {
		if hasOpenTag(s, target) {
			return strings.Contains(s, "</"+target+">")
		}
		skipped := false
		for _, tag := range TransientUserBlockTags {
			if tag == target || !hasOpenTag(s, tag) {
				continue
			}
			rest, ok := trimLeadingTransientBlock(s, tag)
			if !ok {
				return false
			}
			s, skipped = rest, true
			break
		}
		if !skipped {
			return false
		}
	}
}

func hasOpenTag(s, tag string) bool {
	return strings.HasPrefix(s, "<"+tag+">") || strings.HasPrefix(s, "<"+tag+" ")
}

func trimLeadingTransientBlock(content, tag string) (string, bool) {
	closeTag := "</" + tag + ">"
	_, after, ok := strings.Cut(content, closeTag)
	if !ok {
		return content, false
	}
	return strings.TrimLeft(after, " \t\r\n"), true
}

func WithResponseLanguagePreference(ctx context.Context, lang string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, responseLanguageContextKey{}, NormalizeResponseLanguage(lang))
}

func ResponseLanguageFromContext(ctx context.Context) string {
	if ctx == nil {
		return "auto"
	}
	if v, ok := ctx.Value(responseLanguageContextKey{}).(string); ok {
		return NormalizeResponseLanguage(v)
	}
	return "auto"
}

func WithReasoningLanguagePreference(ctx context.Context, lang string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, reasoningLanguageContextKey{}, NormalizeReasoningLanguage(lang))
}

func ReasoningLanguageFromContext(ctx context.Context) string {
	if ctx == nil {
		return "auto"
	}
	if v, ok := ctx.Value(reasoningLanguageContextKey{}).(string); ok {
		return NormalizeReasoningLanguage(v)
	}
	return "auto"
}
