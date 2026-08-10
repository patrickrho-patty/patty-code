package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"patty/internal/bot"
	"patty/internal/botruntime"
	"patty/internal/config"
)

func TestRememberBotRemoteStoresIncomingChatID(t *testing.T) {
	isolateBotUserConfig(t)
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-a", Provider: "custom", Domain: "alpha", Label: "알파", Enabled: true, Status: "connected"},
		{ID: "custom-b", Provider: "custom", Domain: "beta", Label: "베타", Enabled: true, Status: "connected"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	msg := bot.InboundMessage{
		Platform:     bot.Platform("custom"),
		ConnectionID: "custom-b",
		ChatType:     bot.ChatDM,
		ChatID:       "chat-1",
		UserID:       "user-1",
	}
	if err := botruntime.RememberInbound(msg); err != nil {
		t.Fatalf("rememberBotInbound: %v", err)
	}
	if err := botruntime.RememberInbound(msg); err != nil {
		t.Fatalf("rememberBotRemote duplicate: %v", err)
	}

	got := config.LoadForEdit(config.UserConfigPath())
	if len(got.Bot.Connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(got.Bot.Connections))
	}
	var wx config.BotConnectionConfig
	var fs config.BotConnectionConfig
	for _, conn := range got.Bot.Connections {
		switch conn.ID {
		case "custom-b":
			wx = conn
		case "custom-a":
			fs = conn
		}
	}
	if len(fs.SessionMappings) != 0 {
		t.Fatalf("custom-a mappings = %+v, want none", fs.SessionMappings)
	}
	if len(wx.SessionMappings) != 1 {
		t.Fatalf("custom-b mappings = %+v, want one", wx.SessionMappings)
	}
	if m := wx.SessionMappings[0]; m.RemoteID != "chat-1" || m.Scope != "global" || m.WorkspaceRoot != "" || m.UpdatedAt == "" {
		t.Fatalf("custom-b mapping = %+v, want global chat-1 with timestamp", m)
	}
	if got := got.Bot.Allowlist.Users; len(got) != 1 || got[0] != "user-1" {
		t.Fatalf("users = %+v, want user-1", got)
	}
}

