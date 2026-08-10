package main

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/bot"
	"patty/internal/botruntime"
	"patty/internal/config"
)

func TestDesktopBotRuntimePlanStartsSavedConnections(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Enabled = true
	cfg.Bot.Allowlist.Enabled = true
	cfg.Bot.Allowlist.Users = []string{"user-installer"}
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "generic-main", Provider: "generic", Domain: "custom", Enabled: true},
		{ID: "generic-secondary", Provider: "generic", Domain: "secondary", Enabled: true},
		{ID: "custom-main", Provider: "custom", Domain: "custom", Enabled: true},
	}

	plan := desktopBotRuntimePlan(cfg)
	if !plan.Start {
		t.Fatalf("plan = %+v, want start", plan)
	}
	if !plan.Enabled[bot.Platform("generic")] || !plan.Enabled[bot.Platform("custom")] {
		t.Fatalf("enabled = %+v, want generic and custom platforms", plan.Enabled)
	}
}

func TestDesktopBotRuntimePlanBlocksWithoutAllowlist(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Enabled = true
	cfg.Bot.Allowlist.Enabled = true
	cfg.Bot.Pairing.Enabled = false
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "generic-main", Provider: "generic", Domain: "custom", Enabled: true},
	}

	plan := desktopBotRuntimePlan(cfg)
	if plan.Start || plan.Status != "blocked" {
		t.Fatalf("plan = %+v, want blocked without allowlist", plan)
	}
}

func TestDesktopBotRuntimePlanStartsWithPairing(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Enabled = true
	cfg.Bot.Allowlist.Enabled = true
	cfg.Bot.Pairing.Enabled = true
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "generic-main", Provider: "generic", Domain: "custom", Enabled: true},
	}

	plan := desktopBotRuntimePlan(cfg)
	if !plan.Start {
		t.Fatalf("plan = %+v, want start with pairing enabled", plan)
	}
}

func TestDesktopBotRuntimePlanStopsWhenBotDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Enabled = false
	cfg.Bot.Allowlist.Users = []string{"user-installer"}
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "generic-main", Provider: "generic", Domain: "custom", Enabled: true},
	}

	plan := desktopBotRuntimePlan(cfg)
	if plan.Start || plan.Status != "stopped" {
		t.Fatalf("plan = %+v, want stopped when disabled", plan)
	}
}

func TestDesktopBotRuntimeForwardTargetsDeduplicatesMappedChats(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{{
		ID:       "generic-main",
		Provider: "generic",
		Domain:   "custom",
		Enabled:  true,
		SessionMappings: []config.BotConnectionSessionMapping{
			{RemoteID: "group-1", ChatType: string(bot.ChatGroup), UserID: "user-1"},
			{RemoteID: "group-1", ChatType: string(bot.ChatGroup), UserID: "user-2"},
			{RemoteID: "dm-1", ChatType: string(bot.ChatDM), UserID: "user-1"},
		},
	}}

	targets := newDesktopBotRuntime().ForwardTargets(cfg)
	if len(targets) != 2 {
		t.Fatalf("targets = %+v, want one group target plus one dm target", targets)
	}
	seen := map[string]bool{}
	for _, target := range targets {
		key := target.ConnID + "|" + target.Domain + "|" + target.ChatID + "|" + string(target.ChatType)
		if seen[key] {
			t.Fatalf("duplicate target %q in %+v", key, targets)
		}
		seen[key] = true
	}
	if !seen["generic-main|custom|group-1|group"] || !seen["generic-main|custom|dm-1|dm"] {
		t.Fatalf("targets = %+v, want group and dm targets", targets)
	}
}

