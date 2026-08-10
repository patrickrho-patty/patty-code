package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"patty/internal/botruntime"
	"patty/internal/config"
)

type BotConnectionCredentialView struct {
	AppID        string `json:"appId"`
	AppSecretEnv string `json:"appSecretEnv"`
	AccountID    string `json:"accountId"`
	TokenEnv     string `json:"tokenEnv"`
	SecretSet    bool   `json:"secretSet"`
}

type BotConnectionSessionMappingView struct {
	RemoteID      string `json:"remoteId"`
	SessionID     string `json:"sessionId"`
	SessionSource string `json:"sessionSource"`
	ChatType      string `json:"chatType"`
	UserID        string `json:"userId"`
	ThreadID      string `json:"threadId"`
	Scope         string `json:"scope"`
	WorkspaceRoot string `json:"workspaceRoot"`
	UpdatedAt     string `json:"updatedAt"`
}

type BotConnectionView struct {
	ID               string                            `json:"id"`
	Provider         string                            `json:"provider"`
	Domain           string                            `json:"domain"`
	Label            string                            `json:"label"`
	Enabled          bool                              `json:"enabled"`
	Status           string                            `json:"status"`
	Model            string                            `json:"model"`
	ToolApprovalMode string                            `json:"toolApprovalMode"`
	WorkspaceRoot    string                            `json:"workspaceRoot"`
	Access           BotAccessView                     `json:"access"`
	Credential       BotConnectionCredentialView       `json:"credential"`
	SessionMappings  []BotConnectionSessionMappingView `json:"sessionMappings"`
	LastError        string                            `json:"lastError"`
	CreatedAt        string                            `json:"createdAt"`
	UpdatedAt        string                            `json:"updatedAt"`
}

type BotConnectionDiagnostic struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	MessageID    string `json:"messageId"`
	Phase        string `json:"phase"`
	Code         string `json:"code"`
	ReportKind   string `json:"reportKind"`
	ReportDetail string `json:"reportDetail"`
	OccurredAt   string `json:"occurredAt"`
}

func (a *App) DiagnoseBotConnection(id string) (BotConnectionDiagnostic, error) {
	cfg, err := a.loadDesktopBotConfig()
	if err != nil {
		return botConnectionDiagnostic(nil, id, "error", "config", "config_load_failed", err.Error(), true), nil
	}
	for _, conn := range cfg.Bot.Connections {
		if conn.ID == id {
			status := "ok"
			message := "연결 구성이 저장되었습니다."
			phase := "config"
			code := "config_ok"
			reportable := false
			if !conn.Enabled {
				status = "disabled"
				message = "연결이 저장되었지만 활성화되지 않았습니다."
				code = "connection_disabled"
			} else if conn.Status != "connected" {
				status = firstNonEmptyBot(conn.Status, "pending")
				message = firstNonEmptyBot(conn.LastError, "연결이 아직 완료되지 않았습니다.")
				phase = "install"
				code = "connection_not_connected"
				reportable = status == "error" || strings.TrimSpace(conn.LastError) != ""
			} else if conn.Credential.AppSecretEnv != "" && strings.TrimSpace(conn.Credential.AppSecretEnv) != "" && !envIsSet(conn.Credential.AppSecretEnv) {
				status = "warning"
				message = conn.Credential.AppSecretEnv + " 미설정."
				phase = "credential"
				code = "secret_missing"
				reportable = true
			} else if conn.Credential.TokenEnv != "" && strings.TrimSpace(conn.Credential.TokenEnv) != "" && !botCredentialSecretSet(conn) {
				status = "warning"
				message = conn.Credential.TokenEnv + " 미설정, 저장된 로그인 자격 증명도 없음."
				phase = "credential"
				code = "secret_missing"
				reportable = true
			}
			return botConnectionDiagnostic(&conn, conn.ID, status, phase, code, message, reportable), nil
		}
	}
	return botConnectionDiagnostic(nil, id, "missing", "config", "connection_missing", "연결을 찾을 수 없습니다.", true), nil
}