func TestRememberBotRemoteKeepsProjectScopedConnection(t *testing.T) {
	isolateBotUserConfig(t)
	workspace := filepath.Join(t.TempDir(), "project")
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{{
		ID:            "custom-project",
		Provider:      "custom",
		Domain:        "alpha",
		Label:         "알파",
		Enabled:       true,
		Status:        "connected",
		WorkspaceRoot: workspace,
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := botruntime.RememberInbound(bot.InboundMessage{
		Platform: bot.Platform("custom"),
		ChatType: bot.ChatDM,
		ChatID:   "chat-1",
		UserID:   "user-1",
	}); err != nil {
		t.Fatalf("rememberBotInbound: %v", err)
	}

	got := config.LoadForEdit(config.UserConfigPath())
	if len(got.Bot.Connections) != 1 || len(got.Bot.Connections[0].SessionMappings) != 1 {
		t.Fatalf("connections = %+v, want one project mapping", got.Bot.Connections)
	}
	if m := got.Bot.Connections[0].SessionMappings[0]; m.RemoteID != "chat-1" || m.Scope != "project" || m.WorkspaceRoot != workspace {
		t.Fatalf("mapping = %+v, want project scoped remote", m)
	}
	if got := got.Bot.Allowlist.Users; len(got) != 1 || got[0] != "user-1" {
		t.Fatalf("users = %+v, want user-1", got)
	}
}

func TestRememberBotInboundStoresGroupAllowlist(t *testing.T) {
	isolateBotUserConfig(t)
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-a", Provider: "custom", Domain: "alpha", Label: "알파", Enabled: true, Status: "connected"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	msg := bot.InboundMessage{
		Platform: bot.Platform("custom"),
		ChatType: bot.ChatGroup,
		ChatID:   "group-1",
		UserID:   "user-1",
	}
	if err := botruntime.RememberInbound(msg); err != nil {
		t.Fatalf("rememberBotInbound: %v", err)
	}
	if err := botruntime.RememberInbound(msg); err != nil {
		t.Fatalf("rememberBotInbound duplicate: %v", err)
	}

	got := config.LoadForEdit(config.UserConfigPath())
	if users := got.Bot.Allowlist.Users; len(users) != 1 || users[0] != "user-1" {
		t.Fatalf("users = %+v, want one user-1", users)
	}
	if groups := got.Bot.Allowlist.Groups; len(groups) != 1 || groups[0] != "group-1" {
		t.Fatalf("groups = %+v, want one group-1", groups)
	}
}

func TestBotDoctorReportsSessionMappingCounts(t *testing.T) {
	isolateBotUserConfig(t)
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-a", Provider: "custom", Domain: "alpha", Label: "알파", Enabled: true, Status: "connected"},
		{ID: "custom-b", Provider: "custom", Domain: "beta", Label: "베타", Enabled: true, Status: "connected"},
	}
	cfg.Bot.Connections[0].SessionMappings = []config.BotConnectionSessionMapping{{RemoteID: "oc-chat-1", Scope: "global"}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := botDoctor([]string{"--json"}); rc != 0 {
			t.Fatalf("botDoctor rc = %d, want 0", rc)
		}
	})
	for _, want := range []string{
		`"name":"bot.connections","status":"ok","detail":"enabled=2 total=2"`,
		`"name":"bot.connection.custom-a.session_mappings","status":"ok","detail":"provider=custom mappings=1"`,
		`"name":"bot.connection.custom-b.session_mappings","status":"ok","detail":"provider=custom mappings=0"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("bot doctor output missing %s:\n%s", want, out)
		}
	}
}

func TestBotDoctorDeepReportsPairingAndRoles(t *testing.T) {
	isolateBotUserConfig(t)
	cfg := config.Default()
	cfg.Bot.Enabled = true
	cfg.Bot.Pairing.Enabled = true
	cfg.Bot.Allowlist.Enabled = true
	cfg.Bot.Allowlist.Users = []string{"ou-user"}
	cfg.Bot.Allowlist.Approvers = []string{"ou-approver"}
	cfg.Bot.Allowlist.Admins = []string{"ou-admin"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if _, _, err := bot.CreateOrRefreshPairingRequest(bot.InboundMessage{
		Platform:     bot.Platform("custom"),
		ConnectionID: "custom-a",
		ChatType:     bot.ChatDM,
		ChatID:       "chat",
		UserID:       "pending-user",
	}, bot.PairingConfig{Enabled: true}); err != nil {
		t.Fatalf("create pairing: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := botDoctor([]string{"--json", "--deep"}); rc != 0 {
			t.Fatalf("botDoctor rc = %d, want 0", rc)
		}
	})
	for _, want := range []string{
		`"name":"bot.pairing.pending","status":"ok","detail":"1 pending"`,
		`"name":"bot.roles","status":"ok","detail":"approvers=1 admins=1"`,
		`"name":"bot.config.user","status":"ok"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("bot doctor deep output missing %s:\n%s", want, out)
		}
	}
}

func TestBotPairingApproveAddsAllowlistAndFirstAdmin(t *testing.T) {
	isolateBotUserConfig(t)
	cfg := config.Default()
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	req, _, err := bot.CreateOrRefreshPairingRequest(bot.InboundMessage{
		Platform: bot.Platform("custom"),
		ChatType: bot.ChatDM,
		ChatID:   "chat",
		UserID:   "user",
	}, bot.PairingConfig{Enabled: true})
	if err != nil {
		t.Fatalf("create pairing: %v", err)
	}

	if rc := botPairing([]string{"approve", req.Code}); rc != 0 {
		t.Fatalf("botPairing approve rc = %d, want 0", rc)
	}
	got := config.LoadForEdit(config.UserConfigPath())
	if users := got.Bot.Allowlist.Users; len(users) != 1 || users[0] != "user" {
		t.Fatalf("users = %+v, want user", users)
	}
	if admins := got.Bot.Allowlist.Admins; len(admins) != 1 || admins[0] != "user" {
		t.Fatalf("admins = %+v, want first paired admin", admins)
	}
	if approvers := got.Bot.Allowlist.Approvers; len(approvers) != 1 || approvers[0] != "user" {
		t.Fatalf("approvers = %+v, want first paired approver", approvers)
	}
}

