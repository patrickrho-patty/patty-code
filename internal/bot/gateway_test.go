package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/agent"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/provider"
	"patty/internal/tool"
)

// fakeAdapter는 BotGateway 테스트용 인메모리 가짜 어댑터입니다.
type fakeAdapter struct {
	mu       sync.Mutex
	stopOnce sync.Once
	platform Platform
	name     string
	msgCh    chan InboundMessage
	sent     []OutboundMessage
	started  bool
	startErr error
}

type resultAdapter struct {
	*fakeAdapter
	result SendResult
	err    error
}

func (a *resultAdapter) Send(_ context.Context, msg OutboundMessage) (SendResult, error) {
	a.mu.Lock()
	a.sent = append(a.sent, msg)
	a.mu.Unlock()
	return a.result, a.err
}

func newFakeAdapter(platform Platform, name string) *fakeAdapter {
	return &fakeAdapter{
		platform: platform,
		name:     name,
		msgCh:    make(chan InboundMessage, 16),
	}
}

func (f *fakeAdapter) Platform() Platform              { return f.platform }
func (f *fakeAdapter) Name() string                    { return f.name }
func (f *fakeAdapter) Messages() <-chan InboundMessage { return f.msgCh }

func (f *fakeAdapter) Start(ctx context.Context) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.mu.Lock()
	f.started = true
	f.mu.Unlock()
	return nil
}

func (f *fakeAdapter) Stop() error {
	f.stopOnce.Do(func() {
		close(f.msgCh)
	})
	return nil
}

func (f *fakeAdapter) Send(ctx context.Context, msg OutboundMessage) (SendResult, error) {
	f.mu.Lock()
	f.sent = append(f.sent, msg)
	f.mu.Unlock()
	return SendResult{MessageID: "fake_msg_1"}, nil
}

func (f *fakeAdapter) SendTyping(ctx context.Context, chatID string) error { return nil }

func (f *fakeAdapter) sentMessages() []OutboundMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]OutboundMessage, len(f.sent))
	copy(out, f.sent)
	return out
}

type blockingSendAdapter struct {
	*fakeAdapter
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingSendAdapter(platform Platform, name string) *blockingSendAdapter {
	return &blockingSendAdapter{
		fakeAdapter: newFakeAdapter(platform, name),
		entered:     make(chan struct{}),
		release:     make(chan struct{}),
	}
}

func (f *blockingSendAdapter) Send(ctx context.Context, msg OutboundMessage) (SendResult, error) {
	f.once.Do(func() { close(f.entered) })
	select {
	case <-f.release:
	case <-ctx.Done():
		return SendResult{}, ctx.Err()
	}
	return f.fakeAdapter.Send(ctx, msg)
}

type fakeReactionAdapter struct {
	*fakeAdapter
	reactions []string
	cleanups  []string
}

type gatewayFakeProvider struct{}

func (gatewayFakeProvider) Name() string { return "fake" }

func (gatewayFakeProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk)
	close(ch)
	return ch, nil
}

func (f *fakeReactionAdapter) AddPendingReaction(ctx context.Context, messageID string) (func(), error) {
	f.mu.Lock()
	f.reactions = append(f.reactions, messageID)
	f.mu.Unlock()
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.cleanups = append(f.cleanups, messageID)
	}, nil
}

func (f *fakeReactionAdapter) cleanupMessages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.cleanups))
	copy(out, f.cleanups)
	return out
}

type queueTestController struct {
	botController
	mu          sync.Mutex
	steers      []string
	rejectSteer bool
	canceled    bool
}

func (c *queueTestController) Steer(text string) {
	_ = c.TrySteer(text)
}

func (c *queueTestController) TrySteer(text string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rejectSteer {
		return false
	}
	c.steers = append(c.steers, text)
	return true
}

func (c *queueTestController) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.canceled = true
}

func (c *queueTestController) SessionPath() string   { return "" }
func (c *queueTestController) WorkspaceRoot() string { return "" }

func (c *queueTestController) steered() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.steers))
	copy(out, c.steers)
	return out
}

func (c *queueTestController) wasCanceled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.canceled
}

type rotatingBotController struct {
	botController
	path     string
	newPath  string
	newCalls int
	closed   bool
}

func (c *rotatingBotController) Running() bool { return false }
func (c *rotatingBotController) NewSession() error {
	c.newCalls++
	c.path = c.newPath
	return nil
}
func (c *rotatingBotController) SessionPath() string { return c.path }
func (c *rotatingBotController) Close()              { c.closed = true }

type runtimeStatusBotController struct {
	botController
	status        control.RuntimeStatus
	workspaceRoot string
	sessionPath   string
	closed        bool
}

func (c *runtimeStatusBotController) RuntimeStatus() control.RuntimeStatus { return c.status }
func (c *runtimeStatusBotController) WorkspaceRoot() string                { return c.workspaceRoot }
func (c *runtimeStatusBotController) SessionPath() string                  { return c.sessionPath }
func (c *runtimeStatusBotController) Close()                               { c.closed = true }

type blockingApprovalController struct {
	botController
	emit     func(event.Event)
	emitted  chan struct{}
	approved chan struct{}
	done     chan struct{}
	once     sync.Once
}

