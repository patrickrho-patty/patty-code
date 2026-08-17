//go:build profile_public

package config

import (
	"strings"

	"patty/internal/tier"
)

// legacyDeepSeekProviderEntries returns the v1-era BYOK DeepSeek defaults.
// Public-profile capability (ADR G4): enterprise/sovereign builds embed no
// foreign LLM endpoints. The pre-Patty stock catalog is retained only for
// importing and repairing configurations written by older releases.
const (
	legacyDeepSeekDefaultModel  = "deepseek-flash"
	legacyDeepSeekCredentialEnv = "DEEPSEEK_API_KEY"
)

func legacyDeepSeekProviderEntries() []ProviderEntry {
	tier.AssertAllowed(tier.CapPublicPresets)
	return []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: legacyDeepSeekCredentialEnv, BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: deepSeekV4FlashPriceUSD()},
		{Name: "deepseek-pro", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro", APIKeyEnv: legacyDeepSeekCredentialEnv, BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: deepSeekV4ProPriceUSD()},
	}
}

// backfillLegacyDeepSeekBalanceURL injects the DeepSeek wallet endpoint when
// the profile's tier allows balance fetching. Relocated from load.go (Task 7
// addendum) so the endpoint literal compiles only into public builds; the
// no-op twin in provider_legacy_deepseek_stub.go keeps load.go call sites
// unconditional, and outside the public profile injecting a URL the user
// never wrote would arm the boot-side tier lock (ADR G4).
func backfillLegacyDeepSeekBalanceURL(p *ProviderEntry) {
	if p == nil {
		return
	}
	if tier.Default.Allows(tier.CapBalanceFetch) && strings.TrimSpace(p.BalanceURL) == "" {
		p.BalanceURL = "https://api.deepseek.com/user/balance"
	}
}

// isStandardDeepSeekProviderTemplate reports whether p carries the full stock
// official DeepSeek marker set (credential env, wallet endpoint, 1M window).
// Relocated from pricing.go (Task 7 addendum): the marker string exists only
// in public builds, so outside them no config can be a stock template.
func isStandardDeepSeekProviderTemplate(p *ProviderEntry) bool {
	if p == nil || officialProviderKind(p) != "deepseek" {
		return false
	}
	return strings.TrimSpace(p.APIKeyEnv) == "DEEPSEEK_API_KEY" &&
		strings.TrimSpace(p.BalanceURL) == "https://api.deepseek.com/user/balance" &&
		p.ContextWindow == 1_000_000
}

// deepSeekOfficialTemplateEntry returns the stock official DeepSeek entry
// that desktop provider_access can materialize. Public-profile only (ADR G4):
// synthesizing a generic openai-kind provider in enterprise/sovereign would
// both embed the foreign endpoint and arm the boot tier lock against an
// entry the user never wrote.
func deepSeekOfficialTemplateEntry() (ProviderEntry, bool) {
	tier.AssertAllowed(tier.CapPublicPresets)
	return ProviderEntry{
		Name:          "deepseek",
		Kind:          "openai",
		BaseURL:       "https://api.deepseek.com",
		Models:        []string{"deepseek-v4-flash", "deepseek-v4-pro"},
		Default:       "deepseek-v4-flash",
		APIKeyEnv:     "DEEPSEEK_API_KEY",
		ContextWindow: 1_000_000,
	}, true
}
