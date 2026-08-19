//go:build profile_public

package config

import "testing"

// DARI-only amendment (2026-08-19): provider_access=["deepseek"] no longer
// materializes a generic provider in ANY profile — the official-template
// injection compiled out with the BYOK surface. The outside-public twin is
// provider_official_endpoint_profile_test.go.
func TestOfficialDeepSeekInjectionMaterializesInPublicProfile(t *testing.T) {
	c := &Config{Desktop: DesktopConfig{ProviderAccess: []string{"deepseek"}}}
	normalizeDesktopOfficialProviderAccess(c)
	if _, ok := c.Provider("deepseek"); ok {
		t.Fatal("deepseek provider materialized in a DARI-only public build")
	}
}
