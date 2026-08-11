package serve

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"patty/internal/config"
	"patty/internal/netclient"
	"patty/internal/provider"
)

type recordingTitleProvider struct {
	requests []provider.Request
}

func (p *recordingTitleProvider) Name() string { return "recording-title" }

func (p *recordingTitleProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	p.requests = append(p.requests, req)
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: req.Messages[len(req.Messages)-1].Content}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func TestTitleProviderReusesCompleteProviderConfigAndDisablesReasoning(t *testing.T) {
	kind := "title-config-" + strings.ReplaceAll(t.Name(), "/", "-")
	var got provider.Config
	provider.Register(kind, func(cfg provider.Config) (provider.Provider, error) {
		got = cfg
		return &recordingTitleProvider{}, nil
	})
	entry := &config.ProviderEntry{
		Name:              "custom-title",
		Kind:              kind,
		BaseURL:           "https://example.test/v1",
		ChatURL:           "https://chat.example.test/completions",
		Model:             "custom-model",
		APIKeyEnv:         "CUSTOM_TITLE_KEY",
		Headers:           map[string]string{"X-Workspace": "patty"},
		ExtraBody:         map[string]any{"route": "fast"},
		AuthHeader:        true,
		Thinking:          "enabled",
		ReasoningProtocol: "openai",
		SupportedEfforts:  []string{"disabled", "high"},
		DefaultEffort:     "high",
	}
	proxy := netclient.ProxySpec{Mode: netclient.ModeOff}
	if _, err := newTitleProvider(entry, proxy); err != nil {
		t.Fatalf("newTitleProvider: %v", err)
	}
	if got.Name != entry.Name || got.BaseURL != entry.BaseURL || got.Model != entry.Model {
		t.Fatalf("title provider identity = %+v, want entry identity %+v", got, entry)
	}
	if got.Extra["effort"] != "disabled" || got.Extra["thinking"] != "disabled" || got.Extra["chat_url"] != entry.ChatURL || got.Extra["auth_header"] != entry.AuthHeader {
		t.Fatalf("title provider scalar options = %+v", got.Extra)
	}
	if !reflect.DeepEqual(got.Extra["proxy_spec"], proxy) {
		t.Fatalf("title provider proxy = %+v, want %+v", got.Extra["proxy_spec"], proxy)
	}
	headers, _ := got.Extra["headers"].(map[string]string)
	extraBody, _ := got.Extra["extra_body"].(map[string]any)
	if headers["X-Workspace"] != "patty" || extraBody["route"] != "fast" {
		t.Fatalf("title provider request options = %+v", got.Extra)
	}
}

func TestStockTitleProviderUsesPattyMediumWithoutUnsupportedEffort(t *testing.T) {
	cfg := config.Default()
	entry, ok := cfg.ResolveModel(cfg.DefaultModel)
	if !ok {
		t.Fatalf("default model %q did not resolve", cfg.DefaultModel)
	}
	kind := "stock-title-" + strings.ReplaceAll(t.Name(), "/", "-")
	entry.Kind = kind
	var got provider.Config
	provider.Register(kind, func(cfg provider.Config) (provider.Provider, error) {
		got = cfg
		return &recordingTitleProvider{}, nil
	})
	if _, err := newTitleProvider(entry, cfg.NetworkProxySpec()); err != nil {
		t.Fatalf("construct stock title provider: %v", err)
	}
	if got.Extra["effort"] != "" || got.Extra["thinking"] != "" {
		t.Fatalf("stock title reasoning overrides = %+v, want none", got.Extra)
	}
}

func TestAnthropicTitleProviderOmitsAdaptiveThinking(t *testing.T) {
	kind := "anthropic-title-" + strings.ReplaceAll(t.Name(), "/", "-")
	var got provider.Config
	provider.Register(kind, func(cfg provider.Config) (provider.Provider, error) {
		got = cfg
		return &recordingTitleProvider{}, nil
	})
	entry := &config.ProviderEntry{
		Name: "anthropic-title", Kind: kind, BaseURL: "https://api.anthropic.com",
		Model: "claude-test", APIKeyEnv: "ANTHROPIC_API_KEY", Thinking: "adaptive",
	}
	if _, err := newTitleProvider(entry, netclient.ProxySpec{Mode: netclient.ModeOff}); err != nil {
		t.Fatalf("construct adaptive title provider: %v", err)
	}
	if got.Extra["thinking"] != "" {
		t.Fatalf("adaptive title thinking = %q, want omitted", got.Extra["thinking"])
	}
}

func TestGenerateTitleStripsPasteLabelAndUsesShortBudget(t *testing.T) {
	prov := &recordingTitleProvider{}
	s := &Server{titleProv: prov}
	got := s.generateTitle(context.Background(), "[붙여넣은 텍스트 #1 · 20 줄]\nfix the login loop")
	if got != "fix the login loop" {
		t.Fatalf("title = %q, want pasted label removed", got)
	}
	if len(prov.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(prov.requests))
	}
	req := prov.requests[0]
	if req.MaxTokens != 60 {
		t.Fatalf("MaxTokens = %d, want 60", req.MaxTokens)
	}
	if req.Messages[0].Content != titlePrompt || req.Messages[1].Content != "fix the login loop" {
		t.Fatalf("title messages = %+v", req.Messages)
	}
}

func TestSessionTitleCachesByFirstMessageAcrossMtimeChanges(t *testing.T) {
	dir := t.TempDir()
	prov := &recordingTitleProvider{}
	s := &Server{titleProv: prov, titles: newTitleCache(dir)}

	if got := s.sessionTitle(context.Background(), "a.jsonl", "first prompt", 100); got != "first prompt" {
		t.Fatalf("first title = %q", got)
	}
	if got := s.sessionTitle(context.Background(), "a.jsonl", "first prompt", 200); got != "first prompt" {
		t.Fatalf("title after append = %q", got)
	}
	if len(prov.requests) != 1 {
		t.Fatalf("requests after mtime-only change = %d, want 1", len(prov.requests))
	}

	if got := s.sessionTitle(context.Background(), "a.jsonl", "replacement prompt", 300); got != "replacement prompt" {
		t.Fatalf("title after replacing first turn = %q", got)
	}
	if len(prov.requests) != 2 {
		t.Fatalf("requests after replacing first turn = %d, want 2", len(prov.requests))
	}

	freshProv := &recordingTitleProvider{}
	fresh := &Server{titleProv: freshProv, titles: newTitleCache(dir)}
	if got := fresh.sessionTitle(context.Background(), "a.jsonl", "replacement prompt", 400); got != "replacement prompt" {
		t.Fatalf("persisted title = %q", got)
	}
	if len(freshProv.requests) != 0 {
		t.Fatalf("fresh server regenerated persisted title %d time(s)", len(freshProv.requests))
	}
}

func TestPreviewTitleStripsOnlyLeadingPasteLabel(t *testing.T) {
	if got := previewTitle("[Pasted text #2 · 42 lines]\nfunc foo() { return 1 }"); got != "func foo() { return 1 }" {
		t.Fatalf("previewTitle = %q", got)
	}
	const inline = "Explain [Pasted text #2 · 42 lines] handling"
	if got := previewTitle(inline); got != inline {
		t.Fatalf("inline label changed to %q", got)
	}
}
