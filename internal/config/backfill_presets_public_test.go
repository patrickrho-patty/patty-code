//go:build profile_public

// Preset-dependent legacy-migration tests (ADR G4): they build fixtures from
// the curated BYOK catalog or assert the legacy DeepSeek backfill, both of
// which are public-profile capabilities. Enterprise/sovereign legs run the
// profile-clean remainder in backfill_test.go.

package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestBackfillDeepSeekProRestoresPro(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY"},
	}}
	backfillDeepSeekPro(c)
	pro := hasModel(c, "deepseek-v4-pro")
	if pro == nil {
		t.Fatal("deepseek-v4-pro not restored")
	} else if pro.Price == nil || pro.Price.Output != 0.87 || pro.Price.Currency != "$" {
		t.Errorf("pro price = %+v, want default USD preset", pro.Price)
	}
}

func TestBackfillDeepSeekProUsesConfiguredLanguage(t *testing.T) {
	c := &Config{Language: "ko-KR", Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY"},
	}}
	backfillDeepSeekPro(c)
	pro := hasModel(c, "deepseek-v4-pro")
	if pro == nil {
		t.Fatal("deepseek-v4-pro not restored")
	} else if pro.Price == nil || pro.Price.Output != 6 || pro.Price.Currency != "₩" {
		t.Errorf("pro price = %+v, want KRW preset", pro.Price)
	}
}

func TestBackfillDeepSeekProInheritsKeyEnv(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "MY_DS_KEY"},
	}}
	backfillDeepSeekPro(c)
	if pro := hasModel(c, "deepseek-v4-pro"); pro == nil || pro.APIKeyEnv != "MY_DS_KEY" {
		t.Errorf("pro should inherit the flash key env, got %+v", pro)
	}
}

func TestLoadAndSavePreserveStepFunRegionalBaseURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	stepfun, ok := CuratedProviderPreset("stepfun")
	if !ok || len(stepfun.Entries) != 1 {
		t.Fatal("missing stepfun preset")
	}
	stepfunEntry := stepfun.Entries[0]
	stepfunEntry.BaseURL = legacyStepFunOpenAIBaseURL
	stepfunAnthropic, ok := CuratedProviderPreset("stepfun-anthropic")
	if !ok || len(stepfunAnthropic.Entries) != 1 {
		t.Fatal("missing stepfun-anthropic preset")
	}
	stepfunAnthropicEntry := stepfunAnthropic.Entries[0]
	stepfunAnthropicEntry.BaseURL = legacyStepFunAnthropicBaseURL
	cfg.Providers = append(cfg.Providers, stepfunEntry, stepfunAnthropicEntry)
	cfg.Desktop.ProviderAccess = []string{"stepfun", "stepfun-anthropic"}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded := LoadForEdit(path)
	if got, _ := loaded.Provider("stepfun"); got == nil || got.BaseURL != legacyStepFunOpenAIBaseURL {
		t.Fatalf("loaded stepfun = %+v, want global base URL preserved", got)
	}
	if got, _ := loaded.Provider("stepfun-anthropic"); got == nil || got.BaseURL != legacyStepFunAnthropicBaseURL {
		t.Fatalf("loaded stepfun-anthropic = %+v, want global base URL preserved", got)
	}

	var disk Config
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode config after read-only load: %v", err)
	}
	if got, _ := disk.Provider("stepfun"); got == nil || got.BaseURL != legacyStepFunOpenAIBaseURL {
		t.Fatalf("read-only LoadForEdit rewrote stepfun = %+v, want legacy base URL preserved on disk", got)
	}
	if err := loaded.SaveTo(path); err != nil {
		t.Fatalf("explicit SaveTo: %v", err)
	}
	disk = Config{}
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode explicitly saved config: %v", err)
	}
	if got, _ := disk.Provider("stepfun"); got == nil || got.BaseURL != legacyStepFunOpenAIBaseURL {
		t.Fatalf("persisted stepfun = %+v, want global base URL preserved", got)
	}
	if got, _ := disk.Provider("stepfun-anthropic"); got == nil || got.BaseURL != legacyStepFunAnthropicBaseURL {
		t.Fatalf("persisted stepfun-anthropic = %+v, want global base URL preserved", got)
	}
}

