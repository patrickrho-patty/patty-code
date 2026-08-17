//go:build profile_public

package main

import (
	"os"
	"path/filepath"
	"testing"

	"patty/internal/config"
)

// Official-provider injection UX tests (ADR G4): provider_access=["deepseek"]
// materializes the official DeepSeek provider only in the public profile;
// the enterprise/sovereign twins pin that nothing materializes there
// (internal/config/provider_official_endpoint_profile_test.go).

func TestSettingsRepairsLegacyOfficialProviderWithoutModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek-flash"

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
api_key_env = "DEEPSEEK_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := NewApp().Settings()
	for _, p := range got.Providers {
		if p.Name != "deepseek" {
			continue
		}
		if !p.BuiltIn {
			t.Fatalf("deepseek provider should be marked built-in for official endpoint: %+v", p)
		}
		if !p.Added || !p.KeySet || len(p.Models) != 2 || p.Models[0] != "deepseek-v4-flash" || p.Models[1] != "deepseek-v4-pro" || p.Default != "deepseek-v4-flash" {
			t.Fatalf("deepseek provider = %+v, want added repaired official model list", p)
		}
		if got.DefaultModel != "deepseek/deepseek-v4-flash" {
			t.Fatalf("default_model = %q, want deepseek/deepseek-v4-flash", got.DefaultModel)
		}
		return
	}
	t.Fatalf("settings providers missing deepseek: %+v", got.Providers)
}

func TestSettingsInfersLegacyProviderAccessWhenMissing(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek-flash/deepseek-v4-pro"

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
models = ["deepseek-v4-flash", "deepseek-v4-pro"]
default = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[[providers]]
name = "mimo-pro"
kind = "openai"
base_url = "https://token-plan-cn.xiaomimimo.com/v1"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := NewApp().Settings()
	providers := map[string]ProviderView{}
	for _, p := range got.Providers {
		providers[p.Name] = p
	}
	if !providers["deepseek"].Added || !providers["deepseek"].KeySet {
		t.Fatalf("deepseek provider = %+v, want inferred added key-set provider", providers["deepseek"])
	}
	if !providers["mimo-pro"].Added || !providers["mimo-pro"].KeySet || providers["mimo-pro"].BuiltIn {
		t.Fatalf("mimo-pro provider = %+v, want inferred custom key-set provider", providers["mimo-pro"])
	}
	if got.DefaultModel != "deepseek/deepseek-v4-pro" {
		t.Fatalf("default_model = %q, want deepseek/deepseek-v4-pro", got.DefaultModel)
	}
}

func TestRemoveBuiltInProviderAccessRetargetsDefaultToRemainingAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek-flash/deepseek-v4-pro"

[desktop]
provider_access = ["deepseek-flash", "mimo-pro"]

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
models = ["deepseek-v4-flash", "deepseek-v4-pro"]
default = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[[providers]]
name = "mimo-pro"
kind = "openai"
base_url = "https://token-plan-cn.xiaomimimo.com/v1"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := NewApp().RemoveProviderAccess("deepseek"); err != nil {
		t.Fatalf("RemoveProviderAccess: %v", err)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	access := providerAccessSet(cfg.Desktop.ProviderAccess)
	if access["deepseek"] || !access["mimo-pro"] {
		t.Fatalf("provider_access = %+v, want only mimo-pro", cfg.Desktop.ProviderAccess)
	}
	if cfg.DefaultModel != "mimo-pro/mimo-v2.5-pro" {
		t.Fatalf("default_model = %q, want mimo-pro/mimo-v2.5-pro", cfg.DefaultModel)
	}
}

func TestModelsForTabKeepsUserProvidersWithProjectConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")

	userCfg := config.Default()
	userCfg.DefaultModel = "mimo-pro/mimo-v2.5-pro"
	userCfg.Desktop.ProviderAccess = []string{"deepseek-flash", "mimo-pro"}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	projectRoot := t.TempDir()
	projectConfig := `default_model = "deepseek-flash/deepseek-v4-flash"

[desktop]
provider_access = ["deepseek-flash"]

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
`
	if err := os.WriteFile(filepath.Join(projectRoot, "patty.toml"), []byte(projectConfig), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	app := NewApp()
	tab := &WorkspaceTab{ID: "project", WorkspaceRoot: projectRoot, Ready: true}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.activeTabID = tab.ID

	models := app.ModelsForTab(tab.ID)
	refs := modelRefsFromView(models)
	for _, want := range []string{
		"deepseek/deepseek-v4-flash",
		"mimo-pro/mimo-v2.5-pro",
	} {
		if !refs[want] {
			t.Fatalf("ModelsForTab refs = %+v, missing %s", models, want)
		}
	}
}
