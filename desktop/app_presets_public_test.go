//go:build profile_public

// Curated-preset desktop tests (ADR G4): the BYOK preset catalog and its
// one-click install flow are public-profile capabilities; these assertions
// compile only there. Enterprise/sovereign legs exercise the preset-free
// settings surface via the remainder of app_test.go.

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"patty/internal/config"
)

func TestModelsForTabListsMimoAPIPaidAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")

	cfg := config.Default()
	preset, ok := config.CuratedProviderPreset("mimo-api")
	if !ok || len(preset.Entries) == 0 {
		t.Fatal("mimo-api preset missing")
	}
	if err := cfg.UpsertProvider(preset.Entries[0]); err != nil {
		t.Fatalf("upsert mimo-api preset: %v", err)
	}
	cfg.DefaultModel = "mimo-api/mimo-v2.5-pro"
	cfg.Desktop.ProviderAccess = []string{"mimo-api"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	models := NewApp().Models()
	refs := modelRefsFromView(models)
	for _, want := range []string{
		"mimo-api/mimo-v2.5-pro",
		"mimo-api/mimo-v2.5",
	} {
		if !refs[want] {
			t.Fatalf("Models() refs = %+v, missing %s", models, want)
		}
	}
	if len(models) != 2 {
		t.Fatalf("Models() len = %d, want 2: %+v", len(models), models)
	}
}


func TestSettingsSurfacesCuratedProviderPresets(t *testing.T) {
	isolateDesktopUserDirs(t)

	view := NewApp().Settings()
	if len(view.ProviderPresets) < 18 {
		t.Fatalf("Settings().ProviderPresets length = %d, want curated custom presets", len(view.ProviderPresets))
	}
	got := map[string]ProviderPresetView{}
	for _, preset := range view.ProviderPresets {
		got[preset.ID] = preset
	}
	for _, curated := range config.CuratedProviderPresets() {
		id := curated.ID
		preset, ok := got[id]
		if !ok {
			t.Fatalf("Settings().ProviderPresets missing %q: %+v", id, view.ProviderPresets)
		}
		if preset.KeyEnv == "" || len(preset.ProviderNames) == 0 || len(preset.Models) == 0 {
			t.Fatalf("preset %q view has missing fields: %+v", id, preset)
		}
	}
}

func providerPresetViewByID(t *testing.T, view SettingsView, id string) ProviderPresetView {
	t.Helper()
	for _, preset := range view.ProviderPresets {
		if preset.ID == id {
			return preset
		}
	}
	t.Fatalf("Settings().ProviderPresets missing %q: %+v", id, view.ProviderPresets)
	return ProviderPresetView{}
}

func TestSettingsMarksPresetAddedWhenSameNameProviderExistsWithoutAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
[desktop]
provider_access = []

[[providers]]
name = "mimo-api"
kind = "openai"
base_url = "https://custom.example/v1"
models = ["custom-model"]
default = "custom-model"
api_key_env = "MIMO_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	view := NewApp().Settings()
	presetView := providerPresetViewByID(t, view, "mimo-api")
	if !presetView.Added || presetView.Status != providerPresetStatusNameConflict || !reflect.DeepEqual(presetView.StatusProviderNames, []string{"mimo-api"}) {
		t.Fatalf("mimo-api preset view = %+v, want name-conflict because a different same-name provider exists", presetView)
	}

	var providerView *ProviderView
	for i := range view.Providers {
		if view.Providers[i].Name == "mimo-api" {
			providerView = &view.Providers[i]
			break
		}
	}
	if providerView == nil {
		t.Fatal("mimo-api provider view missing")
	}
	if providerView.Added {
		t.Fatalf("mimo-api provider Added = true, want false until provider_access explicitly enables it")
	}
}

