package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"patty/internal/event"
)

type fakeDesktopBridge struct {
	sessions  []DesktopSessionInfo
	watching  map[string]bool
	approved  []string
	denied    []string
	answered  map[string][]event.AskAnswer
	questions map[string][]event.AskQuestion
	takeovers map[string]string // routeKey -> tabID
	driven    []string
	driveErr  error
	watchErr  error
}

func newFakeDesktopBridge() *fakeDesktopBridge {
	return &fakeDesktopBridge{
		watching:  make(map[string]bool),
		answered:  make(map[string][]event.AskAnswer),
		questions: make(map[string][]event.AskQuestion),
		takeovers: make(map[string]string),
	}
}

func (f *fakeDesktopBridge) Sessions() []DesktopSessionInfo { return f.sessions }
func (f *fakeDesktopBridge) SetWatch(route DesktopWatchRoute, enable bool) error {
	f.watching[route.Key()] = enable
	return f.watchErr
}
func (f *fakeDesktopBridge) Watching(route DesktopWatchRoute) bool {
	return f.watching[route.Key()]
}
func (f *fakeDesktopBridge) Approve(id string, allow bool) (string, error) {
	if id == "gone" {
		return "", fmt.Errorf("처리 대기 중인 승인을 찾을 수 없음: %s", id)
	}
	if allow {
		f.approved = append(f.approved, id)
	} else {
		f.denied = append(f.denied, id)
	}
	return "제출됨", nil
}
func (f *fakeDesktopBridge) AskQuestions(id string) ([]event.AskQuestion, bool) {
	qs, ok := f.questions[id]
	return qs, ok
}
func (f *fakeDesktopBridge) Answer(id string, answers []event.AskAnswer) (string, error) {
	f.answered[id] = answers
	return "답변 제출됨", nil
}

func (f *fakeDesktopBridge) Takeover(route DesktopWatchRoute, tabID string) (string, error) {
	for _, s := range f.sessions {
		if s.TabID == tabID {
			f.takeovers[route.Key()] = tabID
			return "인수 완료", nil
		}
	}
	return "", fmt.Errorf("세션을 찾을 수 없음: %s", tabID)
}

func (f *fakeDesktopBridge) Release(route DesktopWatchRoute) (string, error) {
	if _, ok := f.takeovers[route.Key()]; !ok {
		return "", fmt.Errorf("이 채팅은 현재 데스크톱 세션을 인수하지 않았습니다.")
	}
	delete(f.takeovers, route.Key())
	return "인수 해제 완료", nil
}

func (f *fakeDesktopBridge) TakeoverTab(route DesktopWatchRoute) string {
	return f.takeovers[route.Key()]
}

func (f *fakeDesktopBridge) DriveInput(route DesktopWatchRoute, text string) (string, error) {
	if f.driveErr != nil {
		return "", f.driveErr
	}
	f.driven = append(f.driven, text)
	return "", nil
}

func desktopTestMessage(text string) InboundMessage {
	return InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom-main",
		Domain:       "custom",
		ChatType:     ChatDM,
		ChatID:       "chat-god",
		UserID:       "admin-user",
		Text:         text,
	}
}

func TestHandleDesktopCommandWithoutBridge(t *testing.T) {
	gw := &BotGateway{cfg: GatewayConfig{}}
	got := gw.handleDesktopCommand(desktopTestMessage("/desktop status"))
	if !strings.Contains(got, "데스크톱 프로세스에서 실행되지 않음") {
		t.Fatalf("reply = %q, want standalone-mode notice", got)
	}
}

func TestHandleDesktopCommandStatusListsSessions(t *testing.T) {
	bridge := newFakeDesktopBridge()
	bridge.sessions = []DesktopSessionInfo{
		{TabID: "tab-1", Label: "로그인 수정", Workspace: "blade", Running: true, Ready: true},
		{TabID: "tab-2", Label: "", Topic: "주간 보고", Ready: true, PendingPrompt: true},
	}
	gw := &BotGateway{cfg: GatewayConfig{Desktop: bridge}}

	got := gw.handleDesktopCommand(desktopTestMessage("/desktop status"))
	for _, want := range []string{"2 개", "로그인 수정", "▶️ 실행 중", "주간 보고", "⚠️ 승인/답변 대기", "tab-1", "blade"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status reply = %q, want it to contain %q", got, want)
		}
	}
}

func TestHandleDesktopCommandStatusListsPendingIDs(t *testing.T) {
	bridge := newFakeDesktopBridge()
	bridge.sessions = []DesktopSessionInfo{{
		TabID: "tab-1", Label: "로그인 수정", Ready: true, PendingPrompt: true,
		Pending: []DesktopPendingInfo{{ID: "appr-9", Kind: "approval", Tool: "bash"}},
	}}
	gw := &BotGateway{cfg: GatewayConfig{Desktop: bridge}}
	got := gw.handleDesktopCommand(desktopTestMessage("/desktop status"))
	// run /desktop approve <id>.
	if !strings.Contains(got, "appr-9") {
		t.Fatalf("status = %q, want it to list the pending approval id", got)
	}
}