func TestBotPairingApproveAddsUserToConnectionAccess(t *testing.T) {
	isolateBotUserConfig(t)
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{{
		ID:       "custom-a",
		Provider: "custom",
		Domain:   "alpha",
		Label:    "Alpha",
		Enabled:  true,
		Status:   "connected",
		Access: config.BotAccessConfig{
			Enabled:        true,
			PairingEnabled: true,
			Users:          []string{"ou-existing"},
		},
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	req, _, err := bot.CreateOrRefreshPairingRequest(bot.InboundMessage{
		Platform:     bot.Platform("custom"),
		ConnectionID: "custom-a",
		Domain:       "alpha",
		ChatType:     bot.ChatDM,
		ChatID:       "oc-chat",
		UserID:       "ou-new",
	}, bot.PairingConfig{Enabled: true})
	if err != nil {
		t.Fatalf("create pairing: %v", err)
	}

	if rc := botPairing([]string{"approve", req.Code}); rc != 0 {
		t.Fatalf("botPairing approve rc = %d, want 0", rc)
	}
	got := config.LoadForEdit(config.UserConfigPath())
	if users := got.Bot.Allowlist.Users; len(users) != 0 {
		t.Fatalf("global users = %+v, want unchanged global allowlist", users)
	}
	if len(got.Bot.Connections) != 1 {
		t.Fatalf("connections = %+v, want one connection", got.Bot.Connections)
	}
	access := got.Bot.Connections[0].Access
	if !access.Enabled {
		t.Fatal("connection access disabled after approval, want enabled")
	}
	for _, want := range []string{"ou-existing", "ou-new"} {
		if !hasTestString(access.Users, want) {
			t.Fatalf("connection users = %+v, want %s", access.Users, want)
		}
	}
}

func TestBotDoctorPrefersUserBotSettingsOverProjectBotConfig(t *testing.T) {
	isolateBotUserConfig(t)
	userCfg := config.Default()
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.Users = []string{"ou-user"}
	userCfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-a", Provider: "custom", Domain: "alpha", Label: "Alpha", Enabled: true, Status: "connected"},
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "patty.toml"), []byte(`
[bot]
enabled = false
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	t.Chdir(project)

	out := captureStdout(t, func() {
		if rc := botDoctor([]string{"--json"}); rc != 0 {
			t.Fatalf("botDoctor rc = %d, want 0", rc)
		}
	})
	for _, want := range []string{
		`"name":"bot.enabled","status":"ok"`,
		`"name":"bot.connections","status":"ok","detail":"enabled=1 total=1"`,
		`"name":"bot.connection.custom-a.session_mappings","status":"ok","detail":"provider=custom mappings=0"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("bot doctor output missing %s:\n%s", want, out)
		}
	}
}