func (c *blockingApprovalController) RunTurn(ctx context.Context, input string) error {
	c.emit(event.Event{Kind: event.ApprovalRequest, Approval: event.Approval{ID: "appr-1", Tool: "bash", Subject: "sample command"}})
	close(c.emitted)
	select {
	case <-c.approved:
		close(c.done)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *blockingApprovalController) Approve(id string, allow, session, persist bool) {
	c.once.Do(func() { close(c.approved) })
}

type blockingAskController struct {
	botController
	emit     func(event.Event)
	emitted  chan struct{}
	answered chan []event.AskAnswer
	done     chan struct{}
	once     sync.Once
}

func (c *blockingAskController) RunTurn(ctx context.Context, input string) error {
	c.emit(event.Event{Kind: event.AskRequest, Ask: event.Ask{ID: "ask-1", Questions: []event.AskQuestion{{
		ID:     "q1",
		Header: "Planner",
		Prompt: "Which plan?",
		Options: []event.AskOption{
			{Label: "Small patch"},
			{Label: "Refactor"},
		},
	}}}})
	close(c.emitted)
	select {
	case <-c.answered:
		close(c.done)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *blockingAskController) AnswerQuestion(id string, answers []event.AskAnswer) {
	c.once.Do(func() { c.answered <- answers })
}

func TestFakeAdapterInterface(t *testing.T) {
	fa := newFakeAdapter(Platform("channel"), "fake-channel")

	if fa.Platform() != Platform("channel") {
		t.Error("wrong platform")
	}
	if fa.Name() != "fake-channel" {
		t.Error("wrong name")
	}

	ctx := context.Background()
	if err := fa.Start(ctx); err != nil {
		t.Fatal("start:", err)
	}
	if !fa.started {
		t.Error("should be started")
	}

	_, err := fa.Send(ctx, OutboundMessage{ChatID: "c1", Text: "hello"})
	if err != nil {
		t.Fatal("send:", err)
	}

	sent := fa.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if sent[0].Text != "hello" {
		t.Errorf("sent text = %q, want %q", sent[0].Text, "hello")
	}

	if err := fa.Stop(); err != nil {
		t.Fatal("stop:", err)
	}
}

func TestGatewayConstructAndStop(t *testing.T) {
	cfg := GatewayConfig{
		Model:         "test",
		MaxSteps:      10,
		WorkspaceRoot: ".",
		Enabled:       map[Platform]bool{Platform("channel"): true},
		Allowlist:     AllowlistConfig{Enabled: false},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, map[Platform]Adapter{
		Platform("channel"): newFakeAdapter(Platform("channel"), "fake-channel"),
	}, logger)

	// panic
	if gw == nil {
		t.Fatal("gateway should not be nil")
	}
	gw.Stop()
}

func TestGatewayStartsHealthyAdaptersWhenOneFails(t *testing.T) {
	cfg := GatewayConfig{
		Enabled:   map[Platform]bool{Platform("custom"): true, Platform("relay"): true},
		Allowlist: AllowlistConfig{AllowAll: true},
	}
	good := newFakeAdapter(Platform("custom"), "good-custom")
	bad := newFakeAdapter(Platform("relay"), "bad-relay")
	bad.startErr = errors.New("missing token")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGatewayWithAdapterBindings(cfg, []AdapterBinding{
		{ID: "custom-alpha", Platform: Platform("custom"), Adapter: good},
		{ID: "relay-main", Platform: Platform("relay"), Adapter: bad},
	}, logger)

	if err := gw.Start(context.Background()); err != nil {
		t.Fatalf("start should keep healthy adapters running: %v", err)
	}
	defer gw.Stop()
	if got := gw.AdapterCount(); got != 1 {
		t.Fatalf("adapter count = %d, want 1", got)
	}
	if !good.started {
		t.Fatal("healthy adapter was not started")
	}
	if bad.started {
		t.Fatal("failing adapter should not be marked started")
	}
	startErr := gw.StartErrors()
	if len(startErr) != 1 || !strings.Contains(startErr[0].Error(), "relay-main") {
		t.Fatalf("start errors = %#v, want wrapped connection error", startErr)
	}
}

func TestGatewaySendToAdapterReleasesLockBeforeSend(t *testing.T) {
	adapter := newBlockingSendAdapter(Platform("custom"), "blocking-custom")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGatewayWithAdapterBindings(GatewayConfig{}, []AdapterBinding{{
		ID:       "custom-alpha",
		Domain:   "alpha",
		Platform: Platform("custom"),
		Adapter:  adapter,
	}}, logger)

	sendDone := make(chan error, 1)
	go func() {
		_, err := gw.SendToAdapter(context.Background(), "custom-alpha", "alpha", OutboundMessage{ChatID: "chat", Text: "hello"})
		sendDone <- err
	}()

	select {
	case <-adapter.entered:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("adapter send did not start")
	}

	updateDone := make(chan struct{})
	go func() {
		gw.UpdateConnectionToolApprovalMode("custom-alpha", "ask")
		close(updateDone)
	}()
	select {
	case <-updateDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("UpdateConnectionToolApprovalMode blocked behind SendToAdapter")
	}

	close(adapter.release)
	select {
	case err := <-sendDone:
		if err != nil {
			t.Fatalf("SendToAdapter returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SendToAdapter did not finish after release")
	}
}

func TestGatewayReturnsErrorWhenAllAdaptersFail(t *testing.T) {
	cfg := GatewayConfig{
		Enabled:   map[Platform]bool{Platform("relay"): true},
		Allowlist: AllowlistConfig{AllowAll: true},
	}
	bad := newFakeAdapter(Platform("relay"), "bad-relay")
	bad.startErr = errors.New("missing token")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGatewayWithAdapterBindings(cfg, []AdapterBinding{
		{ID: "relay-main", Platform: Platform("relay"), Adapter: bad},
	}, logger)

	err := gw.Start(context.Background())
	if err == nil {
		t.Fatal("start should fail when every adapter fails")
	}
	if !strings.Contains(err.Error(), "relay-main") {
		t.Fatalf("error = %v, want connection id", err)
	}
	if got := gw.AdapterCount(); got != 0 {
		t.Fatalf("adapter count = %d, want 0", got)
	}
	if len(gw.StartErrors()) != 1 {
		t.Fatalf("start errors = %#v, want one", gw.StartErrors())
	}
}

func TestGatewayAllowlistCheck(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				Platform("channel"): {"allowed_user_1"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(Platform("channel"), InboundMessage{Platform: Platform("channel"), ChatType: ChatDM, UserID: "allowed_user_1"}) {
		t.Error("allowed user should pass")
	}
	if gw.checkAllowlist(Platform("channel"), InboundMessage{Platform: Platform("channel"), ChatType: ChatDM, UserID: "unknown_user"}) {
		t.Error("unknown user should not pass")
	}
	// 다른 플랫폼
	if gw.checkAllowlist(Platform("custom"), InboundMessage{Platform: Platform("custom"), ChatType: ChatDM, UserID: "allowed_user_1"}) {
		t.Error("channel allowlist should not apply to custom")
	}
}

func TestGatewayRejectsBeforeInboundEnrichment(t *testing.T) {
	adapter := newFakeAdapter(Platform("custom"), "custom")
	gw := NewGateway(GatewayConfig{Allowlist: AllowlistConfig{
		Enabled: true,
		Users:   map[Platform][]string{Platform("custom"): {"allowed-user"}},
	}}, map[Platform]Adapter{Platform("custom"): adapter}, discardLogger())

	mediaLoads := 0
	nameLoads := 0
	gw.handleMessage(context.Background(), AdapterBinding{ID: "custom", Platform: Platform("custom"), Adapter: adapter}, InboundMessage{
		Platform: Platform("custom"),
		ChatType: ChatDM,
		ChatID:   "chat",
		UserID:   "blocked-user",
		Media: []InboundMedia{{Load: func(context.Context) ([]byte, string, error) {
			mediaLoads++
			return []byte("payload"), "payload.txt", nil
		}}},
		ResolveUserName: func(context.Context) string {
			nameLoads++
			return "Blocked User"
		},
	})

	if mediaLoads != 0 || nameLoads != 0 {
		t.Fatalf("pre-admission enrichment calls = media:%d name:%d, want zero", mediaLoads, nameLoads)
	}
}

func TestGatewayRoleListsGrantAllowlistAdmission(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Admins: map[Platform][]string{
				Platform("custom"): {"admin_user"},
			},
			Approvers: map[Platform][]string{
				Platform("custom"): {"approver_user"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(Platform("custom"), InboundMessage{Platform: Platform("custom"), ChatType: ChatDM, UserID: "admin_user"}) {
		t.Error("admin role should grant base bot admission")
	}
	if !gw.checkAllowlist(Platform("custom"), InboundMessage{Platform: Platform("custom"), ChatType: ChatDM, UserID: "approver_user"}) {
		t.Error("approver role should grant base bot admission")
	}
	if gw.checkAllowlist(Platform("custom"), InboundMessage{Platform: Platform("custom"), ChatType: ChatDM, UserID: "unknown_user"}) {
		t.Error("unknown user should still be rejected")
	}
}

func TestGatewayApproverRoleDoesNotGrantAdminCommands(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Approvers: map[Platform][]string{
				Platform("custom"): {"approver_user"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)
	msg := InboundMessage{Platform: Platform("custom"), ChatType: ChatDM, UserID: "approver_user"}

	if !gw.checkCommandRole(Platform("custom"), msg, "approver") {
		t.Error("approver should be allowed to run approver commands")
	}
	if gw.checkCommandRole(Platform("custom"), msg, "admin") {
		t.Error("approver should not be allowed to run admin commands")
	}
}

func TestGatewayAllowlistDoesNotApplyGroupsToDirectMessages(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				Platform("channel"): {"allowed_user"},
			},
			Groups: map[Platform][]string{
				Platform("channel"): {"allowed_group"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(Platform("channel"), InboundMessage{Platform: Platform("channel"), ChatType: ChatDirect, ChatID: "guild-dm", UserID: "allowed_user"}) {
		t.Error("direct message should not be rejected by group allowlist")
	}
	if gw.checkAllowlist(Platform("channel"), InboundMessage{Platform: Platform("channel"), ChatType: ChatGroup, ChatID: "unknown_group", UserID: "allowed_user"}) {
		t.Error("unknown group should still be rejected by group allowlist")
	}
}

func TestGatewayGroupAllowlistStillNarrowsRoleAdmission(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Admins: map[Platform][]string{
				Platform("custom"): {"admin_user"},
			},
			Groups: map[Platform][]string{
				Platform("custom"): {"allowed_group"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(Platform("custom"), InboundMessage{Platform: Platform("custom"), ChatType: ChatDM, ChatID: "direct", UserID: "admin_user"}) {
		t.Error("admin role admission should still allow direct messages")
	}
	if !gw.checkAllowlist(Platform("custom"), InboundMessage{Platform: Platform("custom"), ChatType: ChatGroup, ChatID: "allowed_group", UserID: "admin_user"}) {
		t.Error("admin role should pass in allowed group")
	}
	if gw.checkAllowlist(Platform("custom"), InboundMessage{Platform: Platform("custom"), ChatType: ChatGroup, ChatID: "unknown_group", UserID: "admin_user"}) {
		t.Error("admin role should still be rejected in an unknown group")
	}
}

func TestGatewayAllowlistGatesOnOperatorNotCardRequester(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				Platform("custom"): {"requester"},
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	stranger := InboundMessage{Platform: Platform("custom"), ChatType: ChatGroup, ChatID: "chat", UserID: "requester", OperatorID: "stranger"}
	if gw.checkAllowlist(Platform("custom"), stranger) {
		t.Error("a non-allowlisted operator must be rejected even when the card carries an allowlisted requester id")
	}

	allowed := InboundMessage{Platform: Platform("custom"), ChatType: ChatGroup, ChatID: "chat", UserID: "requester", OperatorID: "requester"}
	if !gw.checkAllowlist(Platform("custom"), allowed) {
		t.Error("an allowlisted operator should pass")
	}
}

func TestGatewayAllowlistDisabledRejectsByDefault(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{Enabled: false},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if gw.checkAllowlist(Platform("channel"), InboundMessage{Platform: Platform("channel"), ChatType: ChatDM, UserID: "any_user"}) {
		t.Error("disabled allowlist should reject unless allow_all is explicit")
	}
}

func TestGatewayAllowAll(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{AllowAll: true},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(Platform("channel"), InboundMessage{Platform: Platform("channel"), ChatType: ChatDM, UserID: "any_user"}) {
		t.Error("allow_all should allow everyone")
	}
}

func TestGatewayNormalizesNumericApprovalShortcutsOnlyWhenPending(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	key := "session-key"

	if _, ok := gw.normalizeApprovalShortcut(key, "1"); ok {
		t.Fatal("numeric text without a pending approval should stay a normal message")
	}

	gw.controllers[key] = &sessionState{
		pendingApprovals: map[string]event.Approval{
			"42": {ID: "42", Tool: "explore"},
		},
		lastApprovalID: "42",
	}

	got, ok := gw.normalizeApprovalShortcut(key, "1")
	if !ok || got != "/approve 42" {
		t.Fatalf("normalize 1 = %q,%v; want /approve 42,true", got, ok)
	}
	got, ok = gw.normalizeApprovalShortcut(key, "2")
	if !ok || got != "/deny 42" {
		t.Fatalf("normalize 2 = %q,%v; want /deny 42,true", got, ok)
	}
	gw.forgetPendingApproval(key, "42")
	if _, ok := gw.normalizeApprovalShortcut(key, "1"); ok {
		t.Fatal("numeric text after approval is forgotten should stay a normal message")
	}
}

func TestGatewayNormalizesTaskGrantRecoveryShortcuts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	key := "recovery-key"
	gw.controllers[key] = &sessionState{
		pendingApprovals: map[string]event.Approval{
			"r1": {ID: "r1", Kind: "recovery", Recovery: &event.RecoveryApproval{CanGrantTask: true}},
		},
		lastApprovalID: "r1",
	}
	for input, want := range map[string]string{
		"1": "/recovery-continue r1",
		"2": "/recovery-continue-task r1",
		"3": "/recovery-revise r1",
	} {
		got, ok := gw.normalizeApprovalShortcut(key, input)
		if !ok || got != want {
			t.Fatalf("normalize %q = %q,%v; want %q,true", input, got, ok, want)
		}
	}
}

func TestGatewayNormalizesAskShortcutForPendingAsk(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	key := "session-key"

	if _, ok := gw.normalizeAskShortcut(key, "1"); ok {
		t.Fatal("numeric text without a pending ask should stay a normal message")
	}

	gw.controllers[key] = &sessionState{
		pendingAsks: map[string][]event.AskQuestion{
			"ask-1": {{
				ID:     "q1",
				Prompt: "Choose one",
				Options: []event.AskOption{
					{Label: "Allow once"},
					{Label: "Deny"},
				},
			}},
		},
		lastAskID: "ask-1",
	}

	got, ok := gw.normalizeAskShortcut(key, "2")
	if !ok || got != "/answer ask-1 2" {
		t.Fatalf("normalize 2 = %q,%v; want /answer ask-1 2,true", got, ok)
	}
	got, ok = gw.normalizeAskShortcut(key, "1;2")
	if !ok || got != "/answer ask-1 1;2" {
		t.Fatalf("normalize 1;2 = %q,%v; want /answer ask-1 1;2,true", got, ok)
	}
	got, ok = gw.normalizeAskShortcut(key, "freeform answer")
	if !ok || got != "/answer ask-1 freeform answer" {
		t.Fatalf("normalize freeform answer = %q,%v; want /answer ask-1 freeform answer,true", got, ok)
	}

	gw.controllers[key].pendingAsks["ask-2"] = []event.AskQuestion{
		{ID: "q1", Prompt: "First", Options: []event.AskOption{{Label: "A"}}},
		{ID: "q2", Prompt: "Second", Options: []event.AskOption{{Label: "B"}}},
	}
	gw.controllers[key].lastAskID = "ask-2"
	got, ok = gw.normalizeAskShortcut(key, "1")
	if !ok || got != "/answer ask-2 1" {
		t.Fatalf("normalize 1 on multi-question = %q,%v; want /answer ask-2 1,true", got, ok)
	}
	if _, ok := gw.normalizeAskShortcut(key, "/stop"); ok {
		t.Fatal("slash commands should not be normalized/routed by ask shortcut")
	}
}

func TestGatewaySessionOptionsUseConnectionToolApprovalOverride(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:            "default-model",
		ToolApprovalMode: "auto",
		Channels: map[Platform]ChannelConfig{
			Platform("custom"): {Model: "platform-model", ToolApprovalMode: "ask"},
		},
		ConnectionChannels: map[string]ChannelConfig{
			"custom-alpha": {Model: "alpha-model", ToolApprovalMode: "yolo"},
		},
	}, nil, logger)

	model, _, mode := gw.sessionOptionsForMessage(InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom-alpha",
	})
	if model != "alpha-model" || mode != "yolo" {
		t.Fatalf("alpha session options = model %q mode %q, want alpha-model/yolo", model, mode)
	}

	model, _, mode = gw.sessionOptionsForMessage(InboundMessage{Platform: Platform("custom")})
	if model != "platform-model" || mode != "ask" {
		t.Fatalf("platform session options = model %q mode %q, want platform-model/ask", model, mode)
	}
}

func TestGatewayNumericApprovalShortcutActiveWithoutPendingSendsGuidance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{Allowlist: AllowlistConfig{AllowAll: true}}, nil, logger)
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	binding := AdapterBinding{ID: "relay-main", Domain: "relay", Platform: Platform("relay"), Adapter: adapter}
	msg := InboundMessage{
		Platform:     Platform("relay"),
		ConnectionID: "relay-main",
		Domain:       "relay",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "seed",
	}
	key := BuildSessionKey(msg.Session())
	if acquired, _ := gw.sessions.TryAcquire(key, msg); !acquired {
		t.Fatal("failed to mark session active")
	}

	msg.Text = "1"
	gw.handleMessage(context.Background(), binding, msg)

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if !strings.Contains(sent[0].Text, "일치하는 대기 작업을 찾을 수 없습니다") {
		t.Fatalf("sent text = %q, want pending operation guidance", sent[0].Text)
	}
}

func TestGatewayApproveWithoutSessionSendsGuidance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	msg := InboundMessage{ChatType: ChatDM, ChatID: "chat", UserID: "user", Text: "/approve 1"}

	gw.handleSlashCommand(context.Background(), adapter, "missing-session", msg)

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if !strings.Contains(sent[0].Text, "현재 세션에 대기 중인 승인 작업이 없습니다") {
		t.Fatalf("sent text = %q, want missing approval guidance", sent[0].Text)
	}
}

func TestGatewayNewSessionRemembersRotatedSessionPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var remembered string
	gw := NewGateway(GatewayConfig{
		OnSessionReady: func(msg InboundMessage, sessionID string) error {
			remembered = sessionID
			return nil
		},
	}, nil, logger)
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	msg := InboundMessage{
		Platform:     Platform("relay"),
		ConnectionID: "relay-main",
		Domain:       "relay",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/new",
	}
	key := BuildSessionKey(msg.Session())
	sessionDir := t.TempDir()
	oldPath := agent.NewSessionPath(sessionDir, "old-model")
	exec := agent.New(gatewayFakeProvider{}, tool.NewRegistry(), agent.NewSession("system"), agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: exec, SessionDir: sessionDir, SessionPath: oldPath, Label: "fake-model"})
	leases := control.NewSessionLeaseKeeper()
	if err := leases.Rebind(oldPath); err != nil {
		t.Fatalf("bind old session lease: %v", err)
	}
	gw.controllers[key] = &sessionState{ctrl: ctrl, leases: leases, sessionPath: oldPath}

	gw.handleSlashCommand(context.Background(), adapter, key, msg)

	if remembered == "" || !strings.HasPrefix(remembered, "path:") {
		t.Fatalf("remembered session = %q, want path target", remembered)
	}
	if remembered == "path:"+oldPath {
		t.Fatalf("remembered session = %q, want rotated path", remembered)
	}
	if ctrl.SessionPath() == oldPath {
		t.Fatalf("controller session path was not rotated")
	}
	if got := leases.HeldPath(); got != agent.CanonicalSessionPath(ctrl.SessionPath()) {
		t.Fatalf("held lease = %q, want rotated path %q", got, agent.CanonicalSessionPath(ctrl.SessionPath()))
	}
	oldLease, err := agent.TryAcquireSessionLease(oldPath)
	if err != nil {
		t.Fatalf("old session lease was not released: %v", err)
	}
	oldLease.Release()
	gw.closeSessions()
}

func TestGatewayRecoveryRebindsLeaseAndRemembersSessionPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	originalPath := filepath.Join(dir, "session.jsonl")

	disk := agent.NewSession("sys")
	disk.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	disk.Add(provider.Message{Role: provider.RoleAssistant, Content: "disk"})
	if err := disk.Save(originalPath); err != nil {
		t.Fatalf("save disk session: %v", err)
	}

	local := agent.NewSession("sys")
	local.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	local.Add(provider.Message{Role: provider.RoleAssistant, Content: "local"})
	exec := agent.New(nil, nil, local, agent.Options{}, event.Discard)

	var remembered []string
	gw := NewGateway(GatewayConfig{
		OnSessionReady: func(_ InboundMessage, sessionID string) error {
			remembered = append(remembered, sessionID)
			return nil
		},
	}, nil, logger)
	msg := InboundMessage{
		Platform:     Platform("relay"),
		ConnectionID: "relay-main",
		Domain:       "relay",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
	}
	key := BuildSessionKey(msg.Session())
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	sessionSink := &sessionEventSink{}
	sessionSink.setTarget(newRenderSink(
		context.Background(), adapter, msg.ConnectionID, msg.Domain, msg.ChatID,
		msg.ChatType, msg.UserID, msg.MessageID, logger, nil, nil,
	))
	t.Cleanup(func() { sessionSink.setTarget(nil) })
	leases := control.NewSessionLeaseKeeper()
	if err := leases.Rebind(originalPath); err != nil {
		t.Fatalf("bind original session lease: %v", err)
	}
	state := &sessionState{sink: sessionSink, leases: leases, sessionPath: originalPath}
	gw.controllers[key] = state
	gw.sessionOverrides[key] = sessionRuntimeOverride{sessionPath: originalPath, label: "session:original"}
	ctrl := control.New(control.Options{
		Executor:           exec,
		SessionDir:         dir,
		SessionPath:        originalPath,
		Label:              "test",
		Sink:               sessionSink,
		OnSessionRecovered: gw.botSessionRecoveredHandler(key, msg, state),
	})
	state.ctrl = ctrl
	t.Cleanup(gw.closeSessions)

	if err := ctrl.Snapshot(); err != nil {
		t.Fatalf("snapshot diverged session: %v", err)
	}
	recoveryPath := ctrl.SessionPath()
	if recoveryPath == "" || recoveryPath == originalPath {
		t.Fatalf("controller path = %q, want recovery path", recoveryPath)
	}
	if got := leases.HeldPath(); got != agent.CanonicalSessionPath(recoveryPath) {
		t.Fatalf("held lease = %q, want recovery path %q", got, agent.CanonicalSessionPath(recoveryPath))
	}
	oldLease, err := agent.TryAcquireSessionLease(originalPath)
	if err != nil {
		t.Fatalf("original session lease was not released: %v", err)
	}
	oldLease.Release()

	gw.mu.Lock()
	gotStatePath := state.sessionPath
	gotOverridePath := gw.sessionOverrides[key].sessionPath
	gw.mu.Unlock()
	if canonicalBotPath(gotStatePath) != canonicalBotPath(recoveryPath) {
		t.Fatalf("state path = %q, want recovery path %q", gotStatePath, recoveryPath)
	}
	if canonicalBotPath(gotOverridePath) != canonicalBotPath(recoveryPath) {
		t.Fatalf("override path = %q, want recovery path %q", gotOverridePath, recoveryPath)
	}
	if len(remembered) != 1 || remembered[0] != botSessionTarget(recoveryPath) {
		t.Fatalf("remembered sessions = %v, want [%q]", remembered, botSessionTarget(recoveryPath))
	}

	if err := ctrl.Snapshot(); err != nil {
		t.Fatalf("snapshot recovered session: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*-recovery-*.jsonl"))
	if err != nil {
		t.Fatalf("glob recovery sessions: %v", err)
	}
	transcripts := matches[:0]
	for _, path := range matches {
		if !strings.HasSuffix(path, ".events.jsonl") && !strings.HasSuffix(path, ".conflicts.jsonl") {
			transcripts = append(transcripts, path)
		}
	}
	if len(transcripts) != 1 || transcripts[0] != recoveryPath {
		t.Fatalf("recovery transcripts = %v, want only %q", transcripts, recoveryPath)
	}
	if sent := adapter.sentMessages(); len(sent) != 0 {
		t.Fatalf("recovery maintenance leaked into IM messages: %+v", sent)
	}
}

