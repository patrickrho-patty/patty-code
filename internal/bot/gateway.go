package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"patty/internal/agent"
	"patty/internal/boot"
	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/secrets"
)

// GatewayConfig is the configuration for BotGateway.
type GatewayConfig struct {
	Model             string
	ToolApprovalMode  string
	MaxSteps          int
	QueueMode         string
	QueueCap          int
	QueueDrop         string
	PairingEnabled    bool
	PairingTTL        time.Duration
	PairingMaxPending int
	// IgnoreSelfMessages drops messages that are clearly sent by this bot. It
	// uses configured SelfUserIDs plus recently returned outbound message IDs.
	IgnoreSelfMessages bool
	SelfUserIDs        map[Platform][]string
	ControlEnabled     bool
	ControlAddr        string
	ControlToken       string
	// ApprovalTimeout bounds how long a tool-approval/ask prompt blocks a bot
	// session waiting for a remote user's reply. Zero falls back to
	// defaultBotApprovalTimeout so an abandoned prompt can't wedge the bot forever
	// (#4626, #4402). A negative value disables the timeout (wait indefinitely).
	ApprovalTimeout    time.Duration
	WorkspaceRoot      string
	Channels           map[Platform]ChannelConfig
	ConnectionChannels map[string]ChannelConfig
	Routes             []RouteConfig
	ConnectionAccess   map[string]AccessConfig
	Allowlist          AllowlistConfig
	Enabled            map[Platform]bool
	Debounce           time.Duration
	// OnInbound observes every allowlisted inbound message before dispatch.
	//
	// Reentrancy contract for all GatewayConfig callbacks (OnInbound,
	// OnSessionReady, OnToolApprovalModeChange): they run synchronously on
	// gateway-owned dispatch/turn goroutines; OnSessionReady can also run on a
	// controller recovery/autosave goroutine. Stop drains all of those paths
	// before returning. A callback must therefore never call Stop, nor block
	// until a goroutine that does so completes — Stop would wait on the very
	// goroutine running the callback, a guaranteed deadlock. Hosts that want to
	// shut the gateway down in reaction to a callback must trigger the shutdown
	// asynchronously.
	OnInbound func(InboundMessage)
	// OnSessionReady notifies the host after the bot has created, reused, or
	// recovered the controller for an inbound remote. Hosts may persist the
	// concrete session ID or keep the remote as a read-only channel.
	OnSessionReady func(InboundMessage, string) error
	// OnToolApprovalModeChange persists a remote IM request such as /yolo on.
	// The gateway updates the live session and in-memory defaults first; this
	// callback lets desktop save the chosen connection mode to user config.
	OnToolApprovalModeChange func(InboundMessage, string) error
	// Desktop, when the gateway is embedded in the desktop app, gives bot
	// chats a god view over desktop sessions (/desktop commands): global
	// status, event subscriptions, and remote approvals for any live desktop
	// session. Nil when the gateway runs standalone (patcode bot start).
	Desktop DesktopBridge
}

// ChannelConfig overrides gateway defaults for one IM channel.
type ChannelConfig struct {
	Model            string
	ToolApprovalMode string
	WorkspaceRoot    string
	SessionMappings  []SessionMapping
}

// SessionMapping is the runtime subset of a saved bot connection mapping used
// to route a remote chat/user/thread back to its intended workspace.
type SessionMapping struct {
	RemoteID      string
	SessionID     string
	SessionSource string
	ChatType      string
	UserID        string
	ThreadID      string
	Scope         string
	WorkspaceRoot string
	UpdatedAt     string
}

// RouteConfig applies per-remote overrides. Empty match fields are wildcards;
// the first matching route wins.
type RouteConfig struct {
	ConnectionID string
	Platform     Platform
	ChatType     ChatType
	ChatID       string
	UserID       string
	ThreadID     string
	Channel      ChannelConfig
}

// AdapterBinding attaches an adapter instance to one saved bot connection.
// ID/Domain keep sessions, replies, and per-connection settings separated at
// runtime.
type AdapterBinding struct {
	ID       string
	Domain   string
	Platform Platform
	Adapter  Adapter
}

// AllowlistConfig controls which users/groups may use the bot.
type AllowlistConfig struct {
	Enabled   bool
	AllowAll  bool
	Users     map[Platform][]string
	Approvers map[Platform][]string
	Admins    map[Platform][]string
	Groups    map[Platform][]string
}

// AccessConfig controls who may use one concrete bot connection.
type AccessConfig struct {
	Enabled        bool
	AllowAll       bool
	PairingEnabled bool
	Users          []string
	Groups         []string
	Approvers      []string
	Admins         []string
}

// AdapterHealthSnapshot describes the gateway's current view of one adapter.
type AdapterHealthSnapshot struct {
	ID            string    `json:"id"`
	Platform      Platform  `json:"platform"`
	Domain        string    `json:"domain,omitempty"`
	Name          string    `json:"name,omitempty"`
	Status        string    `json:"status"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	LastMessageAt time.Time `json:"last_message_at,omitempty"`
	LastSendAt    time.Time `json:"last_send_at,omitempty"`
	LastErrorAt   time.Time `json:"last_error_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	Messages      int64     `json:"messages"`
	Sends         int64     `json:"sends"`
	SendErrors    int64     `json:"send_errors"`
	Closed        bool      `json:"closed"`
}

// BotGateway is the bot message gateway; it manages Controller lifecycles,
// session concurrency, event rendering, and platform adapters.
type BotGateway struct {
	cfg      GatewayConfig
	adapters []AdapterBinding
	sessions *SessionManager
	startErr []error

	lifecycleMu sync.Mutex
	started     bool
	stopped     bool
	runCancel   context.CancelFunc
	startDone   chan struct{}
	stopDone    chan struct{}
	gatewayWG   sync.WaitGroup
	turnWG      sync.WaitGroup

	mu                      sync.Mutex
	controllers             map[string]*sessionState // session key -> active state
	pendingReactionCleanups map[string][]func()
	allowlist               map[Platform]map[string]bool
	groupAllowlist          map[Platform]map[string]bool
	selfUserIDs             map[Platform]map[string]bool
	outboundMessageIDs      map[string]time.Time
	adapterHealth           map[string]*AdapterHealthSnapshot
	controlServer           *controlHTTPServer
	sessionOverrides        map[string]sessionRuntimeOverride

	logger *slog.Logger
}

// botController is the slice of the controller's driving port the gateway needs:
// session lifecycle, turn execution, and approval/ask handling. The bot never
// touches goals, checkpoints, or memory, so it depends on those sub-ports only —
// not the concrete *control.Controller and its ~99 methods.
type botController interface {
	control.Lifecycle
	control.TurnControl
	control.Approvals
}

type sessionState struct {
	lifecycleMu      sync.Mutex
	retired          bool
	ctrl             botController
	sink             *sessionEventSink
	leases           *control.SessionLeaseKeeper
	platform         Platform
	connectionID     string
	model            string
	workspaceRoot    string
	toolApprovalMode string
	sessionPath      string
	// mappingDegraded records that this state intentionally runs on a fresh
	// session because its session_mappings target could not be used at build
	// time. It keeps later messages (whose profile re-resolves the mapping)
	// from tearing the state down every turn; convergence back onto the
	// mapped file happens on the next gateway restart.
	mappingDegraded  bool
	cancel           context.CancelFunc
	pendingAsks      map[string][]event.AskQuestion
	pendingApprovals map[string]event.Approval
	lastApprovalID   string
	lastAskID        string
	createdAt        time.Time
	lastActive       time.Time
}

var errBotSessionRetired = errors.New("bot session retired during recovery")

type sessionRuntimeProfile struct {
	model            string
	workspaceRoot    string
	toolApprovalMode string
	sessionPath      string
	// sessionPathOptional marks sessionPath as a persisted session_mappings
	// binding rather than an explicit /attach: when the mapped file cannot be
	// loaded or leased, the session degrades to a fresh path instead of
	// dropping the message (#6917).
	sessionPathOptional bool
}

type sessionRuntimeOverride struct {
	channel     ChannelConfig
	sessionPath string
	label       string
}

type sessionEventSink struct {
	mu     sync.RWMutex
	target event.Sink
}

type pendingReactionAdapter interface {
	AddPendingReaction(ctx context.Context, messageID string) (func(), error)
}

const outboundEchoTTL = 10 * time.Minute

func (s *sessionEventSink) setTarget(target event.Sink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.target = target
}

func (s *sessionEventSink) Emit(e event.Event) {
	s.mu.RLock()
	target := s.target
	s.mu.RUnlock()
	if target != nil {
		target.Emit(e)
	}
}

// NewGateway creates a new BotGateway.
func NewGateway(cfg GatewayConfig, adapters map[Platform]Adapter, logger *slog.Logger) *BotGateway {
	bindings := make([]AdapterBinding, 0, len(adapters))
	for plat, adapter := range adapters {
		bindings = append(bindings, AdapterBinding{ID: string(plat), Platform: plat, Adapter: adapter})
	}
	return NewGatewayWithAdapterBindings(cfg, bindings, logger)
}

// NewGatewayWithAdapterBindings creates a gateway with one or more adapter
// instances per platform.
func NewGatewayWithAdapterBindings(cfg GatewayConfig, adapters []AdapterBinding, logger *slog.Logger) *BotGateway {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = 1500 * time.Millisecond
	}
	cfg.QueueMode = NormalizeQueueMode(cfg.QueueMode)
	if cfg.QueueCap <= 0 {
		cfg.QueueCap = DefaultQueueCap
	}
	cfg.QueueDrop = NormalizeQueueDrop(cfg.QueueDrop)
	if cfg.PairingTTL <= 0 {
		cfg.PairingTTL = defaultPairingTTL
	}
	if cfg.PairingMaxPending <= 0 {
		cfg.PairingMaxPending = defaultPairingMaxPending
	}
	gw := &BotGateway{
		cfg:                     cfg,
		adapters:                normalizeAdapterBindings(adapters),
		sessions:                NewSessionManager(cfg.Debounce),
		controllers:             make(map[string]*sessionState),
		pendingReactionCleanups: make(map[string][]func()),
		allowlist:               make(map[Platform]map[string]bool),
		groupAllowlist:          make(map[Platform]map[string]bool),
		selfUserIDs:             make(map[Platform]map[string]bool),
		outboundMessageIDs:      make(map[string]time.Time),
		adapterHealth:           make(map[string]*AdapterHealthSnapshot),
		sessionOverrides:        make(map[string]sessionRuntimeOverride),
		logger:                  logger.With("component", "bot_gateway"),
	}
	gw.buildAllowlist()
	gw.buildSelfUserIDs()
	for _, binding := range gw.adapters {
		gw.setAdapterConfigured(binding)
	}
	return gw
}

func normalizeAdapterBindings(adapters []AdapterBinding) []AdapterBinding {
	out := make([]AdapterBinding, 0, len(adapters))
	for _, binding := range adapters {
		if binding.Adapter == nil {
			continue
		}
		if binding.Platform == "" {
			binding.Platform = binding.Adapter.Platform()
		}
		if strings.TrimSpace(binding.ID) == "" {
			binding.ID = string(binding.Platform)
		}
		binding.ID = strings.TrimSpace(binding.ID)
		binding.Domain = strings.TrimSpace(binding.Domain)
		out = append(out, binding)
	}
	return out
}

func (gw *BotGateway) buildAllowlist() {
	plats := map[Platform]bool{}
	for plat := range gw.cfg.Allowlist.Users {
		plats[plat] = true
	}
	for plat := range gw.cfg.Allowlist.Admins {
		plats[plat] = true
	}
	for plat := range gw.cfg.Allowlist.Approvers {
		plats[plat] = true
	}
	for plat := range gw.cfg.Allowlist.Groups {
		plats[plat] = true
	}
	for plat := range plats {
		gw.allowlist[plat] = make(map[string]bool)
		if !gw.cfg.Allowlist.Enabled {
			continue
		}
		addAllowlistUsers(gw.allowlist[plat], gw.cfg.Allowlist.Users[plat])
		addAllowlistUsers(gw.allowlist[plat], gw.cfg.Allowlist.Admins[plat])
		addAllowlistUsers(gw.allowlist[plat], gw.cfg.Allowlist.Approvers[plat])
		gw.groupAllowlist[plat] = make(map[string]bool)
		for _, gid := range gw.cfg.Allowlist.Groups[plat] {
			gw.groupAllowlist[plat][gid] = true
		}
	}
}

func addAllowlistUsers(dst map[string]bool, users []string) {
	for _, uid := range users {
		uid = strings.TrimSpace(uid)
		if uid != "" {
			dst[uid] = true
		}
	}
}

func (gw *BotGateway) buildSelfUserIDs() {
	for plat := range gw.cfg.SelfUserIDs {
		gw.selfUserIDs[plat] = stringSet(gw.cfg.SelfUserIDs[plat])
	}
}

