package boot

import (
	"testing"

	"patty/internal/config"
	"patty/internal/tier"
)

func TestTierLockInputFlagsExcludedCapabilities(t *testing.T) {
	// main_test.go's TestMain opts the whole binary into PATTY_ALLOW_GENERIC=1
	// for the legacy lifecycle fixtures; this test must exercise the locked
	// path, so neutralize it for its duration (restored automatically).
	t.Setenv("PATTY_ALLOW_GENERIC", "0")
	cfg := &config.Config{Providers: []config.ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", BalanceURL: "https://api.deepseek.com/user/balance"},
		{Name: "org-relay", Kind: "dari"},
	}}
	in := tierLockInput(cfg)
	// Under the default (enterprise) profile the openai entry is excluded
	// and the balance URL is excluded; the DARI relay entry never is.
	if len(in.ExcludedProviders) != 1 || in.ExcludedProviders[0] != "deepseek-flash" {
		t.Fatalf("ExcludedProviders = %v, want [deepseek-flash]", in.ExcludedProviders)
	}
	if len(in.BalanceProviders) != 1 || in.BalanceProviders[0] != "deepseek-flash" {
		t.Fatalf("BalanceProviders = %v, want [deepseek-flash]", in.BalanceProviders)
	}
	if err := tier.Lock(in); err == nil {
		t.Fatal("Lock must fail closed for excluded providers under the default profile")
	}
}

func TestTierLockInputCleanForDARIOnlyConfig(t *testing.T) {
	cfg := &config.Config{Providers: []config.ProviderEntry{{Name: "org-relay", Kind: "dari"}}}
	if err := tier.Lock(tierLockInput(cfg)); err != nil {
		t.Fatalf("DARI-only config must lock clean: %v", err)
	}
}