func TestGatewayRecoveryLeaseFailureKeepsOriginalGeneration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	originalPath := filepath.Join(dir, "original.jsonl")
	recoveryPath := filepath.Join(dir, "recovery.jsonl")
	readyCalls := 0
	gw := NewGateway(GatewayConfig{
		OnSessionReady: func(InboundMessage, string) error {
			readyCalls++
			return nil
		},
	}, nil, logger)
	msg := InboundMessage{Platform: Platform("relay"), ChatType: ChatDM, ChatID: "chat", UserID: "user"}
	key := BuildSessionKey(msg.Session())
	leases := control.NewSessionLeaseKeeper()
	if err := leases.Rebind(originalPath); err != nil {
		t.Fatalf("bind original session lease: %v", err)
	}
	defer leases.Release()
	blocker, err := agent.TryAcquireSessionLease(recoveryPath)
	if err != nil {
		t.Fatalf("bind recovery blocker: %v", err)
	}
	defer blocker.Release()
	state := &sessionState{leases: leases, sessionPath: originalPath}
	gw.controllers[key] = state
	gw.sessionOverrides[key] = sessionRuntimeOverride{sessionPath: originalPath}

	err = gw.botSessionRecoveredHandler(key, msg, state)(control.SessionRecoveryInfo{
		OriginalPath: originalPath,
		RecoveryPath: recoveryPath,
	})
	if err == nil {
		t.Fatal("recovery handoff succeeded while recovery lease was held")
	}
	if got := leases.HeldPath(); got != agent.CanonicalSessionPath(originalPath) {
		t.Fatalf("held lease = %q, want original path %q", got, agent.CanonicalSessionPath(originalPath))
	}
	gw.mu.Lock()
	gotStatePath := state.sessionPath
	gotOverridePath := gw.sessionOverrides[key].sessionPath
	gw.mu.Unlock()
	if gotStatePath != originalPath || gotOverridePath != originalPath {
		t.Fatalf("paths changed after failed handoff: state=%q override=%q", gotStatePath, gotOverridePath)
	}
	if readyCalls != 0 {
		t.Fatalf("session-ready callback ran %d times after failed handoff", readyCalls)
	}
}