// Start launches every enabled platform adapter and begins processing messages.
func (gw *BotGateway) Start(ctx context.Context) (err error) {
	gw.lifecycleMu.Lock()
	if gw.stopped {
		gw.lifecycleMu.Unlock()
		return errors.New("bot gateway already stopped")
	}
	if gw.started {
		gw.lifecycleMu.Unlock()
		return errors.New("bot gateway already started")
	}
	gw.started = true
	runCtx, cancel := context.WithCancel(ctx)
	gw.runCancel = cancel
	startDone := make(chan struct{})
	gw.startDone = startDone
	gw.lifecycleMu.Unlock()
	defer func() {
		if err != nil {
			cancel()
		}
		gw.lifecycleMu.Lock()
		if err != nil {
			gw.runCancel = nil
		}
		close(startDone)
		gw.lifecycleMu.Unlock()
	}()

	started := make([]AdapterBinding, 0, len(gw.adapters))
	var startErr []error
	for _, binding := range gw.adapters {
		if !gw.cfg.Enabled[binding.Platform] {
			gw.logger.Info("platform disabled, skipping", "platform", binding.Platform, "connection", binding.ID)
			gw.markAdapterDisabled(binding)
			continue
		}
		gw.logger.Info("starting adapter", "platform", binding.Platform, "connection", binding.ID, "domain", binding.Domain)
		if err := binding.Adapter.Start(runCtx); err != nil {
			wrapped := fmt.Errorf("start adapter %s: %w", binding.ID, err)
			startErr = append(startErr, wrapped)
			gw.markAdapterStartFailed(binding, err)
			gw.logger.Warn("adapter start failed", "platform", binding.Platform, "connection", binding.ID, "domain", binding.Domain, "err", err)
			continue
		}
		gw.markAdapterStarted(binding)
		started = append(started, binding)
	}
	// SendToAdapter reads gw.adapters under gw.mu; publish the started set under
	// the same lock.
	gw.mu.Lock()
	gw.adapters = started
	gw.startErr = startErr
	gw.mu.Unlock()
	if len(started) == 0 && len(startErr) > 0 {
		return errors.Join(startErr...)
	}
	if err := gw.startControlServer(runCtx); err != nil {
		for _, binding := range started {
			_ = binding.Adapter.Stop()
		}
		return err
	}

	// merge the message channels of every adapter
	for _, binding := range gw.adapters {
		gw.gatewayWG.Go(func() {
			gw.dispatchLoop(runCtx, binding)
		})
	}

	return nil
}

func (gw *BotGateway) AdapterCount() int {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	return len(gw.adapters)
}

func (gw *BotGateway) StartErrors() []error {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	out := make([]error, len(gw.startErr))
	copy(out, gw.startErr)
	return out
}

// AdapterHealth returns a stable snapshot of all configured adapter instances.
func (gw *BotGateway) AdapterHealth() []AdapterHealthSnapshot {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	out := make([]AdapterHealthSnapshot, 0, len(gw.adapterHealth))
	for _, health := range gw.adapterHealth {
		if health == nil {
			continue
		}
		out = append(out, *health)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (gw *BotGateway) setAdapterConfigured(binding AdapterBinding) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.ensureAdapterHealthLocked(binding).Status = "configured"
}

func (gw *BotGateway) markAdapterDisabled(binding AdapterBinding) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	health := gw.ensureAdapterHealthLocked(binding)
	health.Status = "disabled"
	health.Closed = true
}

func (gw *BotGateway) markAdapterStarted(binding AdapterBinding) {
	now := time.Now()
	gw.mu.Lock()
	defer gw.mu.Unlock()
	health := gw.ensureAdapterHealthLocked(binding)
	health.Status = "running"
	health.StartedAt = now
	health.LastError = ""
	health.Closed = false
}

func (gw *BotGateway) markAdapterStartFailed(binding AdapterBinding, err error) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	health := gw.ensureAdapterHealthLocked(binding)
	health.Status = "error"
	health.Closed = true
	health.LastErrorAt = time.Now()
	if err != nil {
		health.LastError = err.Error()
	}
}

func (gw *BotGateway) markAdapterMessage(binding AdapterBinding) {
	now := time.Now()
	gw.mu.Lock()
	defer gw.mu.Unlock()
	health := gw.ensureAdapterHealthLocked(binding)
	health.Status = "running"
	health.LastMessageAt = now
	health.Messages++
	health.Closed = false
}

func (gw *BotGateway) markAdapterClosed(binding AdapterBinding) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	health := gw.ensureAdapterHealthLocked(binding)
	if health.Status == "running" {
		health.Status = "closed"
	}
	health.Closed = true
}

func (gw *BotGateway) markAdapterSend(binding AdapterBinding, err error) {
	now := time.Now()
	gw.mu.Lock()
	defer gw.mu.Unlock()
	health := gw.ensureAdapterHealthLocked(binding)
	if err != nil {
		health.SendErrors++
		health.LastErrorAt = now
		health.LastError = err.Error()
		if health.Status == "running" {
			health.Status = "degraded"
		}
		return
	}
	health.Sends++
	health.LastSendAt = now
	if health.Status == "degraded" {
		health.Status = "running"
	}
}

func (gw *BotGateway) ensureAdapterHealthLocked(binding AdapterBinding) *AdapterHealthSnapshot {
	id := strings.TrimSpace(binding.ID)
	if id == "" && binding.Adapter != nil {
		id = binding.Adapter.Name()
	}
	if id == "" {
		id = string(binding.Platform)
	}
	health := gw.adapterHealth[id]
	if health == nil {
		health = &AdapterHealthSnapshot{ID: id}
		gw.adapterHealth[id] = health
	}
	health.Platform = binding.Platform
	health.Domain = strings.TrimSpace(binding.Domain)
	if binding.Adapter != nil {
		health.Name = binding.Adapter.Name()
	}
	if strings.TrimSpace(health.Status) == "" {
		health.Status = "configured"
	}
	return health
}

// Stop halts every adapter and closes every session. It waits for dispatch and
// turn goroutines to exit, so it must never be called synchronously from a
// GatewayConfig callback (see the OnInbound reentrancy contract), or Stop will
// wait on the goroutine running that callback itself.
func (gw *BotGateway) Stop() {
	gw.lifecycleMu.Lock()
	if gw.stopped {
		stopDone := gw.stopDone
		gw.lifecycleMu.Unlock()
		if stopDone != nil {
			<-stopDone
		}
		return
	}
	gw.stopped = true
	stopDone := make(chan struct{})
	gw.stopDone = stopDone
	cancel := gw.runCancel
	gw.runCancel = nil
	startDone := gw.startDone
	gw.lifecycleMu.Unlock()
	defer close(stopDone)

	if cancel != nil {
		cancel()
	}
	if startDone != nil {
		<-startDone
	}

	// Cancel sessions that already exist before waiting for dispatch to drain.
	// A dispatch already inside handleMessage may still publish a late session,
	// so closeSessions is repeated after gatewayWG and turnWG reach zero.
	gw.closeSessions()
	for _, binding := range gw.adapters {
		if err := binding.Adapter.Stop(); err != nil {
			gw.logger.Warn("error stopping adapter", "platform", binding.Platform, "connection", binding.ID, "err", err)
		}
		gw.markAdapterClosed(binding)
	}
	gw.stopControlServer()
	gw.gatewayWG.Wait()
	gw.closeSessions()
	gw.turnWG.Wait()
	gw.closeSessions()
}

func (gw *BotGateway) closeSessions() {
	var states []*sessionState
	gw.mu.Lock()
	for key, state := range gw.controllers {
		states = append(states, state)
		delete(gw.controllers, key)
	}
	gw.mu.Unlock()
	for _, state := range states {
		gw.closeSessionState(state)
	}
}

// closeSessionState tears down a session state that has been unlinked from
// gw.controllers. runTurn publishes state.cancel under gw.mu on every turn —
// possibly after the state was already unlinked — so snapshot and clear the
// field inside the lock and invoke it outside (the same discipline as
// cancelActiveSession).
func (gw *BotGateway) closeSessionState(state *sessionState) {
	if state == nil {
		return
	}
	// Serialize retirement with recovery ownership handoffs. Stop unlinks
	// sessions before turn goroutines drain, so a recovery callback captured by
	// the controller can still arrive here. Marking the state retired under the
	// same lock prevents that callback from reacquiring a lease after teardown;
	// an already-running handoff completes before the lease is released below.
	state.lifecycleMu.Lock()
	if state.retired {
		state.lifecycleMu.Unlock()
		return
	}
	state.retired = true
	state.lifecycleMu.Unlock()

	gw.mu.Lock()
	cancel := state.cancel
	state.cancel = nil
	gw.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if state.ctrl != nil {
		state.ctrl.Close()
	}
	if state.leases != nil {
		state.leases.Release()
	}
}

// unlinkAndCloseSessionState removes state from the live gateway before closing
// it. It is used when a controller has already rotated its transcript but the
// replacement lease could not be acquired: retaining that state would let the
// next message reuse a controller that no longer owns its active session path.
func (gw *BotGateway) unlinkAndCloseSessionState(key string, state *sessionState) {
	if state == nil {
		return
	}
	gw.mu.Lock()
	if gw.controllers[key] == state {
		delete(gw.controllers, key)
	}
	gw.mu.Unlock()
	gw.closeSessionState(state)
}

func (gw *BotGateway) dispatchLoop(ctx context.Context, binding AdapterBinding) {
	for {
		select {
		case <-ctx.Done():
			gw.markAdapterClosed(binding)
			return
		case msg, ok := <-binding.Adapter.Messages():
			if !ok {
				gw.markAdapterClosed(binding)
				return
			}
			gw.markAdapterMessage(binding)
			gw.handleMessage(ctx, binding, msg)
		}
	}
}

func (gw *BotGateway) handleMessage(ctx context.Context, binding AdapterBinding, msg InboundMessage) {
	msg.Platform = binding.Platform
	if msg.ConnectionID == "" {
		msg.ConnectionID = binding.ID
	}
	if msg.Domain == "" {
		msg.Domain = binding.Domain
	}
	if gw.isSelfMessage(msg) {
		gw.logger.Debug("bot ignored self message", "platform", binding.Platform, "connection", msg.ConnectionID, "chat", hashID(msg.ChatID), "message", hashID(msg.MessageID), "user", hashID(msg.UserID))
		return
	}
	src := msg.Session()
	key := BuildSessionKey(src)
	logFields := []any{
		"platform", binding.Platform,
		"connection", msg.ConnectionID,
		"domain", msg.Domain,
		"chat_type", msg.ChatType,
		"chat", hashID(msg.ChatID),
		"user", hashID(msg.UserID),
		"operator", hashID(msg.OperatorID),
		"thread", hashID(msg.ThreadID),
		"message", hashID(msg.MessageID),
		"text_chars", len([]rune(msg.Text)),
		"session", key[:8],
	}
	gw.logger.Info("bot inbound message", logFields...)

	// allowlist check
	if !gw.checkAllowlist(binding.Platform, msg) {
		gw.logger.Info("user not in allowlist", "platform", binding.Platform, "connection", msg.ConnectionID, "user", hashID(msg.UserID))
		if gw.offerPairing(ctx, binding.Adapter, msg) {
			return
		}
		_ = gw.sendText(ctx, binding.Adapter, msg, "죄송합니다. 이 bot을 사용할 권한이 없습니다.")
		return
	}
	if gw.cfg.OnInbound != nil {
		gw.cfg.OnInbound(msg)
	}

	if normalized, ok := gw.normalizeApprovalShortcut(key, msg.Text); ok {
		msg.Text = normalized
	} else if normalized, ok := gw.normalizeAskShortcut(key, msg.Text); ok {
		msg.Text = normalized
	} else if _, ok := decisionShortcutCommand(msg.Text); ok && gw.sessions.IsActive(key) {
		_ = gw.sendText(ctx, binding.Adapter, msg, "일치하는 대기 작업을 찾을 수 없습니다. 작업을 다시 실행한 뒤 번호로 답하거나, 메시지의 ID로 /approve, /deny, /answer를 사용하세요.")
		return
	}

	// slash command handling
	if IsSlashBypass(msg.Text) {
		gw.logger.Info("bot slash command", logFields...)
		gw.handleSlashCommand(ctx, binding.Adapter, key, msg)
		return
	}

	// A chat that took over a desktop session: plain messages drive that
	// desktop session directly, bypassing the bot's own session machine (slash
	// commands still use the branch above; /desktop release stays reachable).
	if gw.divertToDesktopTakeover(ctx, binding.Adapter, msg) {
		gw.logger.Info("bot message diverted to desktop takeover", logFields...)
		return
	}

	cleanup := gw.addPendingReaction(ctx, binding.Platform, binding.Adapter, msg)

	queueMode := gw.queueMode(key, msg)
	if gw.sessions.IsActive(key) {
		switch queueMode {
		case QueueModeSteer:
			if gw.steerActiveSession(ctx, binding.Adapter, key, msg) {
				gw.logger.Info("bot message steered into active turn", "session", key[:8])
				if cleanup != nil {
					cleanup()
				}
				_ = gw.sendText(ctx, binding.Adapter, msg, "받았습니다. 현재 작업에 병합하겠습니다.")
				return
			}
		case QueueModeInterrupt:
			gw.cancelActiveSession(key)
			runReactionCleanups(gw.takeReactionCleanups(key))
			result := gw.sessions.ReplacePending(key, msg)
			gw.storeReactionCleanup(key, cleanup)
			gw.logger.Info("bot active turn interrupted; newest message queued", "session", key[:8], "pending", result.Pending)
			_ = gw.sendText(ctx, binding.Adapter, msg, "현재 작업을 중지했습니다. 이 새 메시지는 나중에 처리하겠습니다.")
			return
		}
	}

	// session concurrency control
	result := gw.sessions.TryAcquireWithQueue(key, msg, QueueOptions{
		Mode: queueMode,
		Cap:  gw.cfg.QueueCap,
		Drop: gw.cfg.QueueDrop,
	})
	if result.Rejected {
		gw.logger.Warn("bot queue rejected message", "session", key[:8], "pending", result.Pending, "mode", result.Mode)
		if cleanup != nil {
			cleanup()
		}
		_ = gw.sendText(ctx, binding.Adapter, msg, "현재 세션 큐가 가득 찼습니다. 잠시 후 다시 보내거나 /queue interrupt로 현재 작업을 중단하세요.")
		return
	}
	if result.Queued {
		gw.logger.Debug("message queued", "session", key[:8], "mode", result.Mode, "pending", result.Pending, "dropped", result.Dropped)
		gw.storeReactionCleanup(key, cleanup)
		return
	}
	if !result.Acquired {
		gw.logger.Debug("session busy without queue action", "session", key[:8])
		gw.storeReactionCleanup(key, cleanup)
		return
	}

	// Run the turn on its own goroutine so the dispatch loop stays free to read
	// the next inbound message. A turn that hits interactive approval/ask blocks
	// inside RunTurn waiting for ctrl.Approve/AnswerQuestion — and the ONLY path
	// that calls those is handleSlashCommand on this same dispatch goroutine. Run
	// it inline and the loop can never deliver the /approve (or card) reply that
	// would unblock it: the session wedges until restart (#4701, #4863, #4402).
	// Per-session serialization is still held by the session lock (active[key]),
	// which the deferred Release inside runTurn clears.
	gw.turnWG.Go(func() {
		gw.runTurn(ctx, binding.Adapter, key, msg, cleanup)
	})
}

