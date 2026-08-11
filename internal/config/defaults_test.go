package config

import "testing"

func TestDefaultUsesPattyMedium(t *testing.T) {
	cfg := Default()
	if cfg.DefaultModel != "patty/medium" || len(cfg.Providers) != 1 {
		t.Fatalf("default model/providers = %q/%+v", cfg.DefaultModel, cfg.Providers)
	}
	p := cfg.Providers[0]
	if p.Name != "patty" || p.Kind != "openai" || p.BaseURL != "https://omni.agents.patty.io/v1" || p.Model != "medium" || p.APIKeyEnv != "AGENTS_PATTY_API_KEY" || p.ContextWindow != 248124 {
		t.Fatalf("default provider = %+v", p)
	}
	if got := int(float64(p.ContextWindow) * cfg.Agent.CompactRatio); got != 238123 {
		t.Fatalf("auto compact threshold = %d, want 238123", got)
	}
	if cfg.Agent.CompactForceRatio != 0.98 {
		t.Fatalf("force ratio = %v, want 0.98", cfg.Agent.CompactForceRatio)
	}
}
