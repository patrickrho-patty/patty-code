package bot

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/event"
)

func TestApprovalCardCarriesChatType(t *testing.T) {
	if got := renderApprovalText(event.Approval{
		ID: "r1", Tool: "write_file", Subject: "a.go", Kind: "recovery",
		Recovery: &event.RecoveryApproval{
			FailedTool: "bash", FailedSummary: "exit 1", Diagnosis: "nil pointer",
			NextTool: "write_file", NextAction: "edit a.go", ChangeRationale: "strategy change",
			SourceAgent: "subagent",
		},
	}); !strings.Contains(got, "실행 전 확인") || !strings.Contains(got, "1을 답변하면 계속，2를 답변하면 다른 방법으로") || strings.Contains(got, "Auto Guard") {
		t.Fatalf("recovery text = %q", got)
	}

	grantApproval := event.Approval{ID: "r2", Kind: "recovery", Recovery: &event.RecoveryApproval{
		CanGrantTask: true, TaskGrantScope: "git push origin → feature",
	}}
	if got := renderRecoveryText(grantApproval); !strings.Contains(got, "2를 답변하면 이 작업 내에서 동일한 유형의 작업 허용") ||
		!strings.Contains(got, "위험 레벨 상승 시 다시 확인") || !strings.Contains(got, "권한 범위: git push origin → feature") {
		t.Fatalf("task-grant recovery text = %q", got)
	}
	keyboard := recoveryKeyboard(grantApproval)
	if len(keyboard.Rows) != 2 || keyboard.Rows[0].Buttons[1].CallbackID != "/recovery-continue-task r2" {
		t.Fatalf("task-grant keyboard = %#v", keyboard)
	}
	grantCard := recoveryCard(grantApproval, ChatDM, "allowed-user")
	grantActions, ok := grantCard.Elements[1].Extra["actions"].([]map[string]any)
	if !ok || len(grantActions) != 3 {
		t.Fatalf("task-grant card actions = %#v", grantCard.Elements[1].Extra["actions"])
	}

	planApproval := event.Approval{ID: "r3", Kind: "recovery", Recovery: &event.RecoveryApproval{
		ChangeKind: "strategy", NextAction: "replace the storage backend", ChangeRationale: "the original approach cannot satisfy the requirement",
		PlanBefore: "1. Keep the current storage backend", PlanAfter: "1. Replace the storage backend",
	}}
	if got := renderRecoveryText(planApproval); !strings.Contains(got, "실행 계획 결정 필요") ||
		!strings.Contains(got, "기존 계획:\n1. Keep the current storage backend") ||
		!strings.Contains(got, "새 계획:\n1. Replace the storage backend") ||
		!strings.Contains(got, "1을 답변하여 새 계획을 채택하고 계속하세요，2 채택하지 않고 Auto에게 조정 요청") {
		t.Fatalf("plan-change recovery text = %q", got)
	}
	planKeyboard := recoveryKeyboard(planApproval)
	if len(planKeyboard.Rows) != 1 || planKeyboard.Rows[0].Buttons[0].Label != "1 채택하고 계속" || planKeyboard.Rows[0].Buttons[0].Style != 0 {
		t.Fatalf("plan-change keyboard = %#v", planKeyboard)
	}
	planCard := recoveryCard(planApproval, ChatDM, "allowed-user")
	if planCard.Header != "실행 계획 결정 필요" {
		t.Fatalf("plan-change card header = %q", planCard.Header)
	}

	card := approvalCard(event.Approval{ID: "approval-1", Tool: "bash", Subject: "ls"}, ChatDM, "allowed-user")
	if len(card.Elements) < 2 {
		t.Fatalf("approval card elements = %d, want at least 2", len(card.Elements))
	}
	actions, ok := card.Elements[1].Extra["actions"].([]map[string]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("approval card actions missing or wrong type: %#v", card.Elements[1].Extra["actions"])
	}
	value, ok := actions[0]["value"].(map[string]string)
	if !ok {
		t.Fatalf("approval action value has wrong type: %#v", actions[0]["value"])
	}
	if value["command"] != "/approve approval-1" {
		t.Fatalf("command = %q, want /approve approval-1", value["command"])
	}
	if value["chat_type"] != string(ChatDM) {
		t.Fatalf("chat_type = %q, want %q", value["chat_type"], ChatDM)
	}
	if value["user_id"] != "allowed-user" {
		t.Fatalf("user_id = %q, want allowed-user", value["user_id"])
	}
}

