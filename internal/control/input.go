package control

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"patty/internal/ablation"
	"patty/internal/agent"
	"patty/internal/memory"
	"patty/internal/planmode"
	"patty/internal/skill"
)

type InvocationRequest struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Offset int    `json:"offset"`
}

const PlanModeMarker = planmode.Marker

const legacyPlanModeMarker = "[Plan mode — read-only. Explore the codebase first (read_file, ls, grep, glob, web_fetch, task, ask are available; writers are refused by the harness). Before planning, if a decision that is genuinely the user's — tech stack, an ambiguous requirement, scope, an irreversible choice — would materially shape the plan and you can't settle it from the codebase or a sensible default, use the ask tool to clarify it first; otherwise pick the obvious default and state the assumption in the plan instead of asking. Then present a LAYERED plan as your reply and stop — do not write files, edit, or run side-effecting bash. Structure the plan as a two-level markdown list so it becomes a layered task list: each PHASE is a top-level numbered list item (a coherent milestone, e.g. \"1. Add the config loader\"), and each phase's concrete, verifiable sub-steps are bullets indented beneath it (e.g. \"   - parse the TOML into Config\"). Use plain numbered list items for phases — do NOT write phases as markdown headings (##, ###) — so both levels parse. Keep phases few (about 2-6). The user will be asked to approve before any changes are made.]"

const (
	activeGoalOpen  = "<active-goal>"
	activeGoalClose = "</active-goal>"
	hookContextTag  = "hook-context"
)

const (
	maxHookContextChars      = 10000
	maxTotalHookContextChars = 20000
)

const (
	GoalStatusRunning  = "running"
	GoalStatusComplete = "complete"
	GoalStatusBlocked  = "blocked"
	GoalStatusStopped  = "stopped"
)

type GoalResearchMode int

const (
	GoalResearchAuto GoalResearchMode = iota
	GoalResearchOn
	GoalResearchOff
)

func StripComposePrefixes(content string) string {
	s := agent.StripTransientUserBlocks(content)
	s = stripComposeMarker(s, PlanModeMarker)
	s = stripComposeMarker(s, legacyPlanModeMarker)
	s = strings.TrimSpace(s)
	return s
}

func stripComposeMarker(s, marker string) string {
	s = strings.TrimPrefix(s, marker+"\n\n")
	return strings.TrimPrefix(s, marker)
}

func StripReferencedContextPrefix(content string) string {
	const preamble = "Referenced context:"
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, preamble) {
		return content
	}
	s = strings.TrimSpace(s[len(preamble):])
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		if !strings.HasPrefix(s, "<file ") && !strings.HasPrefix(s, "<dir ") &&
			!strings.HasPrefix(s, "<resource ") && !strings.HasPrefix(s, "<image ") {
			break
		}
		tagEnd := strings.IndexByte(s, ' ')
		if tagEnd < 0 {
			break
		}
		tagName := s[1:tagEnd]
		closeTag := "</" + tagName + ">"
		closeIdx := strings.Index(s, closeTag)
		if closeIdx < 0 {
			break
		}
		s = strings.TrimSpace(s[closeIdx+len(closeTag):])
	}
	return s
}

func IsSyntheticUserMessage(content string) bool {
	if trimmed := strings.TrimSpace(agent.StripTransientUserBlocks(content)); trimmed == planApprovedMessage {
		return true
	}
	return agent.IsSyntheticUserText(content)
}

func (c *Controller) Compose(text string) string {
	return c.compose(text, text, true)
}

func (c *Controller) compose(text, source string, includeHookContext bool) string {
	goal, goalStatus, goalResearchMode, autoResearchTaskID := c.goals.snapshot()
	return c.composeWithGoal(
		text,
		source,
		includeHookContext,
		goal,
		goalStatus,
		goalResearchMode,
		autoResearchTaskID,
	)
}

