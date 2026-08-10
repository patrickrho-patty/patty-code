package control

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"patty/internal/agent"
	"patty/internal/capability"
)

const (
	plannerLightResearchRounds = 2
	plannerFullResearchRounds  = 6
)

const (
	plannerReasonExplicitPlanMode    = "explicit_plan_mode"
	plannerReasonSynthetic           = "synthetic"
	plannerReasonSlash               = "slash_command"
	plannerReasonShortReply          = "short_reply"
	plannerReasonConversation        = "conversation"
	plannerReasonUserDirect          = "user_direct"
	plannerReasonUserPlanOnly        = "user_plan_only"
	plannerReasonUserPlanApproval    = "user_plan_for_approval"
	plannerReasonUserPlanAndExecute  = "user_plan_and_execute"
	plannerReasonContextContinuation = "context_continuation"
	plannerReasonLowRiskQuestion     = "low_risk_question"
	plannerReasonHighRisk            = "high_risk"
	plannerReasonCrossSurface        = "cross_surface"
	plannerReasonStructuredRequest   = "structured_request"
	plannerReasonComplexIntent       = "complex_intent"
	plannerReasonAtomicEdit          = "atomic_edit"
	plannerReasonReadOnlyAction      = "read_only_action"
	plannerReasonGuidance            = "complex_guidance"
	plannerReasonGoalActive          = "goal_active"
	plannerReasonAnchoredWork        = "anchored_work"
	plannerReasonAmbiguousWork       = "ambiguous_work"
	plannerReasonWorkRequest         = "work_request"
	plannerReasonDefault             = "default_executor"
)

