package config

// ProviderPreset is the wire type for one curated BYOK preset. It is shared
// across build profiles (public builds embed the catalog; enterprise and
// sovereign builds compile the accessors as no-ops, ADR G4) so both legs
// agree on the shape.
type ProviderPreset struct {
	ID          string
	Label       string
	Description string
	KeyEnv      string
	Entries     []ProviderEntry
}

// Shared preset metadata and legacy-migration catalogs. These identifiers are
// consumed by untagged migration code in load.go that must compile on every
// profile; the curated preset catalog itself (provider_presets.go) is
// public-only.
const (
	ProviderPresetVersion        = 1
	longCat20ContextWindow       = 1_048_576
	legacyLongCat20ContextWindow = 131_072
	longCatOpenAIBaseURL         = "https://api.longcat.chat/openai/v1"
	longCatAnthropicBaseURL      = "https://api.longcat.chat/anthropic"
)

var (
	legacyKimiAPIModels = []string{"kimi-k2.7-code", "kimi-k2.7-code-highspeed", "kimi-k2.6", "kimi-k2.5"}
	kimiAPIModels       = []string{"kimi-k3", "kimi-k2.7-code", "kimi-k2.7-code-highspeed", "kimi-k2.6", "kimi-k2.5"}

	longCat20Models = []string{"LongCat-2.0"}

	legacyOpenCodeGoModels = []string{"glm-5.2", "glm-5.1", "kimi-k2.7-code", "kimi-k2.6", "deepseek-v4-pro", "deepseek-v4-flash", "mimo-v2.5-pro", "mimo-v2.5"}
	opencodeGoModels       = []string{"glm-5.2", "glm-5.1", "kimi-k3", "kimi-k2.7-code", "kimi-k2.6", "deepseek-v4-pro", "deepseek-v4-flash", "mimo-v2.5-pro", "mimo-v2.5"}
)

func kimiK3DirectOverride() ProviderModelOverride {
	return ProviderModelOverride{
		ReasoningProtocol: ReasoningProtocolOpenAI,
		SupportedEfforts:  []string{"low", "high", "max"},
		DefaultEffort:     "max",
		ContextWindow:     1_048_576,
	}
}