func TestGatewayLateRecoveryCannotReplaceCurrentSessionMapping(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.jsonl")
	oldRecoveryPath := filepath.Join(dir, "old-recovery.jsonl")
	currentPath := filepath.Join(dir, "current.jsonl")
	readyCalls := 0
	gw := NewGateway(GatewayConfig{
		OnSessionReady: func(InboundMessage, string) error {
			readyCalls++
			return nil
		},
	}, nil, logger)
	msg := InboundMessage{Platform: Platform("relay"), ChatType: ChatDM, ChatID: "chat", UserID: "user"}
	key := BuildSessionKey(msg.Session())
	oldLeases := control.NewSessionLeaseKeeper()
	if err := oldLeases.Rebind(oldPath); err != nil {
		t.Fatalf("bind old session lease: %v", err)
	}
	defer oldLeases.Release()
	oldState := &sessionState{leases: oldLeases, sessionPath: oldPath}
	currentState := &sessionState{sessionPath: currentPath}
	gw.controllers[key] = currentState
	gw.sessionOverrides[key] = sessionRuntimeOverride{sessionPath: currentPath}

	if err := gw.botSessionRecoveredHandler(key, msg, oldState)(control.SessionRecoveryInfo{
		OriginalPath: oldPath,
		RecoveryPath: oldRecoveryPath,
	}); err != nil {
		t.Fatalf("late recovery handoff: %v", err)
	}
	if got := oldLeases.HeldPath(); got != agent.CanonicalSessionPath(oldRecoveryPath) {
		t.Fatalf("old generation lease = %q, want recovery path %q", got, agent.CanonicalSessionPath(oldRecoveryPath))
	}
	gw.mu.Lock()
	gotCurrentPath := currentState.sessionPath
	gotOverridePath := gw.sessionOverrides[key].sessionPath
	gw.mu.Unlock()
	if gotCurrentPath != currentPath || gotOverridePath != currentPath {
		t.Fatalf("current mapping overwritten by late recovery: state=%q override=%q", gotCurrentPath, gotOverridePath)
	}
	if readyCalls != 0 {
		t.Fatalf("session-ready callback ran %d times for retired generation", readyCalls)
	}
}

