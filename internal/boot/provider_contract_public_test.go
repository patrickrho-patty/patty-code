//go:build profile_public

// Wire-contract tests for the generic OpenAI-compatible chat client: they
// stream against httptest servers impersonating vendor endpoints, so they
// exercise a public-build capability (ADR G4) and compile only there.
package boot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"patty/internal/config"
	"patty/internal/provider"

	// Registers the generic kinds these contract tests construct.
	_ "patty/internal/provider/anthropic"
	_ "patty/internal/provider/openai"
)

func TestNewProviderAppliesConfiguredDefaultEffort(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name:             "custom",
		Kind:             "openai",
		BaseURL:          srv.URL,
		Model:            "m",
		SupportedEfforts: []string{"low", "medium", "high"},
		DefaultEffort:    "MEDIUM",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if got := gotReq["reasoning_effort"]; got != "medium" {
		t.Fatalf("reasoning_effort = %#v, want medium from default_effort", got)
	}
}

func TestNewProviderPreservesExplicitlySupportedKimiK3Efforts(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name:              "opencode-go",
		Kind:              "openai",
		BaseURL:           srv.URL,
		Model:             "kimi-k3",
		ReasoningProtocol: config.ReasoningProtocolOpenAI,
		SupportedEfforts:  []string{"high", "max"},
		DefaultEffort:     "max",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if got := gotReq["reasoning_effort"]; got != "max" {
		t.Fatalf("reasoning_effort = %#v, want explicitly supported max", got)
	}
}

func TestNewProviderAppliesOfficialKimiK3RequestContract(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name:              "kimi-global",
		Kind:              "openai",
		BaseURL:           "https://api.moonshot.ai/v1",
		ChatURL:           srv.URL,
		Model:             "kimi-k3",
		ReasoningProtocol: config.ReasoningProtocolOpenAI,
		SupportedEfforts:  []string{"low", "high", "max"},
		DefaultEffort:     "max",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages:    []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		Temperature: provider.TemperaturePtr(0),
		MaxTokens:   2000,
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if gotReq["reasoning_effort"] != "max" || gotReq["max_completion_tokens"] != float64(2000) {
		t.Fatalf("official Kimi K3 request = %+v, want max effort and max_completion_tokens", gotReq)
	}
	for _, field := range []string{"temperature", "max_tokens"} {
		if _, ok := gotReq[field]; ok {
			t.Fatalf("official Kimi K3 request must omit %q: %+v", field, gotReq)
		}
	}
}

func TestNewProviderPropagatesConfiguredMaxOutputTokens(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name: "openai", Kind: "openai", BaseURL: "https://api.openai.com/v1",
		ChatURL: srv.URL, Model: "o3", MaxOutputTokens: 4096,
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if gotReq["max_completion_tokens"] != float64(4096) {
		t.Fatalf("max_completion_tokens = %#v, want 4096: %+v", gotReq["max_completion_tokens"], gotReq)
	}
	if _, exists := gotReq["max_tokens"]; exists {
		t.Fatalf("official OpenAI request must omit max_tokens: %+v", gotReq)
	}
}

func TestNewProviderAppliesModelReasoningProtocol(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name:    "deepseek-proxy",
		Kind:    "openai",
		BaseURL: srv.URL,
		Model:   "deepseek-v4-flash",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if got := gotReq["reasoning_effort"]; got != "high" {
		t.Fatalf("reasoning_effort = %#v, want high from DeepSeek model capability", got)
	}
	thinking, ok := gotReq["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("thinking = %#v, want enabled", gotReq["thinking"])
	}
}

func TestNewProviderBuildsDeepSeekAnthropicPreset(t *testing.T) {
	preset, ok := config.CuratedProviderPreset("deepseek-anthropic")
	if !ok || len(preset.Entries) != 1 {
		t.Fatalf("DeepSeek Anthropic preset = %+v", preset)
	}
	var cfg config.Config
	if err := cfg.UpsertProvider(preset.Entries[0]); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	entry, ok := cfg.ResolveModel("deepseek-anthropic/deepseek-v4-flash")
	if !ok {
		t.Fatal("ResolveModel failed")
	}
	p, err := NewProvider(entry)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if p.Name() != "deepseek-anthropic" || !provider.RequiresToolCallReasoning(p) || provider.RequiresReasoningRoundTrip(p) {
		t.Fatalf("assembled DeepSeek Anthropic provider = %T/%q policies=%v/%v", p, p.Name(), provider.RequiresToolCallReasoning(p), provider.RequiresReasoningRoundTrip(p))
	}
}

func TestNewProviderAllowsExplicitOfficialDeepSeekVisionModel(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name:         "deepseek",
		Kind:         "openai",
		BaseURL:      "https://api.deepseek.com",
		ChatURL:      srv.URL,
		Model:        "deepseek-v5-vision",
		VisionModels: []string{"deepseek-v5-vision"},
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{
			Role: provider.RoleUser, Content: "describe",
			Images: []string{"data:image/png;base64,AAAA"},
		}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}

	messages, ok := gotReq["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %#v, want one message", gotReq["messages"])
	}
	message, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("message = %#v, want object", messages[0])
	}
	parts, ok := message["content"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("content = %#v, want [text, image_url]", message["content"])
	}
	imagePart, ok := parts[1].(map[string]any)
	if !ok || imagePart["type"] != "image_url" {
		t.Fatalf("image part = %#v, want image_url", parts[1])
	}
}