func TestHandleDesktopCommandStatusEmpty(t *testing.T) {
	gw := &BotGateway{cfg: GatewayConfig{Desktop: newFakeDesktopBridge()}}
	got := gw.handleDesktopCommand(desktopTestMessage("/desktop"))
	if !strings.Contains(got, "live 세션 없음") {
		t.Fatalf("reply = %q, want empty-sessions notice", got)
	}
}

func TestHandleDesktopCommandWatchLifecycle(t *testing.T) {
	bridge := newFakeDesktopBridge()
	gw := &BotGateway{cfg: GatewayConfig{Desktop: bridge}}
	msg := desktopTestMessage("/desktop watch on")

	got := gw.handleDesktopCommand(msg)
	if !strings.Contains(got, "구독했습니다") {
		t.Fatalf("watch on reply = %q", got)
	}
	route := desktopRouteFromMessage(msg)
	if !bridge.watching[route.Key()] {
		t.Fatal("watch on did not subscribe the message route")
	}

	msg.Text = "/desktop watch off"
	got = gw.handleDesktopCommand(msg)
	if !strings.Contains(got, "구독 해지됨") {
		t.Fatalf("watch off reply = %q", got)
	}
	if bridge.watching[route.Key()] {
		t.Fatal("watch off did not unsubscribe the message route")
	}
}

func TestHandleDesktopCommandWatchReportsPersistenceFailure(t *testing.T) {
	bridge := newFakeDesktopBridge()
	bridge.watchErr = fmt.Errorf("disk unavailable")
	gw := &BotGateway{cfg: GatewayConfig{Desktop: bridge}}
	msg := desktopTestMessage("/desktop watch on")

	got := gw.handleDesktopCommand(msg)
	if !strings.Contains(got, "이번 실행 중") || !strings.Contains(got, "저장 실패") {
		t.Fatalf("watch persistence failure reply = %q", got)
	}
	if !bridge.Watching(desktopRouteFromMessage(msg)) {
		t.Fatal("runtime subscription should remain active after persistence failure")
	}
}

func TestHandleDesktopCommandApproveAndDeny(t *testing.T) {
	bridge := newFakeDesktopBridge()
	gw := &BotGateway{cfg: GatewayConfig{Desktop: bridge}}

	if got := gw.handleDesktopCommand(desktopTestMessage("/desktop approve appr-1")); !strings.Contains(got, "제출됨") {
		t.Fatalf("approve reply = %q", got)
	}
	if got := gw.handleDesktopCommand(desktopTestMessage("/desktop deny appr-2")); !strings.Contains(got, "제출됨") {
		t.Fatalf("deny reply = %q", got)
	}
	if len(bridge.approved) != 1 || bridge.approved[0] != "appr-1" {
		t.Fatalf("approved = %v, want [appr-1]", bridge.approved)
	}
	if len(bridge.denied) != 1 || bridge.denied[0] != "appr-2" {
		t.Fatalf("denied = %v, want [appr-2]", bridge.denied)
	}

	if got := gw.handleDesktopCommand(desktopTestMessage("/desktop approve gone")); !strings.Contains(got, "찾을 수 없음") {
		t.Fatalf("missing-approval reply = %q", got)
	}
	if got := gw.handleDesktopCommand(desktopTestMessage("/desktop approve")); got != desktopCommandUsage {
		t.Fatalf("missing-arg reply = %q, want usage", got)
	}
}

func TestHandleDesktopCommandAnswerParsesSelection(t *testing.T) {
	bridge := newFakeDesktopBridge()
	bridge.questions["ask-1"] = []event.AskQuestion{{
		ID:      "q1",
		Prompt:  "하나를 선택하세요",
		Options: []event.AskOption{{Label: "옵션 A"}, {Label: "옵션 B"}},
	}}
	gw := &BotGateway{cfg: GatewayConfig{Desktop: bridge}}

	got := gw.handleDesktopCommand(desktopTestMessage("/desktop answer ask-1 2"))
	if !strings.Contains(got, "답변 제출됨") {
		t.Fatalf("answer reply = %q", got)
	}
	answers := bridge.answered["ask-1"]
	if len(answers) != 1 || answers[0].QuestionID != "q1" {
		t.Fatalf("answers = %+v, want one answer for q1", answers)
	}
	if len(answers[0].Selected) != 1 || answers[0].Selected[0] != "옵션 B" {
		t.Fatalf("selected = %v, want numeric index resolved to 옵션 B", answers[0].Selected)
	}

	if got := gw.handleDesktopCommand(desktopTestMessage("/desktop answer ask-gone 1")); !strings.Contains(got, "없음") {
		t.Fatalf("missing-ask reply = %q", got)
	}
}