func (c *Controller) composeWithGoal(
	text, source string,
	includeHookContext bool,
	goal, goalStatus string,
	goalResearchMode GoalResearchMode,
	autoResearchTaskID string,
) string {
	c.mu.Lock()
	plan := c.planMode
	responseLanguage := c.responseLanguage
	reasoningLanguage := c.reasoningLanguage
	c.mu.Unlock()
	notes := c.memory.drainPending()

	if strings.TrimSpace(goal) != "" && goalStatus == GoalStatusRunning {
		prefix := activeGoalBlock(goal, goalResearchMode)
		if runtime := c.autoResearchRuntimeBlock(autoResearchTaskID); runtime != "" {
			prefix += "\n\n" + runtime
		}
		text = prefix + "\n\n" + text
	}
	if plan {
		text = PlanModeMarker + "\n\n" + text
	}
	text = agent.WithResponseLanguage(text, responseLanguage)
	text = agent.WithReasoningLanguageForSource(text, reasoningLanguage, source)

	if len(notes) > 0 {
		var b strings.Builder
		b.WriteString("<memory-update>\n")
		b.WriteString("The following project-memory changes were just made and apply from now on:\n")
		for _, n := range notes {
			b.WriteString("- " + n + "\n")
		}
		b.WriteString("</memory-update>\n\n")
		text = b.String() + text
	}

	if c.jobs != nil {
		if note := c.jobs.DrainCompletedNoteForSession(c.parentSessionID()); note != "" {
			text = "<background-jobs>\n" + note + "\n</background-jobs>\n\n" + text
		}
	}
	if includeHookContext {
		if block := c.drainHookContextBlock(); block != "" {
			text = block + "\n\n" + text
		}
		if len(notes) == 0 && !c.ablation.Off(ablation.Retrieval) {
			if block := c.memory.recall(source).Block(); block != "" {
				text = strings.TrimRight(text, "\n") + "\n\n" + block
			}
		} else if len(notes) > 0 {
			c.memory.recordRecall(memory.RecallResult{
				Query:      strings.TrimSpace(source),
				Suppressed: "memory update already supplies the new fact",
			})
		}
	}
	return text
}

func (c *Controller) LastMemoryRecall() memory.RecallResult {
	return c.memory.lastRecallResult()
}

func (c *Controller) enqueueHookContexts(contexts []string) {
	if len(contexts) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, context := range contexts {
		context = strings.TrimSpace(context)
		if context == "" {
			continue
		}
		c.hookContexts = append(c.hookContexts, context)
	}
}