func (gw *BotGateway) queueMode(key string, msg InboundMessage) string {
	return gw.sessions.QueueMode(key, gw.cfg.QueueMode)
}

func (gw *BotGateway) steerActiveSession(ctx context.Context, adapter Adapter, key string, msg InboundMessage) bool {
	text := strings.TrimSpace(msg.Text)
	if text == "" && len(msg.MediaURLs) == 0 && len(msg.Media) == 0 {
		return false
	}
	gw.mu.Lock()
	state, ok := gw.controllers[key]
	gw.mu.Unlock()
	if !ok || state.ctrl == nil {
		return false
	}
	text = gw.inputTextWithMedia(ctx, adapter, msg, state)
	if strings.TrimSpace(text) == "" {
		return false
	}
	controller, ok := state.ctrl.(interface{ TrySteer(string) bool })
	return ok && controller.TrySteer(text)
}

func (gw *BotGateway) cancelActiveSession(key string) {
	// state.cancel is rewritten under gw.mu on every turn (runTurn), so copy it
	// inside the lock and invoke it outside.
	var cancel context.CancelFunc
	gw.mu.Lock()
	state, ok := gw.controllers[key]
	if ok && state != nil {
		cancel = state.cancel
	}
	gw.mu.Unlock()
	if !ok || state == nil {
		return
	}
	if cancel != nil {
		cancel()
		return
	}
	if state.ctrl != nil {
		state.ctrl.Cancel()
	}
}

func (gw *BotGateway) storeReactionCleanup(key string, cleanup func()) {
	if cleanup == nil {
		return
	}
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.pendingReactionCleanups[key] = append(gw.pendingReactionCleanups[key], cleanup)
}

func (gw *BotGateway) flushReactionCleanups(key string, cleanup func()) {
	stored := gw.takeReactionCleanups(key)
	runReactionCleanups(stored)
	if cleanup != nil {
		cleanup()
	}
}

func (gw *BotGateway) takeReactionCleanups(key string) []func() {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	stored := gw.pendingReactionCleanups[key]
	delete(gw.pendingReactionCleanups, key)
	return stored
}

func runReactionCleanups(cleanups []func()) {
	for _, cleanup := range cleanups {
		if cleanup != nil {
			cleanup()
		}
	}
}

func makeReactionCleanup(cleanups []func()) func() {
	if len(cleanups) == 0 {
		return nil
	}
	return func() {
		runReactionCleanups(cleanups)
	}
}

func (gw *BotGateway) addPendingReaction(ctx context.Context, plat Platform, adapter Adapter, msg InboundMessage) func() {
	if strings.TrimSpace(msg.MessageID) == "" {
		return nil
	}
	reactor, ok := adapter.(pendingReactionAdapter)
	if !ok {
		return nil
	}
	cleanup, err := reactor.AddPendingReaction(ctx, msg.MessageID)
	if err != nil {
		gw.logger.Warn("pending reaction failed", "platform", plat, "err", err)
		return nil
	}
	return cleanup
}

func (gw *BotGateway) isSelfMessage(msg InboundMessage) bool {
	if !gw.cfg.IgnoreSelfMessages {
		return false
	}
	actor := strings.TrimSpace(msg.UserID)
	if strings.TrimSpace(msg.OperatorID) != "" {
		actor = strings.TrimSpace(msg.OperatorID)
	}
	if actor != "" && gw.selfUserIDs[msg.Platform][actor] {
		return true
	}
	messageID := strings.TrimSpace(msg.MessageID)
	if messageID == "" {
		return false
	}
	key := outboundMessageKey(msg.Platform, msg.ConnectionID, msg.Domain, msg.ChatID, messageID)
	now := time.Now()
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.pruneOutboundMessagesLocked(now)
	_, ok := gw.outboundMessageIDs[key]
	return ok
}

func (gw *BotGateway) rememberOutboundMessage(platform Platform, connID, domain, chatID, messageID string) {
	messageID = strings.TrimSpace(messageID)
	if !gw.cfg.IgnoreSelfMessages || messageID == "" {
		return
	}
	now := time.Now()
	key := outboundMessageKey(platform, connID, domain, chatID, messageID)
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.pruneOutboundMessagesLocked(now)
	gw.outboundMessageIDs[key] = now.Add(outboundEchoTTL)
}

func (gw *BotGateway) pruneOutboundMessagesLocked(now time.Time) {
	for key, expiresAt := range gw.outboundMessageIDs {
		if !expiresAt.After(now) {
			delete(gw.outboundMessageIDs, key)
		}
	}
}

func outboundMessageKey(platform Platform, connID, domain, chatID, messageID string) string {
	return strings.Join([]string{
		string(platform),
		strings.TrimSpace(connID),
		strings.TrimSpace(domain),
		strings.TrimSpace(chatID),
		strings.TrimSpace(messageID),
	}, "\x00")
}

func (gw *BotGateway) connectionAccess(msg InboundMessage) (AccessConfig, bool) {
	if gw.cfg.ConnectionAccess == nil {
		return AccessConfig{}, false
	}
	id := strings.TrimSpace(msg.ConnectionID)
	if id == "" {
		return AccessConfig{}, false
	}
	access, ok := gw.cfg.ConnectionAccess[id]
	if !ok {
		return AccessConfig{}, false
	}
	if !accessConfigActive(access) {
		return AccessConfig{}, false
	}
	return access, true
}

func accessConfigActive(access AccessConfig) bool {
	return access.Enabled ||
		access.AllowAll ||
		access.PairingEnabled ||
		len(access.Users) > 0 ||
		len(access.Groups) > 0 ||
		len(access.Approvers) > 0 ||
		len(access.Admins) > 0
}

func (gw *BotGateway) checkAllowlist(plat Platform, msg InboundMessage) bool {
	if access, ok := gw.connectionAccess(msg); ok {
		return checkConnectionAllowlist(access, msg)
	}
	if gw.cfg.Allowlist.AllowAll {
		return true
	}
	if !gw.cfg.Allowlist.Enabled {
		return false
	}
	actor := msg.UserID
	if msg.OperatorID != "" {
		actor = msg.OperatorID
	}
	if !gw.allowlist[plat][actor] {
		return false
	}
	groups := gw.groupAllowlist[plat]
	if chatUsesGroupAllowlist(msg.ChatType) && len(groups) > 0 && !groups[msg.ChatID] {
		return false
	}
	return true
}

func checkConnectionAllowlist(access AccessConfig, msg InboundMessage) bool {
	if access.AllowAll {
		return true
	}
	if !access.Enabled {
		return false
	}
	actor := msg.UserID
	if msg.OperatorID != "" {
		actor = msg.OperatorID
	}
	users := stringSet(append(append(append([]string{}, access.Users...), access.Admins...), access.Approvers...))
	groups := stringSet(access.Groups)
	actorAllowed := users[actor]
	groupAllowed := chatUsesGroupAllowlist(msg.ChatType) && groups[msg.ChatID]
	if len(users) == 0 && len(groups) == 0 {
		return false
	}
	return actorAllowed || groupAllowed
}

func (gw *BotGateway) requireCommandRole(ctx context.Context, adapter Adapter, msg InboundMessage, role string) bool {
	if gw.checkCommandRole(msg.Platform, msg, role) {
		return true
	}
	_ = gw.sendText(ctx, adapter, msg, "죄송합니다. 이 bot 명령을 실행할 권한이 없습니다.")
	return false
}

func (gw *BotGateway) checkCommandRole(plat Platform, msg InboundMessage, role string) bool {
	actor := msg.UserID
	if msg.OperatorID != "" {
		actor = msg.OperatorID
	}
	if strings.TrimSpace(actor) == "" {
		return false
	}
	if access, ok := gw.connectionAccess(msg); ok {
		admins := stringSet(access.Admins)
		approvers := stringSet(access.Approvers)
		if len(admins) == 0 && len(approvers) == 0 {
			return true
		}
		if admins[actor] {
			return true
		}
		return role == "approver" && approvers[actor]
	}
	admins := stringSet(gw.cfg.Allowlist.Admins[plat])
	approvers := stringSet(gw.cfg.Allowlist.Approvers[plat])
	if len(admins) == 0 && len(approvers) == 0 {
		return true
	}
	if admins[actor] {
		return true
	}
	if role == "approver" && approvers[actor] {
		return true
	}
	return false
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func (gw *BotGateway) offerPairing(ctx context.Context, adapter Adapter, msg InboundMessage) bool {
	if access, ok := gw.connectionAccess(msg); ok {
		if !access.PairingEnabled {
			return false
		}
	} else if !gw.cfg.PairingEnabled {
		return false
	}
	req, created, err := CreateOrRefreshPairingRequest(msg, PairingConfig{
		Enabled:               true,
		RequestTTL:            gw.cfg.PairingTTL,
		MaxPendingPerPlatform: gw.cfg.PairingMaxPending,
	})
	if err != nil {
		gw.logger.Warn("bot pairing request failed", "platform", msg.Platform, "chat_type", msg.ChatType, "err", err)
		return false
	}
	prefix := "먼저 페어링을 완료해야 합니다."
	if !created {
		prefix = "승인 대기 중인 페어링 요청이 있습니다."
	}
	text := fmt.Sprintf("%s\n페어링 코드: %s\n로컬에서 실행하세요: patcode bot pairing approve %s\n이 코드는 %s 후에 만료됩니다.",
		prefix, req.Code, req.Code, req.ExpiresAt.Local().Format("2006-01-02 15:04"))
	_ = gw.sendText(ctx, adapter, msg, text)
	return true
}

func chatUsesGroupAllowlist(chatType ChatType) bool {
	switch chatType {
	case ChatGroup, ChatGuild, ChatThread:
		return true
	default:
		return false
	}
}

func (gw *BotGateway) normalizeApprovalShortcut(key, text string) (string, bool) {
	approvalID := gw.currentPendingApprovalID(key)
	if approvalID == "" {
		return "", false
	}
	if gw.pendingApprovalIsRecovery(key, approvalID) {
		if command, ok := recoveryShortcutCommand(text, gw.pendingRecoveryCanGrantTask(key, approvalID)); ok {
			return command + " " + approvalID, true
		}
		return "", false
	}
	command, ok := approvalShortcutCommand(text)
	if !ok {
		return "", false
	}
	return command + " " + approvalID, true
}

func approvalShortcutCommand(text string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "1", "y", "yes", "ok", "동의", "승인", "허용", "한 번 허용":
		return "/approve", true
	case "2", "0", "n", "no", "deny", "거절":
		return "/deny", true
	default:
		return "", false
	}
}

