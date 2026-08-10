package main

import (
	"encoding/json"
	"strings"
	"testing"

	"patty/internal/config"
)

func TestDiagnoseBotConnectionBuildsReportDetailForMissingSecret(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("CUSTOM_BOT_APP_SECRET_PRIVATE", "")
	app := NewApp()
	if _, err := app.upsertBotConnection(config.BotConnectionConfig{
		ID:            "generic-main",
		Provider:      "generic",
		Domain:        "custom",
		Label:         "Custom",
		Enabled:       true,
		Status:        "connected",
		WorkspaceRoot: "/Users/alice/work/patty",
		Credential: config.BotConnectionCredential{
			AppID:        "cli-private",
			AppSecretEnv: "CUSTOM_BOT_APP_SECRET_PRIVATE",
		},
		SessionMappings: []config.BotConnectionSessionMapping{{
			RemoteID:      "remote-private",
			SessionID:     "session-private",
			Scope:         "project",
			WorkspaceRoot: "/Users/alice/work/patty",
		}},
	}, nil); err != nil {
		t.Fatalf("upsert connection: %v", err)
	}

	diag, err := app.DiagnoseBotConnection("generic-main")
	if err != nil {
		t.Fatalf("DiagnoseBotConnection: %v", err)
	}
	if diag.Status != "warning" || diag.Phase != "credential" || diag.Code != "secret_missing" || diag.ReportKind != "bot" || diag.ReportDetail == "" {
		t.Fatalf("diagnostic = %+v, want warning credential report", diag)
	}
	for _, leaked := range []string{"CUSTOM_BOT_APP_SECRET_PRIVATE", "/Users/alice", "remote-private", "session-private"} {
		if strings.Contains(diag.ReportDetail, leaked) {
			t.Fatalf("diagnostic report leaked %q in %s", leaked, diag.ReportDetail)
		}
	}
	var payload frontendCrashPayload
	if err := json.Unmarshal([]byte(diag.ReportDetail), &payload); err != nil {
		t.Fatalf("report detail is not structured JSON: %v", err)
	}
	if payload.Kind != "bot" || payload.Source != "bot.runtime" || payload.Label != "bot.generic.custom.credential" {
		t.Fatalf("payload = %+v, want bot runtime credential label", payload)
	}
	for _, want := range []string{
		"app_secret_env_configured: true",
		"secret_available: false",
		"workspace_scope: project",
		"session_mappings: 1",
		"summary: required bot credential is not available",
	} {
		if !strings.Contains(payload.Message, want) {
			t.Fatalf("payload message = %q, want it to contain %q", payload.Message, want)
		}
	}
	report, err := crashReportFromDetail(diag.ReportKind, diag.ReportDetail)
	if err != nil {
		t.Fatalf("crashReportFromDetail: %v", err)
	}
	if report.Kind != "bot" || report.Source != "bot.runtime" || report.ErrorType != "BotConnectionDiagnostic" {
		t.Fatalf("report = %+v, want accepted bot report", report)
	}
}

func TestBotConnectionSendFailureReportRedactsEnvNames(t *testing.T) {
	conn := config.BotConnectionConfig{
		ID:       "generic-main",
		Provider: "generic",
		Domain:   "custom",
		Label:    "Custom",
		Enabled:  true,
		Status:   "connected",
		Credential: config.BotConnectionCredential{
			AppSecretEnv: "CUSTOM_BOT_APP_SECRET_PRIVATE",
		},
	}
	diag := botConnectionDiagnostic(&conn, conn.ID, "error", "send", "test_send_failed", "custom app_id or CUSTOM_BOT_APP_SECRET_PRIVATE is not configured", true)
	if diag.ReportKind != "bot" || diag.ReportDetail == "" {
		t.Fatalf("diagnostic = %+v, want reportable bot diagnostic", diag)
	}
	if strings.Contains(diag.ReportDetail, "CUSTOM_BOT_APP_SECRET_PRIVATE") {
		t.Fatalf("diagnostic report leaked env name in %s", diag.ReportDetail)
	}
	var payload frontendCrashPayload
	if err := json.Unmarshal([]byte(diag.ReportDetail), &payload); err != nil {
		t.Fatalf("report detail is not structured JSON: %v", err)
	}
	if !strings.Contains(payload.ErrorMessage, "[redacted-env]") {
		t.Fatalf("payload errorMessage = %q, want redacted env marker", payload.ErrorMessage)
	}
}

