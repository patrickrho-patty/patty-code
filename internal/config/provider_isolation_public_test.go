//go:build profile_public

// Official DeepSeek endpoint-default assertions (ADR G4): the balance-URL
// backfill is a public-profile capability, so these expectations compile
// only there. The isolation tests above stay profile-clean.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// officialDeepSeekTOML declares the official endpoint under name without
// context_window, balance_url or price, which a documented config may omit.
func officialDeepSeekTOML(name string) string {
	return `config_version = 1
default_model = "` + name + `/deepseek-v4-flash"

[[providers]]
name        = "` + name + `"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
`
}

func assertOfficialDeepSeekDefaults(t *testing.T, tag string, p *ProviderEntry) {
	t.Helper()
	if p.ContextWindow != 1_000_000 {
		t.Errorf("%s: official DeepSeek provider has context_window %d, want 1000000; a zero window disables compaction", tag, p.ContextWindow)
	}
	if p.BalanceURL != "https://api.deepseek.com/user/balance" {
		t.Errorf("%s: official DeepSeek provider has balance_url %q, want the vendor wallet endpoint; empty removes the balance readout", tag, p.BalanceURL)
	}
	if p.Prices["deepseek-v4-flash"] == nil {
		t.Errorf("%s: official DeepSeek provider lost its per-model price backfill: prices=%v", tag, p.Prices)
	}
}

// The desktop official-provider injection path carries the wallet endpoint in
// the public profile (twin of the outside-public expectations in
// provider_official_endpoint_profile_test.go).
func TestOfficialDeepSeekInjectionBackfillsBalanceURLInPublicProfile(t *testing.T) {
	c := &Config{Desktop: DesktopConfig{ProviderAccess: []string{"deepseek"}}}
	normalizeDesktopOfficialProviderAccess(c)
	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing")
	}
	if p.BalanceURL != "https://api.deepseek.com/user/balance" {
		t.Fatalf("injected balance_url = %q, want the vendor wallet endpoint", p.BalanceURL)
	}
	if p.ContextWindow != 1_000_000 {
		t.Fatalf("injected context_window = %d, want the official 1M default", p.ContextWindow)
	}
}

// TestOfficialDeepSeekProviderStillGetsItsDefaults pins the other half of the
// contract. Isolating the decoded provider list must not strip the defaults a
// genuinely official endpoint may omit, so they are reapplied by an
// endpoint-keyed backfill that a custom provider can never match.
//
// Both loaders are checked because they normalize through different entry
// points: the runtime loader feeds the agent and compaction, while the edit
// loader is what desktop Settings reads and writes back.
func TestOfficialDeepSeekProviderStillGetsItsDefaults(t *testing.T) {
	for _, name := range []string{"deepseek", "deepseek-flash"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			ws := t.TempDir()
			t.Setenv("PATTY_HOME", home)
			path := filepath.Join(home, "config.toml")
			if err := os.WriteFile(path, []byte(officialDeepSeekTOML(name)), 0o600); err != nil {
				t.Fatal(err)
			}

			cfg, err := LoadForRootReadOnly(ws)
			if err != nil {
				t.Fatal(err)
			}
			p, ok := cfg.Provider(name)
			if !ok {
				t.Fatalf("official %q provider missing after runtime load", name)
			}
			assertOfficialDeepSeekDefaults(t, "LoadForRoot/"+name, p)

			edit := LoadForEditWithoutCredentials(path)
			ep, ok := edit.Provider(name)
			if !ok {
				t.Fatalf("official %q provider missing after edit load", name)
			}
			assertOfficialDeepSeekDefaults(t, "LoadForEdit/"+name, ep)
		})
	}
}
