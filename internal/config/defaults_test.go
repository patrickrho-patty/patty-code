package config

import "testing"

func TestDefaultUsesPattyCodeStandard(t *testing.T) {
	// Per PRD v2 §0.2 the official Harness connects to a PAPER relay; the
	// default model and provider must reflect that, not the legacy
	// OpenAI/Anthropic-compatible default.
	cfg := Default()
	if cfg.DefaultModel != "patty/patty-code-standard" || len(cfg.Providers) != 1 {
		t.Fatalf("default model/providers = %q/%+v", cfg.DefaultModel, cfg.Providers)
	}
	p := cfg.Providers[0]
	if p.Name != "patty" || p.Kind != "dari" || p.BaseURL != "localhost:8444" || p.Model != "patty-code-standard" || p.APIKeyEnv != "DARI_HARNESS_KEY" || p.ContextWindow != 262144 {
		t.Fatalf("default provider = %+v", p)
	}
	if cfg.Agent.CompactForceRatio != 0.98 {
		t.Fatalf("force ratio = %v, want 0.98", cfg.Agent.CompactForceRatio)
	}
}
