//go:build !profile_public

package config

import "testing"

// Official DeepSeek endpoint-default expectations outside the public profile
// (ADR G4, Task 7 addendum): the wallet-endpoint literal compiles only into
// public-tagged twins (provider_legacy_deepseek.go), so in these profiles the
// loader may backfill the vendor context window but must never write a
// balance_url the user didn't declare — injecting one would arm the boot-side
// tier lock against a URL the user never wrote. The public-profile positive
// twins live in provider_isolation_public_test.go.

// The desktop official-provider injection keeps the 1M window but omits the
// wallet endpoint.
func TestOfficialDeepSeekInjectionOmitsBalanceURLOutsidePublicProfile(t *testing.T) {
	c := &Config{Desktop: DesktopConfig{ProviderAccess: []string{"deepseek"}}}
	normalizeDesktopOfficialProviderAccess(c)
	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing")
	}
	if p.BalanceURL != "" {
		t.Fatalf("injected balance_url = %q, want empty outside the public profile", p.BalanceURL)
	}
	if p.ContextWindow != 1_000_000 {
		t.Fatalf("injected context_window = %d, want the official 1M default still backfilled", p.ContextWindow)
	}
}

// The endpoint-keyed backfill behaves the same way: window restored, no
// wallet endpoint.
func TestOfficialDeepSeekBackfillOmitsBalanceURLOutsidePublicProfile(t *testing.T) {
	p := &ProviderEntry{Name: "deepseek", Kind: "openai", BaseURL: "https://api.deepseek.com"}
	backfillDeepSeekOfficialEndpointDefaults(p)
	if p.BalanceURL != "" {
		t.Fatalf("backfilled balance_url = %q, want empty outside the public profile", p.BalanceURL)
	}
	if p.ContextWindow != 1_000_000 {
		t.Fatalf("backfilled context_window = %d, want the official 1M default still backfilled", p.ContextWindow)
	}
}

// No stock DeepSeek template exists outside the public profile, so the
// persisted-currency reset predicate must not match even a hand-pasted copy.
func TestStandardDeepSeekTemplateNeverMatchesOutsidePublicProfile(t *testing.T) {
	p := &ProviderEntry{
		Name:          "deepseek",
		Kind:          "openai",
		BaseURL:       "https://api.deepseek.com",
		APIKeyEnv:     "DEEPSEEK_API_KEY",
		BalanceURL:    "https://api.deepseek.com/user/balance",
		ContextWindow: 1_000_000,
	}
	if isStandardDeepSeekProviderTemplate(p) {
		t.Fatal("isStandardDeepSeekProviderTemplate must be false outside the public profile")
	}
}