func TestLoadForEditAppliesLegacyLongCatContextWindowMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	for _, id := range []string{"longcat-openai", "longcat-anthropic"} {
		preset, ok := CuratedProviderPreset(id)
		if !ok || len(preset.Entries) != 1 {
			t.Fatalf("missing %s preset", id)
		}
		entry := preset.Entries[0]
		entry.ContextWindow = legacyLongCat20ContextWindow
		cfg.Providers = append(cfg.Providers, entry)
	}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded := LoadForEdit(path)
	for _, id := range []string{"longcat-openai", "longcat-anthropic"} {
		if got, _ := loaded.Provider(id); got == nil || got.ContextWindow != longCat20ContextWindow {
			t.Fatalf("loaded %s = %+v, want context_window %d", id, got, longCat20ContextWindow)
		}
	}
}

func TestLoadForEditAppliesLegacyQwenContextWindowMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	preset, ok := CuratedProviderPreset("qwen-cn")
	if !ok || len(preset.Entries) != 1 {
		t.Fatal("missing qwen-cn preset")
	}
	entry := preset.Entries[0]
	entry.ContextWindow = 0
	entry.ModelOverrides = nil
	cfg.Providers = append(cfg.Providers, entry)
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded := LoadForEditWithoutCredentials(path)
	got, ok := loaded.Provider("qwen-cn")
	if !ok || got.ContextWindow != 1_000_000 {
		t.Fatalf("loaded qwen-cn = %+v, want context_window 1000000", got)
	}
	if resolved, ok := loaded.ResolveModel("qwen-cn/glm-5"); !ok || resolved.ContextWindow != 202_752 {
		t.Fatalf("resolved qwen-cn/glm-5 = %+v/%v, want context_window 202752", resolved, ok)
	}

	var disk Config
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode config after read-only load: %v", err)
	}
	if persisted, _ := disk.Provider("qwen-cn"); persisted == nil || persisted.ContextWindow != 0 {
		t.Fatalf("read-only load rewrote qwen-cn = %+v, want legacy config preserved on disk", persisted)
	}
	if err := EditConfigFileWithoutCredentials(path, func(*Config) error { return nil }); err != nil {
		t.Fatalf("EditConfigFileWithoutCredentials: %v", err)
	}
	disk = Config{}
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode config after edit: %v", err)
	}
	persisted, ok := disk.Provider("qwen-cn")
	if !ok || persisted.ContextWindow != 1_000_000 || persisted.ModelOverrides["glm-5"].ContextWindow != 202_752 {
		t.Fatalf("persisted qwen-cn = %+v, want migrated context defaults", persisted)
	}
}

func TestLoadForEditKeepsLegacyKimiK3CatalogMigrationInMemoryUntilSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	preset, ok := CuratedProviderPreset("kimi-cn")
	if !ok || len(preset.Entries) != 1 {
		t.Fatal("missing kimi-cn preset")
	}
	entry := preset.Entries[0]
	entry.Models = append([]string(nil), legacyKimiAPIModels...)
	entry.VisionModels = append([]string(nil), legacyKimiAPIModels...)
	delete(entry.ModelOverrides, "kimi-k3")
	cfg.Providers = append(cfg.Providers, entry)
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded := LoadForEdit(path)
	got, ok := loaded.Provider("kimi-cn")
	if !ok || !got.HasModel("kimi-k3") || !got.HasVisionModel("kimi-k3") {
		t.Fatalf("loaded kimi-cn = %+v, want persisted K3 catalog", got)
	}
	var disk Config
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode config after read-only load: %v", err)
	}
	persisted, ok := disk.Provider("kimi-cn")
	if !ok || persisted.HasModel("kimi-k3") {
		t.Fatalf("read-only LoadForEdit rewrote kimi-cn = %+v, want legacy catalog preserved on disk", persisted)
	}
	if err := loaded.SaveTo(path); err != nil {
		t.Fatalf("explicit SaveTo: %v", err)
	}
	disk = Config{}
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode explicitly saved config: %v", err)
	}
	persisted, ok = disk.Provider("kimi-cn")
	if !ok || !persisted.HasModel("kimi-k3") || persisted.ModelOverrides["kimi-k3"].DefaultEffort != "max" {
		t.Fatalf("persisted kimi-cn = %+v, want K3 defaults", persisted)
	}
}

