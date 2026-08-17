package config

import "testing"

// The pre-DARI official Patty entry (kind=openai, omni.agents.patty.io — the
// vLLM-mimic endpoint) migrates to the stock DARI relay entry at load (PRD v2
// §0.2): a vendor-issued entry follows the vendor's endpoint change instead of
// tripping the boot tier lock, which exists for user-added generic endpoints.
func TestLegacyOmniOfficialProviderMigratesToDARI(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{{
		Name:      "patty",
		Kind:      "openai",
		BaseURL:   "https://omni.agents.patty.io/v1",
		Model:     "medium",
		APIKeyEnv: "AGENTS_PATTY_API_KEY",
	}}}
	normalizeLegacyOmniOfficialProvider(c)
	if len(c.Providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(c.Providers))
	}
	got := c.Providers[0]
	stock, _ := Default().Provider("patty")
	if got.Kind != "dari" || got.BaseURL != stock.BaseURL || got.Model != stock.Model || got.APIKeyEnv != stock.APIKeyEnv {
		t.Fatalf("migrated entry = %+v, want stock DARI entry", got)
	}
}

// Non-official entries at the same host (user-added generic relays) are NOT
// rewritten — they keep failing closed per ADR G4.
func TestLegacyOmniMigrationLeavesNonOfficialEntriesAlone(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{{
		Name:    "my-own",
		Kind:    "openai",
		BaseURL: "https://omni.agents.patty.io/v1",
	}}}
	normalizeLegacyOmniOfficialProvider(c)
	if c.Providers[0].Kind != "openai" || c.Providers[0].Name != "my-own" {
		t.Fatalf("user entry rewritten: %+v", c.Providers[0])
	}
}