func recoveryShortcutCommand(text string, canGrantTask bool) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "1", "y", "yes", "ok", "계속", "이 변경 계속", "continue":
		return "/recovery-continue", true
	case "2", "a", "유사", "이 작업 허용", "allow similar":
		if canGrantTask {
			return "/recovery-continue-task", true
		}
		return "/recovery-revise", true
	case "3":
		if canGrantTask {
			return "/recovery-revise", true
		}
		return "", false
	case "수정", "수정안", "다른 방법", "revise":
		return "/recovery-revise", true
	default:
		return "", false
	}
}

func (gw *BotGateway) pendingRecoveryCanGrantTask(key, id string) bool {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	state, ok := gw.controllers[key]
	if !ok || state.pendingApprovals == nil {
		return false
	}
	a, ok := state.pendingApprovals[id]
	return ok && a.Recovery != nil && a.Recovery.CanGrantTask
}

func (gw *BotGateway) pendingApprovalIsRecovery(key, id string) bool {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	state, ok := gw.controllers[key]
	if !ok || state.pendingApprovals == nil {
		return false
	}
	a, ok := state.pendingApprovals[id]
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a.Kind), "recovery") || a.Recovery != nil
}

func decisionShortcutCommand(text string) (string, bool) {
	if command, ok := approvalShortcutCommand(text); ok {
		return command, true
	}
	if _, ok := askShortcutAnswer(text); ok {
		return "/answer", true
	}
	return "", false
}

func (gw *BotGateway) currentPendingApprovalID(key string) string {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	state, ok := gw.controllers[key]
	if !ok || len(state.pendingApprovals) == 0 {
		return ""
	}
	if state.lastApprovalID != "" {
		if _, ok := state.pendingApprovals[state.lastApprovalID]; ok {
			return state.lastApprovalID
		}
	}
	for id := range state.pendingApprovals {
		return id
	}
	return ""
}

func (gw *BotGateway) forgetPendingApproval(key, id string) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	state, ok := gw.controllers[key]
	if !ok || state.pendingApprovals == nil {
		return
	}
	delete(state.pendingApprovals, id)
	if state.lastApprovalID == id {
		state.lastApprovalID = ""
		for nextID := range state.pendingApprovals {
			state.lastApprovalID = nextID
			break
		}
	}
}

func (gw *BotGateway) normalizeAskShortcut(key, text string) (string, bool) {
	raw := strings.TrimSpace(text)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return "", false
	}
	askID := gw.currentPendingAskIDForReply(key)
	if askID == "" {
		return "", false
	}
	return "/answer " + askID + " " + raw, true
}

func askShortcutAnswer(text string) (string, bool) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return "", false
	}
	if strings.ContainsAny(raw, " \t\n;=") {
		return "", false
	}
	if _, err := strconv.Atoi(raw); err == nil {
		return raw, true
	}
	return "", false
}

func (gw *BotGateway) currentPendingAskIDForReply(key string) string {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	state, ok := gw.controllers[key]
	if !ok || len(state.pendingAsks) == 0 {
		return ""
	}
	if state.lastAskID != "" {
		if _, ok := state.pendingAsks[state.lastAskID]; ok {
			return state.lastAskID
		}
	}
	if len(state.pendingAsks) != 1 {
		return ""
	}
	for id := range state.pendingAsks {
		return id
	}
	return ""
}

func (gw *BotGateway) handleSlashCommand(ctx context.Context, adapter Adapter, key string, msg InboundMessage) {
	switch {
	case strings.HasPrefix(msg.Text, "/stop"):
		var cancel context.CancelFunc
		gw.mu.Lock()
		if state, ok := gw.controllers[key]; ok {
			cancel = state.cancel
		}
		gw.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		gw.sessions.ForceRelease(key)
		_ = gw.sendText(ctx, adapter, msg, "현재 작업을 중지했습니다.")

	case strings.HasPrefix(msg.Text, "/new") || strings.HasPrefix(msg.Text, "/reset"):
		var cancel context.CancelFunc
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		if ok {
			cancel = state.cancel
		}
		gw.mu.Unlock()
		if ok {
			if cancel != nil {
				cancel()
			}
			// NewSession refuses to rotate while a turn is running; the cancel
			// above is asynchronous, so give the turn a bounded window to
			// unwind before rotating.
			deadline := time.Now().Add(5 * time.Second)
			for state.ctrl.Running() && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if err := state.ctrl.NewSession(); err != nil {
				gw.logger.Warn("new session failed", "err", err)
				gw.sessions.ForceRelease(key)
				_ = gw.sendText(ctx, adapter, msg, "새 세션을 만들지 못했습니다. 잠시 후 다시 시도하세요.")
				return
			}
			if state.leases != nil {
				if err := state.leases.Rebind(state.ctrl.SessionPath()); err != nil {
					gw.logger.Warn("new session lease failed", "err", control.SessionInUseMessage(err))
					gw.unlinkAndCloseSessionState(key, state)
					gw.sessions.ForceRelease(key)
					_ = gw.sendText(ctx, adapter, msg, "새 세션 생성 실패: 쓰기 권한을 얻지 못했습니다. 다른 Patty Code 창이나 프로세스를 닫은 뒤 다시 시도하세요.")
					return
				}
			}
			// /new leaves an attached transcript and continues in the freshly
			// rotated path. Clear only the path pin while preserving any project
			// override, otherwise the next message would rebuild the old attached
			// transcript and silently undo the rotation.
			gw.mu.Lock()
			if gw.controllers[key] == state {
				state.sessionPath = ""
				if override, exists := gw.sessionOverrides[key]; exists && override.sessionPath != "" {
					override.sessionPath = ""
					gw.sessionOverrides[key] = override
				}
			}
			gw.mu.Unlock()
			gw.rememberSessionReady(msg, state.ctrl)
		}
		gw.sessions.ForceRelease(key)
		_ = gw.sendText(ctx, adapter, msg, "새 세션을 시작했습니다.")

	case strings.HasPrefix(msg.Text, "/approve"):
		if !gw.requireCommandRole(ctx, adapter, msg, "approver") {
			return
		}
		// parse the approval ID from the message
		parts := strings.Fields(msg.Text)
		if len(parts) < 2 {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /approve <id>")
			return
		}
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		gw.mu.Unlock()
		if ok && state.ctrl != nil {
			// Recovery cards map allow → continue for older clients that only know Approve.
			if gw.pendingApprovalIsRecovery(key, parts[1]) {
				_ = state.ctrl.ResolveRecovery(parts[1], agent.RecoveryActionContinue, "")
			} else {
				state.ctrl.Approve(parts[1], true, false, false)
			}
			gw.forgetPendingApproval(key, parts[1])
			_ = gw.sendText(ctx, adapter, msg, "승인했습니다.")
		} else {
			_ = gw.sendText(ctx, adapter, msg, "현재 세션에 대기 중인 승인 작업이 없습니다. 작업을 다시 실행하세요.")
		}

	case strings.HasPrefix(msg.Text, "/deny"):
		if !gw.requireCommandRole(ctx, adapter, msg, "approver") {
			return
		}
		parts := strings.Fields(msg.Text)
		if len(parts) < 2 {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /deny <id>")
			return
		}
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		gw.mu.Unlock()
		if ok && state.ctrl != nil {
			if gw.pendingApprovalIsRecovery(key, parts[1]) {
				_ = state.ctrl.ResolveRecovery(parts[1], agent.RecoveryActionRevise, "")
			} else {
				state.ctrl.Approve(parts[1], false, false, false)
			}
			gw.forgetPendingApproval(key, parts[1])
			_ = gw.sendText(ctx, adapter, msg, "거절했습니다.")
		} else {
			_ = gw.sendText(ctx, adapter, msg, "현재 세션에 대기 중인 승인 작업이 없습니다. 작업을 다시 실행하세요.")
		}

	case strings.HasPrefix(msg.Text, "/recovery-continue-task"):
		if !gw.requireCommandRole(ctx, adapter, msg, "approver") {
			return
		}
		parts := strings.Fields(msg.Text)
		if len(parts) < 2 {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /recovery-continue-task <id>")
			return
		}
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		gw.mu.Unlock()
		if ok && state.ctrl != nil {
			if err := state.ctrl.ResolveRecovery(parts[1], agent.RecoveryActionContinueTask, ""); err != nil {
				_ = gw.sendText(ctx, adapter, msg, "확인 실패: "+err.Error())
				return
			}
			gw.forgetPendingApproval(key, parts[1])
			_ = gw.sendText(ctx, adapter, msg, "계속합니다. 이 작업 내의 유사 작업은 자동으로 실행되며, 범위 확대나 위험 상승 시에는 다시 확인합니다.")
		} else {
			_ = gw.sendText(ctx, adapter, msg, "현재 세션에 확인 대기 중인 작업이 없습니다.")
		}

	case strings.HasPrefix(msg.Text, "/recovery-continue"):
		if !gw.requireCommandRole(ctx, adapter, msg, "approver") {
			return
		}
		parts := strings.Fields(msg.Text)
		if len(parts) < 2 {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /recovery-continue <id>")
			return
		}
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		gw.mu.Unlock()
		if ok && state.ctrl != nil {
			if err := state.ctrl.ResolveRecovery(parts[1], agent.RecoveryActionContinue, ""); err != nil {
				_ = gw.sendText(ctx, adapter, msg, "확인 실패: "+err.Error())
				return
			}
			gw.forgetPendingApproval(key, parts[1])
			_ = gw.sendText(ctx, adapter, msg, "계속했습니다.")
		} else {
			_ = gw.sendText(ctx, adapter, msg, "현재 세션에 확인 대기 중인 작업이 없습니다.")
		}

	case strings.HasPrefix(msg.Text, "/recovery-revise"):
		if !gw.requireCommandRole(ctx, adapter, msg, "approver") {
			return
		}
		parts := strings.Fields(msg.Text)
		if len(parts) < 2 {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /recovery-revise <id> [보충 요구사항]")
			return
		}
		feedback := strings.TrimSpace(strings.Join(parts[2:], " "))
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		gw.mu.Unlock()
		if ok && state.ctrl != nil {
			if err := state.ctrl.ResolveRecovery(parts[1], agent.RecoveryActionRevise, feedback); err != nil {
				_ = gw.sendText(ctx, adapter, msg, "수정안 적용 실패: "+err.Error())
				return
			}
			gw.forgetPendingApproval(key, parts[1])
			_ = gw.sendText(ctx, adapter, msg, "현재 변경을 거절하고 수정 요구사항을 반영했습니다.")
		} else {
			_ = gw.sendText(ctx, adapter, msg, "현재 세션에서 복구 체크포인트를 찾을 수 없습니다.")
		}

	case strings.HasPrefix(msg.Text, "/recovery-stop"):
		// Backward compatibility for cards rendered by an older client: reject
		// the proposed mutation but leave task cancellation to ordinary /stop.
		if !gw.requireCommandRole(ctx, adapter, msg, "approver") {
			return
		}
		parts := strings.Fields(msg.Text)
		if len(parts) < 2 {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /recovery-stop <id>")
			return
		}
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		gw.mu.Unlock()
		if ok && state.ctrl != nil {
			if err := state.ctrl.ResolveRecovery(parts[1], agent.RecoveryActionRevise, "cancel this proposed action"); err != nil {
				_ = gw.sendText(ctx, adapter, msg, "변경 취소 실패: "+err.Error())
				return
			}
			gw.forgetPendingApproval(key, parts[1])
			_ = gw.sendText(ctx, adapter, msg, "현재 변경을 취소했습니다. 전체 작업을 중지하려면 /stop을 사용하세요.")
		} else {
			_ = gw.sendText(ctx, adapter, msg, "현재 세션에서 복구 체크포인트를 찾을 수 없습니다.")
		}

	case strings.HasPrefix(msg.Text, "/answer"):
		parts := strings.Fields(msg.Text)
		if len(parts) < 3 {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /answer <id> <옵션 또는 q1=옵션;q2=옵션>")
			return
		}
		askID := parts[1]
		rawAnswer := strings.TrimSpace(strings.Join(parts[2:], " "))
		gw.mu.Lock()
		state, ok := gw.controllers[key]
		var questions []event.AskQuestion
		if ok {
			questions = state.pendingAsks[askID]
			delete(state.pendingAsks, askID)
			if state.lastAskID == askID {
				state.lastAskID = ""
				for nextID := range state.pendingAsks {
					state.lastAskID = nextID
					break
				}
			}
		}
		gw.mu.Unlock()
		if !ok || state.ctrl == nil {
			_ = gw.sendText(ctx, adapter, msg, "현재 세션을 찾을 수 없습니다.")
			return
		}
		answers := parseAskAnswers(questions, rawAnswer)
		state.ctrl.AnswerQuestion(askID, answers)
		_ = gw.sendText(ctx, adapter, msg, "답변을 제출했습니다.")

	case strings.HasPrefix(msg.Text, "/yolo") || strings.HasPrefix(msg.Text, "/mode"):
		if !gw.requireCommandRole(ctx, adapter, msg, "admin") {
			return
		}
		mode, statusOnly, ok := parseToolApprovalModeCommand(msg.Text)
		if !ok {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /yolo on|off|auto|status, 또는 /mode yolo|ask|auto")
			return
		}
		if statusOnly {
			_ = gw.sendText(ctx, adapter, msg, gw.toolApprovalModeStatusText(key, msg))
			return
		}
		persistErr := gw.setToolApprovalModeForMessage(key, msg, mode)
		text := toolApprovalModeChangedText(mode)
		if persistErr != nil {
			text += "\n현재 세션에는 적용되었지만 설정 저장에 실패했습니다: " + persistErr.Error()
		}
		_ = gw.sendText(ctx, adapter, msg, text)

	case strings.HasPrefix(msg.Text, "/queue"):
		mode, clear, statusOnly, ok := parseQueueCommand(msg.Text)
		if !ok {
			_ = gw.sendText(ctx, adapter, msg, "사용법: /queue steer|followup|collect|interrupt|status|default")
			return
		}
		if statusOnly {
			_ = gw.sendText(ctx, adapter, msg, gw.queueStatusText(key, msg))
			return
		}
		if clear {
			gw.sessions.ClearQueueMode(key)
			_ = gw.sendText(ctx, adapter, msg, "기본 큐 모드로 복원했습니다: "+queueModeLabel(gw.queueMode(key, msg))+".")
			return
		}
		gw.sessions.SetQueueMode(key, mode)
		_ = gw.sendText(ctx, adapter, msg, "큐 모드를 전환했습니다: "+queueModeLabel(mode)+".")

	case slashCommandVerb(msg.Text) == "/projects":
		if !gw.requireCommandRole(ctx, adapter, msg, "admin") {
			return
		}
		query := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/projects"))
		_ = gw.sendText(ctx, adapter, msg, formatBotProjects(gw.buildProjectIndex(), query, botProjectListLimit))

	case slashCommandVerb(msg.Text) == "/use":
		if !gw.requireCommandRole(ctx, adapter, msg, "admin") {
			return
		}
		_ = gw.sendText(ctx, adapter, msg, gw.handleUseProjectCommand(key, msg.Text))

	case slashCommandVerb(msg.Text) == "/sessions":
		if !gw.requireCommandRole(ctx, adapter, msg, "admin") {
			return
		}
		_ = gw.sendText(ctx, adapter, msg, gw.handleSessionsCommand(msg.Text))

	case slashCommandVerb(msg.Text) == "/attach":
		if !gw.requireCommandRole(ctx, adapter, msg, "admin") {
			return
		}
		_ = gw.sendText(ctx, adapter, msg, gw.handleAttachSessionCommand(key, msg.Text))

	case slashCommandVerb(msg.Text) == "/search":
		if !gw.requireCommandRole(ctx, adapter, msg, "admin") {
			return
		}
		_ = gw.sendText(ctx, adapter, msg, gw.handleProjectSearchCommand(ctx, msg.Text))

	case strings.HasPrefix(msg.Text, "/desktop"):
		// God view over the embedding desktop app: listing every live desktop
		// session and answering its approvals is strictly more power than the
		// per-session approver role, so gate on admin.
		if !gw.requireCommandRole(ctx, adapter, msg, "admin") {
			return
		}
		_ = gw.sendText(ctx, adapter, msg, gw.handleDesktopCommand(msg))

	case strings.HasPrefix(msg.Text, "/status"):
		active := gw.sessions.ActiveCount()
		pending := gw.sessions.PendingCount(key)
		gw.mu.Lock()
		sessions := len(gw.controllers)
		gw.mu.Unlock()
		mode := gw.currentToolApprovalMode(key, msg)
		_ = gw.sendText(ctx, adapter, msg, fmt.Sprintf("활성 작업 수: %d\n보존 세션 수: %d\n도구 승인 모드: %s\n큐 모드: %s\n현재 세션 대기: %d\n연결 상태: %s", active, sessions, toolApprovalModeLabel(mode), queueModeLabel(gw.queueMode(key, msg)), pending, gw.adapterHealthSummaryText()))

	case strings.HasPrefix(msg.Text, "/help"):
		help := "사용 가능한 명령:\n" +
			"/stop - 현재 작업 중지\n" +
			"/new - 새 세션 시작\n" +
			"/reset - 세션 초기화\n" +
			"/approve <id> - 작업 승인\n" +
			"/deny <id> - 작업 거절\n" +
			"/answer <id> <옵션> - ask 질문에 답변\n" +
			"/yolo on|off|auto|status - 도구 승인 모드 전환 또는 확인\n" +
			"/mode yolo|ask|auto - 도구 승인 모드 전환\n" +
			"/queue steer|followup|collect|interrupt|status - 큐 모드 전환 또는 확인\n" +
			"/projects [키워드] - 전환 가능한 프로젝트 인덱스 보기\n" +
			"/use project <id|이름> - 현재 원격 세션을 프로젝트로 전환\n" +
			"/sessions search <키워드> - attach할 수 있는 이전 세션 검색\n" +
			"/attach session <id|키워드> - 현재 원격 세션을 기존 이전 세션에 연결\n" +
			"/search all <키워드> - 인덱싱된 프로젝트에서 파일 내용 검색\n" +
			"/desktop status|watch|approve|deny|answer - 데스크톱 전체 보기(내장 실행 필요)\n" +
			"/status - 상태 보기\n" +
			"/help - 도움말 표시"
		_ = gw.sendText(ctx, adapter, msg, help)
	}
}

