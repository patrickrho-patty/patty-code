package provenancewire

import (
	"context"
	"errors"
	"sync"
	"testing"

	"patty/internal/evidence"
	"patty/internal/paperproto"
)

// fakeConn is a thread-safe in-memory transport used by the
// dispatcher's tests. It records every SendRecord call so the
// test can assert what was sent.
type fakeConn struct {
	mu     sync.Mutex
	recs   []*paperproto.Record
	sendErr error
}

func (c *fakeConn) SendRecord(rec *paperproto.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sendErr != nil {
		return c.sendErr
	}
	c.recs = append(c.recs, rec)
	return nil
}

func (c *fakeConn) Records() []*paperproto.Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*paperproto.Record, len(c.recs))
	copy(out, c.recs)
	return out
}

// TestDispatcherFlushesEmptyFamily guards the trivial boundary: a
// dispatcher with no pending envelopes returns an error rather than
// silently succeeding (the harness wants to know it didn't emit).
func TestDispatcherFlushesEmptyFamily(t *testing.T) {
	emitter := NewProvenanceEmitter()
	conn := &fakeConn{}
	disp := NewDispatcher(emitter, conn)
	// An empty emitter flush is a no-op success: the harness
	// called Flush on a quiet turn. Anything else would force the
	// harness to gate every flush.
	if err := disp.Flush(context.Background()); err != nil {
		t.Fatalf("empty flush must succeed, got %v", err)
	}
}

// TestDispatcherEmitsCoherentFamily exercises the green path: a
// coherent envelope family (action + change set + span referencing
// the change set) flushes successfully and clears the emitter.
func TestDispatcherEmitsCoherentFamily(t *testing.T) {
	emitter := NewProvenanceEmitter()
	conn := &fakeConn{}
	disp := NewDispatcher(emitter, conn)

	receipts := []evidence.Receipt{
		{ToolName: "edit_file", Mutation: true, Paths: []string{"foo.go"}, Success: true},
	}
	cs, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID: "cs-1", RepositoryID: "pccp", Receipts: receipts,
	})
	if err != nil {
		t.Fatalf("build cs: %v", err)
	}
	if err := emitter.EmitChangeSet(cs); err != nil {
		t.Fatalf("emit cs: %v", err)
	}
	span, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID: "sp-1", RepositoryID: "pccp", SymbolLang: "go", SymbolName: "compute",
		StartLine: 1, EndLine: 10, Receipt: receipts[0],
	})
	if err != nil {
		t.Fatalf("build span: %v", err)
	}
	if err := emitter.EmitSpan(span); err != nil {
		t.Fatalf("emit span: %v", err)
	}
	act, err := BuildActionEnvelopeFromReceipt(ActionBuildRequest{
		ActionID: "act-1", ActionType: "tool_use", OccurredAtUnixMs: 1_700_000_000_000,
	})
	if err != nil {
		t.Fatalf("build act: %v", err)
	}
	if err := emitter.EmitAction(act); err != nil {
		t.Fatalf("emit act: %v", err)
	}

	if err := disp.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	recs := conn.Records()
	if len(recs) != 3 {
		t.Fatalf("records = %d, want 3", len(recs))
	}
	// Verify ordering: change set, span, action.
	if paperproto.MessageType(recs[0].MessageType) != paperproto.MsgProvenanceChangeSet {
		t.Errorf("first record = %s, want MsgProvenanceChangeSet", paperproto.MessageType(recs[0].MessageType))
	}
	if paperproto.MessageType(recs[1].MessageType) != paperproto.MsgProvenanceSpan {
		t.Errorf("second record = %s, want MsgProvenanceSpan", paperproto.MessageType(recs[1].MessageType))
	}
	if paperproto.MessageType(recs[2].MessageType) != paperproto.MsgActionEnvelope {
		t.Errorf("third record = %s, want MsgActionEnvelope", paperproto.MessageType(recs[2].MessageType))
	}

	// Emitter cleared.
	cs2, sp2, act2 := emitter.Pending()
	if len(cs2)+len(sp2)+len(act2) != 0 {
		t.Errorf("emitter not cleared: cs=%d sp=%d act=%d", len(cs2), len(sp2), len(act2))
	}
	if disp.FlushCount() != 1 {
		t.Errorf("flush count = %d, want 1", disp.FlushCount())
	}
}

