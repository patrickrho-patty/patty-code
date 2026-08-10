package taskintent

import (
	"strings"
)

var taskFaultSignals = []string{
	"not working", "isn't working", "doesn't work", "dont work", "don't work",
	"broken", "error", "bug", "issue", "failed", "failing", "crash", "exception", "panic", "segfault",
	"문제", "작동 안 함", "오류 발생", "오류", "실패", "충돌", "이상",
	"멈춤", "멈췄어요", "반응 없음", "적용 안 됨", "비정상 종료", "이전 버그", "이전버그",
}

var taskFaultReadonlySignals = []string{
	"diagnose", "diagnosis", "analyze", "analyse", "inspect", "review", "reproduce",
	"identify the root cause", "root cause", "investigate", "audit", "verify", "check why",
	"진단", "추적", "위치 파악", "분석", "검사", "재현", "검토", "검토", "감사", "검증",
	"원인", "근본 원인",
}

func GoalNeedsWriteBudget(input string) bool {
	if NeedsMutation(input) {
		return true
	}
	return goalBareFaultImpliesWrite(input)
}

func goalBareFaultImpliesWrite(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" || !taskInputHasFaultSignal(normalized) {
		return false
	}
	if goalHasNoModifyConstraint(normalized) {
		return false
	}
	if deliveryTaskIsAdvisory(normalized) || goalHasQuestionIntent(normalized) {
		return false
	}
	if goalHasReadonlyDiagnosticIntent(normalized) {
		return false
	}
	return true
}

func taskInputHasFaultSignal(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	for _, phrase := range taskFaultSignals {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func goalHasQuestionIntent(input string) bool {
	if strings.ContainsAny(input, "?？") {
		return true
	}
	for _, phrase := range []string{
		"explain", "what happened", "what's happening", "what is happening",
		"tell me about", "look into why",
		"설명", "설명해 주세요", "무슨 일", "어떤 상황", "무슨 상황",
	} {
		if strings.Contains(input, phrase) {
			return true
		}
	}
	return false
}

func goalHasReadonlyDiagnosticIntent(input string) bool {
	for _, phrase := range taskFaultReadonlySignals {
		if strings.Contains(input, phrase) {
			return true
		}
	}
	return deliveryTaskIsAdvisory(input)
}

func goalHasNoModifyConstraint(input string) bool {
	if _, negated := deliveryTaskMutationIntent(input); negated && !deliveryTaskHasMutationIntent(input) {
		return true
	}
	for _, phrase := range []string{
		"do not fix", "don't fix", "dont fix", "do not change", "don't change", "dont change",
		"do not modify", "don't modify", "dont modify", "do not edit", "don't edit",
		"without fixing", "without changing", "without modifying", "analysis only", "review only",
		"복구하지 마세요", "수정하지 마세요", "변경하지 마세요", "고치지 마세요", "복구하지 말아 주세요", "수정하지 말아 주세요", "수정 금지",
		"분석만", "오직 분석만", "검사만", "오직 검사만", "진단만", "오직 진단만", "위치 파악만", "오직 위치 파악만",
		"추적만", "오직 추적만", "수리하지 마세요",
	} {
		if strings.Contains(input, phrase) {
			return true
		}
	}
	for _, clause := range deliveryTaskClauses(input) {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		for _, phrase := range []string{"복구하지 마세요", "수정하지 마세요", "do not fix", "don't fix", "dont fix"} {
			if strings.Contains(clause, phrase) {
				return true
			}
		}
	}
	return false
}