func TestApprovalCardActionsAreToolAgnostic(t *testing.T) {
	for _, approval := range []event.Approval{
		{ID: "plan-1", Tool: "exit_plan_mode", Subject: "plan"},
		{ID: "task-1", Tool: "task", Subject: "run subtask"},
	} {
		card := approvalCard(approval, ChatGroup, "allowed-user")
		if len(card.Elements) < 2 {
			t.Fatalf("%s card elements = %d, want actions", approval.Tool, len(card.Elements))
		}
		actions, ok := card.Elements[1].Extra["actions"].([]map[string]any)
		if !ok || len(actions) != 2 {
			t.Fatalf("%s actions missing or wrong type: %#v", approval.Tool, card.Elements[1].Extra["actions"])
		}
		allow, ok := actions[0]["value"].(map[string]string)
		if !ok {
			t.Fatalf("%s allow value has wrong type: %#v", approval.Tool, actions[0]["value"])
		}
		deny, ok := actions[1]["value"].(map[string]string)
		if !ok {
			t.Fatalf("%s deny value has wrong type: %#v", approval.Tool, actions[1]["value"])
		}
		if allow["command"] != "/approve "+approval.ID || deny["command"] != "/deny "+approval.ID {
			t.Fatalf("%s commands = %q/%q, want approve/deny by id", approval.Tool, allow["command"], deny["command"])
		}
	}
}

func TestAskCardAddsAnswerButtonsForSingleChoice(t *testing.T) {
	card := askCard(event.Ask{
		ID: "ask-1",
		Questions: []event.AskQuestion{{
			ID:     "q1",
			Prompt: "Choose one",
			Options: []event.AskOption{
				{Label: "한 번 허용"},
				{Label: "거절"},
			},
		}},
	}, "fallback", ChatDM, "allowed-user")

	if len(card.Elements) != 2 {
		t.Fatalf("ask card elements = %d, want markdown + actions", len(card.Elements))
	}
	actions, ok := card.Elements[1].Extra["actions"].([]map[string]any)
	if !ok || len(actions) != 2 {
		t.Fatalf("ask card actions missing or wrong type: %#v", card.Elements[1].Extra["actions"])
	}
	value, ok := actions[0]["value"].(map[string]string)
	if !ok {
		t.Fatalf("ask action value has wrong type: %#v", actions[0]["value"])
	}
	if value["command"] != "/answer ask-1 1" {
		t.Fatalf("command = %q, want /answer ask-1 1", value["command"])
	}
	if value["chat_type"] != string(ChatDM) {
		t.Fatalf("chat_type = %q, want %q", value["chat_type"], ChatDM)
	}
	if value["user_id"] != "allowed-user" {
		t.Fatalf("user_id = %q, want allowed-user", value["user_id"])
	}
}

func TestRenderSinkDoesNotFlushMidSentenceOnTimer(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	sink.lastFlush = time.Now().Add(-2 * time.Second)

	sink.Emit(event.Event{Kind: event.Text, Text: "저는 **"})
	sink.Emit(event.Event{Kind: event.Text, Text: "Patty Code**，코드 실행 작업에 집중하는 AI 프로그래밍 어시스턴트"})

	if sent := adapter.sentMessages(); len(sent) != 0 {
		t.Fatalf("sent = %+v, want no mid-sentence flush", sent)
	}

	sink.Emit(event.Event{Kind: event.TurnDone})
	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want final flush only", len(sent))
	}
	if sent[0].Text != "저는 **Patty Code**，코드 실행 작업에 집중하는 AI 프로그래밍 어시스턴트" {
		t.Fatalf("sent text = %q, want combined sentence", sent[0].Text)
	}
}