func slashCommandVerb(text string) string {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(parts[0])
}

func (gw *BotGateway) handleUseProjectCommand(key, text string) string {
	selector := parseUseProjectSelector(text)
	if selector == "" {
		return "사용법: /use project <프로젝트 id|이름|경로>, 또는 /use project default로 기본 라우팅을 복원하세요."
	}
	if isDefaultBotSelector(selector) {
		if !gw.setSessionRuntimeOverride(key, sessionRuntimeOverride{}, false) {
			return botRuntimeSwitchBusyText()
		}
		return "현재 원격 세션의 기본 프로젝트 라우팅을 복원했습니다. 다음 메시지는 bot 설정에 따라 workspace를 다시 선택합니다."
	}
	projects := gw.buildProjectIndex()
	project, matches := resolveBotProject(projects, selector)
	if project.Root == "" {
		if len(matches) > 0 {
			return "여러 프로젝트가 일치합니다. 프로젝트 id를 사용하세요:\n" + formatBotProjects(matches, "", botProjectListLimit)
		}
		return "일치하는 프로젝트가 없습니다. 먼저 /projects로 현재 인덱스를 확인하세요."
	}
	if !gw.setSessionRuntimeOverride(key, sessionRuntimeOverride{
		channel: ChannelConfig{WorkspaceRoot: project.Root},
		label:   "project:" + project.ID,
	}, true) {
		return botRuntimeSwitchBusyText()
	}
	return fmt.Sprintf("현재 원격 세션을 프로젝트 %s %s로 전환했습니다.\n다음 메시지는 %s에서 실행됩니다.", project.ID, project.Name, displayBotPath(project.Root))
}

func parseUseProjectSelector(text string) string {
	parts := strings.Fields(text)
	if len(parts) < 2 || strings.ToLower(parts[0]) != "/use" {
		return ""
	}
	if len(parts) >= 3 && strings.EqualFold(parts[1], "project") {
		return strings.TrimSpace(strings.Join(parts[2:], " "))
	}
	return strings.TrimSpace(strings.Join(parts[1:], " "))
}

func (gw *BotGateway) handleSessionsCommand(text string) string {
	query := parseSessionsQuery(text)
	projects := gw.buildProjectIndex()
	sessions := gw.buildSessionIndex(projects)
	return formatBotSessions(sessions, query, botSessionListLimit)
}

func parseSessionsQuery(text string) string {
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return ""
	}
	if strings.EqualFold(parts[1], "search") {
		return strings.TrimSpace(strings.Join(parts[2:], " "))
	}
	return strings.TrimSpace(strings.Join(parts[1:], " "))
}

func (gw *BotGateway) handleAttachSessionCommand(key, text string) string {
	selector := parseAttachSessionSelector(text)
	if selector == "" {
		return "사용법: /attach session <세션 id|키워드|path:...>"
	}
	projects := gw.buildProjectIndex()
	sessions := gw.buildSessionIndex(projects)
	session, matches := resolveBotSession(sessions, selector)
	if session.ID == "" {
		if len(matches) > 0 {
			return "여러 세션이 일치합니다. 세션 id를 사용하세요:\n" + formatBotSessions(matches, "", botSessionListLimit)
		}
		return "일치하는 세션이 없습니다. 먼저 /sessions search <키워드>로 현재 인덱스를 확인하세요."
	}
	if session.SessionPath == "" {
		return "이 세션에는 복구 가능한 path: transcript가 없어 지금은 attach할 수 없습니다."
	}
	if info, err := os.Stat(session.SessionPath); err != nil || info.IsDir() {
		return "세션 파일을 사용할 수 없거나 이동되었습니다: " + displayBotPath(session.SessionPath)
	}
	workspaceRoot := session.WorkspaceRoot
	if workspaceRoot == "" {
		project := botProjectForPath(projects, session.SessionPath)
		workspaceRoot = project.Root
	}
	if !gw.setSessionRuntimeOverride(key, sessionRuntimeOverride{
		channel:     ChannelConfig{WorkspaceRoot: workspaceRoot},
		sessionPath: session.SessionPath,
		label:       "session:" + session.ID,
	}, true) {
		return botRuntimeSwitchBusyText()
	}
	projectName := firstNonEmptyString(session.ProjectName, botProjectName(workspaceRoot), "global")
	return fmt.Sprintf("세션 %s(%s)에 attach했습니다.\n다음 메시지는 %s에서 이어집니다.", session.ID, projectName, displayBotPath(session.SessionPath))
}

func parseAttachSessionSelector(text string) string {
	parts := strings.Fields(text)
	if len(parts) < 3 || !strings.EqualFold(parts[0], "/attach") || !strings.EqualFold(parts[1], "session") {
		return ""
	}
	return strings.TrimSpace(strings.Join(parts[2:], " "))
}

func (gw *BotGateway) handleProjectSearchCommand(ctx context.Context, text string) string {
	parts := strings.Fields(text)
	if len(parts) < 3 || !strings.EqualFold(parts[1], "all") {
		return "사용법: /search all <키워드>"
	}
	query := strings.TrimSpace(strings.Join(parts[2:], " "))
	searchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	results, err := searchBotProjects(searchCtx, gw.buildProjectIndex(), query, botSearchListLimit)
	if err != nil {
		return "검색 실패: " + err.Error()
	}
	return formatBotProjectSearchResults(results, botSearchListLimit)
}

func botRuntimeSwitchBusyText() string {
	return "현재 세션에 실행 중이거나 확인 대기 중이거나 백그라운드에서 실행 중인 작업이 있습니다. 먼저 이러한 작업을 완료하거나 중지한 뒤 프로젝트를 전환하거나 세션에 attach하세요."
}

func (gw *BotGateway) setSessionRuntimeOverride(key string, override sessionRuntimeOverride, enabled bool) bool {
	return gw.sessions.runIfIdle(key, func() bool {
		var old *sessionState
		gw.mu.Lock()
		if state, ok := gw.controllers[key]; ok {
			if botSessionHasActiveWork(state) {
				gw.mu.Unlock()
				return false
			}
			old = state
			delete(gw.controllers, key)
		}
		if enabled {
			override.sessionPath = canonicalBotPath(override.sessionPath)
			override.channel.WorkspaceRoot = canonicalBotPath(override.channel.WorkspaceRoot)
			gw.sessionOverrides[key] = override
		} else {
			delete(gw.sessionOverrides, key)
		}
		gw.mu.Unlock()
		gw.closeSessionState(old)
		return true
	})
}

func botSessionHasActiveWork(state *sessionState) bool {
	if state == nil || state.ctrl == nil {
		return false
	}
	status, ok := safeBotControllerRuntimeStatus(state.ctrl)
	if !ok {
		return true
	}
	return status.Running || status.PendingPrompt || status.BackgroundJobs > 0
}

func safeBotControllerRuntimeStatus(ctrl botController) (status control.RuntimeStatus, ok bool) {
	if ctrl == nil {
		return control.RuntimeStatus{}, false
	}
	defer func() {
		if recover() != nil {
			status = control.RuntimeStatus{}
			ok = false
		}
	}()
	return ctrl.RuntimeStatus(), true
}

func (gw *BotGateway) sessionRuntimeOverrideForMessage(msg InboundMessage) (sessionRuntimeOverride, bool) {
	key := BuildSessionKey(msg.Session())
	gw.mu.Lock()
	defer gw.mu.Unlock()
	override, ok := gw.sessionOverrides[key]
	return override, ok
}

