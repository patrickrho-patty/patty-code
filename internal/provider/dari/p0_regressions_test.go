package dari

import (
	"encoding/json"

	"patty/internal/dariproto"
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
	p.dispatchRecord(&dariproto.Record{Kind: dariproto.KindMessage, MessageType: 0x0704, Payload: nack})
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