func TestRenderSinkKeepsSemanticTextUntilFinalResult(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	sink.lastFlush = time.Now().Add(-2 * time.Second)

	sink.Emit(event.Event{Kind: event.Text, Text: "첫 번째 문장。"})

	if sent := adapter.sentMessages(); len(sent) != 0 {
		t.Fatalf("sent = %+v, want semantic text held until final result", sent)
	}

	sink.Emit(event.Event{Kind: event.TurnDone})
	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want final result only", len(sent))
	}
	if sent[0].Text != "첫 번째 문장。" {
		t.Fatalf("sent text = %q, want final result", sent[0].Text)
	}
}

func TestRenderSinkFinalFlushKeepsChunkLimit(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	sink.buf.WriteString(strings.Repeat("긴", renderMaxChunkRunes*2+10))

	sink.Emit(event.Event{Kind: event.TurnDone})

	sent := adapter.sentMessages()
	if len(sent) < 2 {
		t.Fatalf("sent count = %d, want chunked final flush", len(sent))
	}
	for i, msg := range sent {
		if got := len([]rune(msg.Text)); got > renderMaxChunkRunes {
			t.Fatalf("sent[%d] runes = %d, want <= %d", i, got, renderMaxChunkRunes)
		}
	}
}

func TestRenderSinkConsumesEmptyWhitespacePrefix(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	sink.buf.WriteString("\n도구 상태")

	sink.flushPrefix(1)

	if got := sink.buf.String(); got != "도구 상태" {
		t.Fatalf("buffer = %q, want leading newline consumed", got)
	}
	if sent := adapter.sentMessages(); len(sent) != 0 {
		t.Fatalf("sent = %+v, want no empty outbound message", sent)
	}
}

func TestRenderSinkSendsProgressWithoutToolOutput(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.Emit(event.Event{Kind: event.TurnStarted})
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "tool-1", Name: "read_file", ReadOnly: true}})
	sink.lastProgress = time.Now().Add(-renderProgressMinInterval)
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "tool-1", Name: "read_file", ReadOnly: true, Refreshed: true}})
	sink.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{ID: "tool-1", Name: "read_file", Output: "secret output that should stay out of IM"}})
	sink.Emit(event.Event{Kind: event.Text, Text: "완료。"})
	sink.Emit(event.Event{Kind: event.TurnDone})

	sent := adapter.sentMessages()
	if len(sent) != 2 {
		t.Fatalf("sent count = %d, want one progress message plus final result: %+v", len(sent), sent)
	}
	if sent[0].Text != "실행 중: read_file" {
		t.Fatalf("progress text = %q, want concise tool status", sent[0].Text)
	}
	if strings.Contains(sent[0].Text, "secret output") || strings.Contains(sent[1].Text, "secret output") {
		t.Fatalf("tool output leaked into IM messages: %+v", sent)
	}
	if sent[1].Text != "완료。" {
		t.Fatalf("final text = %q, want final result only", sent[1].Text)
	}
}

func TestRenderSinkLimitsProgressMessages(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	for range renderMaxProgressMessages + 2 {
		sink.lastProgress = time.Now().Add(-renderProgressMinInterval)
		sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "tool", Name: "bash"}})
	}

	sent := adapter.sentMessages()
	if len(sent) != renderMaxProgressMessages {
		t.Fatalf("sent count = %d, want capped progress count %d", len(sent), renderMaxProgressMessages)
	}
}

type fakeEditorAdapter struct {
	*fakeAdapter
	mu      sync.Mutex
	edits   []editRecord
	editErr error
}

type editRecord struct {
	messageID string
	text      string
}

func newFakeEditorAdapter() *fakeEditorAdapter {
	return &fakeEditorAdapter{fakeAdapter: newFakeAdapter(Platform("custom"), "fake-custom")}
}