func TestBotDoctorUsesProjectBotConfigWhenUserBotIsUnconfigured(t *testing.T) {
	isolateBotUserConfig(t)
	projectCfg := config.Default()
	projectCfg.Bot.Enabled = true
	projectCfg.Bot.Allowlist.AllowAll = true
	projectCfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-b", Provider: "custom", Domain: "beta", Label: "베타", Enabled: true, Status: "connected"},
	}
	if err := projectCfg.SaveTo("patty.toml"); err != nil {
		t.Fatalf("save project config: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := botDoctor([]string{"--json"}); rc != 0 {
			t.Fatalf("botDoctor rc = %d, want 0", rc)
		}
	})
	for _, want := range []string{
		`"name":"bot.enabled","status":"ok"`,
		`"name":"bot.connections","status":"ok","detail":"enabled=1 total=1"`,
		`"name":"bot.allowlist","status":"open"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("bot doctor output missing %s:\n%s", want, out)
		}
	}
}

func TestBotDoctorUsesProjectBotConfigWhenUserConfigOnlyHasBotDefaults(t *testing.T) {
	isolateBotUserConfig(t)
	userCfg := config.Default()
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}
	projectCfg := config.Default()
	projectCfg.Bot.Enabled = true
	projectCfg.Bot.Allowlist.AllowAll = true
	projectCfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-a", Provider: "custom", Domain: "alpha", Label: "Alpha", Enabled: true, Status: "connected"},
	}
	if err := projectCfg.SaveTo("patty.toml"); err != nil {
		t.Fatalf("save project config: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := botDoctor([]string{"--json"}); rc != 0 {
			t.Fatalf("botDoctor rc = %d, want 0", rc)
		}
	})
	for _, want := range []string{
		`"name":"bot.enabled","status":"ok"`,
		`"name":"bot.connections","status":"ok","detail":"enabled=1 total=1"`,
		`"name":"bot.allowlist","status":"open"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("bot doctor output missing %s:\n%s", want, out)
		}
	}
}

func TestBotConnectionChannelConfigsKeepProvidersSeparate(t *testing.T) {
	connections := []config.BotConnectionConfig{
		{ID: "custom-a", Provider: "custom", Domain: "alpha", Enabled: true, Model: "alpha-model", WorkspaceRoot: "/alpha"},
		{ID: "custom-b", Provider: "custom", Domain: "beta", Enabled: true, Model: "beta-model", WorkspaceRoot: "/beta"},
	}
	channels := botruntime.ConnectionChannelConfigs(connections, true, true)
	if channels["custom-a"].Model != "alpha-model" || channels["custom-a"].WorkspaceRoot != "/alpha" {
		t.Fatalf("custom-a channel = %+v, want alpha override", channels["custom-a"])
	}
	if channels["custom-b"].Model != "beta-model" || channels["custom-b"].WorkspaceRoot != "/beta" {
		t.Fatalf("custom-b channel = %+v, want beta override", channels["custom-b"])
	}
}

func TestRememberBotInboundUsesConnectionID(t *testing.T) {
	isolateBotUserConfig(t)
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-a", Provider: "custom", Domain: "alpha", Label: "알파", Enabled: true, Status: "connected"},
		{ID: "custom-b", Provider: "custom", Domain: "beta", Label: "베타", Enabled: true, Status: "connected"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := botruntime.RememberInbound(bot.InboundMessage{
		Platform:     bot.Platform("custom"),
		ConnectionID: "custom-b",
		Domain:       "beta",
		ChatType:     bot.ChatDM,
		ChatID:       "beta-chat",
		UserID:       "beta-user",
	}); err != nil {
		t.Fatalf("rememberBotInbound: %v", err)
	}

	got := config.LoadForEdit(config.UserConfigPath())
	var alphaConn, betaConn config.BotConnectionConfig
	for _, conn := range got.Bot.Connections {
		switch conn.ID {
		case "custom-a":
			alphaConn = conn
		case "custom-b":
			betaConn = conn
		}
	}
	if len(alphaConn.SessionMappings) != 0 {
		t.Fatalf("custom-a mappings = %+v, want none", alphaConn.SessionMappings)
	}
	if len(betaConn.SessionMappings) != 1 || betaConn.SessionMappings[0].RemoteID != "beta-chat" {
		t.Fatalf("custom-b mappings = %+v, want beta chat only", betaConn.SessionMappings)
	}
}

func isolateBotUserConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Chdir(t.TempDir())
}

func hasTestString(values []string, want string) bool {
	return slices.Contains(values, want)
}