func isDefaultBotSelector(selector string) bool {
	switch strings.ToLower(strings.TrimSpace(selector)) {
	case "default", "reset", "inherit", "global", "none", "기본", "초기화":
		return true
	default:
		return false
	}
}

func parseQueueCommand(text string) (mode string, clear bool, statusOnly bool, ok bool) {
	parts := strings.Fields(text)
	if len(parts) == 0 || strings.ToLower(strings.TrimSpace(parts[0])) != "/queue" {
		return "", false, false, false
	}
	if len(parts) == 1 {
		return "", false, true, true
	}
	switch strings.ToLower(strings.TrimSpace(parts[1])) {
	case "status", "state", "show", "상태", "보기":
		return "", false, true, true
	case "default", "reset", "inherit", "기본", "초기화":
		return "", true, false, true
	default:
		if normalized := NormalizeOptionalQueueMode(parts[1]); normalized != "" {
			return normalized, false, false, true
		}
		return "", false, false, false
	}
}

func (gw *BotGateway) queueStatusText(key string, msg InboundMessage) string {
	return fmt.Sprintf("현재 큐 모드: %s\n현재 세션 대기: %d\n전역 상한: %d\n오버플로 정책: %s\n사용법: /queue steer|followup|collect|interrupt|status|default",
		queueModeLabel(gw.queueMode(key, msg)),
		gw.sessions.PendingCount(key),
		gw.cfg.QueueCap,
		queueDropLabel(gw.cfg.QueueDrop),
	)
}

func queueModeLabel(mode string) string {
	switch NormalizeQueueMode(mode) {
	case QueueModeFollowup:
		return "하나씩 후속 처리"
	case QueueModeCollect:
		return "병합 수집"
	case QueueModeInterrupt:
		return "중단 후 재실행"
	default:
		return "즉시 보충"
	}
}

func queueDropLabel(drop string) string {
	switch NormalizeQueueDrop(drop) {
	case QueueDropOld:
		return "가장 오래된 메시지 폐기"
	case QueueDropNew:
		return "새 메시지 거절"
	default:
		return "압축 요약"
	}
}

func (gw *BotGateway) adapterHealthSummaryText() string {
	snapshots := gw.AdapterHealth()
	if len(snapshots) == 0 {
		return "시작되지 않음"
	}
	parts := make([]string, 0, len(snapshots))
	for _, h := range snapshots {
		label := strings.TrimSpace(h.ID)
		if label == "" {
			label = string(h.Platform)
		}
		status := strings.TrimSpace(h.Status)
		if status == "" {
			status = "unknown"
		}
		parts = append(parts, fmt.Sprintf("%s=%s", label, status))
	}
	return strings.Join(parts, ", ")
}

func parseToolApprovalModeCommand(text string) (mode string, statusOnly bool, ok bool) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", false, false
	}
	cmd := strings.ToLower(strings.TrimSpace(parts[0]))
	switch cmd {
	case "/yolo":
		if len(parts) == 1 {
			return control.ToolApprovalYolo, false, true
		}
		return parseToolApprovalModeArg(parts[1])
	case "/mode":
		if len(parts) == 1 {
			return "", true, true
		}
		return parseToolApprovalModeArg(parts[1])
	default:
		return "", false, false
	}
}

func parseToolApprovalModeArg(arg string) (mode string, statusOnly bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "status", "state", "show", "상태", "보기":
		return "", true, true
	case "on", "enable", "enabled", "true", "1", "yolo", "full", "full-access", "bypass", "켜기", "열기":
		return control.ToolApprovalYolo, false, true
	case "off", "disable", "disabled", "false", "0", "ask", "묻기", "끄기":
		return control.ToolApprovalAsk, false, true
	case "auto", "자동":
		return control.ToolApprovalAuto, false, true
	default:
		return "", false, false
	}
}

func (gw *BotGateway) setToolApprovalModeForMessage(key string, msg InboundMessage, mode string) error {
	mode = normalizeBotToolApprovalMode(mode)
	var ctrl botController

	gw.mu.Lock()
	if state, ok := gw.controllers[key]; ok {
		ctrl = state.ctrl
	}
	gw.updateToolApprovalModeDefaultLocked(msg, mode)
	gw.mu.Unlock()

	if ctrl != nil {
		ctrl.SetToolApprovalMode(mode)
	}
	if gw.cfg.OnToolApprovalModeChange != nil {
		return gw.cfg.OnToolApprovalModeChange(msg, mode)
	}
	return nil
}

func (gw *BotGateway) updateToolApprovalModeDefaultLocked(msg InboundMessage, mode string) {
	if id := strings.TrimSpace(msg.ConnectionID); id != "" {
		if gw.cfg.ConnectionChannels == nil {
			gw.cfg.ConnectionChannels = make(map[string]ChannelConfig)
		}
		channel := gw.cfg.ConnectionChannels[id]
		channel.ToolApprovalMode = mode
		gw.cfg.ConnectionChannels[id] = channel
		return
	}
	if msg.Platform != "" {
		if gw.cfg.Channels == nil {
			gw.cfg.Channels = make(map[Platform]ChannelConfig)
		}
		channel := gw.cfg.Channels[msg.Platform]
		channel.ToolApprovalMode = mode
		gw.cfg.Channels[msg.Platform] = channel
		return
	}
	gw.cfg.ToolApprovalMode = mode
}

func (gw *BotGateway) currentToolApprovalMode(key string, msg InboundMessage) string {
	var ctrl botController
	gw.mu.Lock()
	if state, ok := gw.controllers[key]; ok {
		ctrl = state.ctrl
	}
	gw.mu.Unlock()
	if ctrl != nil {
		return ctrl.ToolApprovalMode()
	}
	_, _, mode := gw.sessionOptionsForMessage(msg)
	return mode
}

func (gw *BotGateway) toolApprovalModeStatusText(key string, msg InboundMessage) string {
	mode := gw.currentToolApprovalMode(key, msg)
	return fmt.Sprintf("현재 도구 승인 모드: %s\n사용법: /yolo on|off|auto|status, 또는 /mode yolo|ask|auto", toolApprovalModeLabel(mode))
}

func toolApprovalModeChangedText(mode string) string {
	switch normalizeBotToolApprovalMode(mode) {
	case control.ToolApprovalYolo:
		return "YOLO를 켰습니다: 일반 도구 승인은 자동으로 처리되며, Ask 질문과 계획 승인은 여전히 확인을 기다립니다."
	case control.ToolApprovalAuto:
		return "자동 모드로 전환했습니다: 정책이 허용하는 도구는 자동으로 처리되며, 묻기나 거절이 필요한 규칙은 유지됩니다."
	default:
		return "묻기 모드로 복귀했습니다: 도구 실행 전에 확인을 요청합니다."
	}
}

func toolApprovalModeLabel(mode string) string {
	switch normalizeBotToolApprovalMode(mode) {
	case control.ToolApprovalYolo:
		return "YOLO"
	case control.ToolApprovalAuto:
		return "자동"
	default:
		return "묻기"
	}
}

func (gw *BotGateway) runTurn(ctx context.Context, adapter Adapter, key string, msg InboundMessage, cleanup func()) {
	gw.logger.Info("bot turn started", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "session", key[:8])
	defer func() {
		// check for messages waiting in the queue
		next := gw.sessions.Release(key)
		if next != nil {
			if cleanup != nil {
				cleanup()
			}
			nextCleanup := makeReactionCleanup(gw.takeReactionCleanups(key))
			gw.logger.Info("bot pending message released", "platform", next.Platform, "chat_type", next.ChatType, "chat", hashID(next.ChatID), "session", key[:8])
			gw.runTurn(ctx, adapter, key, *next, nextCleanup)
			return
		}
		gw.flushReactionCleanups(key, cleanup)
	}()

	// get or create the Controller
	state := gw.getOrCreateSession(ctx, key, msg)
	if state == nil || state.ctrl == nil {
		_ = gw.sendText(ctx, adapter, msg, "내부 오류: 세션을 만들 수 없습니다.")
		return
	}
	gw.rememberSessionReady(msg, state.ctrl)

	// Build the input text: prefix the sender name in group chats and save IM
	// media as @attachment references.
	input := gw.inputTextWithMedia(ctx, adapter, msg, state)
	if msg.ChatType == ChatGroup {
		userName := strings.TrimSpace(msg.UserName)
		if msg.ResolveUserName != nil {
			if resolved := strings.TrimSpace(msg.ResolveUserName(ctx)); resolved != "" {
				userName = resolved
			}
		}
		input = fmt.Sprintf("[%s] %s", userName, input)
	}

	// send the "typing" status
	_ = adapter.SendTyping(ctx, msg.ChatID)

	// create the event rendering sink
	sink := newRenderSink(
		ctx,
		adapter,
		msg.ConnectionID,
		msg.Domain,
		msg.ChatID,
		msg.ChatType,
		msg.UserID,
		msg.MessageID,
		gw.logger,
		func(approval event.Approval) {
			gw.mu.Lock()
			if state.pendingApprovals == nil {
				state.pendingApprovals = make(map[string]event.Approval)
			}
			state.pendingApprovals[approval.ID] = approval
			state.lastApprovalID = approval.ID
			gw.mu.Unlock()
		},
		func(ask event.Ask) {
			gw.mu.Lock()
			if state.pendingAsks == nil {
				state.pendingAsks = make(map[string][]event.AskQuestion)
			}
			state.pendingAsks[ask.ID] = ask.Questions
			state.lastAskID = ask.ID
			gw.mu.Unlock()
		},
	)
	// Finish initializing the sink before publishing it as the live target: once
	// setTarget runs, other goroutines can reach this sink via state.sink.Emit.
	sink.ctrl = state.ctrl
	state.sink.setTarget(sink)
	defer state.sink.setTarget(nil)

	// create a cancellable context
	turnCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	gw.mu.Lock()
	live := gw.controllers[key] == state
	if live {
		state.cancel = cancel
	}
	state.lastActive = time.Now()
	gw.mu.Unlock()
	if !live {
		// The session was closed (gateway stop or runtime rebuild) after this
		// turn picked it up; a cancel published now would never be consumed, so
		// abort the turn instead of running it uncancellable.
		cancel()
	}

	// run one conversation turn
	err := state.ctrl.RunTurn(turnCtx, input)
	sink.Emit(event.Event{Kind: event.TurnDone, Err: err})
	if err != nil {
		gw.logger.Warn("turn error", "session", key[:8], "err", err)
		return
	}
	gw.logger.Info("bot turn completed", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "session", key[:8])
}

func (gw *BotGateway) inputTextWithMedia(ctx context.Context, adapter Adapter, msg InboundMessage, state *sessionState) string {
	input := msg.Text
	if len(msg.MediaURLs) == 0 && len(msg.Media) == 0 {
		return input
	}
	workspaceRoot := ""
	if state != nil && state.ctrl != nil {
		workspaceRoot = state.ctrl.WorkspaceRoot()
	}
	if strings.TrimSpace(workspaceRoot) == "" {
		_, workspaceRoot, _ = gw.sessionOptionsForMessage(msg)
	}
	refs, errs := saveInboundMedia(ctx, workspaceRoot, msg.MediaURLs)
	itemRefs, fallbacks, itemErrs := saveInboundMediaItems(ctx, workspaceRoot, msg.Media)
	refs = append(refs, itemRefs...)
	errs = append(errs, itemErrs...)
	if len(errs) > 0 {
		gw.logger.Warn("bot media attachment failed", "platform", msg.Platform, "chat", hashID(msg.ChatID), "errors", len(errs))
		_ = gw.sendText(ctx, adapter, msg, fmt.Sprintf("첨부 파일 %d개 저장에 실패했습니다. 사용 가능한 내용부터 처리하겠습니다.", len(errs)))
	}
	return appendMediaRefs(appendMediaFallbacks(input, fallbacks), refs)
}

