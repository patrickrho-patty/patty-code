package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/capability"
	"patty/internal/event"
	"patty/internal/evidence"
	"patty/internal/provider"
	"patty/internal/taskintent"
	"patty/internal/tool"
)

type fakeReadFileTool struct{}

func (fakeReadFileTool) Name() string            { return "read_file" }
func (fakeReadFileTool) Description() string     { return "fake read" }
func (fakeReadFileTool) ReadOnly() bool          { return true }
func (fakeReadFileTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (fakeReadFileTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "contents", nil
}

type fakeWriterTool struct{}

func (fakeWriterTool) Name() string            { return "fake_write" }
func (fakeWriterTool) Description() string     { return "fake write" }
func (fakeWriterTool) ReadOnly() bool          { return false }
func (fakeWriterTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (fakeWriterTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "wrote", nil
}

// "resolve" classified every wrapped subagent prompt as a mutation request.
const legacyWorkspaceContext = `<workspace-context event="SubagentWorkspace">
Current workspace: "/w"
File tools resolve relative paths against this workspace. For project inspection, prefer "." or relative paths unless the user explicitly named another absolute path.
</workspace-context>`

func TestDeliveryClassificationUsesTrustedTaskText(t *testing.T) {
// legacy workspace wording ("resolve") plus an extra mutation verb in the
	reg := tool.NewRegistry()
	reg.Add(fakeReadFileTool{})
	reg.Add(fakeWriterTool{})
	pristine := "Review the current state of a.go — bugfixes were applied. Report remaining issues."
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("1", "read_file", `{"path":"a.go"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "reviewed; looks good"}, {Type: provider.ChunkDone}},
	}}
	sub := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true, ClassifierTaskText: pristine}, event.Discard)
	if err := sub.Run(context.Background(), legacyWorkspaceContext+"\n\n"+pristine); err != nil {
		t.Fatalf("wrapped review prompt deadlocked despite trusted task text: %v", err)
	}
	if sub.deliveryMutationExpected {
		t.Fatal("host framing armed the mutation expectation past the trusted override")
	}
}

func TestDeliveryClassificationResistsFramingSpoof(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeReadFileTool{})
	reg.Add(fakeWriterTool{})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "done, consider it fixed"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	err := a.Run(context.Background(), "<workspace-context>fix parser.go</workspace-context>")
	var readinessErr *FinalReadinessError
	if !errors.As(err, &readinessErr) {
		t.Fatalf("spoofed framing disarmed the delivery gates: err=%v", err)
	}
	if !strings.Contains(readinessErr.Reason, "state change") {
		t.Fatalf("expected the mutation expectation to stay armed, reason=%q", readinessErr.Reason)
	}
}

func TestReadOnlyRegistryDisarmsMutationExpectation(t *testing.T) {
	roReg := tool.NewRegistry()
	roReg.Add(fakeReadFileTool{})
	if registryHasWriterTools(roReg) {
		t.Fatal("read-only registry misreported writer tools")
	}
	writerReg := tool.NewRegistry()
	writerReg.Add(fakeReadFileTool{})
	writerReg.Add(fakeWriterTool{})
	if !registryHasWriterTools(writerReg) {
		t.Fatal("writer registry not detected")
	}

// must not deadlock on "the request requires a state change". The scripted
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("1", "read_file", `{"path":"a.go"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "reviewed; two issues found"}, {Type: provider.ChunkDone}},
	}}
	sub := New(prov, roReg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	if err := sub.Run(context.Background(), "fix review: verify the fixes in a.go were applied"); err != nil {
		t.Fatalf("read-only delivery subagent deadlocked: %v", err)
	}
	if sub.deliveryMutationExpected {
		t.Fatal("mutation expectation armed on a read-only registry")
	}
}

func TestDeliveryResolvedReadOnlyBashDoesNotArmMutationReadiness(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubBash{})
	prov := &scriptedProvider{name: "delivery", turns: [][]provider.Chunk{
		{toolCallChunk("pwd-base", "bash", `{"command":"basename \"$(pwd)\""}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "workspace basename inspected"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	if err := a.Run(context.Background(), "inspect and report the current workspace basename"); err != nil {
		t.Fatalf("resolved read-only delivery command: %v", err)
	}
	if _, ok := a.evidence.LatestSuccessfulMutationIndex(); ok {
		t.Fatal("resolved read-only bash was recorded as a mutation")
	}
	msgs := a.session.Snapshot()
	var resolved bool
	for _, msg := range msgs {
		for _, call := range msg.ToolCalls {
			if call.ID == "pwd-base" && call.ResolvedReadOnly != nil && *call.ResolvedReadOnly {
				resolved = true
			}
		}
	}
	if !resolved {
		t.Fatal("session receipt did not preserve resolved_read_only=true")
	}
}

func TestDeliveryConversationTokenSurvivesToNextTurnWithoutActionEvidence(t *testing.T) {
	prov := &scriptedProvider{name: "delivery", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "Understood."}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "ORBIT-42"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, tool.NewRegistry(), NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	if err := a.Run(context.Background(), "Remember ORBIT-42 and answer on the next turn."); err != nil {
		t.Fatalf("deferred conversation turn was blocked: %v", err)
	}
	if err := a.Run(context.Background(), "What was the code?"); err != nil {
		t.Fatalf("answer turn was blocked: %v", err)
	}
	if prov.call != 2 {
		t.Fatalf("provider calls = %d, want exactly two conversational turns", prov.call)
	}
	if got := lastAssistantContent(a.Session()); got != "ORBIT-42" {
		t.Fatalf("last assistant text = %q, want ORBIT-42", got)
	}
}

func TestDeliveryDurableMemoryRequiresRememberWithoutCodeCeremony(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "remember", readOnly: false})
	prov := &scriptedProvider{name: "delivery", turns: [][]provider.Chunk{
		{toolCallChunk("remember", "remember", `{"description":"ORBIT code","body":"ORBIT-42"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "Saved for future sessions."}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	if err := a.Run(context.Background(), "Remember ORBIT-42 permanently across sessions"); err != nil {
		t.Fatalf("durable-memory workflow inherited code-delivery ceremony: %v", err)
	}
	if prov.call != 2 {
		t.Fatalf("provider calls = %d, want remember plus final answer", prov.call)
	}
	if a.deliveryCriteriaEstablished {
		t.Fatal("durable-memory-only workflow should not manufacture code acceptance criteria")
	}

	missing := &scriptedProvider{name: "delivery", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "I'll remember it."}, {Type: provider.ChunkDone}},
	}}
	b := New(missing, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	err := b.Run(context.Background(), "Remember ORBIT-42 permanently across sessions")
	var readiness *FinalReadinessError
	if !errors.As(err, &readiness) || !strings.Contains(readiness.Reason, "remember tool") {
		t.Fatalf("text-only durable-memory claim err = %v", err)
	}
}

func TestNonGoalUpdateGoalWithVisibleTextDoesNotSpendRepairRound(t *testing.T) {
	goalTool, ok := tool.LookupBuiltin("update_goal")
	if !ok {
		t.Fatal("update_goal builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(goalTool)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "Here is the answer."}, toolCallChunk("goal", "update_goal", `{"status":"complete"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "unexpected repair"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession("sys"), Options{}, event.Discard)
	if err := a.Run(context.Background(), "answer normally"); err != nil {
		t.Fatalf("non-Goal update_goal with text: %v", err)
	}
	if prov.call != 1 {
		t.Fatalf("provider calls = %d, want no repair round", prov.call)
	}
	if got := lastAssistantContent(a.Session()); got != "Here is the answer." {
		t.Fatalf("last assistant text = %q", got)
	}
	if got := lastToolResult(a.Session(), "update_goal"); !strings.Contains(got, "only available while an active goal turn") {
		t.Fatalf("paired update_goal result = %q", got)
	}
}

func TestNonGoalToolOnlyUpdateGoalGetsAtMostOneRepairRound(t *testing.T) {
	goalTool, ok := tool.LookupBuiltin("update_goal")
	if !ok {
		t.Fatal("update_goal builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(goalTool)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("goal-1", "update_goal", `{"status":"complete"}`), {Type: provider.ChunkDone}},
		{toolCallChunk("goal-2", "update_goal", `{"status":"complete"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "unexpected third round"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession("sys"), Options{}, event.Discard)
	err := a.Run(context.Background(), "answer normally")
	if err == nil || !strings.Contains(err.Error(), "repeatedly called update_goal outside Goal mode") {
		t.Fatalf("repeated tool-only misuse error = %v", err)
	}
	if prov.call != 2 {
		t.Fatalf("provider calls = %d, want one repair round", prov.call)
	}
}

func TestDeliveryPlanModeReturnsProposalBeforeExecutionReadiness(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeReadFileTool{})
	reg.Add(fakeWriterTool{})
	proposal := "1. Fix the parser\n   - update a.go\n   - run the focused tests"
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: proposal}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	a.SetPlanMode(true)

	if err := a.Run(context.Background(), "fix the parser bug in a.go"); err != nil {
		t.Fatalf("delivery plan proposal was blocked by execution readiness: %v", err)
	}
	if prov.call != 1 {
		t.Fatalf("provider calls = %d, want 1 without readiness retries in plan mode", prov.call)
	}
	if got := lastAssistantContent(a.Session()); got != proposal {
		t.Fatalf("last assistant text = %q, want proposal %q", got, proposal)
	}

	a.SetPlanMode(false)
	if got := a.ReadinessResult(); !strings.Contains(got.Reason, "state change") {
		t.Fatalf("execution readiness did not resume after plan mode: %q", got.Reason)
	}
}

func TestPlanModeDefersCapabilityRequirementsUntilExecution(t *testing.T) {
	reg := tool.NewRegistry()
	a := New(&scriptedProvider{name: "p"}, reg, NewSession("sys"),
		Options{DeliveryProfile: true, CapabilityLedger: capability.NewLedger()}, event.Discard)
	a.SetPlanMode(true)
	a.SeedCapabilityRoute(capability.RouteDecision{Candidates: []capability.RouteCandidate{
		{Entry: capability.Entry{ID: "skill:deploy"}, Policy: capability.AutoUseRequire},
	}})

	if got := a.finalReadinessCheckFor(); got.applies || got.reason != "" {
		t.Fatalf("Plan proposal was forced through delivery capability gates: %+v", got)
	}

	a.SetPlanMode(false)
	got := a.finalReadinessCheckFor()
	if !got.applies || !strings.Contains(got.reason, "required capabilities") {
		t.Fatalf("execution did not restore required capability gate: %+v", got)
	}
}

func TestRunSubAgentReviewReportNudgeRecovers(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeReadFileTool{})
	AttachReviewReportTool(reg)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
// Run 1: reads the file, then finishes with prose only  no report.
		{toolCallChunk("1", "read_file", `{"path":"a.go"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "verdict: pass, no issues"}, {Type: provider.ChunkDone}},
		{toolCallChunk("2", "review_report", `{"kind":"review","verdict":"pass","reviewed_paths":["a.go"]}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "review_report submitted: pass"}, {Type: provider.ChunkDone}},
	}}
	sess := NewSession("sys")
	answer, err := RunSubAgentWithSession(context.Background(), prov, reg, sess, "review a.go",
		Options{RequireReviewReportKind: evidence.ReviewKindReview}, event.Discard)
	if err != nil {
		t.Fatalf("nudge recovery failed: %v", err)
	}
	if !strings.Contains(answer, "pass") {
		t.Fatalf("unexpected final answer %q", answer)
	}
	if !sessionHasUserMessageContaining(sess, "Call review_report now") {
		t.Fatal("expected the host completion nudge in the subagent session")
	}
// The report cited a path read in run 1  only possible because the nudge
	if got := lastToolResult(sess, "review_report"); !strings.Contains(got, "review_report accepted") {
		t.Fatalf("review_report result = %q", got)
	}
}

func TestRunSubAgentReviewReportExhaustionNamesRecovery(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeReadFileTool{})
	AttachReviewReportTool(reg)
	dir := t.TempDir()
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "looks fine"}, {Type: provider.ChunkDone}},
	}}
	sess := NewSession("sys")
	_, err := RunSubAgentWithSession(context.Background(), prov, reg, sess, "review it",
		Options{RequireReviewReportKind: evidence.ReviewKindReview, ArchiveDir: dir}, event.Discard)
	if err == nil {
		t.Fatal("expected failure when the report never arrives")
	}
	for _, want := range []string{"review_report", "host nudges", "re-run the review skill", "parent has no review_report tool"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, "subagent-report-failures", "review-*.jsonl"))
	if globErr != nil || len(matches) != 1 {
		t.Fatalf("expected one dumped transcript, got %v (%v)", matches, globErr)
	}
	if data, readErr := os.ReadFile(matches[0]); readErr != nil || !strings.Contains(string(data), "looks fine") {
		t.Fatalf("dump unreadable or incomplete: %v", readErr)
	}
}

func TestRunSubAgentSalvagesReadinessExhaustedWork(t *testing.T) {
	reg := evidenceRegistry()
	finalText := []provider.Chunk{{Type: provider.ChunkText, Text: "done, explanations added"}, {Type: provider.ChunkDone}}
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("criteria", "todo_write", `{"todos":[{"content":"Add explanations","status":"in_progress"}]}`), {Type: provider.ChunkDone}},
		{toolCallChunk("write", "write_file", `{"path":"qa/bank.md"}`), {Type: provider.ChunkDone}},
		finalText, // block 1 — complete_step/verification receipts missing
		finalText, // block 2 — no new receipts, stalled
		finalText, // block 3 — budget exhausted
	}}
	sess := NewSession("sys")
	answer, err := RunSubAgentWithSession(context.Background(), prov, reg, sess,
		"add explanations to the question bank", Options{DeliveryProfile: true, SubagentDepth: 1}, event.Discard)
	if err != nil {
		t.Fatalf("readiness exhaustion with real work must salvage, got err: %v", err)
	}
	for _, want := range []string{"[unverified]", "done, explanations added", "already on disk"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("salvaged answer %q missing %q", answer, want)
		}
	}
}

func TestRunSubAgentReadinessFailureWithoutMutationStillFails(t *testing.T) {
// An unbacked "done" claim keeps failing: with a mutation expected and no
	reg := tool.NewRegistry()
	reg.Add(fakeReadFileTool{})
	reg.Add(fakeWriterTool{})
	finalText := []provider.Chunk{{Type: provider.ChunkText, Text: "done, all fixed"}, {Type: provider.ChunkDone}}
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{finalText, finalText, finalText}}
	sess := NewSession("sys")
	answer, err := RunSubAgentWithSession(context.Background(), prov, reg, sess,
		"fix the crash in a.go", Options{DeliveryProfile: true, SubagentDepth: 1}, event.Discard)
	var readinessErr *FinalReadinessError
	if !errors.As(err, &readinessErr) {
		t.Fatalf("expected wrapped FinalReadinessError, got %v", err)
	}
	if answer != "" {
		t.Fatalf("mutation-less readiness failure must not salvage, got %q", answer)
	}
}

func TestFinalReadinessFailsImmediatelyWithoutRetries(t *testing.T) {
	newReg := func() *tool.Registry {
		reg := tool.NewRegistry()
		reg.Add(fakeReadFileTool{})
		reg.Add(fakeWriterTool{}) // writer-capable registry keeps mutation expected
		return reg
	}
	finalText := []provider.Chunk{{Type: provider.ChunkText, Text: "done, all fixed"}, {Type: provider.ChunkDone}}
	readCall := func(id string) []provider.Chunk {
		return []provider.Chunk{toolCallChunk(id, "read_file", `{"path":"a.go"}`), {Type: provider.ChunkDone}}
	}

	stalled := &scriptedProvider{name: "p", turns: [][]provider.Chunk{finalText}}
	a := New(stalled, newReg(), NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	err := a.Run(context.Background(), "fix the crash in a.go")
	var readinessErr *FinalReadinessError
	if !errors.As(err, &readinessErr) {
		t.Fatalf("expected FinalReadinessError, got %v", err)
	}
	if readinessErr.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no readiness retries)", readinessErr.Attempts)
	}
	if stalled.call != 1 {
		t.Fatalf("provider calls = %d, want 1 (no hidden retry messages)", stalled.call)
	}
	if !a.deliveryRecoveryPending {
		t.Fatal("delivery recovery must be pending for an explicit continuation")
	}

	converging := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		readCall("1"), finalText,
		readCall("2"), finalText,
	}}
	a2 := New(converging, newReg(), NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	err2 := a2.Run(context.Background(), "fix the crash in a.go")
	var readinessErr2 *FinalReadinessError
	if !errors.As(err2, &readinessErr2) {
		t.Fatalf("expected FinalReadinessError, got %v", err2)
	}
	if converging.call != 2 {
		t.Fatalf("provider calls = %d, want 2 (work turn + one final answer)", converging.call)
	}
}

func TestExplicitDeliveryRecoveryPreservesEvidenceOnce(t *testing.T) {
	reg := evidenceRegistry()
	reg.Add(fakeReadFileTool{})
	finalText := []provider.Chunk{{Type: provider.ChunkText, Text: "premature"}, {Type: provider.ChunkDone}}
	prov := &scriptedProvider{name: "delivery", turns: [][]provider.Chunk{
		{toolCallChunk("todo", "todo_write", `{"todos":[{"content":"Ship main","status":"in_progress"}]}`), {Type: provider.ChunkDone}},
		{toolCallChunk("write", "write_file", `{"path":"main.go"}`), {Type: provider.ChunkDone}},
		finalText,
		{toolCallChunk("review", "read_file", `{"path":"main.go"}`), {Type: provider.ChunkDone}},
		{toolCallChunk("verify", "bash", `{"command":"go test ./..."}`), {Type: provider.ChunkDone}},
		{toolCallChunk("signoff", "complete_step", `{"step":"Ship main","result":"done","evidence":[{"kind":"verification","summary":"tests pass","command":"go test ./..."}]}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "delivered"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	var readinessErr *FinalReadinessError
	if err := a.Run(context.Background(), "implement main"); !errors.As(err, &readinessErr) {
		t.Fatalf("first Run error = %v, want FinalReadinessError", err)
	}
	if !a.PrepareDeliveryRecovery() {
		t.Fatal("explicit recovery should consume the pending readiness failure")
	}
	if a.PrepareDeliveryRecovery() {
		t.Fatal("delivery recovery authorization must be one-shot")
	}
	if err := a.Run(context.Background(), "continue the remaining delivery checks"); err != nil {
		t.Fatalf("recovery Run: %v", err)
	}
	if _, ok := a.evidence.LatestSuccessfulMutationIndex(); !ok {
		t.Fatal("recovery turn lost the prior mutation receipt")
	}
}

func TestOrdinaryFollowUpDoesNotPreserveFailedDeliveryEvidence(t *testing.T) {
	reg := evidenceRegistry()
	finalText := []provider.Chunk{{Type: provider.ChunkText, Text: "premature"}, {Type: provider.ChunkDone}}
	prov := &scriptedProvider{name: "delivery", turns: [][]provider.Chunk{
		{toolCallChunk("todo", "todo_write", `{"todos":[{"content":"Ship main","status":"in_progress"}]}`), {Type: provider.ChunkDone}},
		{toolCallChunk("write", "write_file", `{"path":"main.go"}`), {Type: provider.ChunkDone}},
		finalText,
		finalText,
		finalText,
		finalText,
	}}
	a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
	var firstErr *FinalReadinessError
	if err := a.Run(context.Background(), "implement main"); !errors.As(err, &firstErr) {
		t.Fatalf("first Run error = %v, want FinalReadinessError", err)
	}
	if _, ok := a.evidence.LatestSuccessfulMutationIndex(); !ok {
		t.Fatal("first failed delivery should retain its mutation until the next turn is classified")
	}

	var followUpErr *FinalReadinessError
	if err := a.Run(context.Background(), "fix the unrelated crash in other.go"); !errors.As(err, &followUpErr) {
		t.Fatalf("ordinary follow-up error = %v, want FinalReadinessError", err)
	}
	if _, ok := a.evidence.LatestSuccessfulMutationIndex(); ok {
		t.Fatal("ordinary follow-up inherited stale mutation evidence without explicit recovery")
	}
}

func TestPreviewStripsDeliveryMarkerAndSyntheticTurns(t *testing.T) {
	first := "누구세요?\n\n" + DeliveryRuntimeMarker
	if got := UserPreviewText(first); got != "누구세요?" {
		t.Fatalf("UserPreviewText kept framing: %q", got)
	}
	inline := "Explain this literal: <delivery-runtime>example</delivery-runtime> and keep this sentence"
	if got := UserPreviewText(inline); got != inline {
		t.Fatalf("inline delivery-runtime mention was mangled: %q", got)
	}
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: first},
		{Role: provider.RoleAssistant, Content: "hi"},
		{Role: provider.RoleUser, Content: MidTurnSteerPrefix + "\nslow down"},
		{Role: provider.RoleUser, Content: "콘트라 게임을 만들어 줘\n\n" + DeliveryRuntimeMarker},
	}
	preview, turns := SessionPreviewFromMessages(msgs)
	if preview != "누구세요?" {
		t.Fatalf("preview = %q", preview)
	}
	if turns != 2 {
		t.Fatalf("turns = %d, want 2 (steer excluded)", turns)
	}
}

func TestDeliveryTaskNeedsEvidenceSkipsDiagnosticConversations(t *testing.T) {
// Diagnostic/troubleshooting conversations ask "what's wrong" or "why"
// work  the agent can only give advice, not mutate files.
	diagnostic := []string{
		"what's wrong with my wifi",
		"I don't want to install dependencies",
		"please don't install any dependencies",
		"why can't I install the plugin?",
		"why can't I run WPS?",
		"why can't I check my email in Outlook?",
		"can you analyze why WPS won't open?",
		"why did the plugin update fail?",
		"can you explain why install keeps failing?",
		"why does this make a difference?",
		"why does the node selection matter?",
		"why is `Python` popular?",
		"what does `context.Context` mean?",
		"why can't I open github.com/?",
		"왜 wps가 zetero 참고문헌을 가져올 때 오류가 나는지",
		"왜 플러그인을 설치할 수 없는지",
		"왜 플러그인을 설치하지 못하는지",
		"왜 WPS가 실행되지 않는지",
		"왜 Outlook 메일을 확인할 수 없는지",
		"WPS가 왜 실행되지 않는지 분석해 주세요",
		"왜 플러그인 설치가 실패했는지",
		"왜 구성을 업데이트한 후 오류가 나는지",
		"이게 왜 이런 문제인지 봐 주세요",
		"왜 zotero에 연결이 안 되는지 모르겠어요, 다시 깔기엔 두렵습니다",
		"데이터베이스 연결이 왜 실패했는지 진단해 주세요",
		"이 소프트웨어가 열리지 않는데, 왜 그런가요",
	}
	for _, input := range diagnostic {
		if taskintent.NeedsEvidence(input) {
			t.Errorf("diagnostic input %q incorrectly classified as needing evidence", input)
		}
	}

	taskInputs := []string{
		"fix the crash in a.go",
		"wps의 크래시 문제를 수정해 주세요",
		"create a new login endpoint",
		"단위 테스트를 추가해 주세요",
		"modify the existing config",
		"patch the parser",
		"replace the old endpoint",
		"make the requested changes",
		"기존 구성을 조정해 주세요",
		"기존 인터페이스를 변경해 주세요",
		"thanks for fixing that, now update the tests",
		"고마워요, 계속해서 구성을 수정해 주세요",
	}
	for _, input := range taskInputs {
		if !taskintent.NeedsEvidence(input) {
			t.Errorf("mutation task %q incorrectly classified as NOT needing evidence", input)
		}
	}
}

func TestDeliveryTaskNeedsEvidenceKeepsReadOnlyTechnicalWork(t *testing.T) {
	inputs := []string{
		"review this pull request and report whether it is correct",
		"run go test ./... and tell me why it fails",
		"why does go test fail?",
		"why does go build ./... fail?",
		"why does npm run build fail?",
		"why does git status fail?",
		"why does `custom-lint --strict` fail?",
		"why does ./scripts/verify.sh fail?",
		"왜 go build ./...가 실패하는지",
		"why can't I run main.go?",
		"why does README.md render incorrectly?",
		"reproduce the crash and identify the root cause",
		"inspect main.go for security vulnerabilities",
		"현재 프로젝트의 데이터베이스 연결 실패 원인을 진단해 주세요",
	}
	for _, input := range inputs {
		if !taskintent.NeedsEvidence(input) {
			t.Errorf("read-only technical task %q did not require host-observable evidence", input)
		}
		if taskintent.NeedsMutation(input) {
			t.Errorf("read-only technical task %q incorrectly required a mutation", input)
		}
	}
}

func TestDeliveryTaskNeedsMutationHandlesMixedIntent(t *testing.T) {
	mutationInputs := []string{
		"modify the existing config",
		"patch the parser",
		"make the requested changes",
		"I don't want to install dependencies, but update the existing config",
		"I can't install dependencies; please edit the existing config instead",
		"I can't install dependencies and please update the config",
		"do not install dependencies and please update the config",
		"I can't install dependencies so update the config",
		"I can't install dependencies please update the existing config",
		"can you explain why it fails and fix it",
		"새 의존성을 설치하고 싶지 않아요. 기존 구성을 수정해서 이 문제를 해결해 주세요",
		"새 의존성을 설치할 수 없지만, 기존 구성을 수정해 주세요",
		"새 의존성을 설치할 수 없으니 구성을 수정해 주세요",
		"의존성을 설치하지 말고 구성을 업데이트해 주세요",
		"새 의존성을 설치할 수 없으므로 구성을 수정해 주세요",
		"이 방안이 왜 실패했는지, 수정해 주세요",
		"기존 구성을 조정해 주세요",
		"기존 인터페이스를 변경해 주세요",
	}
	for _, input := range mutationInputs {
		if !taskintent.NeedsMutation(input) {
			t.Errorf("mixed-intent input %q did not require a mutation", input)
		}
	}

	readOnlyInputs := []string{
		"review only; do not fix anything",
		"I don't want to install dependencies",
		"please don't install any dependencies",
		"why can't I install the plugin?",
		"do not install and update dependencies",
		"don't fix or update anything",
		"분석만 하고 코드는 건드리지 마세요",
		"의존성은 새로 깔거나 갱신하지 말아 주세요",
		"팀이 코드를 건드리기를 원하지 않아요",
		"구성은 그대로 두세요",
		"왜 플러그인을 설치할 수 없는지",
		"왜 플러그인을 설치하지 못하는지",
		"왜 zotero에 연결이 안 되는지 모르겠어요, 다시 깔기엔 두렵습니다",
	}
	for _, input := range readOnlyInputs {
		if taskintent.NeedsMutation(input) {
			t.Errorf("read-only input %q incorrectly required a mutation", input)
		}
	}
}

func TestDeliveryMixedIntentRequiresMutationEvidence(t *testing.T) {
	inputs := []string{
		"I can't install dependencies and please update the config",
		"새 의존성을 설치할 수 없으니 구성을 수정해 주세요",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			reg := tool.NewRegistry()
			reg.Add(fakeReadFileTool{})
			reg.Add(fakeWriterTool{})
			answer := []provider.Chunk{
				{Type: provider.ChunkText, Text: "Done; the config is updated."},
				{Type: provider.ChunkDone},
			}
			prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{answer, answer, answer}}
			a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
			err := a.Run(context.Background(), input)
			var readinessErr *FinalReadinessError
			if !errors.As(err, &readinessErr) {
				t.Fatalf("text-only completion escaped the mutation gate: %v", err)
			}
			if !strings.Contains(readinessErr.Reason, "state change") {
				t.Fatalf("readiness reason = %q, want missing state change", readinessErr.Reason)
			}
		})
	}
}

func TestDeliveryReadOnlyTechnicalTaskRequiresEvidence(t *testing.T) {
	inputs := []string{
		"review this pull request and report whether it is correct",
		"why does go build ./... fail?",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			reg := tool.NewRegistry()
			reg.Add(fakeReadFileTool{})
			reg.Add(fakeWriterTool{})
			answer := []provider.Chunk{
				{Type: provider.ChunkText, Text: "Reviewed; everything is correct."},
				{Type: provider.ChunkDone},
			}
			prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{answer, answer, answer}}
			a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
			err := a.Run(context.Background(), input)
			var readinessErr *FinalReadinessError
			if !errors.As(err, &readinessErr) {
				t.Fatalf("text-only technical work escaped the evidence gate: %v", err)
			}
			if !strings.Contains(readinessErr.Reason, "host-observable work") {
				t.Fatalf("readiness reason = %q, want missing host-observable work", readinessErr.Reason)
			}
			if strings.Contains(readinessErr.Reason, "state change") {
				t.Fatalf("read-only work incorrectly required a mutation: %q", readinessErr.Reason)
			}
		})
	}
}

func TestDeliveryDiagnosticConversationCompletes(t *testing.T) {
// keywords must complete without a FinalReadinessError  the agent can
// give advice but can't write files on the user's machine.
	inputs := []string{
		"왜 wps가 zetero 참고문헌을 가져올 때 오류가 나는지, 진단해 주세요",
		"WPS가 왜 실행되지 않는지 분석해 주세요",
		"why can't I check my email in Outlook?",
		"what does `context.Context` mean?",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			reg := tool.NewRegistry()
			reg.Add(fakeReadFileTool{})
			reg.Add(fakeWriterTool{})
// The model gives advice text (no tool calls)  a diagnostic response.
			advice := []provider.Chunk{
				{Type: provider.ChunkText, Text: "다음 단계를 시도해 보세요: 1. 포트 수신 상태 확인 2. 플러그인 다시 등록"},
				{Type: provider.ChunkDone},
			}
			prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{advice}}
			a := New(prov, reg, NewSession("sys"), Options{DeliveryProfile: true}, event.Discard)
			if err := a.Run(context.Background(), input); err != nil {
				t.Fatalf("diagnostic conversation deadlocked: %v", err)
			}
			if prov.call != 1 {
				t.Fatalf("diagnostic conversation had %d provider calls, want 1 (no readiness retries)", prov.call)
			}
		})
	}
}