func (c *Controller) drainHookContextBlock() string {
	c.mu.Lock()
	contexts := c.hookContexts
	c.hookContexts = nil
	c.mu.Unlock()
	if len(contexts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<hook-context event="SessionStart">`)
	b.WriteString("\n")
	total := 0
	for i, context := range contexts {
		text, truncated := clipHookContext(context, maxHookContextChars)
		remaining := maxTotalHookContextChars - total
		if remaining <= 0 {
			fmt.Fprintf(&b, "[truncated: omitted %d additional hook context item(s)]\n", len(contexts)-i)
			break
		}
		text, totalTruncated := clipHookContext(text, remaining)
		total += len([]rune(text))
		if i > 0 {
			b.WriteString("\n---\n")
		}
		b.WriteString(escapeHookContext(text))
		b.WriteString("\n")
		if truncated || totalTruncated {
			b.WriteString("[truncated]\n")
		}
	}
	b.WriteString(`</hook-context>`)
	return b.String()
}

func clipHookContext(s string, max int) (string, bool) {
	r := []rune(s)
	if len(r) <= max {
		return s, false
	}
	if max < 0 {
		max = 0
	}
	return string(r[:max]), true
}

func escapeHookContext(s string) string {
	return strings.ReplaceAll(s, "</"+hookContextTag+">", "<\\/"+hookContextTag+">")
}

func (c *Controller) autoResearchRuntimeBlock(taskID string) string {
	if !c.autoResearch.enabled() || strings.TrimSpace(taskID) == "" {
		return ""
	}
	summary, err := c.autoResearch.summary(taskID)
	if err != nil {
		return "<autoresearch-runtime>\nstatus: invalid\nerror: " + strings.ReplaceAll(err.Error(), autoResearchRuntimeClose, "<\\/autoresearch-runtime>") + "\n</autoresearch-runtime>"
	}
	var b strings.Builder
	b.WriteString("<autoresearch-runtime>\n")
	b.WriteString("task_id: " + summary.TaskID + "\n")
	b.WriteString("status: " + summary.Status + "\n")
	b.WriteString("iteration: ")
	b.WriteString(strconv.Itoa(summary.Iteration))
	b.WriteString("\n")
	b.WriteString("current_direction: " + summary.CurrentDirection + "\n")
	b.WriteString("stale_count: ")
	b.WriteString(strconv.Itoa(summary.StaleCount))
	b.WriteString("\n")
	b.WriteString("pivot_count: ")
	b.WriteString(strconv.Itoa(summary.PivotCount))
	b.WriteString("\n")
	if summary.PivotRequired {
		b.WriteString("pivot_required: true\n")
	} else {
		b.WriteString("pivot_required: false\n")
	}
	b.WriteString("open_success_criteria: ")
	b.WriteString(strconv.Itoa(len(summary.OpenCriteria)))
	b.WriteString("\n")
	for _, criterion := range summary.OpenCriteria {
		b.WriteString("- ")
		b.WriteString(criterion.ID)
		b.WriteString(": ")
		b.WriteString(strings.ReplaceAll(criterion.Description, "\n", " "))
		b.WriteString("\n")
	}
	if summary.Blocker != "" {
		b.WriteString("blocker: " + summary.Blocker + "\n")
	}
	b.WriteString("next_required_action: " + summary.NextRequiredAction + "\n")
	b.WriteString("</autoresearch-runtime>")
	return b.String()
}

const autoResearchRuntimeClose = "</autoresearch-runtime>"

func reasoningLanguageBlock(lang string) string {
	return agent.ReasoningLanguageBlock(lang)
}

func (c *Controller) ComposeSynthetic(text string) string {
	c.mu.Lock()
	responseLang := c.responseLanguage
	lang := c.reasoningLanguage
	c.mu.Unlock()
	text = agent.WithResponseLanguage(text, responseLang)
	return agent.WithReasoningLanguageForSource(text, lang, text)
}

func activeGoalBlock(goal string, researchMode GoalResearchMode) string {
	goal = strings.TrimSpace(goal)
	goal = strings.ReplaceAll(goal, activeGoalClose, "<\\/active-goal>")
	var b strings.Builder
	b.WriteString(activeGoalOpen)
	b.WriteString("\n")
	b.WriteString(goal)
	b.WriteString("\n\n")
	b.WriteString(goalTaskContractInstructions)
	if shouldUseAutoResearch(goal, researchMode) {
		b.WriteString("\n\n")
		b.WriteString(autoResearchGoalInstructions)
	}
	b.WriteString("\n")
	b.WriteString(activeGoalClose)
	return b.String()
}

const goalTaskContractInstructions = `Goal mode: pursue this goal autonomously. Treat the user's goal as a task contract:
- Honor Context, Request, Output format, Constraints, and Checkpoint/Pause policy sections when present; otherwise infer a lightweight contract from the conversation and workspace.
- Preserve scope and output format. Do not invent requirements or hide uncertainty; state assumptions when sensible defaults are enough to proceed.
- Pause only when the next step involves an irreversible or externally visible operation, the requested scope has changed, or progress requires information only the user can provide. Otherwise keep working and report assumptions at the end.
- Complete only when the concrete request is done, the output format and constraints are satisfied, and relevant verification was attempted or reported unavailable.

Do not stop after describing a plan; execute the next useful step. End every goal-mode turn by calling the update_goal tool with your disposition: continue (work is ongoing — give the next concrete step in next_action), complete (only when fully done and verified), or blocked (only when the user can unblock). The host validates your claim and decides whether to continue automatically.`

const autoResearchGoalInstructions = `AutoResearch protocol: this goal looks like long-horizon research, debugging, optimization, or implementation work. Treat AutoResearch as a durable strategy for this Goal, not as a background daemon or a global skill.
- Say briefly in the first visible reply that the goal is being handled with AutoResearch and that host-owned state lives under .patty/autoresearch/<task-id>/, using the actual task_id from <autoresearch-runtime>.
- Keep dynamic state out of PATTY_CODE.md, AGENTS.md, project memory, system prompts, and tool schemas. Use project-local .patty/autoresearch/ state only.
- Use the task_id and open_success_criteria in <autoresearch-runtime> as authoritative. The host creates task ids and owns state/task_spec.json, state/progress.json, state/findings.jsonl, state/directions_tried.json, state/iteration_log.jsonl, and logs/heartbeat.jsonl.
- Do not hand-edit the host-owned AutoResearch state files. When you have direct evidence for an open criterion, include an <autoresearch-evidence> block in your assistant reply so the host can persist it:
<autoresearch-evidence>
{"criterion_id":"objective_evidence","kind":"file","summary":"What was directly observed","source":"file","paths":["relative/path"],"accepted":true}
</autoresearch-evidence>
- Before each iteration, use the runtime summary as authoritative, choose a direction that differs materially from directions already tried, execute the smallest evidence-producing chunk, verify it, and report accepted evidence with <autoresearch-evidence> blocks.
- Increment stale_count when an iteration lacks accepted evidence or repeats a prior direction. At stale_count >= 2, make a structural pivot such as changing evidence source, entrypoint, implementation boundary, test oracle, benchmark, decomposition, environment, platform, or refutation angle. At stale_count >= 4, stop autonomous digging and ask for the smallest external input needed.
- Workers or subagents may gather evidence, but the orchestrator owns canonical state writes. Workers must not publish, push, delete, contact external systems, or write canonical state unless explicitly designated.
- Complete only after auditing every open success criterion in <autoresearch-runtime> against direct evidence. Public publishing, destructive changes, credential use, payments, external notifications, privacy-sensitive output, and cache-sensitive changes still require the normal Patty Code gates.`

func shouldUseAutoResearch(goal string, mode GoalResearchMode) bool {
	switch mode {
	case GoalResearchOn:
		return true
	case GoalResearchOff:
		return false
	}
	return isAutoResearchGoal(goal)
}

func isAutoResearchGoal(goal string) bool {
	trimmed := strings.TrimSpace(goal)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, ".patty/autoresearch/") {
		return true
	}
	for _, kw := range autoResearchStrongKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return autoResearchPhaseCount(lower) >= 4
}

func autoResearchPhaseCount(lower string) int {
	categories := 0
	for _, group := range autoResearchPhaseKeywords {
		if containsAnyGoalKeyword(lower, group) {
			categories++
		}
	}
	return categories
}

var autoResearchStrongKeywords = []string{
	"지속",
	"장기",
	"철저히",
	"근본 원인까지",
	"근본 원인 명확",
	"여러 라운드",
	"제자리에서 맴돌지 마세요",
	"제자리에서 맴돌지 말아요",
	"완전한 방안",
	"완전히 완성된 방안",
	"실험 돌리기",
	"반복 검증",
	"장기 최적화",
	"체계적 연구",
	"지속 연구",
	"지속 추적",
	"지속 추진",
	"장기 실행",
	"long-horizon",
	"long horizon",
	"long-running",
	"keep researching",
	"keep working",
	"root cause",
	"until the root cause",
	"do not spin",
	"don't spin",
	"thoroughly",
	"systematically",
}

var autoResearchPhaseKeywords = [][]string{
	{"연구", "조사", "추적", "분석", "위치 파악", "진단", "research", "investigate", "diagnose", "analyze", "analysis"},
	{"구현", "복구", "개조", "개발", "리팩토링", "implement", "build", "fix", "refactor"},
	{"검증", "테스트", "재현", "연동 조정", "benchmark", "verify", "validate", "test", "reproduce"},
	{"최적화", "완성", "개선", "수렴", "optimize", "improve", "tune", "polish"},
	{"문서", "방안", "설명", "요약", "document", "docs", "writeup", "plan"},
	{"배포", "출시", "커미트", "pull request", "publish", "ship", "deploy"},
}

func containsAnyGoalKeyword(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func MemoryQuickAddNote(input string) (note string, ok bool) {
	trimmed := strings.TrimSpace(input)
	if strings.Contains(trimmed, "\n") {
		return "", false
	}
	if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "#\t") {
		return strings.TrimSpace(trimmed[1:]), true
	}
	return "", false
}

func RememberCommandNote(input string) (note string, ok bool) {
	trimmed := strings.TrimSpace(input)
	switch {
	case trimmed == "/remember":
		return "", true
	case strings.HasPrefix(trimmed, "/remember ") || strings.HasPrefix(trimmed, "/remember\t"):
		return strings.TrimSpace(trimmed[len("/remember"):]), true
	default:
		return "", false
	}
}

type GoalCommandAction int

const (
	GoalCommandStatus GoalCommandAction = iota + 1
	GoalCommandSet
	GoalCommandClear
	GoalCommandPause
	GoalCommandResume
)

type GoalCommand struct {
	Action       GoalCommandAction
	Text         string
	Strict       bool
	ResearchMode GoalResearchMode
}

func ParseGoalCommand(input string) (GoalCommand, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed != "/goal" && !strings.HasPrefix(trimmed, "/goal ") && !strings.HasPrefix(trimmed, "/goal\t") {
		return GoalCommand{}, false
	}
	args := strings.TrimSpace(trimmed[len("/goal"):])
	strict, researchMode, actionArgs := parseLeadingGoalFlags(args)

	switch strings.ToLower(actionArgs) {
	case "", "status":
		return GoalCommand{Action: GoalCommandStatus, Strict: strict, ResearchMode: researchMode}, true
	case "clear", "off", "stop", "done":
		return GoalCommand{Action: GoalCommandClear, Strict: strict, ResearchMode: researchMode}, true
	case "pause":
		return GoalCommand{Action: GoalCommandPause, Strict: strict, ResearchMode: researchMode}, true
	case "resume":
		return GoalCommand{Action: GoalCommandResume, Strict: strict, ResearchMode: researchMode}, true
	default:
		return GoalCommand{Action: GoalCommandSet, Text: actionArgs, Strict: strict, ResearchMode: researchMode}, true
	}
}

func parseLeadingGoalFlags(args string) (bool, GoalResearchMode, string) {
	strict := false
	mode := GoalResearchAuto
	rest := strings.TrimLeftFunc(args, unicode.IsSpace)
	for rest != "" {
		token, after := leadingGoalToken(rest)
		switch strings.ToLower(token) {
		case "--strict":
			strict = true
		case "--research", "--auto-research", "--deep":
			mode = GoalResearchOn
		case "--simple", "--no-research":
			mode = GoalResearchOff
		default:
			return strict, mode, strings.TrimSpace(rest)
		}
		rest = strings.TrimLeftFunc(after, unicode.IsSpace)
	}
	return strict, mode, ""
}

func leadingGoalToken(s string) (string, string) {
	for i, r := range s {
		if unicode.IsSpace(r) {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func (c *Controller) CustomCommand(input string) (sent string, found bool) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return "", false
	}
	name := strings.TrimPrefix(fields[0], "/")
	for _, cmd := range c.Commands() {
		if cmd.Name == name {
			return cmd.Render(fields[1:]), true
		}
	}
	return "", false
}

func (c *Controller) resolveSkillInvocation(input string) (skill.Skill, string, bool) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return skill.Skill{}, "", false
	}
	name := strings.TrimPrefix(fields[0], "/")
	sk, ok := c.skills.bySlashName(name)
	if !ok {
		return skill.Skill{}, "", false
	}
	return sk, strings.Join(fields[1:], " "), true
}

func (c *Controller) RunSkill(input string) (sent string, found bool) {
	sk, task, ok := c.resolveSkillInvocation(input)
	if !ok {
		return "", false
	}
	return c.skills.render(sk, task), true
}

func (c *Controller) MCPPrompt(ctx context.Context, input string) (sent string, found bool, err error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return "", false, nil
	}
	name := strings.TrimPrefix(fields[0], "/")

	prompts := c.mcp.prompts()
	idx := -1
	for i := range prompts {
		if prompts[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", false, nil
	}

	args := map[string]string{}
	for i, a := range prompts[idx].Args {
		if i+1 < len(fields) {
			args[a.Name] = fields[i+1]
		}
	}
	text, err := prompts[idx].Get(ctx, args)
	if err != nil {
		return "", true, err
	}
	return text, true, nil
}
