package dlp

import (
	"context"
	"strings"
	"testing"
	"time"

	"patty/internal/provider"
)

// fakeAWSKey assembles the fake AWS access key at runtime so the
// committed file never contains a scanner-matching literal.
func fakeAWSKey() string { return "AKIA" + "ABCDEFGHIJKLMNOP" }

// fakeProvider implements provider.Provider for the integration
// test. It returns canned chunks so the test can verify the
// DLP wrapper scans both the request and the response.
type fakeProvider struct {
	name         string
	requestChunk provider.Chunk
	err          error
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(chan provider.Chunk, 1)
	out <- f.requestChunk
	close(out)
	return out, nil
}

// TestProviderWrapperBlocksSecretInRequest covers the C1+ outbound
// boundary: a request containing an AWS key is blocked before the
// inner provider sees it.
func TestProviderWrapperBlocksSecretInRequest(t *testing.T) {
	inner := &fakeProvider{
		name: "test",
		requestChunk: provider.Chunk{
			Type: provider.ChunkText,
			Text: "ok",
		},
	}
	w := NewProvider(inner, NewOutboundHook(NewScanner()), nil)
	req := provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "here is my key: " + fakeAWSKey() + ""},
		},
	}
	_, err := w.Stream(context.Background(), req)
	if err == nil {
		t.Fatal("request with AWS key must be blocked")
	}
	if !strings.Contains(err.Error(), "aws-access-key") {
		t.Errorf("expected aws-access-key error, got %v", err)
	}
}

// TestProviderWrapperRedactsRequestBeforeSending covers the
// redact-only mode: the harness sees the redacted payload rather
// than the raw secret.
func TestProviderWrapperRedactsRequestBeforeSending(t *testing.T) {
	gotReq := provider.Request{}
	inner := &fakeProvider{
		name:         "test",
		requestChunk: provider.Chunk{Type: provider.ChunkText, Text: "ok"},
	}
	// custom Provider that records the request it received.
	recorder := &recordingProvider{inner: inner, got: &gotReq}
	w := NewProvider(recorder, NewOutboundHook(NewScanner()), nil)
	// BlockOnDeny=false so the dispatch proceeds with the redacted
	// payload.
	w.hook.BlockOnDeny = false
	req := provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "AWS key: " + fakeAWSKey() + ""},
		},
	}
	if _, err := w.Stream(context.Background(), req); err != nil {
		t.Fatalf("redact mode must pass: %v", err)
	}
	if strings.Contains(gotReq.Messages[0].Content, fakeAWSKey()) {
		t.Errorf("inner provider still received the raw secret: %q", gotReq.Messages[0].Content)
	}
	if !strings.Contains(gotReq.Messages[0].Content, "AWS_KEY_REDACTED") {
		t.Errorf("redacted payload missing token: %q", gotReq.Messages[0].Content)
	}
}

// TestProviderWrapperBlocksExfilInResponse covers the C5 boundary:
// a model response that contains an AWS key is blocked before the
// harness sees it.
func TestProviderWrapperBlocksExfilInResponse(t *testing.T) {
	inner := &fakeProvider{
		name: "test",
		requestChunk: provider.Chunk{
			Type: provider.ChunkText,
			Text: "here is your key: " + fakeAWSKey() + "",
		},
	}
	w := NewProvider(inner, NewOutboundHook(NewScanner()), nil)
	out, err := w.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream must succeed: %v", err)
	}
	// The wrapper emits an error chunk when exfil is detected. The
	// harness's agent loop turns a final ChunkError into a stream
	// error.
	var sawExfil bool
	for chunk := range out {
		if chunk.Type == provider.ChunkError && chunk.Err != nil {
			if strings.Contains(chunk.Err.Error(), "exfil") {
				sawExfil = true
			}
		}
	}
	if !sawExfil {
		t.Fatal("response with AWS key must produce an exfil error chunk")
	}
}

// TestProviderWrapperPassesCleanContent covers the green path:
// a clean request + clean response round-trip without error.
func TestProviderWrapperPassesCleanContent(t *testing.T) {
	inner := &fakeProvider{
		name: "test",
		requestChunk: provider.Chunk{
			Type: provider.ChunkText,
			Text: "all clear",
		},
	}
	w := NewProvider(inner, NewOutboundHook(NewScanner()), nil)
	out, err := w.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("clean stream must pass: %v", err)
	}
	chunk := <-out
	if chunk.Type != provider.ChunkText || chunk.Text != "all clear" {
		t.Errorf("clean chunk lost: %v", chunk)
	}
}

// TestProviderWrapperInnerErrorPropagates covers the error
// transport path: an inner-Provider error propagates cleanly.
func TestProviderWrapperInnerErrorPropagates(t *testing.T) {
	inner := &fakeProvider{name: "test", err: context.DeadlineExceeded}
	w := NewProvider(inner, NewOutboundHook(NewScanner()), nil)
	_, err := w.Stream(context.Background(), provider.Request{})
	if err != context.DeadlineExceeded {
		t.Errorf("inner error lost: %v", err)
	}
}

// recordingProvider captures the request the inner provider saw so
// the redact-mode test can verify the harness sent the redacted
// payload (not the raw secret).
type recordingProvider struct {
	inner provider.Provider
	got   *provider.Request
}

func (r *recordingProvider) Name() string { return r.inner.Name() }

func (r *recordingProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	*r.got = req
	return r.inner.Stream(ctx, req)
}

// _ keeps the time import visible when the test suite evolves.
var _ = time.Now