func TestGatewayLateRecoveryAfterRetirementDoesNotReacquireLease(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	originalPath := filepath.Join(dir, "original.jsonl")
	recoveryPath := filepath.Join(dir, "recovery.jsonl")
	gw := NewGateway(GatewayConfig{}, nil, logger)
	msg := InboundMessage{Platform: Platform("relay"), ChatType: ChatDM, ChatID: "chat", UserID: "user"}
	key := BuildSessionKey(msg.Session())
	leases := control.NewSessionLeaseKeeper()
	if err := leases.Rebind(originalPath); err != nil {
		t.Fatalf("bind original session lease: %v", err)
	}
	state := &sessionState{leases: leases, sessionPath: originalPath}
	gw.controllers[key] = state
	handler := gw.botSessionRecoveredHandler(key, msg, state)

	gw.closeSessions()
	if got := leases.HeldPath(); got != "" {
		t.Fatalf("lease after retirement = %q, want empty", got)
	}
	if err := handler(control.SessionRecoveryInfo{
		OriginalPath: originalPath,
		RecoveryPath: recoveryPath,
	}); !errors.Is(err, errBotSessionRetired) {
		t.Fatalf("late recovery error = %v, want %v", err, errBotSessionRetired)
	}
	if got := leases.HeldPath(); got != "" {
		t.Fatalf("late recovery reacquired lease after retirement: %q", got)
	}
	probe, err := agent.TryAcquireSessionLease(recoveryPath)
	if err != nil {
		t.Fatalf("recovery lease remained unavailable after late callback: %v", err)
	}
	probe.Release()
}