func (a *App) TestBotConnection(id, target string) (BotConnectionDiagnostic, error) {
	cfg, err := a.loadDesktopBotConfig()
	if err != nil {
		return botConnectionDiagnostic(nil, id, "error", "config", "config_load_failed", err.Error(), true), nil
	}
	var conn *config.BotConnectionConfig
	for i := range cfg.Bot.Connections {
		if cfg.Bot.Connections[i].ID == strings.TrimSpace(id) {
			conn = &cfg.Bot.Connections[i]
			break
		}
	}
	if conn == nil {
		return botConnectionDiagnostic(nil, id, "missing", "config", "connection_missing", "연결을 찾을 수 없음.", true), nil
	}
	return botConnectionDiagnostic(conn, conn.ID, "warning", "send", "test_send_unsupported", "현재 채널은 데스크톱에서 테스트 메시지를 보낼 수 없음. 진단으로 기본 구성을 확인하세요.", false), nil
}

func botConnectionDiagnostic(conn *config.BotConnectionConfig, id, status, phase, code, message string, reportable bool) BotConnectionDiagnostic {
	id = strings.TrimSpace(id)
	label := ""
	if conn != nil {
		id = firstNonEmptyBot(strings.TrimSpace(conn.ID), id)
		label = strings.TrimSpace(conn.Label)
	}
	occurredAt := time.Now().UTC().Format(time.RFC3339)
	diag := BotConnectionDiagnostic{
		ID:         id,
		Label:      label,
		Status:     strings.TrimSpace(status),
		Message:    strings.TrimSpace(message),
		Phase:      strings.TrimSpace(phase),
		Code:       strings.TrimSpace(code),
		OccurredAt: occurredAt,
	}
	if reportable {
		diag.ReportKind = "bot"
		diag.ReportDetail = botConnectionReportDetail(conn, id, diag.Status, diag.Phase, diag.Code, diag.Message, occurredAt)
		if diag.ReportDetail == "" {
			diag.ReportKind = ""
		}
	}
	return diag
}

func botConnectionReportDetail(conn *config.BotConnectionConfig, fallbackID, status, phase, code, message, occurredAt string) string {
	provider := "unknown"
	domain := "unknown"
	configuredStatus := ""
	enabled := false
	workspaceScope := "global"
	sessionMappings := 0
	appIDSet := false
	appSecretEnvConfigured := false
	tokenEnvConfigured := false
	secretAvailable := false
	if conn != nil {
		provider = firstNonEmptyBot(strings.TrimSpace(conn.Provider), provider)
		domain = firstNonEmptyBot(strings.TrimSpace(conn.Domain), domain)
		configuredStatus = strings.TrimSpace(conn.Status)
		enabled = conn.Enabled
		if strings.TrimSpace(conn.WorkspaceRoot) != "" {
			workspaceScope = "project"
		}
		sessionMappings = len(conn.SessionMappings)
		appIDSet = strings.TrimSpace(conn.Credential.AppID) != ""
		appSecretEnvConfigured = strings.TrimSpace(conn.Credential.AppSecretEnv) != ""
		tokenEnvConfigured = strings.TrimSpace(conn.Credential.TokenEnv) != ""
		secretAvailable = botCredentialSecretSet(*conn)
	}
	summary := botConnectionReportSummary(code, message)
	lines := []string{
		"Bot connection diagnostic",
		"",
		"connection_id: " + safeBotReportValue(fallbackID),
		"provider: " + safeBotReportValue(provider),
		"domain: " + safeBotReportValue(domain),
		"status: " + safeBotReportValue(status),
		"phase: " + safeBotReportValue(phase),
		"code: " + safeBotReportValue(code),
		fmt.Sprintf("enabled: %t", enabled),
		"configured_status: " + safeBotReportValue(configuredStatus),
		fmt.Sprintf("app_id_set: %t", appIDSet),
		fmt.Sprintf("app_secret_env_configured: %t", appSecretEnvConfigured),
		fmt.Sprintf("token_env_configured: %t", tokenEnvConfigured),
		fmt.Sprintf("secret_available: %t", secretAvailable),
		"workspace_scope: " + workspaceScope,
		fmt.Sprintf("session_mappings: %d", sessionMappings),
		"",
		"summary: " + summary,
	}
	payload := frontendCrashPayload{
		SchemaVersion: 2,
		Kind:          "bot",
		Source:        "bot.runtime",
		Label:         botConnectionReportLabel(provider, domain, phase),
		Message:       strings.Join(lines, "\n"),
		ErrorType:     "BotConnectionDiagnostic",
		ErrorMessage:  summary,
		TopFrame:      "bot." + safeBotReportSegment(phase),
		OccurredAt:    occurredAt,
	}
	detail, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(detail)
}