func TestLoadForEditKeepsLegacyOpenCodeGoKimiK3CatalogMigrationInMemoryUntilSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	preset, ok := CuratedProviderPreset("opencode-go")
	if !ok || len(preset.Entries) != 1 {
		t.Fatal("missing opencode-go preset")
	}
	entry := preset.Entries[0]
	entry.Models = append([]string(nil), legacyOpenCodeGoModels...)
	entry.VisionModels = nil
	delete(entry.ModelOverrides, "kimi-k3")
	cfg.Providers = append(cfg.Providers, entry)
	cfg.Desktop.ProviderAccess = []string{"opencode-go"}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded := LoadForEdit(path)
	got, ok := loaded.Provider("opencode-go")
	if !ok || !got.HasModel("kimi-k3") || !got.HasVisionModel("kimi-k3") {
		t.Fatalf("loaded opencode-go = %+v, want persisted Kimi K3 catalog", got)
	}
	if ov := got.ModelOverrides["kimi-k3"]; ov.DefaultEffort != "max" || ov.ContextWindow != 1_048_576 {
		t.Fatalf("loaded Kimi K3 override = %+v", ov)
	}

	var disk Config
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode config after read-only load: %v", err)
	}
	persisted, ok := disk.Provider("opencode-go")
	if !ok || persisted.HasModel("kimi-k3") {
		t.Fatalf("read-only LoadForEdit rewrote opencode-go = %+v, want legacy catalog preserved on disk", persisted)
	}
	if err := loaded.SaveTo(path); err != nil {
		t.Fatalf("explicit SaveTo: %v", err)
	}
	disk = Config{}
	if _, err := toml.DecodeFile(path, &disk); err != nil {
		t.Fatalf("decode explicitly saved config: %v", err)
	}
	persisted, ok = disk.Provider("opencode-go")
	if !ok || !persisted.HasModel("kimi-k3") || !persisted.HasVisionModel("kimi-k3") {
		t.Fatalf("persisted opencode-go = %+v, want Kimi K3 catalog", persisted)
	}
}