func TestGatewayNewSessionLeaseFailureRetiresSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	readyCalls := 0
	gw := NewGateway(GatewayConfig{
		OnSessionReady: func(InboundMessage, string) error {
			readyCalls++
			return nil
		},
	}, nil, logger)
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	msg := InboundMessage{
		Platform:     Platform("relay"),
		ConnectionID: "relay-main",
		Domain:       "relay",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/new",
	}
	key := BuildSessionKey(msg.Session())
	sessionDir := t.TempDir()
	oldPath := filepath.Join(sessionDir, "old.jsonl")
	newPath := filepath.Join(sessionDir, "new.jsonl")
	ctrl := &rotatingBotController{path: oldPath, newPath: newPath}
	leases := control.NewSessionLeaseKeeper()
	if err := leases.Rebind(oldPath); err != nil {
		t.Fatalf("bind old session lease: %v", err)
	}
	blocker, err := agent.TryAcquireSessionLease(newPath)
	if err != nil {
		t.Fatalf("hold rotated session lease: %v", err)
	}
	defer blocker.Release()
	gw.controllers[key] = &sessionState{ctrl: ctrl, leases: leases, sessionPath: oldPath}

	gw.handleSlashCommand(context.Background(), adapter, key, msg)

	gw.mu.Lock()
	_, exists := gw.controllers[key]
	gw.mu.Unlock()
	if exists {
		t.Fatal("lease-failed session remains registered")
	}
	if ctrl.newCalls != 1 || !ctrl.closed {
		t.Fatalf("controller lifecycle = new calls %d closed %v, want 1/true", ctrl.newCalls, ctrl.closed)
	}
	if got := leases.HeldPath(); got != "" {
		t.Fatalf("old lease remained held after retirement: %q", got)
	}
	oldLease, err := agent.TryAcquireSessionLease(oldPath)
	if err != nil {
		t.Fatalf("old session lease was not released after retirement: %v", err)
	}
	oldLease.Release()
	if readyCalls != 0 {
		t.Fatalf("session-ready callback ran %d times after failed creation", readyCalls)
	}
	sent := adapter.sentMessages()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "새 세션 생성 실패") || strings.Contains(sent[0].Text, "새 세션을 시작했습니다") {
		t.Fatalf("sent messages = %+v, want a single creation-failed response", sent)
	}
}