func TestSettingsMarksLegacyEquivalentPresetAsInstalled(t *testing.T) {
	isolateDesktopUserDirs(t)
	preset, ok := config.CuratedProviderPreset("mimo-api")
	if !ok || len(preset.Entries) == 0 {
		t.Fatal("missing mimo-api preset")
	}
	legacy := preset.Entries[0]
	legacy.PresetID = ""
	legacy.PresetVersion = 0
	cfg := config.Default()
	if err := cfg.UpsertProvider(legacy); err != nil {
		t.Fatalf("upsert legacy provider: %v", err)
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	view := NewApp().Settings()
	presetView := providerPresetViewByID(t, view, "mimo-api")
	if !presetView.Added || presetView.Status != providerPresetStatusInstalled || !reflect.DeepEqual(presetView.StatusProviderNames, []string{"mimo-api"}) {
		t.Fatalf("mimo-api preset view = %+v, want installed for legacy equivalent config", presetView)
	}
}

func TestSettingsMarksPresetWithChangedCoreConfigAsModified(t *testing.T) {
	isolateDesktopUserDirs(t)
	preset, ok := config.CuratedProviderPreset("mimo-api")
	if !ok || len(preset.Entries) == 0 {
		t.Fatal("missing mimo-api preset")
	}
	modified := preset.Entries[0]
	modified.BaseURL = "https://custom.example/v1"
	cfg := config.Default()
	if err := cfg.UpsertProvider(modified); err != nil {
		t.Fatalf("upsert modified provider: %v", err)
	}
	cfg.Desktop.ProviderAccess = []string{"mimo-api"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	view := NewApp().Settings()
	presetView := providerPresetViewByID(t, view, "mimo-api")
	if !presetView.Added || presetView.Status != providerPresetStatusInstalledModified || !reflect.DeepEqual(presetView.StatusProviderNames, []string{"mimo-api"}) {
		t.Fatalf("mimo-api preset view = %+v, want installed-modified for edited preset provider", presetView)
	}
}

func TestSettingsPreservesStepFunRegionalPresetBaseURLs(t *testing.T) {
	isolateDesktopUserDirs(t)

	cfg := config.Default()
	stepfun, ok := config.CuratedProviderPreset("stepfun")
	if !ok || len(stepfun.Entries) != 1 {
		t.Fatal("missing stepfun preset")
	}
	stepfunEntry := stepfun.Entries[0]
	stepfunEntry.BaseURL = "https://api.stepfun.ai/step_plan/v1"
	stepfunAnthropic, ok := config.CuratedProviderPreset("stepfun-anthropic")
	if !ok || len(stepfunAnthropic.Entries) != 1 {
		t.Fatal("missing stepfun-anthropic preset")
	}
	stepfunAnthropicEntry := stepfunAnthropic.Entries[0]
	stepfunAnthropicEntry.BaseURL = "https://api.stepfun.ai/step_plan"
	cfg.Providers = append(cfg.Providers, stepfunEntry, stepfunAnthropicEntry)
	cfg.Desktop.ProviderAccess = []string{"stepfun", "stepfun-anthropic"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	view := NewApp().Settings()
	for _, id := range []string{"stepfun", "stepfun-anthropic"} {
		presetView := providerPresetViewByID(t, view, id)
		if !presetView.Added || presetView.Status != providerPresetStatusInstalledModified {
			t.Fatalf("%s preset view = %+v, want installed-modified for a preserved regional endpoint", id, presetView)
		}
	}

	loaded := config.LoadForEdit(config.UserConfigPath())
	stepfunEntryView, ok := loaded.Provider("stepfun")
	if !ok {
		t.Fatal("stepfun provider missing after load")
	}
	if got := stepfunEntryView.BaseURL; got != "https://api.stepfun.ai/step_plan/v1" {
		t.Fatalf("stepfun base_url = %q, want preserved regional URL", got)
	}
	stepfunAnthropicEntryView, ok := loaded.Provider("stepfun-anthropic")
	if !ok {
		t.Fatal("stepfun-anthropic provider missing after load")
	}
	if got := stepfunAnthropicEntryView.BaseURL; got != "https://api.stepfun.ai/step_plan" {
		t.Fatalf("stepfun-anthropic base_url = %q, want preserved regional URL", got)
	}
}

func TestSettingsMarksSimilarProviderPresetWithoutBlockingAdd(t *testing.T) {
	isolateDesktopUserDirs(t)
	preset, ok := config.CuratedProviderPreset("mimo-api")
	if !ok || len(preset.Entries) == 0 {
		t.Fatal("missing mimo-api preset")
	}
	similar := preset.Entries[0]
	similar.Name = "my-mimo"
	similar.PresetID = ""
	similar.PresetVersion = 0
	cfg := config.Default()
	if err := cfg.UpsertProvider(similar); err != nil {
		t.Fatalf("upsert similar provider: %v", err)
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	view := NewApp().Settings()
	presetView := providerPresetViewByID(t, view, "mimo-api")
	if presetView.Added || presetView.Status != providerPresetStatusSimilarExisting || !reflect.DeepEqual(presetView.StatusProviderNames, []string{"my-mimo"}) {
		t.Fatalf("mimo-api preset view = %+v, want non-blocking similar-existing status", presetView)
	}
}

func TestAddProviderPresetAccessSavesEditableProviderAndKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("MIMO_API_KEY", "")
	os.Unsetenv("MIMO_API_KEY")

	if warning, err := NewApp().AddProviderPresetAccess("mimo-api", "sk-mimo"); err != nil {
		t.Fatalf("AddProviderPresetAccess: %v", err)
	} else if warning != "" {
		t.Fatalf("AddProviderPresetAccess warning = %q, want none", warning)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	p, ok := cfg.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider not saved")
	}
	if p.Kind != "openai" || p.BaseURL != "https://api.xiaomimimo.com/v1" || p.Default != "mimo-v2.5-pro" {
		t.Fatalf("mimo-api provider after preset add = %+v", p)
	}
	if p.PresetID != "mimo-api" || p.PresetVersion != config.ProviderPresetVersion {
		t.Fatalf("mimo-api preset metadata = %q/%d, want mimo-api/%d", p.PresetID, p.PresetVersion, config.ProviderPresetVersion)
	}
	if !p.NoProxy {
		t.Fatal("mimo-api preset should save no_proxy = true")
	}
	if !p.HasVisionModel("mimo-v2.5") || p.HasVisionModel("mimo-v2.5-pro") {
		t.Fatalf("mimo vision_models = %+v, want only vision-capable MiMo models", p.VisionModels)
	}
	if price := p.PriceForModel("mimo-v2.5-pro"); price == nil || price.Currency != "₩" {
		t.Fatalf("mimo-v2.5-pro price = %+v, want KRW pricing", price)
	}
	if !providerAccessSet(cfg.Desktop.ProviderAccess)["mimo-api"] {
		t.Fatalf("provider_access missing mimo-api: %+v", cfg.Desktop.ProviderAccess)
	}
	data, err := os.ReadFile(config.UserCredentialsPath())
	if err != nil {
		t.Fatalf("read saved credentials: %v", err)
	}
	if !strings.Contains(string(data), "MIMO_API_KEY=sk-mimo") {
		t.Fatalf("saved credentials missing MiMo key:\n%s", data)
	}

	view := NewApp().Settings()
	var presetView *ProviderPresetView
	var providerView *ProviderView
	for i := range view.ProviderPresets {
		if view.ProviderPresets[i].ID == "mimo-api" {
			presetView = &view.ProviderPresets[i]
		}
	}
	for i := range view.Providers {
		if view.Providers[i].Name == "mimo-api" {
			providerView = &view.Providers[i]
		}
	}
	if presetView == nil || !presetView.Added || presetView.Status != providerPresetStatusInstalled || !presetView.KeySet {
		t.Fatalf("mimo-api preset view = %+v, want installed/key-set", presetView)
	}
	if providerView == nil || providerView.BuiltIn || !providerView.Added || !providerView.KeySet {
		t.Fatalf("mimo provider view = %+v, want editable added custom provider with key", providerView)
	}
}

func TestAddProviderPresetAccessDoesNotOverwriteExistingProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("MIMO_API_KEY", "")
	os.Unsetenv("MIMO_API_KEY")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-original")

	cfg := config.Default()
	custom := config.ProviderEntry{
		Name:      "mimo-api",
		Kind:      "openai",
		BaseURL:   "https://custom.example/v1",
		Models:    []string{"custom-model"},
		Default:   "custom-model",
		APIKeyEnv: "MIMO_API_KEY",
		Headers:   map[string]string{"X-Custom": "keep-me"},
	}
	if err := cfg.UpsertProvider(custom); err != nil {
		t.Fatalf("upsert custom provider: %v", err)
	}
	cfg.Desktop.ProviderAccess = []string{"mimo-api"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if warning, err := NewApp().AddProviderPresetAccess("mimo-api", "sk-new"); err == nil {
		t.Fatal("AddProviderPresetAccess unexpectedly overwrote an existing provider")
	} else if !strings.Contains(err.Error(), "provider name(s) already exist") {
		t.Fatalf("AddProviderPresetAccess error = %v, want name-exists guard", err)
	} else if warning != "" {
		t.Fatalf("AddProviderPresetAccess warning = %q, want none on rejected add", warning)
	}

	cfg = config.LoadForEdit(config.UserConfigPath())
	got, ok := cfg.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing after rejected add")
	}
	if got.BaseURL != custom.BaseURL || got.DefaultModel() != custom.DefaultModel() || !reflect.DeepEqual(got.ModelList(), custom.ModelList()) || !reflect.DeepEqual(got.Headers, custom.Headers) {
		t.Fatalf("mimo-api provider was overwritten: %+v, want custom %+v", got, custom)
	}
	data, err := os.ReadFile(config.UserCredentialsPath())
	if err != nil {
		t.Fatalf("read saved credentials: %v", err)
	}
	if strings.Contains(string(data), "sk-new") || !strings.Contains(string(data), "MIMO_API_KEY=sk-original") {
		t.Fatalf("credentials changed after rejected add:\n%s", data)
	}
}

func TestResetProviderPresetAccessOverwritesSameNameProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("MIMO_API_KEY", "")
	os.Unsetenv("MIMO_API_KEY")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-original")

	cfg := config.Default()
	custom := config.ProviderEntry{
		Name:      "mimo-api",
		Kind:      "openai",
		BaseURL:   "https://custom.example/v1",
		Models:    []string{"custom-model"},
		Default:   "custom-model",
		APIKeyEnv: "MIMO_API_KEY",
		Headers:   map[string]string{"X-Custom": "remove-me"},
	}
	if err := cfg.UpsertProvider(custom); err != nil {
		t.Fatalf("upsert custom provider: %v", err)
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := NewApp().ResetProviderPresetAccess("mimo-api"); err != nil {
		t.Fatalf("ResetProviderPresetAccess: %v", err)
	}

	cfg = config.LoadForEdit(config.UserConfigPath())
	got, ok := cfg.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing after reset")
	}
	if got.BaseURL != "https://api.xiaomimimo.com/v1" || got.DefaultModel() != "mimo-v2.5-pro" || got.PresetID != "mimo-api" || got.PresetVersion != config.ProviderPresetVersion {
		t.Fatalf("mimo-api provider after reset = %+v, want preset template", got)
	}
	if len(got.Headers) != 0 {
		t.Fatalf("mimo-api headers after reset = %+v, want preset headers", got.Headers)
	}
	if !providerAccessSet(cfg.Desktop.ProviderAccess)["mimo-api"] {
		t.Fatalf("provider_access missing mimo-api after reset: %+v", cfg.Desktop.ProviderAccess)
	}
	data, err := os.ReadFile(config.UserCredentialsPath())
	if err != nil {
		t.Fatalf("read saved credentials: %v", err)
	}
	if !strings.Contains(string(data), "MIMO_API_KEY=sk-original") {
		t.Fatalf("credentials changed after reset:\n%s", data)
	}

	presetView := providerPresetViewByID(t, NewApp().Settings(), "mimo-api")
	if !presetView.Added || presetView.Status != providerPresetStatusInstalled {
		t.Fatalf("mimo-api preset view = %+v, want installed after reset", presetView)
	}
}

func TestResetProviderPresetAccessRejectsMissingSameNameProvider(t *testing.T) {
	isolateDesktopUserDirs(t)

	if err := NewApp().ResetProviderPresetAccess("mimo-api"); err == nil {
		t.Fatal("ResetProviderPresetAccess unexpectedly reset a missing provider")
	} else if !strings.Contains(err.Error(), "no same-name provider exists") {
		t.Fatalf("ResetProviderPresetAccess error = %v, want missing same-name provider guard", err)
	}
}

func TestAddEveryProviderPresetAccessInstallsTemplate(t *testing.T) {
	for _, preset := range config.CuratedProviderPresets() {
		t.Run(preset.ID, func(t *testing.T) {
			isolateDesktopUserDirs(t)

			if warning, err := NewApp().AddProviderPresetAccess(preset.ID, "sk-test"); err != nil {
				t.Fatalf("AddProviderPresetAccess(%q): %v", preset.ID, err)
			} else if warning != "" {
				t.Fatalf("AddProviderPresetAccess(%q) warning = %q, want none", preset.ID, warning)
			}

			cfg := config.LoadForEdit(config.UserConfigPath())
			access := providerAccessSet(cfg.Desktop.ProviderAccess)
			for _, entry := range preset.Entries {
				got, ok := cfg.Provider(entry.Name)
				if !ok {
					t.Fatalf("provider %q from preset %q was not saved", entry.Name, preset.ID)
				}
				if !access[entry.Name] {
					t.Fatalf("provider_access for preset %q missing %q: %+v", preset.ID, entry.Name, cfg.Desktop.ProviderAccess)
				}
				if got.Kind != entry.Kind || got.BaseURL != entry.BaseURL || got.DefaultModel() != entry.DefaultModel() || got.APIKeyEnv != entry.APIKeyEnv || got.AuthHeader != entry.AuthHeader || got.NoProxy != entry.NoProxy {
					t.Fatalf("provider %q core fields = %+v, want template %+v", entry.Name, got, entry)
				}
				if got.PresetID != preset.ID || got.PresetVersion != config.ProviderPresetVersion {
					t.Fatalf("provider %q preset metadata = %q/%d, want %q/%d", entry.Name, got.PresetID, got.PresetVersion, preset.ID, config.ProviderPresetVersion)
				}
				if got.ContextWindow != entry.ContextWindow || got.Thinking != entry.Thinking || got.DefaultEffort != entry.DefaultEffort || got.ReasoningProtocol != entry.ReasoningProtocol {
					t.Fatalf("provider %q capability fields = %+v, want template %+v", entry.Name, got, entry)
				}
				if !reflect.DeepEqual(got.ModelList(), entry.ModelList()) || !reflect.DeepEqual(got.VisionModels, entry.VisionModels) || !reflect.DeepEqual(got.SupportedEfforts, entry.SupportedEfforts) {
					t.Fatalf("provider %q models/capabilities = %+v, want template %+v", entry.Name, got, entry)
				}
				if !reflect.DeepEqual(got.Headers, entry.Headers) || !reflect.DeepEqual(got.ExtraBody, entry.ExtraBody) {
					t.Fatalf("provider %q request extras = %+v, want template %+v", entry.Name, got, entry)
				}
			}

			view := NewApp().Settings()
			var presetView *ProviderPresetView
			for i := range view.ProviderPresets {
				if view.ProviderPresets[i].ID == preset.ID {
					presetView = &view.ProviderPresets[i]
					break
				}
			}
			if presetView == nil || !presetView.Added || presetView.Status != providerPresetStatusInstalled || !presetView.KeySet || !presetView.Configured {
				t.Fatalf("preset view for %q = %+v, want installed/key-set/configured", preset.ID, presetView)
			}
		})
	}
}