func (gw *BotGateway) getOrCreateSession(ctx context.Context, key string, msg InboundMessage) *sessionState {
	profile := gw.sessionProfileForMessage(msg)
	var stale *sessionState
	gw.mu.Lock()
	if state, ok := gw.controllers[key]; ok {
		if !sessionStateMatchesRuntime(state, profile) {
			if botSessionHasActiveWork(state) {
				gw.mu.Unlock()
				safeBotSetToolApprovalMode(state.ctrl, profile.toolApprovalMode)
				gw.logger.Warn("bot session runtime change deferred while work is active", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "session", key[:8])
				return state
			}
			delete(gw.controllers, key)
			stale = state
			gw.mu.Unlock()
			gw.closeSessionState(stale)
			gw.logger.Warn("bot session runtime changed; rebuilding", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "session", key[:8], "old_workspace_set", strings.TrimSpace(stale.workspaceRoot) != "", "new_workspace_set", profile.workspaceRoot != "", "old_model", stale.model, "new_model", profile.model)
		} else {
			updateSessionStateRuntime(state, msg, profile)
			gw.mu.Unlock()
			safeBotSetToolApprovalMode(state.ctrl, profile.toolApprovalMode)
			gw.logger.Info("bot session reused", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "session", key[:8])
			return state
		}
	} else {
		gw.mu.Unlock()
	}

	// Create the lease owner before the controller so automatic conflict
	// recovery can move ownership to the recovery branch before the controller
	// commits to writing it. Without this callback the bot kept guarding the
	// original path while continuing on an unleased recovery path.
	sessionSink := &sessionEventSink{}
	leases := control.NewSessionLeaseKeeper()
	state := &sessionState{
		sink:             sessionSink,
		leases:           leases,
		platform:         msg.Platform,
		connectionID:     strings.TrimSpace(msg.ConnectionID),
		model:            profile.model,
		workspaceRoot:    profile.workspaceRoot,
		toolApprovalMode: profile.toolApprovalMode,
		sessionPath:      profile.sessionPath,
		pendingAsks:      make(map[string][]event.AskQuestion),
		createdAt:        time.Now(),
		lastActive:       time.Now(),
	}
	gw.logger.Info("bot session creating", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "session", key[:8], "model", profile.model, "workspace_set", profile.workspaceRoot != "", "tool_approval_mode", profile.toolApprovalMode)
	ctrl, err := boot.Build(ctx, boot.Options{
		Model:              profile.model,
		MaxSteps:           gw.cfg.MaxSteps,
		MaxStepsKey:        "bot.max_steps",
		RequireKey:         true,
		Sink:               sessionSink,
		StatsSource:        "bot",
		WorkspaceRoot:      profile.workspaceRoot,
		SessionDir:         botSessionDir(profile.workspaceRoot),
		ApprovalTimeout:    gw.approvalTimeout(),
		OnSessionRecovered: gw.botSessionRecoveredHandler(key, msg, state),
	})
	if err != nil {
		leases.Release()
		gw.logger.Error("build controller failed", "err", secrets.RedactError(err))
		return nil
	}
	state.ctrl = ctrl
	if profile.sessionPath != "" {
		// A mapped binding degrades to a fresh session on failure; only an
		// explicit /attach is allowed to hard-fail the message, because the
		// user named that exact session.
		degrade := func(reason string, err error) bool {
			if !profile.sessionPathOptional {
				return false
			}
			gw.logger.Warn("mapped bot session unavailable; starting fresh", "reason", reason, "session_path", profile.sessionPath, "err", err)
			profile.sessionPath = ""
			state.sessionPath = ""
			state.mappingDegraded = true
			return true
		}
		if err := leases.Rebind(profile.sessionPath); err != nil {
			if !degrade("lease held elsewhere", err) {
				ctrl.Close()
				leases.Release()
				gw.logger.Error("attached bot session is in use", "err", control.SessionInUseMessage(err))
				return nil
			}
		} else if loaded, err := agent.LoadSession(profile.sessionPath); err != nil {
			if !degrade("load failed", err) {
				ctrl.Close()
				leases.Release()
				if os.IsNotExist(err) {
					gw.logger.Error("attached bot session missing", "session_path", profile.sessionPath)
				} else {
					gw.logger.Error("attached bot session load failed", "session_path", profile.sessionPath, "err", err)
				}
				return nil
			}
		} else {
			ctrl.Resume(loaded, profile.sessionPath)
		}
	}
	ctrl.EnableInteractiveApproval()
	ctrl.SetToolApprovalMode(profile.toolApprovalMode)
	ctrl.EnsureSessionPath()
	if err := leases.Rebind(ctrl.SessionPath()); err != nil {
		ctrl.Close()
		leases.Release()
		gw.logger.Error("bot session lease failed", "err", control.SessionInUseMessage(err))
		return nil
	}

	var replace *sessionState
	gw.mu.Lock()
	// Re-check under the lock: while we were off-lock in boot.Build, a second
	// message for the same key may have built and registered its own session.
	// Reuse it only when it still targets this message's runtime profile.
	if existing, ok := gw.controllers[key]; ok {
		if sessionStateMatchesRuntime(existing, profile) {
			updateSessionStateRuntime(existing, msg, profile)
			gw.mu.Unlock()
			ctrl.Close()
			leases.Release()
			safeBotSetToolApprovalMode(existing.ctrl, profile.toolApprovalMode)
			gw.logger.Info("bot session built concurrently; discarding duplicate", "platform", msg.Platform, "chat", hashID(msg.ChatID), "session", key[:8])
			return existing
		}
		delete(gw.controllers, key)
		replace = existing
	}
	gw.controllers[key] = state
	gw.mu.Unlock()
	gw.closeSessionState(replace)

	gw.logger.Info("bot session created", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "session", key[:8])
	return state
}

func updateSessionStateRuntime(state *sessionState, msg InboundMessage, profile sessionRuntimeProfile) {
	if state == nil {
		return
	}
	if state.connectionID == "" {
		state.connectionID = strings.TrimSpace(msg.ConnectionID)
	}
	if state.platform == "" {
		state.platform = msg.Platform
	}
	state.model = profile.model
	state.workspaceRoot = profile.workspaceRoot
	state.toolApprovalMode = profile.toolApprovalMode
	state.sessionPath = profile.sessionPath
	state.lastActive = time.Now()
}

func (gw *BotGateway) sessionProfileForMessage(msg InboundMessage) sessionRuntimeProfile {
	model, workspaceRoot, toolApprovalMode := gw.sessionOptionsForMessage(msg)
	var sessionPath string
	sessionPathOptional := false
	if override, ok := gw.sessionRuntimeOverrideForMessage(msg); ok {
		sessionPath = override.sessionPath
	}
	// A persisted session_mappings binding is the durable chat→session link
	// the desktop writes into the connection config. Without consuming it
	// here, every gateway restart or runtime rebuild opened a brand-new
	// session file for the chat and the configured binding was display-only
	// (#6917, #6934).
	if sessionPath == "" {
		if mapped := gw.sessionMappingPathForMessage(msg); mapped != "" {
			sessionPath = mapped
			sessionPathOptional = true
		}
	}
	return sessionRuntimeProfile{
		model:               strings.TrimSpace(model),
		workspaceRoot:       strings.TrimSpace(workspaceRoot),
		toolApprovalMode:    normalizeBotToolApprovalMode(toolApprovalMode),
		sessionPath:         canonicalBotPath(sessionPath),
		sessionPathOptional: sessionPathOptional,
	}
}

// sessionMappingPathForMessage resolves the persisted session_mappings entry
// for a message to an existing session file. Only bindings that resolve to a
// present, readable file participate — a moved or deleted target quietly
// degrades to normal session creation rather than blocking the chat.
func (gw *BotGateway) sessionMappingPathForMessage(msg InboundMessage) string {
	gw.mu.Lock()
	var mappings []SessionMapping
	if msg.ConnectionID != "" {
		if channel, ok := gw.cfg.ConnectionChannels[msg.ConnectionID]; ok {
			mappings = channel.SessionMappings
		}
	}
	if len(mappings) == 0 {
		if channel, ok := gw.cfg.Channels[msg.Platform]; ok {
			mappings = channel.SessionMappings
		}
	}
	gw.mu.Unlock()
	mapping, ok := matchingSessionMapping(mappings, msg)
	if !ok {
		return ""
	}
	path := botSessionPathFromTarget(mapping.SessionID)
	if path == "" {
		path = botSessionPathFromTarget(mapping.SessionSource)
	}
	if path == "" {
		return ""
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return ""
	}
	return path
}

func sessionStateMatchesRuntime(state *sessionState, profile sessionRuntimeProfile) bool {
	if state == nil || state.ctrl == nil {
		return false
	}
	if stateModel := strings.TrimSpace(state.model); stateModel != "" && profile.model != "" && stateModel != profile.model {
		return false
	}
	stateRoot := strings.TrimSpace(state.workspaceRoot)
	wantRoot := strings.TrimSpace(profile.workspaceRoot)
	if stateRoot == "" {
		root, ok := safeBotControllerWorkspaceRoot(state.ctrl)
		if ok {
			stateRoot = strings.TrimSpace(root)
		} else if wantRoot != "" {
			return false
		}
	}
	if stateRoot != wantRoot {
		return false
	}
	// A state that already degraded off its mapped session keeps running on
	// its fresh path even though the profile re-resolves the mapping each
	// message; rebuilding here would spawn a new session per message while the
	// mapped file stays unavailable.
	if profile.sessionPathOptional && state.mappingDegraded {
		return true
	}
	if canonicalBotPath(state.sessionPath) != canonicalBotPath(profile.sessionPath) {
		return false
	}
	if profile.sessionPath != "" && canonicalBotPath(state.ctrl.SessionPath()) != canonicalBotPath(profile.sessionPath) {
		return false
	}
	return true
}

func safeBotControllerWorkspaceRoot(ctrl botController) (root string, ok bool) {
	if ctrl == nil {
		return "", false
	}
	defer func() {
		if recover() != nil {
			root = ""
			ok = false
		}
	}()
	return ctrl.WorkspaceRoot(), true
}

func safeBotSetToolApprovalMode(ctrl botController, mode string) {
	if ctrl == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	ctrl.SetToolApprovalMode(mode)
}

// defaultBotApprovalTimeout caps how long a bot session waits for a remote
// user's approval/ask reply before treating it as denied, so an abandoned
// prompt (or a dropped IM event) can't leave the session wedged forever
// (#4626, #4402). 30 minutes is generous for a human reply yet bounded.
const defaultBotApprovalTimeout = 30 * time.Minute

// approvalTimeout resolves the configured bot approval wait: zero uses the
// bounded default; a negative value opts out (wait indefinitely).
func (gw *BotGateway) approvalTimeout() time.Duration {
	switch {
	case gw.cfg.ApprovalTimeout < 0:
		return 0
	case gw.cfg.ApprovalTimeout == 0:
		return defaultBotApprovalTimeout
	default:
		return gw.cfg.ApprovalTimeout
	}
}

func botSessionDir(workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return config.SessionDir()
	}
	if dir := config.ProjectSessionDir(workspaceRoot); dir != "" {
		return dir
	}
	return config.SessionDir()
}

func (gw *BotGateway) rememberSessionReady(msg InboundMessage, ctrl botController) {
	if gw.cfg.OnSessionReady == nil || ctrl == nil {
		return
	}
	gw.rememberSessionPath(msg, ctrl.SessionPath())
}

func (gw *BotGateway) rememberSessionPath(msg InboundMessage, sessionPath string) {
	if gw.cfg.OnSessionReady == nil {
		return
	}
	sessionID := botSessionTarget(sessionPath)
	if sessionID == "" {
		return
	}
	if err := gw.cfg.OnSessionReady(msg, sessionID); err != nil {
		gw.logger.Warn("remember bot session failed", "platform", msg.Platform, "connection", msg.ConnectionID, "err", err)
	}
}

// botSessionRecoveredHandler keeps the controller path, its writer lease, and
// the remote-to-session mapping on the same recovery generation. The lease
// handoff runs first and is failure-atomic: if the recovery path is already
// owned, the controller stays on the original path and the old lease remains
// held. Mapping updates are limited to this exact sessionState so a late
// callback from a retired controller cannot overwrite its replacement.
func (gw *BotGateway) botSessionRecoveredHandler(key string, msg InboundMessage, state *sessionState) func(control.SessionRecoveryInfo) error {
	return func(info control.SessionRecoveryInfo) error {
		if state == nil || state.leases == nil {
			return nil
		}
		// Keep the lease handoff and mapping publication atomic with respect to
		// state retirement. In particular, never let a callback that outlives
		// Stop reacquire a lease after closeSessionState has released it.
		state.lifecycleMu.Lock()
		defer state.lifecycleMu.Unlock()
		if state.retired {
			return errBotSessionRetired
		}
		if err := state.leases.HandleSessionRecovered(info); err != nil {
			return err
		}

		originalPath := canonicalBotPath(info.OriginalPath)
		recoveryPath := canonicalBotPath(info.RecoveryPath)
		live := false
		gw.mu.Lock()
		if gw.controllers[key] == state {
			live = true
			if canonicalBotPath(state.sessionPath) == originalPath {
				state.sessionPath = recoveryPath
			}
			if override, ok := gw.sessionOverrides[key]; ok && canonicalBotPath(override.sessionPath) == originalPath {
				override.sessionPath = recoveryPath
				gw.sessionOverrides[key] = override
			}
		}
		gw.mu.Unlock()

		if live {
			gw.rememberSessionPath(msg, recoveryPath)
		}
		return nil
	}
}

func botSessionTarget(sessionPath string) string {
	sessionPath = strings.TrimSpace(sessionPath)
	if sessionPath == "" {
		return ""
	}
	return "path:" + sessionPath
}