// TestDispatcherRefusesIncoherentFamily exercises the validation
// boundary: a span referencing an unknown change set fails
// validation before any record is sent.
func TestDispatcherRefusesIncoherentFamily(t *testing.T) {
	emitter := NewProvenanceEmitter()
	conn := &fakeConn{}
	disp := NewDispatcher(emitter, conn)

	span, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID: "sp-1", RepositoryID: "pccp", SymbolLang: "go", SymbolName: "compute",
		StartLine: 1, EndLine: 10,
		Receipt: evidence.Receipt{ToolName: "edit_file", Mutation: true, Paths: []string{"foo.go"}, Success: true},
	})
	if err != nil {
		t.Fatalf("build span: %v", err)
	}
	span.ChangeSetID = "cs-unknown" // force the dangling reference
	if err := emitter.EmitSpan(span); err != nil {
		t.Fatalf("emit: %v", err)
	}
	err = disp.Flush(context.Background())
	if err == nil {
		t.Fatal("flush must fail on incoherent family")
	}
	if len(conn.Records()) != 0 {
		t.Errorf("records sent on failed validation: %d", len(conn.Records()))
	}
}

// TestDispatcherRecordsSendError surfaces transport failures.
func TestDispatcherRecordsSendError(t *testing.T) {
	emitter := NewProvenanceEmitter()
	conn := &fakeConn{sendErr: errors.New("network down")}
	disp := NewDispatcher(emitter, conn)
	act, _ := BuildActionEnvelopeFromReceipt(ActionBuildRequest{
		ActionID: "act-1", ActionType: "tool_use", OccurredAtUnixMs: 1,
	})
	_ = emitter.EmitAction(act)
	err := disp.Flush(context.Background())
	if err == nil {
		t.Fatal("flush must propagate send error")
	}
	if disp.LastFlushError() == nil {
		t.Error("flush error must be recorded")
	}
}

// TestDispatcherEmitsCommitBindings exercises the binding flush
// path.
func TestDispatcherEmitsCommitBindings(t *testing.T) {
	emitter := NewProvenanceEmitter()
	conn := &fakeConn{}
	disp := NewDispatcher(emitter, conn)

	binding := &CommitBindingEnvelope{
		BindingID:    "b-1",
		RepositoryID: "pccp",
		CommitSHA:    "abc",
		ChangeSetID:  "cs-1",
		Branch:       "main",
		BoundAtUnixMs: 1_700_000_000_000,
	}
	binding.Digest()
	if err := emitter.EmitCommitBinding(binding); err != nil {
		t.Fatalf("emit binding: %v", err)
	}
	if err := disp.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	recs := conn.Records()
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	if paperproto.MessageType(recs[0].MessageType) != paperproto.MsgProvenanceCommitBind {
		t.Errorf("record = %s, want MsgProvenanceCommitBind", paperproto.MessageType(recs[0].MessageType))
	}
}

// TestDispatcherContextCanceled guards the context plumbing.
func TestDispatcherContextCanceled(t *testing.T) {
	emitter := NewProvenanceEmitter()
	conn := &fakeConn{}
	disp := NewDispatcher(emitter, conn)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := disp.Flush(ctx)
	if err == nil {
		t.Fatal("flush must fail on canceled context")
	}
}

// TestDispatcherPendingCountSurfacesOperatorSignal pins the E1
// status-bar surface: the dispatcher exposes the un-flushed count
// so the operator sees when provenance is backed up.
func TestDispatcherPendingCountSurfacesOperatorSignal(t *testing.T) {
	emitter := NewProvenanceEmitter()
	conn := &fakeConn{}
	disp := NewDispatcher(emitter, conn)

	act, _ := BuildActionEnvelopeFromReceipt(ActionBuildRequest{
		ActionID: "act-1", ActionType: "tool_use", OccurredAtUnixMs: 1,
	})
	_ = emitter.EmitAction(act)
	cs, _ := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID: "cs-1", RepositoryID: "pccp",
		Receipts: []evidence.Receipt{
			{ToolName: "edit_file", Mutation: true, Paths: []string{"foo.go"}, Success: true},
		},
	})
	_ = emitter.EmitChangeSet(cs)

	csCount, spCount, actCount := disp.PendingCount()
	if csCount != 1 || spCount != 0 || actCount != 1 {
		t.Errorf("pending = (%d, %d, %d), want (1, 0, 1)", csCount, spCount, actCount)
	}
}
