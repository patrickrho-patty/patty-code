// Package agent_test contains the DLP integration tests for the
// agent package. The tests are in the production package's _test
// variant so they can call New() directly and access the
// package-private wrapper. The DLP wrapper defaults to disabled
// (dlpEnabled() is false unless PATTY_DLP_ENABLED=1), so each test
// opts in via withDLPEnabled to verify the wrapper actually fires.
package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"patty/internal/event"
	"patty/internal/provider"
	"patty/internal/tool"
)

// withDLPEnabled forces the DLP wrapper on for the duration of
// the test, then restores the previous env-var state via
// t.Cleanup. The harness's own dlpEnabled() defaults to disabled
// in non-test contexts; these tests explicitly opt back in so
// the integration assertions fire.
func withDLPEnabled(t *testing.T) {
	t.Helper()
	prev, hadPrev := os.LookupEnv("PATTY_DLP_ENABLED")
	os.Setenv("PATTY_DLP_ENABLED", "1")
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv("PATTY_DLP_ENABLED", prev)
		} else {
			os.Unsetenv("PATTY_DLP_ENABLED")
		}
	})
}

// fakeAWSKey assembles the fake AWS access key at runtime so the
// committed file never contains a scanner-matching literal.
func fakeAWSKey() string { return "AKIA" + "ABCDEFGHIJKLMNOP" }

// fakeStreamProvider emits a canned response so the harness's
// DLP wrapper can scan it. The agent's Stream call wraps the
// inner provider.
type fakeStreamProvider struct {
	name string
	text string
	err  error
}

func (f *fakeStreamProvider) Name() string { return f.name }

func (f *fakeStreamProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(chan provider.Chunk, 1)
	out <- provider.Chunk{Type: provider.ChunkText, Text: f.text}
	close(out)
	return out, nil
}

// TestAgentNewWrapsDLPAndBlocksSecretInRequest is the C1+C5
// integration test: an agent created via New() must refuse a
// request containing an AWS key, with the DLP rule ID surfaced to
// the harness's error return.
func TestAgentNewWrapsDLPAndBlocksSecretInRequest(t *testing.T) {
	withDLPEnabled(t)
	prov := &fakeStreamProvider{name: "fake", text: "ok"}
	tools := tool.NewRegistry()
	a := New(prov, tools, nil, Options{}, event.Discard)
	req := provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "here is my AWS key: " + fakeAWSKey() + ""},
		},
	}
	_, err := a.prov.Stream(context.Background(), req)
	if err == nil {
		t.Fatal("request with AWS key must be blocked")
	}
	if !strings.Contains(err.Error(), "aws-access-key") {
		t.Errorf("expected aws-access-key error, got %v", err)
	}
}

// TestAgentNewWrapsDLPAndBlocksSecretInResponse is the C5
// integration test: a model response containing an AWS key must
// be blocked before the harness surfaces it.
func TestAgentNewWrapsDLPAndBlocksSecretInResponse(t *testing.T) {
	withDLPEnabled(t)
	prov := &fakeStreamProvider{
		name: "fake",
		text: "model output: here is your key " + fakeAWSKey() + "",
	}
	tools := tool.NewRegistry()
	a := New(prov, tools, nil, Options{}, event.Discard)
	req := provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "hi"},
		},
	}
	out, err := a.prov.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("stream must succeed: %v", err)
	}
	var sawExfil bool
	for chunk := range out {
		if chunk.Type == provider.ChunkError && chunk.Err != nil {
			if strings.Contains(chunk.Err.Error(), "exfil") {
				sawExfil = true
			}
		}
	}
	if !sawExfil {
		t.Fatal("response with AWS key must trigger exfil block")
	}
}

// TestAgentNewPassesCleanStream covers the green path: clean
// request + clean response round-trip without DLP alerts.
func TestAgentNewPassesCleanStream(t *testing.T) {
	withDLPEnabled(t)
	prov := &fakeStreamProvider{name: "fake", text: "all clear"}
	tools := tool.NewRegistry()
	a := New(prov, tools, nil, Options{}, event.Discard)
	out, err := a.prov.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("clean stream must pass: %v", err)
	}
	chunk := <-out
	if chunk.Type != provider.ChunkText || chunk.Text != "all clear" {
		t.Errorf("clean chunk lost: %v", chunk)
	}
}
