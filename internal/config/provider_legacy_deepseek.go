//go:build profile_public

package config

// legacyDeepSeekProviderEntries returns the v1-era BYOK DeepSeek defaults.
// Public-profile capability (ADR G4): enterprise/sovereign builds embed no
// foreign LLM endpoints. The pre-Patty stock catalog is retained only for
// importing and repairing configurations written by older releases.
const (
	legacyDeepSeekDefaultModel  = "deepseek-flash"
	legacyDeepSeekCredentialEnv = "DEEPSEEK_API_KEY"
)

func legacyDeepSeekProviderEntries() []ProviderEntry {
	return []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: legacyDeepSeekCredentialEnv, BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: deepSeekV4FlashPriceUSD()},
		{Name: "deepseek-pro", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro", APIKeyEnv: legacyDeepSeekCredentialEnv, BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: deepSeekV4ProPriceUSD()},
	}
}
