//go:build profile_public

package config

import "testing"

// DARI-only amendment (2026-08-19): the desktop official-provider injection
// path carries no wallet endpoint in any profile — no generic provider
// exists to hang it on. (Twin of the outside-public expectations in
// provider_official_endpoint_profile_test.go.)
func TestOfficialDeepSeekInjectionBackfillsBalanceURLInPublicProfile(t *testing.T) {
	c := &Config{Desktop: DesktopConfig{ProviderAccess: []string{"deepseek"}}}
	normalizeDesktopOfficialProviderAccess(c)
	if p, ok := c.Provider("deepseek"); ok {
		t.Fatalf("deepseek provider materialized: %+v", p)
	}
}