func TestNormalizeLegacyQwenContextWindowsMigratesOnlyOfficialPresets(t *testing.T) {
	qwenIDs := []string{
		"qwen-cn",
		"qwen-global",
		"qwen-coding-plan-cn",
		"qwen-coding-plan-cn-anthropic",
		"qwen-coding-plan-global",
		"qwen-coding-plan-global-anthropic",
	}
	c := &Config{}
	for _, id := range qwenIDs {
		preset, ok := CuratedProviderPreset(id)
		if !ok || len(preset.Entries) != 1 {
			t.Fatalf("missing %s preset", id)
		}
		entry := preset.Entries[0]
		entry.ContextWindow = 0
		entry.ModelOverrides = nil
		c.Providers = append(c.Providers, entry)
	}

	preset, _ := CuratedProviderPreset("qwen-cn")
	customWindow := preset.Entries[0]
	customWindow.Name = "qwen-custom-window"
	customWindow.ContextWindow = 131_072
	customWindow.ModelOverrides = nil
	customEndpoint := preset.Entries[0]
	customEndpoint.Name = "qwen-custom-endpoint"
	customEndpoint.BaseURL = "https://gateway.example.com/v1"
	customEndpoint.ContextWindow = 0
	customEndpoint.ModelOverrides = nil
	customCatalog := preset.Entries[0]
	customCatalog.Name = "qwen-custom-catalog"
	customCatalog.Models = append(customCatalog.Models, "private-model")
	customCatalog.ContextWindow = 0
	customCatalog.ModelOverrides = nil
	customOverride := preset.Entries[0]
	customOverride.Name = "qwen-custom-override"
	customOverride.ContextWindow = 0
	customOverride.VisionModels = []string{}
	customOverride.ModelOverrides = map[string]ProviderModelOverride{
		"GLM-5": {
			ReasoningProtocol: ReasoningProtocolNone,
			ContextWindow:     123_456,
		},
	}
	c.Providers = append(c.Providers, customWindow, customEndpoint, customCatalog, customOverride)

	if !normalizeLegacyQwenContextWindows(c) {
		t.Fatal("legacy Qwen context-window migration did not report a change")
	}
	wantOverrides := qwenModelContextOverrides()
	for i, id := range qwenIDs {
		got := &c.Providers[i]
		if got.ContextWindow != 1_000_000 {
			t.Fatalf("%s context_window = %d, want 1000000", id, got.ContextWindow)
		}
		for model, want := range wantOverrides {
			if got.ModelOverrides[model].ContextWindow != want.ContextWindow {
				t.Fatalf("%s/%s context_window = %d, want %d", id, model, got.ModelOverrides[model].ContextWindow, want.ContextWindow)
			}
		}
	}

	if got := c.Providers[len(qwenIDs)]; got.ContextWindow != 131_072 || got.ModelOverrides != nil {
		t.Fatalf("custom provider-wide context changed: %+v", got)
	}
	if got := c.Providers[len(qwenIDs)+1]; got.ContextWindow != 0 || got.ModelOverrides != nil {
		t.Fatalf("custom endpoint changed: %+v", got)
	}
	if got := c.Providers[len(qwenIDs)+2]; got.ContextWindow != 0 || got.ModelOverrides != nil {
		t.Fatalf("custom catalog changed: %+v", got)
	}
	gotCustom := c.Providers[len(qwenIDs)+3]
	if gotCustom.ContextWindow != 1_000_000 || gotCustom.ModelOverrides["GLM-5"].ContextWindow != 123_456 {
		t.Fatalf("custom model override changed: %+v", gotCustom)
	}
	if _, duplicate := gotCustom.ModelOverrides["glm-5"]; duplicate {
		t.Fatal("migration added a duplicate case-insensitive GLM-5 override")
	}
	if got := gotCustom.ModelOverrides["MiniMax-M2.5"].ContextWindow; got != 196_608 {
		t.Fatalf("missing MiniMax override was not backfilled: %d", got)
	}
	if gotCustom.VisionModels == nil || len(gotCustom.VisionModels) != 0 {
		t.Fatalf("custom vision models changed: %#v", gotCustom.VisionModels)
	}
}

