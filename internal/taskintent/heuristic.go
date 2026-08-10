package taskintent

import (
	"slices"
	"strings"
	"unicode/utf8"
)

func heuristicInputIsTask(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}

	normalized := strings.ToLower(strings.Trim(trimmed, " \t\r\n.!?。！？,，;；:："))

	shortGreetings := []string{
		"hello", "hi", "hey", "안녕", "안녕하세요", "nihao",
		"thanks", "thank you", "감사", "감사합니다",
		"ok", "okay", "좋습니다", "응", "좋음",
		"got it", "i see", "이해", "이해했어요", "확인", "알겠습니다", "일단 괜찮아요",
	}

	words := strings.Fields(normalized)
	if len(words) <= 3 {
		if slices.Contains(shortGreetings, normalized) {
			return false
		}
	}

	chatPhrases := []string{
		"thanks for", "thank you for", "i'll check later", "i will check later",
		"i'll test it later", "i will test it later", "that test was helpful", "the test was helpful",
		"고마워요", "수고하셨습니다",
	}
	for _, phrase := range chatPhrases {
		before, after, ok := strings.Cut(normalized, phrase)
		if !ok {
			continue
		}
		if prefix := strings.TrimSpace(before); prefix != "" && heuristicInputHasStrongTaskSignal(prefix) {
			return true
		}
		if deliveryTaskHasFollowUpAfterChat(after) {
			return true
		}
		return false
	}
	return heuristicInputHasStrongTaskSignal(normalized)
}

func heuristicInputHasStrongTaskSignal(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if deliveryTaskHasFileReference(normalized) || deliveryTaskHasCommand(normalized) {
		return true
	}
	if deliveryTaskHasMutationIntent(normalized) || NeedsPersistentAction(normalized) {
		return true
	}

	if taskInputHasFaultSignal(normalized) {
		return true
	}
	helpPhrases := []string{
		"can you help", "help with", "cannot", "can't",
		"불가능합니다", "할 수 없습니다",
	}
	for _, phrase := range helpPhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}

	actionNeedles := []string{
		"fix", "debug", "repair", "resolve", "reproduce",
		"create", "add", "write", "edit", "update", "change", "delete", "remove", "rename",
		"review", "inspect", "analyze", "check", "audit", "verify", "test", "run", "build", "implement", "refactor", "modify", "patch", "replace",
		"configure", "upgrade", "downgrade", "enable", "disable", "merge", "make changes", "make a change", "make the changes",
		"make the requested changes", "make the necessary changes", "make these changes", "make those changes", "make code changes",
		"continue work", "continue the", "continue this",
		"복구", "디버깅", "해겵", "재현", "생성하기", "새로 만들기", "추가", "사작", "편집", "수정", "업데이트",
		"삭제", "제거", "이름 재정의", "검토", "검사", "분석", "감사", "검증", "테스트", "실행", "빌드", "구현", "리팩토링", "계속처리",
		"조정", "바꿔기", "이동", "액스레이제이션", "다웄그레이제이션", "확성화", "비확성화", "벵합", "변경", "패치",
		"봐주세요", "한번 봐주세요", "보여주세요", "처리해 주세요", "추적", "위치 파악",
	}

	for _, needle := range actionNeedles {
		if containsTaskNeedle(normalized, needle) {
			return true
		}
	}

	return false
}

func deliveryTaskHasFollowUpAfterChat(input string) bool {
	for index, current := range input {
		switch current {
		case '.', ',', ';', '!', '?', '。', '，', '；', '！', '？':
			candidate := strings.TrimSpace(input[index+utf8.RuneLen(current):])
			if candidate != "" && heuristicInputHasStrongTaskSignal(candidate) {
				return true
			}
		}
	}
	for _, cue := range []string{
		" but ", " however ", " nevertheless ", " now ", " then ", " and ", " please ", " so ", " therefore ",
		"하지만", "하지만 부탁드립니다", "그런데", "지금", "그다음", "그래서", "부탁드립니다", "계속", "다시",
	} {
		for rest := input; ; {
			index := strings.Index(rest, cue)
			if index < 0 {
				break
			}
			candidate := strings.TrimSpace(rest[index+len(cue):])
			if candidate != "" && heuristicInputHasStrongTaskSignal(candidate) {
				return true
			}
			rest = rest[index+len(cue):]
		}
	}
	return false
}

func containsTaskNeedle(input, needle string) bool {
	if needle == "" {
		return false
	}
	if containsNonASCII(needle) || strings.Contains(needle, " ") {
		return strings.Contains(input, needle)
	}
	return slices.Contains(strings.FieldsFunc(input, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_'
	}), needle)
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}
