package botruntime

import (
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"patty/internal/bot"
	"patty/internal/config"
)

// EnabledPlatforms resolves the requested channel list against the saved config.
func EnabledPlatforms(cfg *config.Config, channels []string) (map[bot.Platform]bool, []string) {
	enabled := make(map[bot.Platform]bool)
	var warnings []string
	if len(channels) > 0 {
		for _, ch := range channels {
			ch = strings.TrimSpace(ch)
			if ch == "" {
				continue
			}
			if PlatformConfigured(cfg, bot.Platform(ch)) {
				enabled[bot.Platform(ch)] = true
			} else {
				warnings = append(warnings, ch)
			}
		}
		return enabled, warnings
	}
	for _, conn := range cfg.Bot.Connections {
		if conn.Enabled {
			enabled[bot.Platform(strings.TrimSpace(conn.Provider))] = true
		}
	}
	return enabled, warnings
}

func HasEnabledPlatform(enabled map[bot.Platform]bool) bool {
	for _, value := range enabled {
		if value {
			return true
		}
	}
	return false
}

func PlatformConfigured(cfg *config.Config, platform bot.Platform) bool {
	if cfg == nil || strings.TrimSpace(string(platform)) == "" {
		return false
	}
	for _, conn := range cfg.Bot.Connections {
		if conn.Enabled && bot.Platform(strings.TrimSpace(conn.Provider)) == platform {
			return true
		}
	}
	return false
}