func TestGatewayCloseSessionStateReleasesSessionLease(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	path := filepath.Join(t.TempDir(), "session.jsonl")
	leases := control.NewSessionLeaseKeeper()
	if err := leases.Rebind(path); err != nil {
		t.Fatalf("bind session lease: %v", err)
	}
	ctrl := control.New(control.Options{})

	gw.closeSessionState(&sessionState{ctrl: ctrl, leases: leases})

	lease, err := agent.TryAcquireSessionLease(path)
	if err != nil {
		t.Fatalf("session lease was not released after controller close: %v", err)
	}
	lease.Release()
}

func TestGatewayYoloCommandUpdatesCurrentSessionAndConnectionDefault(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var persistedMode string
	var persistedConnection string
	gw := NewGateway(GatewayConfig{
		ToolApprovalMode: "ask",
		ConnectionChannels: map[string]ChannelConfig{
			"custom-alpha": {ToolApprovalMode: "ask"},
		},
		OnToolApprovalModeChange: func(msg InboundMessage, mode string) error {
			persistedConnection = msg.ConnectionID
			persistedMode = mode
			return nil
		},
	}, nil, logger)
	adapter := newFakeAdapter(Platform("custom"), "fake-alpha")
	msg := InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom-alpha",
		Domain:       "alpha",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/yolo on",
	}
	key := BuildSessionKey(msg.Session())
	ctrl := control.New(control.Options{})
	ctrl.SetToolApprovalMode(control.ToolApprovalAsk)
	gw.controllers[key] = &sessionState{ctrl: ctrl}

	gw.handleSlashCommand(context.Background(), adapter, key, msg)

	if got := ctrl.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("current session mode = %q, want yolo", got)
	}
	if got := gw.cfg.ConnectionChannels["custom-alpha"].ToolApprovalMode; got != control.ToolApprovalYolo {
		t.Fatalf("connection default mode = %q, want yolo", got)
	}
	if persistedConnection != "custom-alpha" || persistedMode != control.ToolApprovalYolo {
		t.Fatalf("persisted = %q/%q, want custom-alpha/yolo", persistedConnection, persistedMode)
	}
	sent := adapter.sentMessages()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "YOLO를 켰습니다") {
		t.Fatalf("sent = %#v, want yolo confirmation", sent)
	}
}

func TestGatewayUpdateConnectionToolApprovalModeUpdatesHashedActiveSessions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		ToolApprovalMode: "ask",
		ConnectionChannels: map[string]ChannelConfig{
			"custom-alpha": {ToolApprovalMode: "yolo"},
		},
	}, nil, logger)

	msg := InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom-alpha",
		Domain:       "alpha",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
	}
	key := BuildSessionKey(msg.Session())
	if strings.HasPrefix(key, msg.ConnectionID) {
		t.Fatalf("test setup expected hashed key, got %q", key)
	}
	ctrl := control.New(control.Options{})
	ctrl.SetToolApprovalMode(control.ToolApprovalYolo)
	gw.controllers[key] = &sessionState{ctrl: ctrl, platform: msg.Platform, connectionID: msg.ConnectionID}

	gw.UpdateConnectionToolApprovalMode("custom-alpha", control.ToolApprovalAsk)

	if got := ctrl.ToolApprovalMode(); got != control.ToolApprovalAsk {
		t.Fatalf("active session mode = %q, want ask", got)
	}
	if got := gw.cfg.ConnectionChannels["custom-alpha"].ToolApprovalMode; got != control.ToolApprovalAsk {
		t.Fatalf("connection default mode = %q, want ask", got)
	}
}

func TestGatewayUpdateConnectionToolApprovalModeInheritsGatewayDefault(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		ToolApprovalMode: control.ToolApprovalAuto,
		ConnectionChannels: map[string]ChannelConfig{
			"custom-alpha": {ToolApprovalMode: control.ToolApprovalYolo},
			"custom-main":  {ToolApprovalMode: control.ToolApprovalYolo},
		},
	}, nil, logger)

	alphaMsg := InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom-alpha",
		Domain:       "alpha",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
	}
	alphaKey := BuildSessionKey(alphaMsg.Session())
	alphaCtrl := control.New(control.Options{})
	alphaCtrl.SetToolApprovalMode(control.ToolApprovalYolo)
	gw.controllers[alphaKey] = &sessionState{ctrl: alphaCtrl, platform: alphaMsg.Platform, connectionID: alphaMsg.ConnectionID}

	otherCtrl := control.New(control.Options{})
	otherCtrl.SetToolApprovalMode(control.ToolApprovalYolo)
	gw.controllers["other-hashed-key"] = &sessionState{ctrl: otherCtrl, platform: Platform("custom"), connectionID: "custom-main"}

	gw.UpdateConnectionToolApprovalMode("custom-alpha", "")

	if got := gw.cfg.ConnectionChannels["custom-alpha"].ToolApprovalMode; got != "" {
		t.Fatalf("connection override = %q, want empty inherit", got)
	}
	if got := alphaCtrl.ToolApprovalMode(); got != control.ToolApprovalAuto {
		t.Fatalf("alpha active session mode = %q, want inherited auto", got)
	}
	if got := otherCtrl.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("other connection mode = %q, want unchanged yolo", got)
	}
}

func TestGatewayApprovalReplyUnblocksWedgedTurn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{Allowlist: AllowlistConfig{AllowAll: true}}, nil, logger)
	adapter := newFakeAdapter(Platform("custom"), "fake-custom")
	binding := AdapterBinding{ID: "custom", Platform: Platform("custom"), Adapter: adapter}
	msg := InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "delete everything",
	}
	key := BuildSessionKey(msg.Session())
	sink := &sessionEventSink{}
	ctrl := &blockingApprovalController{
		emit:     sink.Emit,
		emitted:  make(chan struct{}),
		approved: make(chan struct{}),
		done:     make(chan struct{}),
	}
	gw.controllers[key] = &sessionState{
		ctrl:             ctrl,
		sink:             sink,
		pendingApprovals: make(map[string]event.Approval),
		pendingAsks:      make(map[string][]event.AskQuestion),
	}

	ctx := t.Context()
	go gw.dispatchLoop(ctx, binding)

	adapter.msgCh <- msg
	select {
	case <-ctrl.emitted:
	case <-time.After(2 * time.Second):
		t.Fatal("approval request was never emitted; turn did not start")
	}

	adapter.msgCh <- InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/approve appr-1",
	}

	select {
	case <-ctrl.done:
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: /approve reply was not delivered while the turn blocked on approval")
	}
}

