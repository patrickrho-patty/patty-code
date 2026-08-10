package control

import (
	"context"
	"testing"

	"patty/internal/agent"
)

func TestTaskWarrantsPlanner(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"   ", false},
		{"/init", false},
		{"1", false},
		{"2.", false},
		{"A", false},
		{"좋습니다", false},
		{"계속", false},
		{"선택 1", false},
		{"what does this function do?", false}, // low-risk question → executor only
		{"why did the test fail", false},
		{"이 코드 좀 설명해 줘", false},
		{reasoningLanguageBlock("ko-KR") + "\n\nwhat does this function do?", false},
		{reasoningLanguageBlock("en") + "\n\n" + PlanModeMarker + "\n\nfix the bug", false},
		{reasoningLanguageBlock("en") + "\n\nfix the bug", true},
		{"fix the bug", true},        // terse, but a work request → still planned
		{"add a login button", true}, // ditto
		{"run the tests", false},
		{"review this PR", false},
		{"inspect internal/foo.go", false},
		{"수정 실행", true},
		{"마이그레이션 시작", true},
		{"리팩토링 계속", true},
		{"continue fixing tests", true},
		{"implement the new caching layer across the backend", true},
		{"who wrote this file?", false},
		{"where is the config file?", false},
		{"when does this run?", false},
		{"which file has the error?", false},
		{"explain this code", false},
		{"describe the architecture", false},
		{"tell me about this function", false},
		{"is this safe?", false},
		{"are we done?", false},
		{"can you help?", false},
		{"can you fix the failing tests across the backend", true},
		{"could you update the README", false}, // explicit single-target edit → executor only
		{"should we remove the stale config option", true},
		{"would you add a regression test", true},
		{"do you fix flaky tests here", true},
		{"does it work?", false},
		{"did the test pass?", false},
		{"should I use mutex here?", false},
		{"would this approach work?", false},
		{"list all the endpoints", false},
		{"summarize the changes", false},
		{"compare these two approaches", false},
		{"what's the status?", false},
		{"소개해 주세요", false},
		{"말해 주세요, 이 함수의 역할", false},
		{"이 오류 좀 봐주세요", false},
		{"무슨 뜻이야", false},
		{"기존 방안이 있나요", false},
		{"이렇게 해도 될까", false},
		{"이거 어떻게 쓰는 거예요", false},
		{"how do I implement a new caching layer", true},
		{"what's the best way to refactor this module", true},
		{"explain how to migrate from v1 to v2", true},
		{goalContinueTurn, false},
		{"Goal signaled complete but issues remain:\n- the following tasks are still incomplete:\n  - Fix login (in_progress)\nFix or use todo_write/complete_step to mark done, then report complete again via update_goal.", false},
		{activeGoalBlock("execute plan: fix the parser", GoalResearchAuto) + "\n\n" + goalContinueTurn, false},
		{activeGoalBlock("implement the new caching layer", GoalResearchAuto) + "\n\nimplement the new caching layer across the backend", true},
	}
	for _, c := range cases {
		if got := TaskWarrantsPlanner(c.input); got != c.want {
			t.Errorf("TaskWarrantsPlanner(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestNewPlannerGateUsesDeterministicTaskPolicy(t *testing.T) {
	gate := NewPlannerGate()
	if gate == nil {
		t.Fatal("NewPlannerGate returned nil")
	}
	if got := gate(context.Background(), "what is this?"); got {
		t.Error("planner gate should skip low-risk questions")
	}
	if got := gate(context.Background(), "fix the bug"); !got {
		t.Error("planner gate should plan work requests")
	}
}

func TestDecidePlannerRouteMatrix(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		meta   plannerTurnMetadata
		route  agent.PlannerRoute
		depth  agent.PlannerDepth
		reason string
	}{
		{
			name:   "explicit plan mode bypasses dual planner",
			input:  "fix the bug",
			meta:   plannerTurnMetadata{ExplicitPlanMode: true},
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonExplicitPlanMode,
		},
		{
			name:   "trusted synthetic turn bypasses dual planner",
			input:  "perform a brand new implementation",
			meta:   plannerTurnMetadata{Synthetic: true},
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonSynthetic,
		},
		{
			name:   "user asks for plan only",
			input:  "먼저 이 인증 마이그레이션을 계획하고, 실행하지 마세요",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "bare english plan only",
			input:  "Give me a plan only for the auth migration.",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "bare chinese plan only with embedded target",
			input:  "오직 방안만 주세요",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "task first english plan only boundary",
			input:  "Review the auth migration and give me a plan only; do not execute.",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "task first english plan only without redundant no execution",
			input:  "Review the auth migration and give me a plan only.",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "work request with no execution boundary",
			input:  "Implement the auth migration, but do not execute.",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "task first chinese plan only boundary",
			input:  "인증 마이그레이션을 검토하고, 방안만 주세요. 코드는 수정하지 마세요",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "bare plan first continues to executor",
			input:  "먼저 계획을 세우고 이 인증 마이그레이션을 진행해 줘",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanAndExecute,
		},
		{
			name:   "english plan first continues to executor",
			input:  "plan first, then handle the authentication migration",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanAndExecute,
		},
		{
			name:   "user asks to approve plan before execution",
			input:  "먼저 계획을 세우고, 제가 확인한 후에 실행해 줘",
			route:  agent.PlannerRoutePlanForApproval,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanApproval,
		},
		{
			name:   "task first english approval boundary",
			input:  "Implement the auth migration, but show me the plan and wait for my approval.",
			route:  agent.PlannerRoutePlanForApproval,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanApproval,
		},
		{
			name:   "work request with approval boundary",
			input:  "Implement the auth migration and wait for my approval.",
			route:  agent.PlannerRoutePlanForApproval,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanApproval,
		},
		{
			name:   "task first chinese approval boundary",
			input:  "인증 마이그레이션을 구현하되, 먼저 방안을 주고 제가 확인할 때까지 기다려 주세요",
			route:  agent.PlannerRoutePlanForApproval,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanApproval,
		},
		{
			name:   "conditional no execution remains approval boundary",
			input:  "Plan the auth migration and do not execute until I approve.",
			route:  agent.PlannerRoutePlanForApproval,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanApproval,
		},
		{
			name:   "user asks to plan then execute",
			input:  "먼저 계획한 후 이 인증 마이그레이션을 실행해 줘",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanAndExecute,
		},
		{
			name:   "user explicitly skips planner",
			input:  "auth.go를 바로 수정하고, 계획하지 마세요",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonUserDirect,
		},
		{
			name:   "task first english direct execution boundary",
			input:  "Implement the auth migration, but do not plan.",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonUserDirect,
		},
		{
			name:   "do not plan to is a scope constraint not a routing directive",
			input:  "Plan the auth migration, but do not plan to change the database schema.",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonComplexIntent,
		},
		{
			name:   "task first chinese direct execution boundary",
			input:  "인증 마이그레이션을 구현하고, 계획하지 말고, 바로 수정해 줘",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonUserDirect,
		},
		{
			name:   "quoted directive is not an override",
			input:  "설명해 주세요, '바로 고쳐'가 무슨 뜻인지",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonLowRiskQuestion,
		},
		{
			name:   "quoted no execution phrase is not a boundary",
			input:  "설명해 주세요, '하지 마'가 무슨 뜻인지",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonLowRiskQuestion,
		},
		{
			name:   "quoted english approval phrase is not a boundary",
			input:  "Explain what \"wait for my approval\" means.",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonLowRiskQuestion,
		},
		{
			name:   "single quoted english boundary is not an override",
			input:  "Review the README wording 'wait for my approval'.",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonAtomicEdit,
		},
		{
			name:   "apostrophe in contraction remains a boundary",
			input:  "Review the README but don't execute.",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "expanded approval negation is not an approval boundary",
			input:  "Plan the migration; you do not need to wait for my approval.",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonComplexIntent,
		},
		{
			name:   "negated approval does not override plan only",
			input:  "인증 마이그레이션 방안만 주세요, 실행하지 마세요, 제가 확인할 때까지 기다릴 필요도 없어요",
			route:  agent.PlannerRoutePlanOnly,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonUserPlanOnly,
		},
		{
			name:   "context dependent fix stays with executor",
			input:  "fix it",
			meta:   plannerTurnMetadata{HasConversationContext: true},
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonContextContinuation,
		},
		{
			name:   "standalone context dependent fix remains ambiguous",
			input:  "fix it",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthLight,
			reason: plannerReasonWorkRequest,
		},
		{
			name:   "atomic readme edit skips planner",
			input:  "fix typo in README",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonAtomicEdit,
		},
		{
			name:   "atomic nil check skips planner",
			input:  "add a nil check in internal/foo.go",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonAtomicEdit,
		},
		{
			name:   "bounded anchored work gets light plan",
			input:  "add regression coverage in internal/foo_test.go",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthLight,
			reason: plannerReasonAnchoredWork,
		},
		{
			name:   "single step read only command skips planner",
			input:  "run the tests",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonReadOnlyAction,
		},
		{
			name:   "single file inspection skips planner",
			input:  "inspect internal/foo.go",
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonReadOnlyAction,
		},
		{
			name:   "delivery keeps pure read only command direct",
			input:  "review this PR",
			meta:   plannerTurnMetadata{DeliveryProfile: true},
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonReadOnlyAction,
		},
		{
			name:   "high risk read only audit still gets full plan",
			input:  "audit the authentication authorization flow",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonHighRisk,
		},
		{
			name:   "ambiguous bug gets full plan",
			input:  "fix the bug",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonAmbiguousWork,
		},
		{
			name:   "anchored ambiguous bug still gets full plan",
			input:  "fix the bug in internal/foo.go",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonAmbiguousWork,
		},
		{
			name:   "high risk overrides single target",
			input:  "fix the token authorization race in auth.go",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonHighRisk,
		},
		{
			name:   "cross surface gets full plan",
			input:  "update frontend and backend for the new profile field",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonCrossSurface,
		},
		{
			name:   "complex guidance gets light plan",
			input:  "how do I implement a local cache?",
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthLight,
			reason: plannerReasonGuidance,
		},
		{
			name:   "delivery upgrades non atomic work",
			input:  "add a login button",
			meta:   plannerTurnMetadata{DeliveryProfile: true},
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonWorkRequest,
		},
		{
			name:   "delivery keeps atomic edit direct",
			input:  "fix typo in README",
			meta:   plannerTurnMetadata{DeliveryProfile: true},
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonAtomicEdit,
		},
		{
			name:   "active goal upgrades non atomic work",
			input:  "add a login button",
			meta:   plannerTurnMetadata{GoalActive: true},
			route:  agent.PlannerRoutePlanAndExecute,
			depth:  agent.PlannerDepthFull,
			reason: plannerReasonGoalActive,
		},
		{
			name:   "active goal alone does not force an atomic edit through planner",
			input:  "fix typo in README",
			meta:   plannerTurnMetadata{GoalActive: true},
			route:  agent.PlannerRouteExecutorOnly,
			depth:  agent.PlannerDepthNone,
			reason: plannerReasonAtomicEdit,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := withPlannerTurnMetadata(context.Background(), tc.meta)
			got := DecidePlannerRoute(ctx, tc.input)
			if got.Route != tc.route || got.Depth != tc.depth || got.Reason != tc.reason {
				t.Fatalf("decision = %+v, want route=%s depth=%s reason=%s", got, tc.route, tc.depth, tc.reason)
			}
			if got.Route != agent.PlannerRouteExecutorOnly && got.MaxResearchRounds <= 0 {
				t.Fatalf("planned decision has no research budget: %+v", got)
			}
		})
	}
}

func TestPlannerPolicyUsesPristineMetadataInsteadOfInjectedContext(t *testing.T) {
	ctx := withPlannerTurnMetadata(context.Background(), plannerTurnMetadata{
		UserText: "fix typo in README",
	})
	input := activeGoalBlock("migrate authentication across the backend", GoalResearchAuto) +
		"\n\n<capability-route>\nhigh risk migration\n</capability-route>\n\nfix typo in README"
	got := DecidePlannerRoute(ctx, input)
	if got.Route != agent.PlannerRouteExecutorOnly || got.Reason != plannerReasonAtomicEdit {
		t.Fatalf("decision used injected context instead of pristine user text: %+v", got)
	}
}