var (
	directOptionReplyRE   = regexp.MustCompile(`(?i)^\s*(?:\d+|[a-z])\s*[.)、。]?\s*$`)
	prefixedOptionReplyRE = regexp.MustCompile(`(?i)^\s*(?:)\s*(?:\d+|[일이삼사오육칠팔구십]|[a-z])\s*(?:번째|번|개|방안|옵션|option|choice)?\s*[.)、。!！?？]?\s*$`)
	plannerListRE         = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+[.)、])\s+\S`)
	plannerFileRefRE      = regexp.MustCompile(`(?i)(?:^|[\s@` + "`" + `"'(])(?:[\w.-]+[/\\])*[\w.-]+\.(?:go|ts|tsx|js|jsx|py|rs|java|kt|md|json|ya?ml|toml|sql|sh|css|html)(?:$|[\s,;:!?，；：！？)` + "`" + `"'])`)
)

type plannerTurnMetadata struct {
	UserText               string
	Synthetic              bool
	ExplicitPlanMode       bool
	GoalActive             bool
	DeliveryProfile        bool
	HasConversationContext bool
}

type plannerTurnMetadataKey struct{}

func withPlannerTurnMetadata(ctx context.Context, meta plannerTurnMetadata) context.Context {
	return context.WithValue(ctx, plannerTurnMetadataKey{}, meta)
}

func plannerTurnMetadataFromContext(ctx context.Context) (plannerTurnMetadata, bool) {
	if ctx == nil {
		return plannerTurnMetadata{}, false
	}
	meta, ok := ctx.Value(plannerTurnMetadataKey{}).(plannerTurnMetadata)
	return meta, ok
}

func (c *Controller) withPlannerTurnMetadata(ctx context.Context, userText string, synthetic bool, priorMessages int) context.Context {
	return withPlannerTurnMetadata(ctx, plannerTurnMetadata{
		UserText:               userText,
		Synthetic:              synthetic,
		ExplicitPlanMode:       c.PlanMode(),
		GoalActive:             c.goals.active(),
		DeliveryProfile:        c.runtimeProfile == capability.ProfileDelivery,
		HasConversationContext: priorMessages > 1,
	})
}

func DecidePlannerRoute(ctx context.Context, input string) agent.PlannerDecision {
	meta, hasMeta := plannerTurnMetadataFromContext(ctx)
	composedText := strings.TrimSpace(agent.StripTransientUserBlocks(input))
	text := composedText
	if hasMeta && strings.TrimSpace(meta.UserText) != "" {
		text = strings.TrimSpace(meta.UserText)
	}

	if meta.ExplicitPlanMode || strings.HasPrefix(composedText, PlanModeMarker) {
		return plannerExecutorDecision(plannerReasonExplicitPlanMode)
	}
	if meta.Synthetic || IsSyntheticUserMessage(text) {
		return plannerExecutorDecision(plannerReasonSynthetic)
	}
	if text == "" {
		return plannerExecutorDecision(plannerReasonConversation)
	}
	if strings.HasPrefix(text, "/") {
		return plannerExecutorDecision(plannerReasonSlash)
	}
	if isContextDependentShortReply(text) {
		return plannerExecutorDecision(plannerReasonShortReply)
	}
	if isConversationalTurn(text) {
		return plannerExecutorDecision(plannerReasonConversation)
	}

	lower := normalizePlannerText(text)
	if requestsPlanApproval(lower) {
		return plannerPlanDecision(agent.PlannerRoutePlanForApproval, agent.PlannerDepthFull, plannerReasonUserPlanApproval)
	}
	if requestsPlanOnly(lower) {
		return plannerPlanDecision(agent.PlannerRoutePlanOnly, agent.PlannerDepthFull, plannerReasonUserPlanOnly)
	}
	if hasLeadingDirective(lower, planAndExecuteDirectives) || hasLeadingDirective(lower, planFirstDirectives) {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonUserPlanAndExecute)
	}
	if requestsDirectExecution(lower) {
		return plannerExecutorDecision(plannerReasonUserDirect)
	}
	if meta.HasConversationContext && isContextDependentAction(text) {
		return plannerExecutorDecision(plannerReasonContextContinuation)
	}
	if isLowRiskQuestion(lower) {
		return plannerExecutorDecision(plannerReasonLowRiskQuestion)
	}

	features := plannerFeaturesFor(text, lower)
	if features.work && features.highRisk {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonHighRisk)
	}
	if features.multiFile || features.crossSurface {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonCrossSurface)
	}
	if features.structured {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonStructuredRequest)
	}
	if features.complex {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonComplexIntent)
	}
	if features.atomic {
		return plannerExecutorDecision(plannerReasonAtomicEdit)
	}
	if features.guidance {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthLight, plannerReasonGuidance)
	}
	if features.readOnly && !features.ambiguous {
		return plannerExecutorDecision(plannerReasonReadOnlyAction)
	}
	if meta.GoalActive && features.work {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonGoalActive)
	}
	if meta.DeliveryProfile && features.work {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonWorkRequest)
	}
	if features.work && features.ambiguous {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthFull, plannerReasonAmbiguousWork)
	}
	if features.work && features.anchored {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthLight, plannerReasonAnchoredWork)
	}
	if features.work {
		return plannerPlanDecision(agent.PlannerRoutePlanAndExecute, agent.PlannerDepthLight, plannerReasonWorkRequest)
	}
	return plannerExecutorDecision(plannerReasonDefault)
}

func plannerExecutorDecision(reason string) agent.PlannerDecision {
	return agent.PlannerDecision{
		Route:  agent.PlannerRouteExecutorOnly,
		Depth:  agent.PlannerDepthNone,
		Reason: reason,
	}
}

func plannerPlanDecision(route agent.PlannerRoute, depth agent.PlannerDepth, reason string) agent.PlannerDecision {
	rounds := plannerLightResearchRounds
	if depth == agent.PlannerDepthFull {
		rounds = plannerFullResearchRounds
	}
	return agent.PlannerDecision{
		Route:             route,
		Depth:             depth,
		Reason:            reason,
		MaxResearchRounds: rounds,
	}
}

type plannerFeatures struct {
	work         bool
	highRisk     bool
	multiFile    bool
	crossSurface bool
	structured   bool
	complex      bool
	atomic       bool
	readOnly     bool
	guidance     bool
	anchored     bool
	ambiguous    bool
}

func plannerFeaturesFor(text, lower string) plannerFeatures {
	fileRefs := plannerFileRefRE.FindAllString(text, -1)
	anchored := len(fileRefs) > 0 || strings.Contains(text, "@") || containsAnyLexical(lower, plannerNamedTargets)
	work := containsAnyLexical(lower, plannerWorkTerms)
	highRisk := containsAnyLexical(lower, plannerHighRiskTerms)
	multiFile := len(fileRefs) >= 2 || strings.Count(text, "@") >= 2
	crossSurface := containsAnyLexical(lower, plannerCrossSurfaceTerms)
	structured := utf8.RuneCountInString(text) >= 240 || plannerListRE.MatchString(text) || strings.Count(text, "\n") >= 2
	complex := containsAnyLexical(lower, complexIntentTerms)
	guidance := isComplexGuidanceQuestion(lower)
	ambiguous := work && containsAnyLexical(lower, plannerAmbiguousScopeTerms)
	readOnly := work && containsAnyLexical(lower, plannerReadOnlyWorkTerms) &&
		!containsAnyLexical(lower, plannerMutationWorkTerms)
	atomic := work && anchored && !highRisk && !multiFile && !crossSurface && !structured && !complex &&
		utf8.RuneCountInString(text) <= 140 && containsAnyLexical(lower, plannerAtomicTerms)
	return plannerFeatures{
		work:         work,
		highRisk:     highRisk,
		multiFile:    multiFile,
		crossSurface: crossSurface,
		structured:   structured,
		complex:      complex,
		atomic:       atomic,
		readOnly:     readOnly,
		guidance:     guidance,
		anchored:     anchored,
		ambiguous:    ambiguous,
	}
}

func normalizePlannerText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.ReplaceAll(text, "’", "'")
	return text
}

func hasLeadingDirective(lower string, directives []string) bool {
	lower = strings.TrimSpace(lower)
	for _, polite := range []string{"please ", "please, ", "부탁드립니다 ", "부탁드립니다 먼저 ", "죄송하지만 ", "죄송하지만 먼저 "} {
		if after, ok := strings.CutPrefix(lower, polite); ok {
			lower = strings.TrimSpace(after)
			break
		}
	}
	for _, directive := range directives {
		if strings.HasPrefix(lower, directive) {
			return true
		}
	}
	return false
}

var planAndExecuteDirectives = []string{
	"먼저 계획한 후 실행", "먼저 계획 다시 구현", "먼저 방안을 제시한 후 실행", "먼저 방안 제시 다시 구현",
	"plan first, then", "plan first then", "plan then implement", "plan and implement",
}

var planFirstDirectives = []string{
	"먼저 계획", "먼저 솔루션 제공", "먼저 방안 제시",
	"plan first", "draft a plan", "give me a plan", "make a plan",
}

var planOnlyDirectives = []string{
	"계획만", "오직 계획만", "솔루션만 제공", "오직 방안만", "솔루션만 주세요",
	"plan only", "only plan", "just plan", "give me only a plan", "give me a plan only",
}

var planOnlyBoundaryTerms = []string{
	"give me only a plan", "give me a plan only", "only give me the plan",
	"솔루션만 주세요", "방안만 원함",
}

var plannerNoExecutionTerms = []string{
	"실행하지 마세요", "먼저 실행하지 마세요", "일단 실행하지 않음", "구현하지 마세요", "일단 구현하지 마세요", "당분간 구현하지 마세요",
	"수정하지 마세요", "일단 수정하지 마세요", "코드를 고치지 마세요", "일단 코드를 고치지 마세요", "코드를 건드리지 마세요",
	"do not execute", "don't execute", "do not implement", "don't implement",
	"do not make changes", "don't make changes", "without executing",
	"without implementation", "no execution", "no implementation",
}

var plannerApprovalTerms = []string{
	"제가 확인할 때까지 기다려 주세요", "제가 확인할 때까지 기다려 주세요", "제가 확인한 후", "확인한 후에",
	"제가 승인할 때까지 기다려 주세요", "제가 승인할 때까지 기다려 주세요", "제가 승인한 후", "승인한 후에",
	"wait for my approval", "wait for approval", "after i approve", "after my approval",
	"until i approve", "until my approval", "let me approve", "let me confirm",
	"after i confirm", "after my confirmation",
}

var directExecutionDirectives = []string{
	"바로 수정", "바로 수정", "바로 실행", "바로 실행", "계획하지 마세요", "계획하지 마세요", "계획 불필요",
	"just do it", "skip the plan",
}

func requestsPlanOnly(lower string) bool {
	directiveText := plannerDirectiveText(lower)
	if hasLeadingDirective(directiveText, planOnlyDirectives) {
		return true
	}
	if containsAnyLexical(directiveText, planOnlyBoundaryTerms) {
		return true
	}
	if (strings.Contains(directiveText, "단지 제공만") || strings.Contains(directiveText, "오직 ~만")) &&
		containsAnyLexical(directiveText, plannerIntentTerms) {
		return true
	}
	return containsAnyLexical(directiveText, plannerNoExecutionTerms) &&
		(containsAnyLexical(directiveText, plannerIntentTerms) ||
			containsAnyLexical(directiveText, plannerWorkTerms))
}

func requestsPlanApproval(lower string) bool {
	directiveText := plannerDirectiveText(lower)
	return (containsAnyLexical(directiveText, plannerIntentTerms) ||
		containsAnyLexical(directiveText, plannerWorkTerms)) &&
		containsUnnegatedPlannerApproval(directiveText)
}

func requestsDirectExecution(lower string) bool {
	directiveText := plannerDirectiveText(lower)
	if containsAnyLexical(directiveText, directExecutionDirectives) {
		return true
	}
	for _, term := range []string{"don't plan", "do not plan"} {
		offset := 0
		for offset < len(directiveText) {
			idx := strings.Index(directiveText[offset:], term)
			if idx < 0 {
				break
			}
			idx += offset
			after := strings.TrimSpace(directiveText[idx+len(term):])
			if !strings.HasPrefix(after, "to ") {
				return true
			}
			offset = idx + len(term)
		}
	}
	return false
}

var plannerIntentTerms = []string{
	"plan", "planning", "방안", "계획", "계획",
}

func containsUnnegatedPlannerApproval(text string) bool {
	for _, term := range plannerApprovalTerms {
		offset := 0
		for offset < len(text) {
			idx := strings.Index(text[offset:], term)
			if idx < 0 {
				break
			}
			idx += offset
			if !plannerApprovalNegated(text[:idx]) {
				return true
			}
			offset = idx + len(term)
		}
	}
	return false
}

func plannerApprovalNegated(prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	for _, negation := range []string{
		"하지 마세요", "필요 없어요", "필요 없음", "필요 없음", "안 해도 돼요", "안 그래도 돼요", "하지 마세요",
		"do not", "don't", "not", "no need to", "do not need to", "don't need to",
		"not necessary to", "without",
	} {
		if strings.HasSuffix(prefix, negation) {
			return true
		}
	}
	return false
}

func plannerDirectiveText(text string) string {
	var b strings.Builder
	var closing rune
	escaped := false
	runes := []rune(text)
	for i, r := range runes {
		if closing != 0 {
			if escaped {
				escaped = false
				b.WriteRune(' ')
				continue
			}
			if (closing == '"' || closing == '`') && r == '\\' {
				escaped = true
				b.WriteRune(' ')
				continue
			}
			if r == closing && (closing != '\'' || !plannerInlineApostrophe(runes, i)) {
				closing = 0
			}
			b.WriteRune(' ')
			continue
		}
		switch r {
		case '"':
			closing = '"'
			b.WriteRune(' ')
		case '“':
			closing = '”'
			b.WriteRune(' ')
		case '‘':
			closing = '\''
			b.WriteRune(' ')
		case '\'':
			if plannerSingleQuoteStart(runes, i) {
				closing = '\''
				b.WriteRune(' ')
				continue
			}
			b.WriteRune(r)
		case '`':
			closing = '`'
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func plannerSingleQuoteStart(runes []rune, i int) bool {
	if i+1 >= len(runes) || !unicode.IsLetter(runes[i+1]) {
		return false
	}
	return i == 0 || !unicode.IsLetter(runes[i-1]) && !unicode.IsDigit(runes[i-1])
}

func plannerInlineApostrophe(runes []rune, i int) bool {
	return i > 0 && i+1 < len(runes) &&
		(unicode.IsLetter(runes[i-1]) || unicode.IsDigit(runes[i-1])) &&
		(unicode.IsLetter(runes[i+1]) || unicode.IsDigit(runes[i+1]))
}

func isContextDependentAction(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsAny(text, "\n\r") || utf8.RuneCountInString(text) > 48 {
		return false
	}
	if plannerFileRefRE.MatchString(text) || strings.Contains(text, "@") {
		return false
	}
	lower := normalizePlannerText(text)
	for _, prefix := range []string{
		"fix it", "fix this", "do it", "apply it", "make that change", "go ahead with it",
		"수정해 주세요", "변경해 주세요", "이렇게 수정해 주세요", "이렇게 진행해 주세요", "이것 실행", "이렇게 수정하세요", "이 문제 수정",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isConversationalTurn(text string) bool {
	normalized := strings.Trim(strings.ToLower(strings.TrimSpace(text)), " \t\r\n.!?。！？,，;；:：")
	return conversationalTurns[normalized]
}

var conversationalTurns = map[string]bool{
	"hello": true, "hi": true, "hey": true, "thanks": true, "thank you": true,
	"안녕": true, "안녕하세요": true, "감사": true, "수고하셨습니다": true, "확인": true, "이해": true,
}

func isContextDependentShortReply(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsAny(text, "\n\r") {
		return false
	}
	if directOptionReplyRE.MatchString(text) || prefixedOptionReplyRE.MatchString(text) {
		return true
	}
	lower := strings.ToLower(text)
	if containsAnyLexical(lower, complexIntentTerms) || containsAnyLexical(lower, plannerWorkTerms) {
		return false
	}
	if shortContextReplies[lower] {
		return true
	}
	if utf8.RuneCountInString(text) > 16 {
		return false
	}
	for _, prefix := range shortContextReplyPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

var shortContextReplies = map[string]bool{
	"ok": true, "okay": true, "yes": true, "y": true, "no": true, "n": true,
	"sure": true, "go ahead": true, "proceed": true, "continue": true, "next": true,
	"sounds good": true, "좋음": true, "좋습니다": true, "가능": true,
	"응": true, "맞음": true, "예": true, "확인": true, "동의": true, "계속": true,
	"계속하세요": true, "다음 단계": true, "시작": true, "시작하세요": true,
	"이렇게": true, "문제없음": true,
}

var shortContextReplyPrefixes = []string{
	"계속", "실행", "시작", "다음 단계", "go ahead", "proceed", "continue",
}

func isLowRiskQuestion(lower string) bool {
	lower = strings.TrimSpace(lower)
	normalized := strings.ReplaceAll(lower, "'", "")
	if strings.HasPrefix(lower, "what ") || strings.HasPrefix(normalized, "whats ") ||
		strings.HasPrefix(lower, "why ") || strings.HasPrefix(lower, "how ") ||
		strings.HasPrefix(lower, "who ") || strings.HasPrefix(lower, "where ") ||
		strings.HasPrefix(lower, "when ") || strings.HasPrefix(lower, "which ") ||
		strings.HasPrefix(lower, "whose ") || strings.HasPrefix(lower, "whom ") ||
		strings.HasPrefix(lower, "explain ") || strings.HasPrefix(lower, "describe ") ||
		strings.HasPrefix(lower, "tell ") || strings.HasPrefix(lower, "show ") ||
		strings.HasPrefix(lower, "list ") || strings.HasPrefix(lower, "summarize ") ||
		strings.HasPrefix(lower, "summarise ") || strings.HasPrefix(lower, "compare ") ||
		strings.HasPrefix(lower, "difference ") || strings.HasPrefix(lower, "is ") ||
		strings.HasPrefix(lower, "are ") || strings.HasPrefix(lower, "can ") ||
		strings.HasPrefix(lower, "could ") || strings.HasPrefix(lower, "do ") ||
		strings.HasPrefix(lower, "does ") || strings.HasPrefix(lower, "did ") ||
		strings.HasPrefix(lower, "should ") || strings.HasPrefix(lower, "would ") ||
		strings.HasPrefix(lower, "will ") ||
		strings.HasPrefix(lower, "what's") || strings.HasPrefix(normalized, "whats") ||
		strings.HasPrefix(lower, "설명") || strings.HasPrefix(lower, "설명") ||
		strings.HasPrefix(lower, "어떻게 보세요") || strings.HasPrefix(lower, "확인해 주세요") ||
		strings.HasPrefix(lower, "소개해 주세요") ||
		strings.HasPrefix(lower, "말해 주세요") || strings.HasPrefix(lower, "보여주세요") ||
		strings.HasPrefix(lower, "확인해 주세요") || strings.HasPrefix(lower, "예 무엇") ||
		strings.HasPrefix(lower, "있나요") || strings.HasPrefix(lower, "할 수 없습니다") ||
		strings.HasPrefix(lower, "가능한가요") || strings.HasPrefix(lower, "맞나요") ||
		strings.HasPrefix(lower, "예 아니요") || strings.HasPrefix(lower, "질문드립니다") {
		return !containsAnyLexical(lower, plannerQuestionWorkTerms)
	}
	return false
}

func isComplexGuidanceQuestion(lower string) bool {
	for _, prefix := range []string{
		"how do i ", "how should i ", "how would you ", "what's the best way ",
		"what is the best way ", "explain how to ", "어떻게 구현", "어떻게 구현", "어떻게 마이그레이션", "어떻게 마이그레이션",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func containsAnyLexical(s string, terms []string) bool {
	for _, term := range terms {
		if containsLexicalTerm(s, term) {
			return true
		}
	}
	return false
}

func containsLexicalTerm(s, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	if containsNonASCII(term) || strings.ContainsAny(term, " -_/") {
		return strings.Contains(s, term)
	}
	return slices.Contains(strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}), term)
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

var complexIntentTerms = []string{
	"refactor", "migrate", "migration", "redesign", "end-to-end", "e2e", "wire up",
	"integration", "architecture", "release", "package", "리팩토링", "마이그레이션", "개조",
	"엔드투엔드", "연동 조정", "연결", "아키텍처", "배포", "패키징",
}

var plannerWorkTerms = []string{
	"fix", "fixing", "update", "updating", "remove", "removing", "delete", "deleting",
	"edit", "editing", "write", "writing", "create", "creating", "add", "adding", "repair",
	"patch", "run", "running", "build", "building", "implement", "implementing", "refactor",
	"refactoring", "migrate", "migrating", "redesign", "review", "reviewing", "audit",
	"inspect", "debug", "test", "tests", "testing", "수정", "복구", "업데이트", "삭제", "제거",
	"편집", "작성", "생성하기", "신규 추가", "추가", "실행", "빌드", "구현", "리팩토링", "마이그레이션",
	"개조", "검토", "검토", "추적", "디버깅", "테스트", "하나 추가", "하나라고", "보완해주세요", "보완",
}

var plannerMutationWorkTerms = []string{
	"fix", "fixing", "update", "updating", "remove", "removing", "delete", "deleting",
	"edit", "editing", "write", "writing", "create", "creating", "add", "adding", "repair",
	"patch", "build", "building", "implement", "implementing", "refactor", "refactoring",
	"migrate", "migrating", "redesign", "수정", "복구", "업데이트", "삭제", "제거", "편집",
	"작성", "생성하기", "신규 추가", "추가", "빌드", "구현", "리팩토링", "마이그레이션", "개조", "하나 추가",
	"하나라고", "보완해주세요", "보완",
}

var plannerReadOnlyWorkTerms = []string{
	"run", "running", "review", "reviewing", "audit", "inspect", "debug",
	"test", "tests", "testing", "실행", "검토", "검토", "추적", "디버깅", "테스트",
}

var plannerQuestionWorkTerms = []string{
	"fix", "fixing", "update", "updating", "remove", "removing", "delete", "deleting",
	"edit", "editing", "write", "writing", "create", "creating", "add", "adding", "repair",
	"patch", "implement", "implementing", "refactor", "refactoring", "migrate", "migrating",
	"redesign", "수정", "복구", "업데이트", "삭제", "제거", "편집", "작성", "생성하기", "신규 추가",
	"추가", "구현", "리팩토링", "마이그레이션", "개조", "하나 추가", "하나라고", "보완해주세요", "보완",
}

var plannerHighRiskTerms = []string{
	"auth", "authentication", "authorization", "permission", "token", "secret",
	"credential", "payment", "billing", "race", "concurrency", "deadlock", "transaction",
	"encryption", "signature", "sandbox", "privilege", "권한", "인증", "인증", "토큰",
	"비밀 키", "결제", "청구", "동시성", "경합", "경합", "데드락", "트랜잭션", "암호화", "서명",
	"샌드박스", "권한 상승",
}

var plannerCrossSurfaceTerms = []string{
	"multiple files", "several files", "across", "frontend and backend", "backend and frontend",
	"api and ui", "ui and api", "database and api", "여러 파일", "여러 곳", "프론트엔드와 백엔드",
	"전체 모듈", "전체 프로젝트", "전체 링크", "모듈 간",
}

var plannerAtomicTerms = []string{
	"typo", "wording", "copy", "readme", "changelog", "nil check", "null check",
	"log line", "one line", "rename", "카피라이팅", "오타", "철자", "널 포인터 검사",
	"nil 검사", "로그 한 줄", "이름 바꾸기", "이름 재정의",
}

var plannerNamedTargets = []string{
	"readme", "changelog", "makefile", "dockerfile",
}

var plannerAmbiguousScopeTerms = []string{
	"the bug", "the issue", "the problem", "performance", "everything", "whole module",
	"이 버그", "이 버그", "이 문제", "성능", "전체 모듈", "모든 문제",
}

func TaskWarrantsPlanner(input string) bool {
	return DecidePlannerRoute(context.Background(), input).Route != agent.PlannerRouteExecutorOnly
}

func NewPlannerPolicy() agent.PlannerPolicy {
	return DecidePlannerRoute
}

func NewPlannerGate() func(context.Context, string) bool {
	return func(ctx context.Context, input string) bool {
		return DecidePlannerRoute(ctx, input).Route != agent.PlannerRouteExecutorOnly
	}
}
