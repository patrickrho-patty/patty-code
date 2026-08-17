package agent

import (
	"context"
	"encoding/json"
	"sync"

	"patty/internal/event"
)

// recordSink captures every event plus protocol-recovery audits, for tests
// that assert on the observed stream. Provider-agnostic, so it stays
// compiled under every build profile (ADR G4 tags only generic-provider
// tests out of non-public builds).
type recordSink struct {
	mu       sync.Mutex
	evs      []event.Event
	recovery []event.ProtocolRecoveryAudit
}

func (s *recordSink) Emit(e event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, e)
}

func (s *recordSink) kinds(k event.Kind) []event.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []event.Event
	for _, e := range s.evs {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

func (s *recordSink) RecordProtocolRecovery(a event.ProtocolRecoveryAudit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recovery = append(s.recovery, a)
}

func (s *recordSink) recoveryCount(kind event.ProtocolRecoveryKind) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int
	for _, audit := range s.recovery {
		if audit.Kind == kind {
			count++
		}
	}
	return count
}

// echoTool is a read-only echo tool shared by agent E2E tests.
type echoTool struct{}

func (echoTool) Name() string        { return "echo" }
func (echoTool) Description() string { return "echo back the given text" }
func (echoTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`)
}
func (echoTool) ReadOnly() bool { return true }
func (echoTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(args, &a)
	return "echoed: " + a.Text, nil
}