func TestHandleDesktopCommandTakeoverAndRelease(t *testing.T) {
	bridge := newFakeDesktopBridge()
	bridge.sessions = []DesktopSessionInfo{{TabID: "tab-1", Label: "세션 1"}}
	gw := &BotGateway{cfg: GatewayConfig{Desktop: bridge}}
	msg := desktopTestMessage("/desktop takeover tab-1")

	if got := gw.handleDesktopCommand(msg); !strings.Contains(got, "인수 완료") {
		t.Fatalf("takeover reply = %q", got)
	}
	route := desktopRouteFromMessage(msg)
	if bridge.takeovers[route.Key()] != "tab-1" {
		t.Fatalf("takeovers = %v, want route bound to tab-1", bridge.takeovers)
	}

	msg.Text = "/desktop release"
	if got := gw.handleDesktopCommand(msg); !strings.Contains(got, "인수 해제") {
		t.Fatalf("release reply = %q", got)
	}
	if got := gw.handleDesktopCommand(desktopTestMessage("/desktop takeover missing")); !strings.Contains(got, "찾을 수 없음") {
		t.Fatalf("missing-tab reply = %q", got)
	}
}

func TestDivertToDesktopTakeover(t *testing.T) {
	bridge := newFakeDesktopBridge()
	bridge.sessions = []DesktopSessionInfo{{TabID: "tab-1", Label: "세션 1"}}
	gw := &BotGateway{
		cfg:           GatewayConfig{Desktop: bridge},
		logger:        discardLogger(),
		adapterHealth: map[string]*AdapterHealthSnapshot{},
	}
	adapter := newFakeAdapter(Platform("custom"), "fake-custom")
	msg := desktopTestMessage("테스트 좀 실행해 줘")

	// 미인수: 분기하지 않음。
	if gw.divertToDesktopTakeover(context.Background(), adapter, msg) {
		t.Fatal("message should not divert without a takeover binding")
	}

	route := desktopRouteFromMessage(msg)
	bridge.takeovers[route.Key()] = "tab-1"
	msg.Text = "/desktop status"
	if gw.divertToDesktopTakeover(context.Background(), adapter, msg) {
		t.Fatal("slash commands must remain in the bot command path during takeover")
	}
	msg.Text = "테스트 좀 실행해 줘"
	if !gw.divertToDesktopTakeover(context.Background(), adapter, msg) {
		t.Fatal("message should divert to the taken-over session")
	}
	if len(bridge.driven) != 1 || bridge.driven[0] != "테스트 좀 실행해 줘" {
		t.Fatalf("driven = %v, want the plain message text", bridge.driven)
	}

	// 드라이브 실패: 오류 메시지를 사용자에게 전달합니다。
	bridge.driveErr = fmt.Errorf("세션이 실행 중")
	if !gw.divertToDesktopTakeover(context.Background(), adapter, msg) {
		t.Fatal("drive failure should still consume the message")
	}
	sent := adapter.sentMessages()
	if len(sent) == 0 || !strings.Contains(sent[len(sent)-1].Text, "실행 중") {
		t.Fatalf("sent = %+v, want drive error relayed to the chat", sent)
	}
}

func TestDivertToDesktopTakeoverRevokesFormerAdmin(t *testing.T) {
	bridge := newFakeDesktopBridge()
	bridge.sessions = []DesktopSessionInfo{{TabID: "tab-1", Label: "세션 1"}}
	gw := &BotGateway{
		cfg: GatewayConfig{
			Desktop: bridge,
			Allowlist: AllowlistConfig{Admins: map[Platform][]string{
				Platform("custom"): {"current-admin"},
			}},
		},
		logger:        discardLogger(),
		adapterHealth: map[string]*AdapterHealthSnapshot{},
	}
	adapter := newFakeAdapter(Platform("custom"), "fake-custom")
	msg := desktopTestMessage("run tests")
	route := desktopRouteFromMessage(msg)
	bridge.takeovers[route.Key()] = "tab-1"

	if !gw.divertToDesktopTakeover(context.Background(), adapter, msg) {
		t.Fatal("revoked takeover message should be consumed with an explanation")
	}
	if bridge.TakeoverTab(route) != "" || len(bridge.driven) != 0 {
		t.Fatalf("revoked takeover remained active or drove input: tab=%q driven=%v", bridge.TakeoverTab(route), bridge.driven)
	}
	sent := adapter.sentMessages()
	if len(sent) == 0 || !strings.Contains(sent[len(sent)-1].Text, "권한 없음") {
		t.Fatalf("sent = %+v, want admin-revocation explanation", sent)
	}
}
