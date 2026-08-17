//go:build profile_public

package config

import "testing"

// The official DeepSeek desktop injection is a public-profile capability
// (ADR G4): provider_access=["deepseek"] materializes the stock official
// entry — with the wallet endpoint, since public also allows
// CapBalanceFetch. Outside the public profile nothing materializes; the
// !profile_public twin is provider_official_endpoint_profile_test.go.

func TestOfficialDeepSeekInjectionMaterializesInPublicProfile(t *testing.T) {
	c := &Config{Desktop: DesktopConfig{ProviderAccess: []string{"deepseek"}}}
	normalizeDesktopOfficialProviderAccess(c)
	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing in the public profile")
	}
	if p.BaseURL != "https://api.deepseek.com" || p.Kind != "openai" {
		t.Fatalf("injected entry = %s/%s, want official template", p.Kind, p.BaseURL)
	}
	if p.ContextWindow != 1_000_000 {
		t.Fatalf("injected context_window = %d, want the official 1M default", p.ContextWindow)
	}
	if p.BalanceURL != "https://api.deepseek.com/user/balance" {
		t.Fatalf("injected balance_url = %q, want the public-profile wallet endpoint", p.BalanceURL)
	}
}