func TestNormalizeLegacyOpenCodeGoKimiK3CatalogMigratesOnlyUntouchedPreset(t *testing.T) {
	legacyEntry := ProviderEntry{
		Name:          "opencode-go",
		Kind:          "openai",
		BaseURL:       "https://opencode.ai/zen/go/v1/",
		Models:        append([]string(nil), legacyOpenCodeGoModels...),
		Default:       "glm-5.2",
		APIKeyEnv:     "CUSTOM_OPENCODE_GO_KEY",
		ContextWindow: 256_000,
		PresetID:      "opencode-go",
		ModelOverrides: map[string]ProviderModelOverride{
			"deepseek-v4-pro": {
				ReasoningProtocol: ReasoningProtocolDeepSeek,
				SupportedEfforts:  []string{"high", "max"},
				DefaultEffort:     "high",
			},
		},
	}
	customModels := legacyEntry
	customModels.Name = "opencode-go-custom-models"
	customModels.Models = append(append([]string(nil), customModels.Models...), "private-model")
	customModels.ModelOverrides = cloneModelOverrideMap(legacyEntry.ModelOverrides)
	customEndpoint := legacyEntry
	customEndpoint.Name = "opencode-go-custom-endpoint"
	customEndpoint.BaseURL = "https://gateway.example.com/v1"
	customEndpoint.ModelOverrides = cloneModelOverrideMap(legacyEntry.ModelOverrides)
	noPresetIdentity := legacyEntry
	noPresetIdentity.Name = "opencode-go-copy"
	noPresetIdentity.PresetID = ""
	noPresetIdentity.ModelOverrides = cloneModelOverrideMap(legacyEntry.ModelOverrides)
	c := &Config{Providers: []ProviderEntry{legacyEntry, customModels, customEndpoint, noPresetIdentity}}

	if !normalizeLegacyOpenCodeGoKimiK3Catalog(c) {
		t.Fatal("legacy OpenCode Go catalog migration did not report a change")
	}
	got := &c.Providers[0]
	if !got.HasModel("kimi-k3") || !got.HasVisionModel("kimi-k3") {
		t.Fatalf("migrated OpenCode Go catalog = %+v, want Kimi K3 with vision", got)
	}
	k3 := got.ModelOverrides["kimi-k3"]
	if k3.ReasoningProtocol != ReasoningProtocolOpenAI ||
		!stringSlicesEqual(k3.SupportedEfforts, []string{"high", "max"}) ||
		k3.DefaultEffort != "max" ||
		k3.ContextWindow != 1_048_576 {
		t.Fatalf("migrated Kimi K3 override = %+v", k3)
	}
	if got.APIKeyEnv != "CUSTOM_OPENCODE_GO_KEY" || got.ContextWindow != 256_000 {
		t.Fatalf("unrelated provider edits were not preserved: %+v", got)
	}
	if _, ok := got.ModelOverrides["deepseek-v4-pro"]; !ok {
		t.Fatal("existing model override was dropped")
	}
	for i := 1; i < len(c.Providers); i++ {
		if c.Providers[i].HasModel("kimi-k3") {
			t.Fatalf("customized provider %q was unexpectedly migrated", c.Providers[i].Name)
		}
	}

	preIdentity := legacyEntry
	preIdentity.PresetID = ""
	preIdentity.ModelOverrides = cloneModelOverrideMap(legacyEntry.ModelOverrides)
	preIdentityConfig := &Config{Providers: []ProviderEntry{preIdentity}}
	if !normalizeLegacyOpenCodeGoKimiK3Catalog(preIdentityConfig) || !preIdentityConfig.Providers[0].HasModel("kimi-k3") {
		t.Fatal("pre-preset-identity OpenCode Go install was not migrated")
	}

	vision := false
	customK3 := legacyEntry
	customK3.ModelOverrides = cloneModelOverrideMap(legacyEntry.ModelOverrides)
	delete(customK3.ModelOverrides, "kimi-k3")
	wantK3 := ProviderModelOverride{
		ReasoningProtocol: ReasoningProtocolNone,
		SupportedEfforts:  []string{"low"},
		DefaultEffort:     "low",
		Vision:            &vision,
		ContextWindow:     262_144,
	}
	customK3.ModelOverrides["KIMI-K3"] = wantK3
	customK3Config := &Config{Providers: []ProviderEntry{customK3}}
	if !normalizeLegacyOpenCodeGoKimiK3Catalog(customK3Config) {
		t.Fatal("legacy OpenCode Go catalog with custom Kimi K3 override was not migrated")
	}
	gotK3, ok := customK3Config.Providers[0].ModelOverrides["KIMI-K3"]
	if !ok || !reflect.DeepEqual(gotK3, wantK3) {
		t.Fatalf("custom Kimi K3 override = %+v, want preserved %+v", gotK3, wantK3)
	}
	if _, duplicate := customK3Config.Providers[0].ModelOverrides["kimi-k3"]; duplicate {
		t.Fatal("migration added a duplicate case-insensitive Kimi K3 override")
	}
}

