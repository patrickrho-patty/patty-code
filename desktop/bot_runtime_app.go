package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"patty/internal/bot"
	"patty/internal/botruntime"
	"patty/internal/config"
)

type BotRuntimeStatusView struct {
	Running     bool   `json:"running"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Connections int    `json:"connections"`
	StartedAt   string `json:"startedAt"`
}

type desktopBotRuntime struct {
// [lifecycleMu serializes start/stop transitions so two apply/stop calls]
// [cant race a gateway into existence. The slow work (gw.Stop teardown,]
// [gw.Start dials) runs while holding it but NOT r.mu, so statussend reads]
	lifecycleMu sync.Mutex
	mu          sync.Mutex
	cancel      context.CancelFunc
	gw          *bot.BotGateway
	status      BotRuntimeStatusView
}

func newDesktopBotRuntime() *desktopBotRuntime {
	return &desktopBotRuntime{status: BotRuntimeStatusView{Status: "stopped", Message: "bot runtime is not started"}}
}

func (a *App) refreshBotRuntimeAsync() {
	if a.ctx == nil {
		return
	}
	a.goSafe("refreshBotRuntime", a.refreshBotRuntime)
}

func (a *App) refreshBotRuntime() {
	if a.botRuntime == nil {
		return
	}
	var watcherVersion uint64
	if a.botBridge != nil {
		watcherVersion = a.botBridge.watcherVersion()
	}
	cfg, err := a.loadDesktopBotConfig()
	if err != nil {
		a.botRuntime.stop("error", err.Error())
		return
	}
// [Assign through a typed local so a nil botBridgeHub never becomes a]
	var bridge bot.DesktopBridge
	if a.botBridge != nil {
// [[]]
// [/desktop watch 。]
		a.botBridge.seedWatchers(bridgeRoutesFromConfig(cfg.Bot.DesktopWatchers), watcherVersion)
		bridge = a.botBridge
	}
	_ = a.botRuntime.apply(a.bootContext(), cfg, globalTabWorkspaceRoot(), a.persistRemoteBotToolApprovalMode, bridge)
}

func (a *App) loadDesktopBotConfig() (*config.Config, error) {
	cfg, _, err := a.loadDesktopUserConfigForViewWithCredentials()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (a *App) stopBotRuntime() {
	if a.botRuntime != nil {
		a.botRuntime.stop("stopped", "bot runtime stopped")
	}
}

func (a *App) BotRuntimeStatus() BotRuntimeStatusView {
	if a.botRuntime == nil {
		return BotRuntimeStatusView{Status: "stopped", Message: "bot runtime is not started"}
	}
	return a.botRuntime.snapshot()
}

func (r *desktopBotRuntime) apply(parent context.Context, cfg *config.Config, workspaceRoot string, onToolApprovalModeChange func(bot.InboundMessage, string) error, bridge bot.DesktopBridge) error {
	if r == nil {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	plan := desktopBotRuntimePlan(cfg)
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	r.stopCurrent()
	if !plan.Start {
		r.setStatus(BotRuntimeStatusView{Status: plan.Status, Message: plan.Message})
		return nil
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancel := context.WithCancel(parent)
	modelName := botruntime.ModelName(cfg, "")
	channels := botruntime.ChannelConfigs(cfg.Bot.Connections, true, true)
	connectionChannels := botruntime.ConnectionChannelConfigs(cfg.Bot.Connections, true, true)
	gwCfg := bot.GatewayConfig{
		Model:              modelName,
		ToolApprovalMode:   cfg.Bot.ToolApprovalMode,
		MaxSteps:           cfg.Bot.MaxSteps,
		QueueMode:          cfg.Bot.QueueMode,
		QueueCap:           cfg.Bot.QueueCap,
		QueueDrop:          cfg.Bot.QueueDrop,
		PairingEnabled:     cfg.Bot.Pairing.Enabled,
		PairingTTL:         time.Duration(cfg.Bot.Pairing.RequestTTLMinutes) * time.Minute,
		PairingMaxPending:  cfg.Bot.Pairing.MaxPendingPerPlatform,
		IgnoreSelfMessages: cfg.Bot.IgnoreSelfMessages,
		SelfUserIDs: map[bot.Platform][]string{
			bot.Platform("desktop"): cfg.Bot.SelfUserIDs.Desktop,
		},
		ControlEnabled:     cfg.Bot.Control.Enabled,
		ControlAddr:        cfg.Bot.Control.Addr,
		ControlToken:       os.Getenv(strings.TrimSpace(cfg.Bot.Control.TokenEnv)),
		WorkspaceRoot:      workspaceRoot,
		Channels:           channels,
		ConnectionChannels: connectionChannels,
		Routes:             botruntime.RouteConfigs(cfg.Bot.Routes, true, true),
		ConnectionAccess:   botruntime.ConnectionAccessConfigs(cfg),
		Enabled:            plan.Enabled,
		Allowlist: bot.AllowlistConfig{
			Enabled:  cfg.Bot.Allowlist.Enabled,
			AllowAll: cfg.Bot.Allowlist.AllowAll,
			Users: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Users,
			},
			Approvers: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Approvers,
			},
			Admins: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Admins,
			},
			Groups: map[bot.Platform][]string{
				bot.Platform("default"): cfg.Bot.Allowlist.Groups,
			},
		},
		Debounce:                 time.Duration(cfg.Bot.DebounceMs) * time.Millisecond,
		OnInbound:                botruntime.NewRemoteRememberer(logger),
		OnSessionReady:           botruntime.NewSessionRemembererWithWorkspace(logger, workspaceRoot),
		OnToolApprovalModeChange: onToolApprovalModeChange,
		Desktop:                  bridge,
	}
	bindings := botruntime.AdapterBindings(cfg, plan.Enabled, logger)
	if len(bindings) == 0 {
		cancel()
		r.setStatus(BotRuntimeStatusView{Status: "stopped", Message: "no bot adapters configured"})
		return nil
	}
	gw := bot.NewGatewayWithAdapterBindings(gwCfg, bindings, logger)
	if err := gw.Start(ctx); err != nil {
		cancel()
		gw.Stop()
		r.setStatus(BotRuntimeStatusView{Status: "error", Message: err.Error(), Connections: gw.AdapterCount()})
		return err
	}
	runningConnections := gw.AdapterCount()
	startErrors := gw.StartErrors()
	status := "running"
	message := fmt.Sprintf("%d bot connection(s) running", runningConnections)
	if len(startErrors) > 0 {
		status = "degraded"
		message = fmt.Sprintf("%d bot connection(s) running; %d failed to start: %s", runningConnections, len(startErrors), summarizeBotRuntimeErrors(startErrors))
	}
	r.mu.Lock()
	r.cancel = cancel
	r.gw = gw
	r.status = BotRuntimeStatusView{
		Running:     true,
		Status:      status,
		Message:     message,
		Connections: runningConnections,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	r.mu.Unlock()
	return nil
}

func (a *App) persistRemoteBotToolApprovalMode(msg bot.InboundMessage, mode string) error {
	mode = normalizeBotConnectionToolApprovalMode(mode)
	if mode == "" {
		return nil
	}
	return a.applyConfigOnly(func(c *config.Config) error {
		id := strings.TrimSpace(msg.ConnectionID)
		now := time.Now().UTC().Format(time.RFC3339)
		if id != "" {
			for i := range c.Bot.Connections {
				if c.Bot.Connections[i].ID == id || botruntime.ConnectionRuntimeID(c.Bot.Connections[i]) == id {
					c.Bot.Connections[i].ToolApprovalMode = mode
					c.Bot.Connections[i].UpdatedAt = now
					return nil
				}
			}
		}
		c.Bot.ToolApprovalMode = mode
		return nil
	})
}

func summarizeBotRuntimeErrors(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, err.Error())
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) > 3 {
		hidden := len(parts) - 3
		parts = append(parts[:3], fmt.Sprintf("%d more", hidden))
	}
	return strings.Join(parts, "; ")
}

type botRuntimePlan struct {
	Start   bool
	Status  string
	Message string
	Enabled map[bot.Platform]bool
}

func desktopBotRuntimePlan(cfg *config.Config) botRuntimePlan {
	if cfg == nil {
		return botRuntimePlan{Status: "error", Message: "config is unavailable"}
	}
	if !cfg.Bot.Enabled {
		return botRuntimePlan{Status: "stopped", Message: "bot is disabled"}
	}
	if !botruntime.BotConfigHasAccessControl(cfg.Bot) {
		return botRuntimePlan{Status: "blocked", Message: "bot requires an allowlist, pairing, per-bot access, or allow_all=true"}
	}
	enabled, unknown := botruntime.EnabledPlatforms(cfg, nil)
	if len(unknown) > 0 {
		return botRuntimePlan{Status: "error", Message: "unknown bot channel: " + strings.Join(unknown, ", ")}
	}
	if !botruntime.HasEnabledPlatform(enabled) {
		return botRuntimePlan{Status: "stopped", Message: "no bot channels enabled"}
	}
	return botRuntimePlan{Start: true, Status: "running", Message: "bot runtime can start", Enabled: enabled}
}

func (r *desktopBotRuntime) stop(status, message string) {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	r.stopCurrent()
	r.setStatus(BotRuntimeStatusView{Status: status, Message: message})
}

// [grace each) and must not stall statussend readers. Callers hold lifecycleMu.]
func (r *desktopBotRuntime) stopCurrent() {
	r.mu.Lock()
	cancel := r.cancel
	gw := r.gw
	r.cancel = nil
	r.gw = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if gw != nil {
		gw.Stop()
	}
}

func (r *desktopBotRuntime) setStatus(status BotRuntimeStatusView) {
	r.mu.Lock()
	r.status = status
	r.mu.Unlock()
}

func (r *desktopBotRuntime) snapshot() BotRuntimeStatusView {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

// [updateConnectionToolApprovalMode updates a connections tool approval mode]
func (r *desktopBotRuntime) updateConnectionToolApprovalMode(connID, mode string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gw == nil {
		return false
	}
	mode = normalizeBotConnectionToolApprovalMode(mode)
	r.gw.UpdateConnectionToolApprovalMode(connID, mode)
	return true
}

// [SendToAdapter sends a message through the running gateways adapter]
func (r *desktopBotRuntime) SendToAdapter(ctx context.Context, connID, domain string, msg bot.OutboundMessage) (bot.SendResult, error) {
	r.mu.Lock()
	gw := r.gw
	r.mu.Unlock()
	if gw == nil {
		return bot.SendResult{}, nil // gateway not running — silent no-op
	}
	return gw.SendToAdapter(ctx, connID, domain, msg)
}

func (r *desktopBotRuntime) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gw != nil
}

// [current configs bot connections and their session mappings. Each mapping]
func (r *desktopBotRuntime) ForwardTargets(cfg *config.Config) []botForwardTarget {
	if cfg == nil {
		return nil
	}
	var targets []botForwardTarget
	seen := make(map[botForwardTarget]bool)
	for _, conn := range cfg.Bot.Connections {
		if !conn.Enabled {
			continue
		}
		connID := botruntime.ConnectionRuntimeID(conn)
		domain := strings.TrimSpace(conn.Domain)
		for _, sm := range conn.SessionMappings {
			remoteID := strings.TrimSpace(sm.RemoteID)
			if remoteID == "" {
				continue
			}
			chatType := bot.ChatDM
			if sm.ChatType != "" {
				chatType = bot.ChatType(sm.ChatType)
			}
			target := botForwardTarget{
				ConnID:   connID,
				Domain:   domain,
				ChatID:   remoteID,
				ChatType: chatType,
			}
			if seen[target] {
				continue
			}
			seen[target] = true
			targets = append(targets, target)
		}
	}
	return targets
}