func TestDesktopBotRuntimeConfigUsesUserBotSettings(t *testing.T) {
	isolateDesktopUserDirs(t)

	userCfg := config.LoadForEdit(config.UserConfigPath())
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.Users = []string{"user-installer"}
	userCfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "generic-main", Provider: "generic", Domain: "custom", Enabled: true, Status: "connected"},
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "patty.toml"), []byte(`
[bot]
enabled = false
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	plan := desktopBotRuntimePlan(got)
	if !plan.Start || !plan.Enabled[bot.Platform("generic")] {
		t.Fatalf("desktop runtime plan = %+v, want user-level generic connection to start", plan)
	}
}

func TestDesktopBotRuntimeConfigLoadsAllSavedCredentialsAfterRestart(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Cleanup(func() {
		_ = os.Unsetenv("GENERIC_BOT_APP_SECRET")
		_ = os.Unsetenv("CUSTOM_BOT_APP_SECRET")
	})

	userCfg := config.Default()
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.Users = []string{"user-installer"}
	userCfg.Bot.Connections = []config.BotConnectionConfig{
		{
			ID:       "generic-main",
			Provider: "generic",
			Domain:   "custom",
			Enabled:  true,
			Status:   "connected",
			Credential: config.BotConnectionCredential{
				AppID:        "cli-generic",
				AppSecretEnv: "GENERIC_BOT_APP_SECRET",
			},
		},
		{
			ID:       "custom-secondary",
			Provider: "custom",
			Domain:   "secondary",
			Enabled:  true,
			Status:   "connected",
			Credential: config.BotConnectionCredential{
				AppID:        "cli-custom",
				AppSecretEnv: "CUSTOM_BOT_APP_SECRET",
			},
		},
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(config.UserCredentialsPath()), 0o755); err != nil {
		t.Fatalf("create credentials dir: %v", err)
	}
	if err := os.WriteFile(config.UserCredentialsPath(), []byte("GENERIC_BOT_APP_SECRET=generic-secret\nCUSTOM_BOT_APP_SECRET=custom-secret\n"), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	_ = os.Unsetenv("GENERIC_BOT_APP_SECRET")
	_ = os.Unsetenv("CUSTOM_BOT_APP_SECRET")

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	views := botConnectionViews(got.Bot.Connections)
	if len(views) != 2 {
		t.Fatalf("connection views = %+v, want generic and custom connections", views)
	}
	for _, view := range views {
		if !view.Credential.SecretSet {
			t.Fatalf("connection %s credential = %+v, want saved credential loaded after restart", view.ID, view.Credential)
		}
	}
	plan := desktopBotRuntimePlan(got)
	if !plan.Start || !plan.Enabled[bot.Platform("generic")] || !plan.Enabled[bot.Platform("custom")] {
		t.Fatalf("desktop runtime plan = %+v, want saved generic/custom connections to start", plan)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bindings := botruntime.AdapterBindings(got, plan.Enabled, logger)
	if len(bindings) != 0 {
		t.Fatalf("adapter bindings = %+v, want none (desktop ships no built-in IM adapters)", bindings)
	}
}

func TestDesktopBotRuntimeMigratesLegacyProjectBotSettings(t *testing.T) {
	isolateDesktopUserDirs(t)

	userCfg := config.Default()
	if err := userCfg.SetDesktopAppearance("dark", "graphite"); err != nil {
		t.Fatalf("set desktop appearance: %v", err)
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "patty.toml"), []byte(`
[bot]
enabled = true

[bot.allowlist]
enabled = true
users = ["user-legacy"]

[[bot.connections]]
id = "generic-main"
provider = "generic"
domain = "custom"
label = "Custom"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	app := NewApp()
	got, err := app.loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	if !got.Bot.Enabled || len(got.Bot.Connections) != 1 || got.Bot.Connections[0].ID != "generic-main" {
		t.Fatalf("desktop bot config = %+v, want migrated generic connection", got.Bot)
	}

	// The bot-runtime load is a pure read: the merge above stays in memory and
	// the user config file is not rewritten.
	preWrite := config.LoadForEdit(config.UserConfigPath())
	if preWrite.Bot.Enabled || len(preWrite.Bot.Connections) != 0 {
		t.Fatalf("read path persisted bot config = %+v, want disk untouched until a locked write", preWrite.Bot)
	}

	// The first locked write path performs the on-disk migration.
	if err := app.applyConfigOnly(func(*config.Config) error { return nil }); err != nil {
		t.Fatalf("applyConfigOnly: %v", err)
	}
	persisted := config.LoadForEdit(config.UserConfigPath())
	if !persisted.Bot.Enabled || len(persisted.Bot.Connections) != 1 || persisted.Bot.Connections[0].ID != "generic-main" {
		t.Fatalf("persisted bot config = %+v, want migrated generic connection", persisted.Bot)
	}
	if persisted.DesktopTheme() != "dark" {
		t.Fatalf("desktop theme = %q, want preserved user preference", persisted.DesktopTheme())
	}
}