func (f *fakeEditorAdapter) EditMessage(ctx context.Context, messageID string, msg OutboundMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.editErr != nil {
		return f.editErr
	}
	f.edits = append(f.edits, editRecord{messageID: messageID, text: msg.Text})
	return nil
}

func (f *fakeEditorAdapter) editRecords() []editRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]editRecord, len(f.edits))
	copy(out, f.edits)
	return out
}

func TestRenderSinkStreamsIntoLiveMessage(t *testing.T) {
	adapter := newFakeEditorAdapter()
	sink := newRenderSink(context.Background(), adapter, "custom-main", "custom", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	// 첫 번째 증분：소프트 윈도우를 초과한 후 live 메시지를 생성합니다。
	sink.lastFlush = time.Now().Add(-2 * renderSoftFlushAfter)
	sink.Emit(event.Event{Kind: event.Text, Text: "첫 번째 내용"})
	if sent := adapter.sentMessages(); len(sent) != 1 || sent[0].Text != "첫 번째 내용" {
		t.Fatalf("sent = %+v, want live message created with first chunk", sent)
	}
	if sink.liveMsgID != "fake_msg_1" {
		t.Fatalf("liveMsgID = %q, want fake_msg_1", sink.liveMsgID)
	}

	// 두 번째 증분：같은 메시지를 제자리에서 편집하고 새로 보내지 않습니다。
	sink.lastEdit = time.Now().Add(-2 * renderSoftFlushAfter)
	sink.Emit(event.Event{Kind: event.Text, Text: "，두 번째 내용。"})
	if sent := adapter.sentMessages(); len(sent) != 1 {
		t.Fatalf("sent count = %d, want still one message after streaming edit", len(sent))
	}
	edits := adapter.editRecords()
	if len(edits) != 1 || edits[0].messageID != "fake_msg_1" {
		t.Fatalf("edits = %+v, want one edit to live message", edits)
	}
	if edits[0].text != "첫 번째 내용，두 번째 내용。" {
		t.Fatalf("edit text = %q, want cumulative content", edits[0].text)
	}

	// 턴 종료：최종 내용을 live 메시지에 편집하고 새로 보내지 않습니다。
	sink.Emit(event.Event{Kind: event.Text, Text: "마무리。"})
	sink.Emit(event.Event{Kind: event.TurnDone})
	if sent := adapter.sentMessages(); len(sent) != 1 {
		t.Fatalf("sent count = %d, want no extra message at turn end", len(sent))
	}
	edits = adapter.editRecords()
	final := edits[len(edits)-1]
	if final.text != "첫 번째 내용，두 번째 내용。마무리。" {
		t.Fatalf("final edit = %q, want full content", final.text)
	}
	if sink.liveMsgID != "" {
		t.Fatalf("liveMsgID = %q, want cleared after turn done", sink.liveMsgID)
	}
}

func TestRenderSinkStreamingThrottledBySoftWindow(t *testing.T) {
	adapter := newFakeEditorAdapter()
	sink := newRenderSink(context.Background(), adapter, "custom-main", "custom", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	// 소프트 윈도우 안의 증분은 네트워크 호출을 발생시키지 않습니다。
	sink.Emit(event.Event{Kind: event.Text, Text: "방금 시작한 내용"})
	if sent := adapter.sentMessages(); len(sent) != 0 {
		t.Fatalf("sent = %+v, want throttled inside soft window", sent)
	}
	if edits := adapter.editRecords(); len(edits) != 0 {
		t.Fatalf("edits = %+v, want none inside soft window", edits)
	}
}

func TestRenderSinkStreamingEditFailureRotatesWithoutDuplication(t *testing.T) {
	adapter := newFakeEditorAdapter()
	sink := newRenderSink(context.Background(), adapter, "custom-main", "custom", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.lastFlush = time.Now().Add(-2 * renderSoftFlushAfter)
	sink.Emit(event.Event{Kind: event.Text, Text: "이미 전송된 내용。"})
	if sink.liveMsgID == "" {
		t.Fatal("live message should be created")
	}

	// 편집 실패：블록을 교체하고 이미 전송된 앞부분은 다시 보내지 않습니다。
	adapter.mu.Lock()
	adapter.editErr = context.DeadlineExceeded
	adapter.mu.Unlock()
	sink.lastEdit = time.Now().Add(-2 * renderSoftFlushAfter)
	sink.Emit(event.Event{Kind: event.Text, Text: "이어지는 내용。"})
	if sink.liveMsgID != "" {
		t.Fatalf("liveMsgID = %q, want rotation after edit failure", sink.liveMsgID)
	}

	adapter.mu.Lock()
	adapter.editErr = nil
	adapter.mu.Unlock()
	sink.Emit(event.Event{Kind: event.TurnDone})
	sent := adapter.sentMessages()
	if len(sent) != 2 {
		t.Fatalf("sent count = %d, want live message plus rotated tail: %+v", len(sent), sent)
	}
	if sent[1].Text != "이어지는 내용。" {
		t.Fatalf("rotated tail = %q, want only undelivered content", sent[1].Text)
	}
}

func TestRenderSinkStreamingHardCapRotatesBlocks(t *testing.T) {
	adapter := newFakeEditorAdapter()
	sink := newRenderSink(context.Background(), adapter, "custom-main", "custom", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.lastFlush = time.Now().Add(-2 * renderSoftFlushAfter)
	sink.Emit(event.Event{Kind: event.Text, Text: "첫 번째 문장。"})
	if sink.liveMsgID == "" {
		t.Fatal("live message should be created")
	}

	// 하드 상한 초과：live 메시지를 의미 경계에서 마무리하고 나머지는 다음 블록으로 넘깁니다。
	sink.Emit(event.Event{Kind: event.Text, Text: strings.Repeat("긴", renderHardChunkRunes) + "。꼬리"})
	if sink.liveMsgID != "" {
		t.Fatalf("liveMsgID = %q, want block closed at hard cap", sink.liveMsgID)
	}
	edits := adapter.editRecords()
	if len(edits) == 0 {
		t.Fatal("hard cap should finalize the live message via edit")
	}
	if got := len([]rune(edits[len(edits)-1].text)); got > renderMaxChunkRunes {
		t.Fatalf("finalized block runes = %d, want <= %d", got, renderMaxChunkRunes)
	}

	sink.Emit(event.Event{Kind: event.TurnDone})
	sent := adapter.sentMessages()
	if len(sent) < 2 {
		t.Fatalf("sent count = %d, want new message for the next block", len(sent))
	}
}

// streaming engages) but fails every edit, simulating a rate-limited  recalled
type failingEditorAdapter struct {
	*fakeAdapter
}

func (f *failingEditorAdapter) EditMessage(ctx context.Context, id string, msg OutboundMessage) error {
	return fmt.Errorf("simulated edit failure")
}

func TestRenderSinkStreamingEditFailureDoesNotDuplicate(t *testing.T) {
	adapter := &failingEditorAdapter{fakeAdapter: newFakeAdapter(Platform("custom"), "fake-custom")}
	sink := newRenderSink(context.Background(), adapter, "custom-main", "custom", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.lastFlush = time.Now().Add(-2 * renderSoftFlushAfter)
	head := strings.Repeat("a", 2000)
	sink.Emit(event.Event{Kind: event.Text, Text: head})
	if sink.liveMsgID == "" {
		t.Fatalf("streaming did not engage; sent=%d", len(adapter.sentMessages()))
	}
	tail := strings.Repeat("b", renderHardChunkRunes)
	sink.Emit(event.Event{Kind: event.Text, Text: tail})
	sink.Emit(event.Event{Kind: event.TurnDone})

	var shown strings.Builder
	shown.WriteString(head) // live message frozen at last successful state
	for _, m := range adapter.sentMessages()[1:] {
		shown.WriteString(m.Text)
	}
	wantRunes := len([]rune(head + tail))
	if gotRunes := len([]rune(shown.String())); gotRunes > wantRunes {
		t.Fatalf("duplication: user would see %d runes, expected at most %d (%d duplicated)", gotRunes, wantRunes, gotRunes-wantRunes)
	}
}

func TestRenderSinkStreamingFinalizesInOneEditWithoutSplit(t *testing.T) {
	adapter := newFakeEditorAdapter()
	sink := newRenderSink(context.Background(), adapter, "custom-main", "custom", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.lastFlush = time.Now().Add(-2 * renderSoftFlushAfter)
	sink.Emit(event.Event{Kind: event.Text, Text: "결론은 다음과 같습니다. 코드는 여기:\n```go\nfmt.Println(\"x\")\n```"})
	sink.Emit(event.Event{Kind: event.TurnDone})

	if sent := adapter.sentMessages(); len(sent) != 1 {
		t.Fatalf("sent %d messages, want exactly one live message (no split): %+v", len(sent), sent)
	}
	edits := adapter.editRecords()
	if len(edits) == 0 || !strings.Contains(edits[len(edits)-1].text, "```") {
		t.Fatalf("final edit should carry the full text including the code fence: %+v", edits)
	}
}

func TestRenderSinkSuppressesReasoning(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.Emit(event.Event{Kind: event.Reasoning, Text: "internal reasoning"})
	sink.Emit(event.Event{Kind: event.Text, Text: "보이는 결과"})
	sink.Emit(event.Event{Kind: event.TurnDone})

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want one final result", len(sent))
	}
	if strings.Contains(sent[0].Text, "internal reasoning") {
		t.Fatalf("reasoning leaked into IM message: %q", sent[0].Text)
	}
}

func TestRenderSinkSuppressesOperatorNoticesWithoutHidingUserWarnings(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "please resend your message"})
	for _, code := range []string{
		event.NoticeCodeSessionRecoveryForked,
		event.NoticeCodeSessionRecoveryAdopted,
		event.NoticeCodeSessionRecoveryAdoptedCovered,
		event.NoticeCodeSessionRecoveryDepthCap,
		event.NoticeCodeSessionShutdownRecoveryForked,
	} {
		sink.Emit(event.Event{
			Kind: event.Notice, Level: event.LevelWarn,
			Audience: event.NoticeAudienceOperator,
			Code:     code,
			Text:     "local session maintenance",
		})
	}

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent = %+v, want only the actionable user warning", sent)
	}
	if sent[0].Text != "⚠️ please resend your message" {
		t.Fatalf("sent text = %q, want the ordinary user warning", sent[0].Text)
	}
}

func TestRenderSinkIgnoresSubagentProgress(t *testing.T) {
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sink := newRenderSink(context.Background(), adapter, "relay-main", "relay", "chat-1", ChatDM, "user-1", "msg-1", slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)

	sink.Emit(event.Event{Kind: event.TurnStarted})
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "task-1", Name: "task"}})
	sink.Emit(event.Event{Kind: event.ToolProgress, Tool: event.Tool{
		ID: "task-1", Name: event.SubagentProgressStatusName, Output: "reasoning",
	}})
	sink.Emit(event.Event{Kind: event.ToolProgress, Tool: event.Tool{
		ID: "task-1", Name: event.SubagentProgressReasoningName, Output: "thinking out loud",
	}})
	sink.Emit(event.Event{Kind: event.ToolProgress, Tool: event.Tool{
		ID: "task-1", Name: event.SubagentProgressTextName, Output: "answer preview",
	}})
	sink.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{ID: "task-1", Name: "task", Output: "final"}})
	sink.Emit(event.Event{Kind: event.TurnDone})

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want only the dispatch status: %+v", len(sent), sent)
	}
	for i, m := range sent {
		if strings.Contains(m.Text, "thinking out loud") || strings.Contains(m.Text, "answer preview") {
			t.Fatalf("sub-agent preview leaked into IM message %d: %+v", i, m)
		}
	}
}