func botConnectionReportSummary(code, message string) string {
	switch strings.TrimSpace(code) {
	case "config_load_failed":
		return "desktop bot config could not be loaded: " + scrubSensitiveText(message)
	case "connection_missing":
		return "bot connection record was not found"
	case "connection_not_connected":
		return "bot connection is not connected: " + scrubSensitiveText(message)
	case "secret_missing":
		return "required bot credential is not available"
	case "test_send_failed":
		return "bot test message failed: " + scrubSensitiveText(message)
	default:
		if strings.TrimSpace(message) == "" {
			return strings.TrimSpace(code)
		}
		return scrubSensitiveText(message)
	}
}

func botConnectionReportLabel(provider, domain, phase string) string {
	parts := []string{"bot", safeBotReportSegment(provider), safeBotReportSegment(domain), safeBotReportSegment(phase)}
	return strings.Trim(strings.Join(parts, "."), ".")
}

func safeBotReportSegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		if b.Len() == 0 || strings.HasSuffix(b.String(), ".") {
			continue
		}
		b.WriteByte('.')
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		return "unknown"
	}
	return out
}

func safeBotReportValue(s string) string {
	s = safeBotReportSegment(s)
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

func (a *App) upsertBotConnection(conn config.BotConnectionConfig, updateLegacy func(*config.Config)) (BotConnectionView, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if conn.CreatedAt == "" {
		conn.CreatedAt = now
	}
	conn.UpdatedAt = now
	if conn.Status == "" {
		conn.Status = "connected"
	}
	if normalizeBotConnectionToolApprovalMode(conn.ToolApprovalMode) == "" {
		conn.ToolApprovalMode = "ask"
	}
	if conn.ID == "" {
		conn.ID = connectionID(conn.Provider, conn.Domain)
	}
	err := a.applyConfigOnly(func(c *config.Config) error {
		if updateLegacy != nil {
			updateLegacy(c)
		}
		replaced := false
		for i, existing := range c.Bot.Connections {
			if existing.ID == conn.ID {
				conn.CreatedAt = firstNonEmptyBot(existing.CreatedAt, conn.CreatedAt)
				if !botruntime.BotAccessActive(conn.Access) && botruntime.BotAccessActive(existing.Access) {
					conn.Access = existing.Access
				}
				c.Bot.Connections[i] = conn
				replaced = true
				break
			}
		}
		if !replaced {
			c.Bot.Connections = append(c.Bot.Connections, conn)
		}
		return nil
	})
	return botConnectionView(conn), err
}

func (a *App) rememberBotConnectionRemote(id, remoteID string) error {
	id = strings.TrimSpace(id)
	remoteID = strings.TrimSpace(remoteID)
	if id == "" || remoteID == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return a.applyConfigOnly(func(c *config.Config) error {
		for i := range c.Bot.Connections {
			if c.Bot.Connections[i].ID != id {
				continue
			}
			for j := range c.Bot.Connections[i].SessionMappings {
				if c.Bot.Connections[i].SessionMappings[j].RemoteID == remoteID {
					workspaceRoot := firstNonEmptyBot(c.Bot.Connections[i].SessionMappings[j].WorkspaceRoot, c.Bot.Connections[i].WorkspaceRoot)
					scope := botMappingScope(c.Bot.Connections[i].SessionMappings[j].Scope, workspaceRoot)
					c.Bot.Connections[i].SessionMappings[j].Scope = scope
					c.Bot.Connections[i].SessionMappings[j].WorkspaceRoot = botMappingWorkspaceRoot(scope, workspaceRoot)
					c.Bot.Connections[i].SessionMappings[j].UpdatedAt = now
					c.Bot.Connections[i].UpdatedAt = now
					return nil
				}
			}
			scope := botMappingScope("", c.Bot.Connections[i].WorkspaceRoot)
			c.Bot.Connections[i].SessionMappings = append(c.Bot.Connections[i].SessionMappings, config.BotConnectionSessionMapping{
				RemoteID:      remoteID,
				SessionID:     "",
				Scope:         scope,
				WorkspaceRoot: botMappingWorkspaceRoot(scope, c.Bot.Connections[i].WorkspaceRoot),
				UpdatedAt:     now,
			})
			c.Bot.Connections[i].UpdatedAt = now
			return nil
		}
		return nil
	})
}

func firstSessionRemoteID(mappings []config.BotConnectionSessionMapping) string {
	for _, mapping := range mappings {
		if strings.TrimSpace(mapping.RemoteID) != "" {
			return strings.TrimSpace(mapping.RemoteID)
		}
	}
	return ""
}

func botConnectionView(conn config.BotConnectionConfig) BotConnectionView {
	return BotConnectionView{
		ID: conn.ID, Provider: conn.Provider, Domain: conn.Domain, Label: conn.Label, Enabled: conn.Enabled, Status: conn.Status,
		Model: conn.Model, ToolApprovalMode: normalizeBotConnectionToolApprovalMode(conn.ToolApprovalMode), WorkspaceRoot: conn.WorkspaceRoot,
		Access: botAccessViewFromConfig(conn.Access),
		Credential: BotConnectionCredentialView{
			AppID: conn.Credential.AppID, AppSecretEnv: conn.Credential.AppSecretEnv, AccountID: conn.Credential.AccountID, TokenEnv: conn.Credential.TokenEnv,
			SecretSet: botCredentialSecretSet(conn),
		},
		SessionMappings: botSessionMappingViews(conn.SessionMappings, conn.WorkspaceRoot),
		LastError:       conn.LastError, CreatedAt: conn.CreatedAt, UpdatedAt: conn.UpdatedAt,
	}
}

func botCredentialSecretSet(conn config.BotConnectionConfig) bool {
	if conn.Credential.AppSecretEnv != "" {
		return envIsSet(conn.Credential.AppSecretEnv)
	}
	return conn.Credential.TokenEnv != "" && envIsSet(conn.Credential.TokenEnv)
}

func botConnectionViews(connections []config.BotConnectionConfig) []BotConnectionView {
	if connections == nil {
		return []BotConnectionView{}
	}
	out := make([]BotConnectionView, 0, len(connections))
	for _, conn := range connections {
		out = append(out, botConnectionView(conn))
	}
	return out
}

func botConnectionConfig(view BotConnectionView) config.BotConnectionConfig {
	return config.BotConnectionConfig{
		ID:               strings.TrimSpace(view.ID),
		Provider:         strings.TrimSpace(view.Provider),
		Domain:           strings.TrimSpace(view.Domain),
		Label:            strings.TrimSpace(view.Label),
		Enabled:          view.Enabled,
		Status:           strings.TrimSpace(view.Status),
		Model:            strings.TrimSpace(view.Model),
		ToolApprovalMode: firstNonEmptyBot(normalizeBotConnectionToolApprovalMode(view.ToolApprovalMode), "ask"),
		WorkspaceRoot:    strings.TrimSpace(view.WorkspaceRoot),
		Access:           botAccessConfigFromView(view.Access),
		Credential: config.BotConnectionCredential{
			AppID:        strings.TrimSpace(view.Credential.AppID),
			AppSecretEnv: strings.TrimSpace(view.Credential.AppSecretEnv),
			AccountID:    strings.TrimSpace(view.Credential.AccountID),
			TokenEnv:     strings.TrimSpace(view.Credential.TokenEnv),
		},
		SessionMappings: botSessionMappingConfigs(view.SessionMappings, view.WorkspaceRoot),
		LastError:       strings.TrimSpace(view.LastError),
		CreatedAt:       strings.TrimSpace(view.CreatedAt),
		UpdatedAt:       strings.TrimSpace(view.UpdatedAt),
	}
}

func normalizeBotConnectionToolApprovalMode(mode string) string {
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

func botConnectionConfigs(views []BotConnectionView) []config.BotConnectionConfig {
	if views == nil {
		return nil
	}
	out := make([]config.BotConnectionConfig, 0, len(views))
	for _, view := range views {
		cfg := botConnectionConfig(view)
		if cfg.ID == "" || cfg.Provider == "" {
			continue
		}
		out = append(out, cfg)
	}
	return out
}

func botMappingScope(scope, workspaceRoot string) string {
	if strings.TrimSpace(scope) == "project" {
		return "project"
	}
	if strings.TrimSpace(workspaceRoot) != "" {
		return "project"
	}
	return "global"
}

func botMappingWorkspaceRoot(scope, workspaceRoot string) string {
	if botMappingScope(scope, workspaceRoot) != "project" {
		return ""
	}
	return strings.TrimSpace(workspaceRoot)
}

func botSessionMappingViews(mappings []config.BotConnectionSessionMapping, connectionWorkspaceRoot string) []BotConnectionSessionMappingView {
	if mappings == nil {
		return []BotConnectionSessionMappingView{}
	}
	out := make([]BotConnectionSessionMappingView, 0, len(mappings))
	for _, m := range mappings {
		workspaceRoot := firstNonEmptyBot(m.WorkspaceRoot, connectionWorkspaceRoot)
		scope := botMappingScope(m.Scope, workspaceRoot)
		out = append(out, BotConnectionSessionMappingView{
			RemoteID:      m.RemoteID,
			SessionID:     m.SessionID,
			SessionSource: m.SessionSource,
			ChatType:      m.ChatType,
			UserID:        m.UserID,
			ThreadID:      m.ThreadID,
			Scope:         scope,
			WorkspaceRoot: botMappingWorkspaceRoot(scope, workspaceRoot),
			UpdatedAt:     m.UpdatedAt,
		})
	}
	return out
}

func botSessionMappingConfigs(mappings []BotConnectionSessionMappingView, connectionWorkspaceRoot string) []config.BotConnectionSessionMapping {
	if mappings == nil {
		return nil
	}
	out := make([]config.BotConnectionSessionMapping, 0, len(mappings))
	for _, m := range mappings {
		workspaceRoot := firstNonEmptyBot(m.WorkspaceRoot, connectionWorkspaceRoot)
		scope := botMappingScope(m.Scope, workspaceRoot)
		out = append(out, config.BotConnectionSessionMapping{
			RemoteID:      strings.TrimSpace(m.RemoteID),
			SessionID:     strings.TrimSpace(m.SessionID),
			SessionSource: strings.TrimSpace(m.SessionSource),
			ChatType:      strings.TrimSpace(m.ChatType),
			UserID:        strings.TrimSpace(m.UserID),
			ThreadID:      strings.TrimSpace(m.ThreadID),
			Scope:         scope,
			WorkspaceRoot: botMappingWorkspaceRoot(scope, workspaceRoot),
			UpdatedAt:     strings.TrimSpace(m.UpdatedAt),
		})
	}
	return out
}

func connectionID(provider, domain string) string {
	return strings.Trim(strings.ToLower(provider+"-"+domain), "-")
}

func botInstallAccess(userID string) config.BotAccessConfig {
	userID = strings.TrimSpace(userID)
	access := config.BotAccessConfig{Enabled: true, PairingEnabled: true}
	if userID != "" {
		access.Users = []string{userID}
	}
	return access
}

func envIsSet(name string) bool {
	return strings.TrimSpace(name) != "" && strings.TrimSpace(os.Getenv(name)) != ""
}

func firstNonEmptyBot(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func appendUniqueBotString(values []string, next string) []string {
	next = strings.TrimSpace(next)
	if next == "" {
		return values
	}
	for _, value := range values {
		if strings.TrimSpace(value) == next {
			return values
		}
	}
	return append(values, next)
}