func TestDesktopBotRuntimePersistsLegacyProjectBotWhenUserConfigMissing(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "patty.toml"), []byte(`
[desktop]
theme = "dark"

[bot]
enabled = true

[bot.allowlist]
enabled = true
users = ["user-legacy"]

[[bot.connections]]
id = "generic-main"
provider = "generic"
domain = "custom"
label = "Custom"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	app := NewApp()
	got, err := app.loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	if !got.Bot.Enabled || len(got.Bot.Connections) != 1 || got.Bot.Connections[0].ID != "generic-main" {
		t.Fatalf("desktop bot config = %+v, want migrated generic connection", got.Bot)
	}

	// The bot-runtime load is a pure read: it serves the legacy config from
	// memory and must not create the user config file.
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("read path must not create the user config, stat err = %v", err)
	}

	// The first locked write path creates the user config with the migrated
	// bot settings (adopting the legacy config, ConfigVersion-bumped).
	if err := app.applyConfigOnly(func(*config.Config) error { return nil }); err != nil {
		t.Fatalf("applyConfigOnly: %v", err)
	}
	persisted := config.LoadForEdit(config.UserConfigPath())
	if !persisted.Bot.Enabled || len(persisted.Bot.Connections) != 1 || persisted.Bot.Connections[0].ID != "generic-main" {
		t.Fatalf("persisted bot config = %+v, want migrated generic connection", persisted.Bot)
	}
}

func TestDesktopSettingsBotMigrationPersistsOnlyBotBeforeFirstEdit(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "patty.toml"), []byte(`
[desktop]
theme = "dark"
close_behavior = "quit"

[bot]
enabled = true

[bot.allowlist]
enabled = true
users = ["user-legacy"]

[[bot.connections]]
id = "generic-main"
provider = "generic"
domain = "custom"
label = "Custom"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	settings := NewApp().Settings()
	if !settings.Bot.Enabled || len(settings.Bot.Connections) != 1 || settings.Bot.Connections[0].ID != "generic-main" {
		t.Fatalf("settings bot = %+v, want migrated generic connection", settings.Bot)
	}
	if settings.DesktopTheme != "dark" || settings.CloseBehavior != "quit" {
		t.Fatalf("settings desktop prefs = theme:%q close:%q, want legacy seed visible before first edit", settings.DesktopTheme, settings.CloseBehavior)
	}

	persisted := config.LoadForEdit(config.UserConfigPath())
	if persisted.DesktopTheme() == "dark" || persisted.DesktopCloseBehavior() == "quit" {
		t.Fatalf("persisted desktop prefs = theme:%q close:%q, want bot-only migration", persisted.DesktopTheme(), persisted.DesktopCloseBehavior())
	}
}

func TestDesktopBotRuntimeMigrationDoesNotOverwriteUserBotSettings(t *testing.T) {
	isolateDesktopUserDirs(t)

	userCfg := config.Default()
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.Users = []string{"user-main"}
	userCfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "custom-main", Provider: "custom", Domain: "custom", Enabled: true, Status: "connected"},
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "patty.toml"), []byte(`
[bot]
enabled = true

[bot.allowlist]
enabled = true
users = ["user-legacy"]

[[bot.connections]]
id = "generic-main"
provider = "generic"
domain = "custom"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	if len(got.Bot.Connections) != 1 || got.Bot.Connections[0].ID != "custom-main" {
		t.Fatalf("desktop bot config = %+v, want existing user custom connection", got.Bot)
	}
}

func TestSummarizeBotRuntimeErrorsCapsOutput(t *testing.T) {
	got := summarizeBotRuntimeErrors([]error{
		errors.New("first"),
		nil,
		errors.New("second"),
		errors.New("third"),
		errors.New("fourth"),
	})

	for _, want := range []string{"first", "second", "third", "1 more"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "fourth") {
		t.Fatalf("summary = %q, should cap extra errors", got)
	}
}