func ChannelConfigs(connections []config.BotConnectionConfig, includeModel bool, includeWorkspaceRoot bool) map[bot.Platform]bot.ChannelConfig {
	if len(connections) == 0 {
		return nil
	}
	out := make(map[bot.Platform]bot.ChannelConfig)
	for _, conn := range connections {
		if !conn.Enabled {
			continue
		}
		plat := bot.Platform(strings.TrimSpace(conn.Provider))
		if strings.TrimSpace(string(plat)) == "" {
			continue
		}
		channel := out[plat]
		if includeModel {
			channel.Model = strings.TrimSpace(conn.Model)
		}
		if includeWorkspaceRoot {
			channel.WorkspaceRoot = strings.TrimSpace(conn.WorkspaceRoot)
		}
		if value := normalizeToolApprovalMode(conn.ToolApprovalMode); value != "" {
			channel.ToolApprovalMode = value
		}
		if channel.Model != "" || channel.WorkspaceRoot != "" || channel.ToolApprovalMode != "" {
			out[plat] = channel
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func ConnectionChannelConfigs(connections []config.BotConnectionConfig, includeModel bool, includeWorkspaceRoot bool) map[string]bot.ChannelConfig {
	if len(connections) == 0 {
		return nil
	}
	out := make(map[string]bot.ChannelConfig)
	for _, conn := range connections {
		if !conn.Enabled {
			continue
		}
		id := ConnectionRuntimeID(conn)
		if id == "" {
			continue
		}
		var channel bot.ChannelConfig
		if includeModel {
			channel.Model = strings.TrimSpace(conn.Model)
		}
		if includeWorkspaceRoot {
			channel.WorkspaceRoot = strings.TrimSpace(conn.WorkspaceRoot)
			channel.SessionMappings = botSessionMappings(conn.SessionMappings)
		}
		if value := normalizeToolApprovalMode(conn.ToolApprovalMode); value != "" {
			channel.ToolApprovalMode = value
		}
		if channel.Model != "" || channel.WorkspaceRoot != "" || channel.ToolApprovalMode != "" || len(channel.SessionMappings) > 0 {
			out[id] = channel
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func ConnectionAccessConfigs(cfg *config.Config) map[string]bot.AccessConfig {
	if cfg == nil {
		return nil
	}
	out := make(map[string]bot.AccessConfig)
	for _, conn := range cfg.Bot.Connections {
		if !conn.Enabled {
			continue
		}
		id := ConnectionRuntimeID(conn)
		if id == "" || !BotAccessActive(conn.Access) {
			continue
		}
		out[id] = botAccessConfig(conn.Access)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func BotAccessActive(access config.BotAccessConfig) bool {
	return access.Enabled ||
		access.AllowAll ||
		access.PairingEnabled ||
		len(access.Users) > 0 ||
		len(access.Groups) > 0 ||
		len(access.Approvers) > 0 ||
		len(access.Admins) > 0
}

func botAccessConfig(access config.BotAccessConfig) bot.AccessConfig {
	return bot.AccessConfig{
		Enabled:        access.Enabled,
		AllowAll:       access.AllowAll,
		PairingEnabled: access.PairingEnabled,
		Users:          trimStringSlice(access.Users),
		Groups:         trimStringSlice(access.Groups),
		Approvers:      trimStringSlice(access.Approvers),
		Admins:         trimStringSlice(access.Admins),
	}
}

func trimStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func botSessionMappings(mappings []config.BotConnectionSessionMapping) []bot.SessionMapping {
	if len(mappings) == 0 {
		return nil
	}
	out := make([]bot.SessionMapping, 0, len(mappings))
	for _, mapping := range mappings {
		out = append(out, bot.SessionMapping{
			RemoteID:      strings.TrimSpace(mapping.RemoteID),
			SessionID:     strings.TrimSpace(mapping.SessionID),
			SessionSource: strings.TrimSpace(mapping.SessionSource),
			ChatType:      strings.TrimSpace(mapping.ChatType),
			UserID:        strings.TrimSpace(mapping.UserID),
			ThreadID:      strings.TrimSpace(mapping.ThreadID),
			Scope:         strings.TrimSpace(mapping.Scope),
			WorkspaceRoot: strings.TrimSpace(mapping.WorkspaceRoot),
			UpdatedAt:     strings.TrimSpace(mapping.UpdatedAt),
		})
	}
	return out
}

func RouteConfigs(routes []config.BotRouteConfig, includeModel bool, includeWorkspaceRoot bool) []bot.RouteConfig {
	if len(routes) == 0 {
		return nil
	}
	out := make([]bot.RouteConfig, 0, len(routes))
	for _, route := range routes {
		var channel bot.ChannelConfig
		if includeModel {
			channel.Model = strings.TrimSpace(route.Model)
		}
		if includeWorkspaceRoot {
			channel.WorkspaceRoot = strings.TrimSpace(route.WorkspaceRoot)
		}
		if value := normalizeToolApprovalMode(route.ToolApprovalMode); value != "" {
			channel.ToolApprovalMode = value
		}
		if channel.Model == "" && channel.WorkspaceRoot == "" && channel.ToolApprovalMode == "" {
			continue
		}
		out = append(out, bot.RouteConfig{
			ConnectionID: strings.TrimSpace(route.ConnectionID),
			Platform:     bot.Platform(strings.TrimSpace(route.Platform)),
			ChatType:     bot.ChatType(strings.TrimSpace(route.ChatType)),
			ChatID:       strings.TrimSpace(route.ChatID),
			UserID:       strings.TrimSpace(route.UserID),
			ThreadID:     strings.TrimSpace(route.ThreadID),
			Channel:      channel,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeToolApprovalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "ask":
		return "ask"
	case "auto":
		return "auto"
	case "yolo", "full", "full-access", "bypass":
		return "yolo"
	default:
		return ""
	}
}

func AdapterBindings(cfg *config.Config, enabled map[bot.Platform]bool, logger *slog.Logger) []bot.AdapterBinding {
	// Patty Code no longer ships built-in IM platform adapters; connections
	// with a custom adapter host register their own bindings via NewGatewayWithAdapterBindings.
	return nil
}

func ConnectionRuntimeID(conn config.BotConnectionConfig) string {
	if id := strings.TrimSpace(conn.ID); id != "" {
		return id
	}
	provider := strings.TrimSpace(conn.Provider)
	domain := strings.TrimSpace(conn.Domain)
	if provider == "" {
		return ""
	}
	if domain == "" {
		return provider
	}
	return provider + "-" + domain
}

func ModelName(cfg *config.Config, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	if cfg == nil {
		return ""
	}
	if strings.TrimSpace(cfg.Bot.Model) != "" {
		return strings.TrimSpace(cfg.Bot.Model)
	}
	return strings.TrimSpace(cfg.DefaultModel)
}

func AllowlistUserCount(a config.BotAllowlist) int {
	return len(a.Users) + len(a.Approvers) + len(a.Admins)
}

func BotAccessUserCount(access config.BotAccessConfig) int {
	return len(access.Users) + len(access.Groups) + len(access.Approvers) + len(access.Admins)
}

func BotConfigHasAccessControl(bc config.BotConfig) bool {
	if bc.Allowlist.AllowAll || bc.Pairing.Enabled || (bc.Allowlist.Enabled && AllowlistUserCount(bc.Allowlist) > 0) {
		return true
	}
	for _, conn := range bc.Connections {
		if conn.Enabled && BotAccessActive(conn.Access) {
			return true
		}
	}
	return false
}

func NewRemoteRememberer(logger *slog.Logger) func(bot.InboundMessage) {
	var mu sync.Mutex
	seen := make(map[string]bool)
	return func(msg bot.InboundMessage) {
		remoteID := strings.TrimSpace(msg.ChatID)
		if remoteID == "" {
			return
		}
		key := strings.Join([]string{
			string(msg.Platform),
			strings.TrimSpace(msg.ConnectionID),
			strings.TrimSpace(msg.Domain),
			string(msg.ChatType),
			remoteID,
			strings.TrimSpace(msg.UserID),
		}, "\x00")
		mu.Lock()
		if seen[key] {
			mu.Unlock()
			return
		}
		seen[key] = true
		mu.Unlock()

		if err := RememberInbound(msg); err != nil && logger != nil {
			logger.Warn("remember bot remote failed", "platform", msg.Platform, "err", err)
		}
	}
}

func NewSessionRememberer(logger *slog.Logger) func(bot.InboundMessage, string) error {
	return NewSessionRemembererWithWorkspace(logger, "")
}

func NewSessionRemembererWithWorkspace(logger *slog.Logger, workspaceRoot string) func(bot.InboundMessage, string) error {
	return func(msg bot.InboundMessage, sessionID string) error {
		if err := RememberInboundSessionWorkspace(msg, sessionID, workspaceRoot); err != nil {
			if logger != nil {
				logger.Warn("remember bot session failed", "platform", msg.Platform, "err", err)
			}
			return err
		}
		return nil
	}
}

func RememberInbound(msg bot.InboundMessage) error {
	return rememberInbound(msg, "", "")
}

func RememberInboundSession(msg bot.InboundMessage, sessionID string) error {
	return RememberInboundSessionWorkspace(msg, sessionID, "")
}

func RememberInboundSessionWorkspace(msg bot.InboundMessage, sessionID string, workspaceRoot string) error {
	return rememberInbound(msg, strings.TrimSpace(sessionID), strings.TrimSpace(workspaceRoot))
}

func ForgetAutoSessionMappingsForPath(sessionPath string) error {
	target := normalizedBotSessionPath(sessionPath)
	if target == "" {
		return nil
	}
	userPath := config.UserConfigPath()
	if strings.TrimSpace(userPath) == "" {
		return nil
	}
	unlock := config.LockUserConfigEdits()
	defer unlock()

	cfg := config.LoadForEdit(userPath)
	now := time.Now().UTC().Format(time.RFC3339)
	changed := false
	for i := range cfg.Bot.Connections {
		conn := &cfg.Bot.Connections[i]
		next := conn.SessionMappings[:0]
		removed := false
		for _, mapping := range conn.SessionMappings {
			if strings.TrimSpace(mapping.SessionSource) == "auto" && normalizedBotSessionPath(mapping.SessionID) == target {
				removed = true
				continue
			}
			next = append(next, mapping)
		}
		if !removed {
			continue
		}
		conn.SessionMappings = next
		conn.UpdatedAt = now
		changed = true
	}
	if !changed {
		return nil
	}
	return cfg.SaveTo(userPath)
}

func rememberInbound(msg bot.InboundMessage, sessionID string, actualWorkspaceRoot string) error {
	userPath := config.UserConfigPath()
	platform := msg.Platform
	remoteID := strings.TrimSpace(msg.ChatID)
	if userPath == "" || remoteID == "" {
		return nil
	}
	unlock := config.LockUserConfigEdits()
	defer unlock()

	cfg := config.LoadForEdit(userPath)
	now := time.Now().UTC().Format(time.RFC3339)
	changed := false
	for i := range cfg.Bot.Connections {
		conn := &cfg.Bot.Connections[i]
		if strings.TrimSpace(conn.Provider) != string(platform) || !conn.Enabled || !connectionMatchesInbound(*conn, msg) {
			continue
		}
		mappingIndex := -1
		for j := range conn.SessionMappings {
			if botSessionMappingMatches(conn.SessionMappings[j], msg) {
				mappingIndex = j
				break
			}
		}
		if mappingIndex >= 0 {
			if sessionID == "" {
				continue
			}
			mapping := &conn.SessionMappings[mappingIndex]
			current := strings.TrimSpace(mapping.SessionID)
			if current == sessionID || botSessionMappingHasExplicitTarget(*mapping) {
				continue
			}
			mapping.SessionID = sessionID
			mapping.SessionSource = "auto"
			mapping.UpdatedAt = now
			conn.UpdatedAt = now
			changed = true
			continue
		}
		scope := "global"
		workspaceRoot := ""
		if strings.TrimSpace(conn.WorkspaceRoot) != "" {
			scope = "project"
			workspaceRoot = strings.TrimSpace(conn.WorkspaceRoot)
		} else if actualWorkspaceRoot != "" {
			scope = "project"
			workspaceRoot = actualWorkspaceRoot
		}
		chatType, userID, threadID := botSessionMappingIdentity(msg)
		conn.SessionMappings = append(conn.SessionMappings, config.BotConnectionSessionMapping{
			RemoteID:      remoteID,
			SessionID:     sessionID,
			SessionSource: botSessionSource(sessionID),
			ChatType:      chatType,
			UserID:        userID,
			ThreadID:      threadID,
			Scope:         scope,
			WorkspaceRoot: workspaceRoot,
			UpdatedAt:     now,
		})
		conn.UpdatedAt = now
		changed = true
	}
	if rememberAllowlist(&cfg.Bot.Allowlist, platform, msg.UserID, remoteID, msg.ChatType) {
		changed = true
	}
	if !changed {
		return nil
	}
	return cfg.SaveTo(userPath)
}

func botSessionMappingMatches(mapping config.BotConnectionSessionMapping, msg bot.InboundMessage) bool {
	if strings.TrimSpace(mapping.RemoteID) != strings.TrimSpace(msg.ChatID) {
		return false
	}
	chatType, userID, threadID := botSessionMappingIdentity(msg)
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

func botSessionMappingIdentity(msg bot.InboundMessage) (chatType string, userID string, threadID string) {
	switch msg.ChatType {
	case bot.ChatGroup, bot.ChatGuild:
		chatType = string(msg.ChatType)
		userID = strings.TrimSpace(msg.UserID)
	case bot.ChatThread:
		chatType = string(msg.ChatType)
		threadID = strings.TrimSpace(msg.ThreadID)
		if threadID == "" {
			threadID = strings.TrimSpace(msg.ChatID)
		}
	}
	return chatType, userID, threadID
}

func botSessionMappingHasExplicitTarget(mapping config.BotConnectionSessionMapping) bool {
	sessionID := strings.TrimSpace(mapping.SessionID)
	if sessionID == "" || strings.TrimSpace(mapping.SessionSource) == "auto" {
		return false
	}
	return true
}

func botSessionSource(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return ""
	}
	return "auto"
}

func normalizedBotSessionPath(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(sessionID), "path:") {
		sessionID = strings.TrimSpace(sessionID[5:])
	}
	if sessionID == "" {
		return ""
	}
	if !(strings.HasSuffix(sessionID, ".jsonl") || strings.Contains(sessionID, "/") || strings.Contains(sessionID, `\`) || strings.HasPrefix(sessionID, "~")) {
		return ""
	}
	return filepath.Clean(sessionID)
}

func connectionMatchesInbound(conn config.BotConnectionConfig, msg bot.InboundMessage) bool {
	if msg.ConnectionID != "" {
		return ConnectionRuntimeID(conn) == strings.TrimSpace(msg.ConnectionID)
	}
	if msg.Domain != "" && strings.TrimSpace(conn.Domain) != "" {
		return strings.EqualFold(strings.TrimSpace(conn.Domain), strings.TrimSpace(msg.Domain))
	}
	return true
}

func rememberAllowlist(allowlist *config.BotAllowlist, platform bot.Platform, userID string, chatID string, chatType bot.ChatType) bool {
	if allowlist == nil {
		return false
	}
	changed := false
	userID = strings.TrimSpace(userID)
	if userID != "" {
		allowlist.Users, changed = appendUniqueString(allowlist.Users, userID)
	}
	if !chatUsesGroupAllowlist(chatType) {
		return changed
	}
	groupID := strings.TrimSpace(chatID)
	if groupID == "" {
		return changed
	}
	groupsChanged := false
	allowlist.Groups, groupsChanged = appendUniqueString(allowlist.Groups, groupID)
	return changed || groupsChanged
}

func appendUniqueString(values []string, next string) ([]string, bool) {
	next = strings.TrimSpace(next)
	if next == "" {
		return values, false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == next {
			return values, false
		}
	}
	return append(values, next), true
}

func chatUsesGroupAllowlist(chatType bot.ChatType) bool {
	switch chatType {
	case bot.ChatGroup, bot.ChatGuild, bot.ChatThread:
		return true
	default:
		return false
	}
}

func firstNonEmptyString(vals ...string) string {
	for _, val := range vals {
		if strings.TrimSpace(val) != "" {
			return val
		}
	}
	return ""
}
