package taskintent

import "testing"

func TestGoalNeedsWriteBudgetMatrix(t *testing.T) {
	const userReported = "데이터 모델 관리자에 또 이전 버그가 생겼어…"
	if !GoalNeedsWriteBudget(userReported) {
		t.Fatalf("GoalNeedsWriteBudget(%q) = false, want write", userReported)
	}
	// (observable read / evidence, not write-required).
	if NeedsMutation(userReported) {
		t.Fatalf("NeedsMutation(%q) changed; ordinary Delivery must stay non-mutation", userReported)
	}

	writeCases := []string{
		userReported,
		"설정을 열 때 앱이 충돌해",
		"the auth service crashes on login",
		"parser throws an exception on empty input",
		"fix the crash in a.go",
		"wps의 충돌 문제를 수정해 줘",
		"why does it fail and fix it",
		"왜 실패했는지, 수정해 줘",
		"실패 원인을 설명하고 수정해 줘",
	}
	for _, input := range writeCases {
		if !GoalNeedsWriteBudget(input) {
			t.Errorf("Goal write budget missing for %q", input)
		}
	}

	simpleCases := []string{
		"왜 이 버그가 생기는 거야?",
		"원인만 분석하고, 코드는 건드리지 마세요.",
		"데이터베이스 연결 실패 원인을 진단해 줘.",
		"문제를 재현하고 위치를 파악하되, 복구하지 마세요.",
		"why does this bug happen?",
		"explain the crash without changing code",
		"reproduce the crash and identify the root cause",
		"review only and do not fix anything",
		"hello",
	}
	for _, input := range simpleCases {
		if GoalNeedsWriteBudget(input) {
			t.Errorf("Goal simple budget expected for %q", input)
		}
	}
}

func TestGoalBareFaultDoesNotChangeDeliveryClassification(t *testing.T) {
	// Pin ordinary Delivery consultation/diagnosis so Goal write inference
	readonly := []string{
		"왜 이 버그가 생기는 거야?",
		"데이터베이스 연결 실패 원인을 진단해 줘.",
		"reproduce the crash and identify the root cause",
		"설정을 열 때 앱이 충돌해",
		"데이터 모델 관리자에 또 이전 버그가 생겼어…",
	}
	for _, input := range readonly {
		if NeedsMutation(input) {
			t.Errorf("delivery mutation incorrectly true for %q", input)
		}
	}
	mutation := []string{
		"fix the crash in a.go",
		"왜 실패했는지, 수정해 줘",
	}
	for _, input := range mutation {
		if !NeedsMutation(input) {
			t.Errorf("delivery mutation incorrectly false for %q", input)
		}
	}
}

func TestTaskFaultSignalsSharedWithGoalClassification(t *testing.T) {
	for _, phrase := range []string{"bug", "crash", "충돌", "이상", "오류 발생", "실패"} {
		input := "something " + phrase + " happened"
		if !taskInputHasFaultSignal(input) {
			t.Errorf("taskInputHasFaultSignal missing %q", phrase)
		}
		if !heuristicInputIsTask(input) && !stringsContainsCJK(phrase) {
			if !heuristicInputIsTask("the service has a " + phrase) {
				t.Errorf("heuristic task signal missing for fault %q", phrase)
			}
		}
	}
}

func stringsContainsCJK(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}