func TestDiagnoseBotConnectionWarnsWhenTokenEnvUnset(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("CUSTOM_BOT_TOKEN", "")
	app := NewApp()
	if _, err := app.upsertBotConnection(config.BotConnectionConfig{
		ID:       "generic-main",
		Provider: "generic",
		Domain:   "custom",
		Label:    "Custom",
		Enabled:  true,
		Status:   "connected",
		Credential: config.BotConnectionCredential{
			TokenEnv: "CUSTOM_BOT_TOKEN",
		},
	}, nil); err != nil {
		t.Fatalf("upsert connection: %v", err)
	}

	diag, err := app.DiagnoseBotConnection("generic-main")
	if err != nil {
		t.Fatalf("DiagnoseBotConnection: %v", err)
	}
	if diag.Status != "warning" || diag.Phase != "credential" || diag.Code != "secret_missing" || diag.ReportKind != "bot" || diag.ReportDetail == "" {
		t.Fatalf("diagnostic = %+v, want missing local credential warning", diag)
	}
	if strings.Contains(diag.ReportDetail, "CUSTOM_BOT_TOKEN") {
		t.Fatalf("diagnostic report leaked token env name in %s", diag.ReportDetail)
	}
}

func TestDiagnoseBotConnectionReportsMissingConnection(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	diag, err := app.DiagnoseBotConnection("no-such-connection")
	if err != nil {
		t.Fatalf("DiagnoseBotConnection: %v", err)
	}
	if diag.Status != "missing" || diag.Code != "connection_missing" || diag.ReportKind != "bot" || diag.ReportDetail == "" {
		t.Fatalf("diagnostic = %+v, want missing connection report", diag)
	}
	var payload frontendCrashPayload
	if err := json.Unmarshal([]byte(diag.ReportDetail), &payload); err != nil {
		t.Fatalf("report detail is not structured JSON: %v", err)
	}
	if !strings.Contains(payload.Message, "summary: bot connection record was not found") {
		t.Fatalf("payload message = %q, want missing connection summary", payload.Message)
	}
}