func (gw *BotGateway) sessionOptionsForMessage(msg InboundMessage) (model string, workspaceRoot string, toolApprovalMode string) {
	// cfg.ToolApprovalMode / Channels / ConnectionChannels are rewritten under
	// gw.mu at runtime (/yolo, UpdateConnectionToolApprovalMode), so snapshot them
	// under a short lock and resolve outside it — applyRuntimeOverrideOptions
	// takes gw.mu itself. Copying the ChannelConfig value is enough: writers
	// replace whole map entries and never mutate SessionMappings in place.
	gw.mu.Lock()
	model = gw.cfg.Model
	workspaceRoot = gw.cfg.WorkspaceRoot
	toolApprovalMode = normalizeBotToolApprovalMode(gw.cfg.ToolApprovalMode)
	var connChannel ChannelConfig
	connOK := false
	if msg.ConnectionID != "" {
		connChannel, connOK = gw.cfg.ConnectionChannels[msg.ConnectionID]
	}
	platChannel, platOK := gw.cfg.Channels[msg.Platform]
	gw.mu.Unlock()

	var mappings []SessionMapping
	if connOK {
		applyBotChannelOptions(connChannel, &model, &workspaceRoot, &toolApprovalMode)
		mappings = connChannel.SessionMappings
		if mapping, ok := matchingSessionMapping(mappings, msg); ok {
			workspaceRoot = workspaceRootForSessionMapping(mapping, workspaceRoot)
		}
		model, workspaceRoot, toolApprovalMode = gw.applyRouteOptions(msg, model, workspaceRoot, toolApprovalMode)
		model, workspaceRoot, toolApprovalMode = gw.applyRuntimeOverrideOptions(msg, model, workspaceRoot, toolApprovalMode)
		return model, workspaceRoot, toolApprovalMode
	}
	if platOK {
		applyBotChannelOptions(platChannel, &model, &workspaceRoot, &toolApprovalMode)
		mappings = platChannel.SessionMappings
	}
	if mapping, ok := matchingSessionMapping(mappings, msg); ok {
		workspaceRoot = workspaceRootForSessionMapping(mapping, workspaceRoot)
	}
	model, workspaceRoot, toolApprovalMode = gw.applyRouteOptions(msg, model, workspaceRoot, toolApprovalMode)
	model, workspaceRoot, toolApprovalMode = gw.applyRuntimeOverrideOptions(msg, model, workspaceRoot, toolApprovalMode)
	return model, workspaceRoot, toolApprovalMode
}

func (gw *BotGateway) applyRuntimeOverrideOptions(msg InboundMessage, model, workspaceRoot, toolApprovalMode string) (string, string, string) {
	if override, ok := gw.sessionRuntimeOverrideForMessage(msg); ok {
		applyBotChannelOptions(override.channel, &model, &workspaceRoot, &toolApprovalMode)
	}
	return model, workspaceRoot, toolApprovalMode
}

func (gw *BotGateway) applyRouteOptions(msg InboundMessage, model, workspaceRoot, toolApprovalMode string) (string, string, string) {
	for _, route := range gw.cfg.Routes {
		if routeMatchesMessage(route, msg) {
			applyBotChannelOptions(route.Channel, &model, &workspaceRoot, &toolApprovalMode)
			break
		}
	}
	return model, workspaceRoot, toolApprovalMode
}

func applyBotChannelOptions(channel ChannelConfig, model *string, workspaceRoot *string, toolApprovalMode *string) {
	if value := strings.TrimSpace(channel.Model); value != "" {
		*model = value
	}
	if value := strings.TrimSpace(channel.WorkspaceRoot); value != "" {
		*workspaceRoot = value
	}
	if value := normalizeOptionalBotToolApprovalMode(channel.ToolApprovalMode); value != "" {
		*toolApprovalMode = value
	}
}

func matchingSessionMapping(mappings []SessionMapping, msg InboundMessage) (SessionMapping, bool) {
	for i := range mappings {
		if sessionMappingMatches(mappings[i], msg) {
			return mappings[i], true
		}
	}
	return SessionMapping{}, false
}

func sessionMappingMatches(mapping SessionMapping, msg InboundMessage) bool {
	if strings.TrimSpace(mapping.RemoteID) != strings.TrimSpace(msg.ChatID) {
		return false
	}
	chatType, userID, threadID := sessionMappingIdentity(msg)
	mappingChatType := strings.TrimSpace(mapping.ChatType)
	if mappingChatType == "" {
		return chatType == ""
	}
	if mappingChatType != chatType {
		return false
	}
	if strings.TrimSpace(mapping.UserID) != userID {
		return false
	}
	return strings.TrimSpace(mapping.ThreadID) == threadID
}

func sessionMappingIdentity(msg InboundMessage) (chatType string, userID string, threadID string) {
	switch msg.ChatType {
	case ChatGroup, ChatGuild:
		chatType = string(msg.ChatType)
		userID = strings.TrimSpace(msg.UserID)
	case ChatThread:
		chatType = string(msg.ChatType)
		threadID = strings.TrimSpace(msg.ThreadID)
		if threadID == "" {
			threadID = strings.TrimSpace(msg.ChatID)
		}
	}
	return chatType, userID, threadID
}

func workspaceRootForSessionMapping(mapping SessionMapping, fallback string) string {
	if root := strings.TrimSpace(mapping.WorkspaceRoot); root != "" {
		return root
	}
	if strings.EqualFold(strings.TrimSpace(mapping.Scope), "global") {
		return ""
	}
	return fallback
}

func routeMatchesMessage(route RouteConfig, msg InboundMessage) bool {
	if value := strings.TrimSpace(route.ConnectionID); value != "" && value != strings.TrimSpace(msg.ConnectionID) {
		return false
	}
	if route.Platform != "" && route.Platform != msg.Platform {
		return false
	}
	if route.ChatType != "" && route.ChatType != msg.ChatType {
		return false
	}
	if value := strings.TrimSpace(route.ChatID); value != "" && value != strings.TrimSpace(msg.ChatID) {
		return false
	}
	if value := strings.TrimSpace(route.UserID); value != "" && value != strings.TrimSpace(msg.UserID) {
		return false
	}
	if value := strings.TrimSpace(route.ThreadID); value != "" && value != strings.TrimSpace(msg.ThreadID) {
		return false
	}
	return true
}

func normalizeBotToolApprovalMode(mode string) string {
	if value := normalizeOptionalBotToolApprovalMode(mode); value != "" {
		return value
	}
	return control.ToolApprovalAsk
}

func normalizeOptionalBotToolApprovalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case control.ToolApprovalAsk:
		return control.ToolApprovalAsk
	case control.ToolApprovalAuto:
		return control.ToolApprovalAuto
	case control.ToolApprovalYolo, "full", "full-access", "bypass":
		return control.ToolApprovalYolo
	default:
		return ""
	}
}

func (gw *BotGateway) sendText(ctx context.Context, adapter Adapter, msg InboundMessage, text string) error {
	out := OutboundMessage{
		ConnectionID: msg.ConnectionID,
		Domain:       msg.Domain,
		ChatID:       msg.ChatID,
		ChatType:     msg.ChatType,
		Text:         text,
		ReplyToMsgID: msg.MessageID,
	}
	binding := AdapterBinding{
		ID:       strings.TrimSpace(msg.ConnectionID),
		Domain:   strings.TrimSpace(msg.Domain),
		Platform: msg.Platform,
		Adapter:  adapter,
	}
	if binding.Platform == "" && adapter != nil {
		binding.Platform = adapter.Platform()
	}
	if binding.ID == "" && adapter != nil {
		binding.ID = adapter.Name()
	}
	result, err := gw.sendViaAdapter(ctx, binding, out)
	if err != nil {
		gw.logger.Warn("bot send failed", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "reply_to", hashID(msg.MessageID), "err", err)
		return err
	}
	gw.logger.Info("bot send completed", "platform", msg.Platform, "chat_type", msg.ChatType, "chat", hashID(msg.ChatID), "reply_to", hashID(msg.MessageID), "message", hashID(result.MessageID))
	return err
}

func (gw *BotGateway) sendViaAdapter(ctx context.Context, binding AdapterBinding, msg OutboundMessage) (SendResult, error) {
	if binding.Adapter == nil {
		return SendResult{}, errors.New("bot send: adapter is nil")
	}
	if strings.TrimSpace(msg.ConnectionID) == "" {
		msg.ConnectionID = binding.ID
	}
	if strings.TrimSpace(msg.Domain) == "" {
		msg.Domain = binding.Domain
	}
	result, err := binding.Adapter.Send(ctx, msg)
	gw.markAdapterSend(binding, err)
	for _, messageID := range result.DeliveredMessageIDs() {
		gw.rememberOutboundMessage(binding.Platform, binding.ID, binding.Domain, msg.ChatID, messageID)
	}
	return result, err
}

func parseAskAnswers(questions []event.AskQuestion, raw string) []event.AskAnswer {
	raw = strings.TrimSpace(raw)
	if len(questions) == 0 {
		return []event.AskAnswer{{Selected: []string{raw}}}
	}
	byID := make(map[string]*event.AskQuestion, len(questions))
	for i := range questions {
		q := &questions[i]
		byID[q.ID] = q
		byID[fmt.Sprintf("%d", i+1)] = q
	}
	answerMap := make(map[string][]string, len(questions))
	if strings.Contains(raw, "=") {
		for part := range strings.SplitSeq(raw, ";") {
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			q := byID[strings.TrimSpace(k)]
			if q == nil {
				continue
			}
			answerMap[q.ID] = normalizeAskSelection(*q, strings.TrimSpace(v))
		}
	} else if len(questions) == 1 {
		answerMap[questions[0].ID] = normalizeAskSelection(questions[0], raw)
	}
	out := make([]event.AskAnswer, 0, len(questions))
	for _, q := range questions {
		out = append(out, event.AskAnswer{QuestionID: q.ID, Selected: answerMap[q.ID]})
	}
	return out
}

func normalizeAskSelection(q event.AskQuestion, raw string) []string {
	parts := []string{raw}
	if q.Multi && strings.Contains(raw, ",") {
		parts = strings.Split(raw, ",")
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if idx, err := strconv.Atoi(part); err == nil && idx >= 1 && idx <= len(q.Options) {
			out = append(out, q.Options[idx-1].Label)
			continue
		}
		out = append(out, part)
	}
	return out
}

// UpdateConnectionToolApprovalMode updates the in-memory tool approval mode for
// a single bot connection without restarting the gateway. Empty mode clears the
// connection override, so existing sessions inherit the current gateway default.
func (gw *BotGateway) UpdateConnectionToolApprovalMode(connID, mode string) {
	connID = strings.TrimSpace(connID)
	if connID == "" {
		return
	}
	mode = normalizeOptionalBotToolApprovalMode(mode)
	type controllerMode struct {
		ctrl botController
		mode string
	}
	var updates []controllerMode

	gw.mu.Lock()
	if gw.cfg.ConnectionChannels == nil {
		gw.cfg.ConnectionChannels = make(map[string]ChannelConfig)
	}
	ch := gw.cfg.ConnectionChannels[connID]
	ch.ToolApprovalMode = mode
	gw.cfg.ConnectionChannels[connID] = ch
	// Update every active session that belongs to this connection.
	for _, state := range gw.controllers {
		if state == nil || state.ctrl == nil || strings.TrimSpace(state.connectionID) != connID {
			continue
		}
		effectiveMode := mode
		if effectiveMode == "" {
			effectiveMode = normalizeBotToolApprovalMode(gw.cfg.ToolApprovalMode)
		}
		updates = append(updates, controllerMode{ctrl: state.ctrl, mode: effectiveMode})
	}
	gw.mu.Unlock()

	for _, update := range updates {
		update.ctrl.SetToolApprovalMode(update.mode)
	}
}

// SendToAdapter sends a message through the adapter identified by connID.
// Returns an error if no matching adapter is found.
func (gw *BotGateway) SendToAdapter(ctx context.Context, connID, domain string, msg OutboundMessage) (SendResult, error) {
	connID = strings.TrimSpace(connID)
	domain = strings.TrimSpace(domain)
	var target AdapterBinding
	gw.mu.Lock()
	for _, binding := range gw.adapters {
		if strings.TrimSpace(binding.ID) == connID &&
			(domain == "" || strings.EqualFold(strings.TrimSpace(binding.Domain), domain)) {
			target = binding
			break
		}
	}
	gw.mu.Unlock()
	if target.Adapter != nil {
		return gw.sendViaAdapter(ctx, target, msg)
	}
	return SendResult{}, fmt.Errorf("SendToAdapter: no adapter found for connection %q (domain %q)", connID, domain)
}

// SendTextToAdapter sends a plain text message through the adapter identified by connID.
func (gw *BotGateway) SendTextToAdapter(ctx context.Context, connID, domain, chatID string, chatType ChatType, text string) (SendResult, error) {
	return gw.SendToAdapter(ctx, connID, domain, OutboundMessage{
		ChatID:   chatID,
		ChatType: chatType,
		Text:     text,
	})
}
