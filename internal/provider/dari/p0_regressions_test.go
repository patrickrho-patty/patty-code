package dari

import (
	"context"
	"encoding/json"

	"patty/internal/dariproto"
	"patty/internal/provider"
	"testing"
	"time"

	"patty/internal/changeboard"
	"patty/internal/workflow"
)

// p0_regressions_test.go pins the audit's P0 fixes.

// P0-1: AirGap()/Awareness() must not deadlock (the old double-lock
// hung every real boot at the governance push).
func TestAirGapAccessorsDoNotDeadlock(t *testing.T) {
	p := &Provider{}
	done := make(chan struct{})
	go func() {
		_ = p.AirGap()
		_ = p.Awareness()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("DEADLOCK REPRODUCED: AirGap()/Awareness() did not return")
	}
}

// P0-2: the directive executor must operate on the SHARED durable board
// (boot calls SetChangeBoard; ChangeBoard() must return that instance,
// never a fresh one).
func TestChangeBoardSharedWithDirectiveExecutor(t *testing.T) {
	p := &Provider{}
	shared := changeboard.NewBoard()
	p.SetChangeBoard(shared)
	if p.ChangeBoard() != shared {
		t.Fatal("ChangeBoard() must return the installed shared board")
	}
	// Approve on the shared board is visible through the provider path.
	sub, _ := shared.Submit(&changeboard.Submission{
		SubmissionID: "sub-p0", RepositoryID: "r", RiskClass: changeboard.RiskCrypto,
	})
	_ = p.ChangeBoard().Approve(sub.SubmissionID, "rev", "ok", time.Now().UnixMilli())
	got, ok := shared.Get(sub.SubmissionID)
	if !ok || !got.IsApproved() {
		t.Fatal("directive-path approval must mutate the shared board")
	}
}

// P0-3: CHANGESET_NACK clears the pending turn paths (no retry storm)
// and does not crash the pump.
func TestChangeSetNackClearsPendingPaths(t *testing.T) {
	p := &Provider{}
	p.provTurnPaths = map[string]bool{"a.go": true, "b.go": true}
	nack, _ := json.Marshal(map[string]string{"error": "change freeze active", "freeze_reason": "quarter close"})
	p.dispatchRecord(nil, &dariproto.Record{Kind: dariproto.KindMessage, MessageType: 0x0704, Payload: nack})
	p.mu.Lock()
	remaining := len(p.provTurnPaths)
	p.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("pending paths after NACK = %d, want 0", remaining)
	}
}

// P0-4: the D1 ack path unblocks the gate locally.
func TestAcknowledgeGovernanceAckUnblocksGate(t *testing.T) {
	p := &Provider{userID: "u", harnessID: "h"}
	gates := workflow.NewGatesClient("org-p0", "1.0.0", "stable")
	gates.SetAcknowledgements([]workflow.AcknowledgementRequirement{
		{OrganizationID: "org-p0", PolicyEpochID: "rule-p0", SummaryKo: "변경 통제 정책", Blocking: true},
	})
	p.SetGovernedGates(gates)
	if err := p.AcknowledgeGovernanceAck("rule-p0"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	// The gate now allows dispatch.
	if dec := gates.CheckDispatch("tool_use", "repo", "model"); !dec.Allow {
		t.Fatalf("after ack, dispatch blocked: %+v", dec)
	}
}

// C2 (code review): patty.toml [sovereign] must actually enable the
// air-gap even when the airgap was initialized before the config
// arrived — SetSovereignConfig is authoritative, never fail-open.
func TestSovereignConfigAuthoritative(t *testing.T) {
	p := &Provider{}
	// Simulate the bad ordering: airgap initialized BEFORE config.
	_ = p.AirGap()
	if p.AirGap().IsEnabled() {
		t.Fatal("precondition: airgap should start disabled")
	}
	p.SetSovereignConfig(true, []string{"mirror.internal"})
	if !p.AirGap().IsEnabled() {
		t.Fatal("sovereign config must apply even after prior initialization")
	}
	if p.AirGap().AllowsDial("mirror.internal") != true || p.AirGap().AllowsDial("evil.example") != false {
		t.Fatal("allowlist must be enforced from config")
	}
}

// M3 (code review): nested MarshalCBOR is canonical — map-key order
// must not leak Go map iteration order (byte-matches the root kernel).
func TestMarshalCBORCanonicalMapOrder(t *testing.T) {
	m1 := map[int][]byte{13: []byte("idem"), 7: []byte("peer")}
	m2 := map[int][]byte{7: []byte("peer"), 13: []byte("idem")}
	b1, err1 := dariproto.MarshalCBOR(m1)
	b2, err2 := dariproto.MarshalCBOR(m2)
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if string(b1) != string(b2) {
		t.Fatalf("non-canonical map encoding:\n%x\n%x", b1, b2)
	}
}

// R2-H1 (code review round 2): replacing or ending a stream while the
// pump emits must never send on a closed channel — send/kill are
// mutex-guarded on the stream itself, and kill is idempotent.
func TestStreamReplaceAndKillNeverPanics(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()
	out1 := make(chan provider.Chunk, 1)
	p.registerStream(out1, ctx)
	if !p.emit(provider.Chunk{Type: provider.ChunkText, Text: "x"}) {
		t.Fatal("live stream must accept a chunk")
	}
	// Replacing the subscription closes out1 (a consumer ranging it
	// must terminate), and a concurrent emit target can never panic.
	out2 := make(chan provider.Chunk, 1)
	p.registerStream(out2, ctx)
	drained := 0
	for range out1 { // buffered chunk first, then closure
		drained++
	}
	if drained != 1 {
		t.Fatalf("replaced stream delivered %d chunks, want the 1 buffered", drained)
	}
	p.endStream()
	if _, open := <-out2; open {
		t.Fatal("ended stream channel must be closed")
	}
	p.endStream() // idempotent — no double-close panic
	// Emit on an idle pump is a no-op success (control-plane only).
	if !p.emit(provider.Chunk{Type: provider.ChunkDone}) {
		t.Fatal("idle emit must be a no-op success")
	}
	// A killed stream rejects further sends without panicking.
	st := &streamState{out: make(chan provider.Chunk, 1), ctx: ctx}
	st.kill()
	st.kill()
	if st.send(provider.Chunk{Type: provider.ChunkDone}) {
		t.Fatal("send after kill must report false")
	}
}