func TestRememberBotConnectionRemoteStoresStableScope(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	if _, err := app.upsertBotConnection(config.BotConnectionConfig{
		ID:       "generic-main",
		Provider: "generic",
		Domain:   "custom",
		Label:    "main",
		Enabled:  true,
		Status:   "connected",
	}, nil); err != nil {
		t.Fatalf("upsert global connection: %v", err)
	}
	if err := app.rememberBotConnectionRemote("generic-main", "remote_global"); err != nil {
		t.Fatalf("remember global remote: %v", err)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if got := cfg.Bot.Connections[0].SessionMappings[0]; got.Scope != "global" || got.WorkspaceRoot != "" || got.RemoteID != "remote_global" {
		t.Fatalf("global mapping = %+v, want scope=global without workspace", got)
	}

	if _, err := app.upsertBotConnection(config.BotConnectionConfig{
		ID:            "generic-project",
		Provider:      "generic",
		Domain:        "project",
		Label:         "project",
		Enabled:       true,
		Status:        "connected",
		WorkspaceRoot: "/tmp/patty-project",
	}, nil); err != nil {
		t.Fatalf("upsert project connection: %v", err)
	}
	if err := app.rememberBotConnectionRemote("generic-project", "remote_project"); err != nil {
		t.Fatalf("remember project remote: %v", err)
	}
	cfg = config.LoadForEdit(config.UserConfigPath())
	var projectMapping config.BotConnectionSessionMapping
	for _, conn := range cfg.Bot.Connections {
		if conn.ID == "generic-project" && len(conn.SessionMappings) == 1 {
			projectMapping = conn.SessionMappings[0]
		}
	}
	if projectMapping.Scope != "project" || projectMapping.WorkspaceRoot != "/tmp/patty-project" || projectMapping.RemoteID != "remote_project" {
		t.Fatalf("project mapping = %+v, want project scope and workspace", projectMapping)
	}
}

func TestBotConnectionViewRoundTripPreservesGenericConnection(t *testing.T) {
	conn := config.BotConnectionConfig{
		ID: "generic-main", Provider: "generic", Domain: "custom", Label: "Custom",
		Enabled: true, Status: "connected", Model: "gpt-4.1",
		ToolApprovalMode: "bypass",
		WorkspaceRoot:    "/work/patty",
		Credential: config.BotConnectionCredential{
			AppID: "cli-1", AppSecretEnv: "CUSTOM_BOT_APP_SECRET",
		},
		SessionMappings: []config.BotConnectionSessionMapping{{
			RemoteID: "remote-1", SessionID: "sess-1", SessionSource: "auto", Scope: "project", WorkspaceRoot: "/work/patty",
		}},
	}
	view := botConnectionView(conn)
	if view.ToolApprovalMode != "yolo" {
		t.Fatalf("tool approval mode = %q, want normalized yolo", view.ToolApprovalMode)
	}
	back := botConnectionConfig(view)
	if back.ID != conn.ID || back.Provider != conn.Provider || back.Domain != conn.Domain || back.Label != conn.Label ||
		back.ToolApprovalMode != "yolo" || back.WorkspaceRoot != conn.WorkspaceRoot || back.Credential.AppID != "cli-1" ||
		back.Credential.AppSecretEnv != "CUSTOM_BOT_APP_SECRET" {
		t.Fatalf("round trip = %+v, want preserved connection", back)
	}
	if len(back.SessionMappings) != 1 || back.SessionMappings[0].Scope != "project" || back.SessionMappings[0].WorkspaceRoot != "/work/patty" {
		t.Fatalf("round trip mappings = %+v", back.SessionMappings)
	}
}

func TestBotConnectionConfigsSkipsIncompleteViews(t *testing.T) {
	views := []BotConnectionView{
		{ID: "generic-main", Provider: "generic", Domain: "custom", Label: "Custom"},
		{ID: "", Provider: "generic"},
		{ID: "no-provider", Provider: ""},
	}
	got := botConnectionConfigs(views)
	if len(got) != 1 || got[0].ID != "generic-main" {
		t.Fatalf("configs = %+v, want only the complete view", got)
	}
}

func TestUpsertBotConnectionPreservesExistingAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	if _, err := app.upsertBotConnection(config.BotConnectionConfig{
		ID: "generic-main", Provider: "generic", Domain: "custom", Label: "Custom", Enabled: true, Status: "connected",
		Access: config.BotAccessConfig{Enabled: true, PairingEnabled: true, Users: []string{"user-1"}},
	}, nil); err != nil {
		t.Fatalf("upsert with access: %v", err)
	}
	if _, err := app.upsertBotConnection(config.BotConnectionConfig{
		ID: "generic-main", Provider: "generic", Domain: "custom", Label: "Custom", Enabled: true, Status: "connected",
	}, nil); err != nil {
		t.Fatalf("upsert without access: %v", err)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	got := cfg.Bot.Connections[0].Access
	if !got.Enabled || !got.PairingEnabled || len(got.Users) != 1 || got.Users[0] != "user-1" {
		t.Fatalf("access = %+v, want preserved existing access", got)
	}
}