func TestGatewayAskReplyUnblocksWedgedTurn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{Allowlist: AllowlistConfig{AllowAll: true}}, nil, logger)
	adapter := newFakeAdapter(Platform("custom"), "fake-custom")
	binding := AdapterBinding{ID: "custom", Platform: Platform("custom"), Adapter: adapter}
	msg := InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "choose a plan",
	}
	key := BuildSessionKey(msg.Session())
	sink := &sessionEventSink{}
	ctrl := &blockingAskController{
		emit:     sink.Emit,
		emitted:  make(chan struct{}),
		answered: make(chan []event.AskAnswer, 1),
		done:     make(chan struct{}),
	}
	gw.controllers[key] = &sessionState{
		ctrl:             ctrl,
		sink:             sink,
		pendingApprovals: make(map[string]event.Approval),
		pendingAsks:      make(map[string][]event.AskQuestion),
	}

	ctx := t.Context()
	go gw.dispatchLoop(ctx, binding)

	adapter.msgCh <- msg
	select {
	case <-ctrl.emitted:
	case <-time.After(2 * time.Second):
		t.Fatal("ask request was never emitted; turn did not start")
	}

	adapter.msgCh <- InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "1",
	}

	select {
	case <-ctrl.done:
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: ask reply was not delivered while the turn blocked on user choice")
	}
}

func TestGatewayModeCommandSupportsAskAutoAndStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		ConnectionChannels: map[string]ChannelConfig{
			"relay-main": {ToolApprovalMode: "ask"},
		},
	}, nil, logger)
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	msg := InboundMessage{
		Platform:     Platform("relay"),
		ConnectionID: "relay-main",
		Domain:       "relay",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
	}
	key := BuildSessionKey(msg.Session())

	msg.Text = "/mode auto"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	if got := gw.cfg.ConnectionChannels["relay-main"].ToolApprovalMode; got != control.ToolApprovalAuto {
		t.Fatalf("/mode auto default = %q, want auto", got)
	}

	msg.Text = "/yolo off"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	if got := gw.cfg.ConnectionChannels["relay-main"].ToolApprovalMode; got != control.ToolApprovalAsk {
		t.Fatalf("/yolo off default = %q, want ask", got)
	}

	msg.Text = "/mode"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	sent := adapter.sentMessages()
	if len(sent) != 3 {
		t.Fatalf("sent count = %d, want 3", len(sent))
	}
	if !strings.Contains(sent[2].Text, "현재 도구 승인 모드: 묻기") {
		t.Fatalf("status = %q, want ask status", sent[2].Text)
	}
}

func TestGatewayHelpMentionsYoloCommands(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	adapter := newFakeAdapter(Platform("custom"), "fake-custom")
	msg := InboundMessage{ChatType: ChatDM, ChatID: "chat", UserID: "user", Text: "/help"}

	gw.handleSlashCommand(context.Background(), adapter, "session-key", msg)

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if !strings.Contains(sent[0].Text, "/yolo on|off|auto|status") || !strings.Contains(sent[0].Text, "/mode yolo|ask|auto") {
		t.Fatalf("help = %q, want yolo commands", sent[0].Text)
	}
	if !strings.Contains(sent[0].Text, "/projects") || !strings.Contains(sent[0].Text, "/attach session") || !strings.Contains(sent[0].Text, "/search all") {
		t.Fatalf("help = %q, want project/session commands", sent[0].Text)
	}
}

func TestGatewayProjectCommandsListAndUseProjectOverride(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	base := t.TempDir()
	alpha := filepath.Join(base, "alpha-project")
	beta := filepath.Join(base, "beta-project")
	if err := os.MkdirAll(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(beta, 0o755); err != nil {
		t.Fatal(err)
	}
	gw := NewGateway(GatewayConfig{
		WorkspaceRoot: alpha,
		ConnectionChannels: map[string]ChannelConfig{
			"relay-main": {WorkspaceRoot: beta},
		},
	}, nil, logger)
	adapter := newFakeAdapter(Platform("relay"), "fake-relay")
	msg := InboundMessage{
		Platform:     Platform("relay"),
		ConnectionID: "relay-main",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/projects",
	}
	key := BuildSessionKey(msg.Session())

	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	sent := adapter.sentMessages()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "alpha-project") || !strings.Contains(sent[0].Text, "beta-project") {
		t.Fatalf("/projects sent = %#v, want both projects", sent)
	}

	msg.Text = "/use project alpha"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	_, root, _ := gw.sessionOptionsForMessage(msg)
	if canonicalBotPath(root) != canonicalBotPath(alpha) {
		t.Fatalf("workspace after /use project = %q, want %q", root, alpha)
	}

	msg.Text = "/use project default"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	_, root, _ = gw.sessionOptionsForMessage(msg)
	if canonicalBotPath(root) != canonicalBotPath(beta) {
		t.Fatalf("workspace after /use project default = %q, want connection default %q", root, beta)
	}
}

func TestGatewaySessionsSearchAndAttachSessionOverride(t *testing.T) {
	t.Setenv("PATTY_HOME", t.TempDir())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	projectRoot := filepath.Join(t.TempDir(), "attach-project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionDir := botSessionDir(projectRoot)
	sessionPath := filepath.Join(sessionDir, "attached.jsonl")
	sess := agent.NewSession("system")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "needle attach conversation"})
	if err := sess.Save(sessionPath); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	if err := agent.UpdateSessionMeta(sessionPath, "model-a", "needle attach conversation", 1, true); err != nil {
		t.Fatalf("UpdateSessionMeta: %v", err)
	}
	gw := NewGateway(GatewayConfig{WorkspaceRoot: projectRoot}, nil, logger)
	adapter := newFakeAdapter(Platform("custom"), "fake-custom")
	msg := InboundMessage{
		Platform:     Platform("custom"),
		ConnectionID: "custom-alpha",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/sessions search needle",
	}
	key := BuildSessionKey(msg.Session())

	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	sent := adapter.sentMessages()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "needle attach") || !strings.Contains(sent[0].Text, "s1") {
		t.Fatalf("/sessions sent = %#v, want indexed session", sent)
	}

	msg.Text = "/attach session s1"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	profile := gw.sessionProfileForMessage(msg)
	if canonicalBotPath(profile.sessionPath) != canonicalBotPath(sessionPath) {
		t.Fatalf("attached session path = %q, want %q", profile.sessionPath, sessionPath)
	}
	if canonicalBotPath(profile.workspaceRoot) != canonicalBotPath(projectRoot) {
		t.Fatalf("attached workspace root = %q, want %q", profile.workspaceRoot, projectRoot)
	}
}
