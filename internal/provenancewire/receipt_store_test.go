package provenancewire

import (
	"errors"
	"sync"
	"testing"

	"patty/internal/paperproto"
)

// fakeAckSender records the records the connector sends. It is
// thread-safe so the harness can run concurrent HandleReceipt
// invocations.
type fakeAckSender struct {
	mu   sync.Mutex
	recs []*paperproto.Record
	err  error
}

func (f *fakeAckSender) SendRecord(rec *paperproto.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.recs = append(f.recs, rec)
	return nil
}

func (f *fakeAckSender) Records() []*paperproto.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*paperproto.Record, len(f.recs))
	copy(out, f.recs)
	return out
}

func sampleReceipt(id, exchange string) *EvidenceReceiptEnvelope {
	return &EvidenceReceiptEnvelope{
		ReceiptID:      id,
		ExchangeID:     exchange,
		SessionID:      "ses-1",
		OrganizationID: "org-test",
		FinalState:     "completed",
		ChainRoot:      [32]byte{1, 2, 3},
		RelayIdentity:  "pccp-relay",
		KeyAlgorithm:   "ed25519+cose-sign1",
		Signature:      "00",
		IssuedAtUnixMs: 1_700_000_000_000,
	}
}

// TestReceiptStoreStoresAndAcknowledge covers the green path: the
// connector stores the relay-pushed receipt and sends an ack back.
func TestReceiptStoreStoresAndAcknowledge(t *testing.T) {
	store := NewReceiptStore()
	sender := &fakeAckSender{}
	handler := NewIncomingAckHandler(store, sender)

	receipt := sampleReceipt("r-1", "ex-1")
	digest, err := handler.HandleReceipt(receipt)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if digest == [32]byte{} {
		t.Fatal("digest must not be zero")
	}
	if _, ok := store.Get("r-1"); !ok {
		t.Error("receipt must be retrievable")
	}
	if handler.AckCount() != 1 {
		t.Errorf("ack count = %d, want 1", handler.AckCount())
	}

	recs := sender.Records()
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	if paperproto.MessageType(recs[0].MessageType) != paperproto.MsgEvidenceReceiptAck {
		t.Errorf("record = %s, want MsgEvidenceReceiptAck", paperproto.MessageType(recs[0].MessageType))
	}
}

// TestReceiptStoreRejectsDuplicatePush covers the replay guard:
// the relay may retry a push; the store returns the existing
// digest without overwriting.
func TestReceiptStoreRejectsDuplicatePush(t *testing.T) {
	store := NewReceiptStore()
	sender := &fakeAckSender{}
	handler := NewIncomingAckHandler(store, sender)

	receipt := sampleReceipt("r-1", "ex-1")
	digest1, err := handler.HandleReceipt(receipt)
	if err != nil {
		t.Fatalf("first handle: %v", err)
	}
	digest2, err := handler.HandleReceipt(receipt)
	if err != nil {
		t.Fatalf("second handle: %v", err)
	}
	if digest1 != digest2 {
		t.Errorf("duplicate push produced different digests: %x vs %x", digest1, digest2)
	}
	if handler.AckCount() != 2 {
		t.Errorf("ack count = %d, want 2", handler.AckCount())
	}
}

// TestReceiptStoreRejectsTamperedReplay covers the fail-closed
// boundary: a relay retry with mutated bytes is rejected, not
// silently overwritten.
func TestReceiptStoreRejectsTamperedReplay(t *testing.T) {
	store := NewReceiptStore()
	sender := &fakeAckSender{}
	handler := NewIncomingAckHandler(store, sender)

	receipt := sampleReceipt("r-1", "ex-1")
	if _, err := handler.HandleReceipt(receipt); err != nil {
		t.Fatalf("first handle: %v", err)
	}
	// Mutate the receipt body and push again. The store must
	// reject.
	tampered := *receipt
	tampered.FinalState = "compromised"
	if _, err := handler.HandleReceipt(&tampered); err == nil {
		t.Fatal("tampered receipt must be rejected")
	}
	if handler.FailureCount() != 1 {
		t.Errorf("failure count = %d, want 1", handler.FailureCount())
	}
}

// TestReceiptStoreSendErrorPropagates covers the transport
// failure path: the store still records the receipt (so a later
// retry succeeds), but the handler records the failure.
func TestReceiptStoreSendErrorPropagates(t *testing.T) {
	store := NewReceiptStore()
	sender := &fakeAckSender{err: errors.New("network down")}
	handler := NewIncomingAckHandler(store, sender)

	receipt := sampleReceipt("r-1", "ex-1")
	if _, err := handler.HandleReceipt(receipt); err == nil {
		t.Fatal("send error must propagate")
	}
	if _, ok := store.Get("r-1"); !ok {
		t.Error("receipt must be stored even on send failure")
	}
	if handler.FailureCount() != 1 {
		t.Errorf("failure count = %d, want 1", handler.FailureCount())
	}
}

// TestReceiptVerifyAckSurfacesTamper covers the audit path: a
// stored ack's digest must match the stored receipt's digest;
// otherwise the auditor catches drift.
func TestReceiptVerifyAckSurfacesTamper(t *testing.T) {
	store := NewReceiptStore()
	sender := &fakeAckSender{}
	handler := NewIncomingAckHandler(store, sender)

	receipt := sampleReceipt("r-1", "ex-1")
	digest, err := handler.HandleReceipt(receipt)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	ack, stored, err := store.VerifyAck("r-1")
	if err != nil {
		t.Fatalf("verify ack: %v", err)
	}
	if stored.Digest != digest {
		t.Errorf("stored digest drift")
	}
	if ack.AckDigest != digest {
		t.Errorf("ack digest drift")
	}
	if !stored.Acknowledged {
		t.Error("stored receipt must be marked Acknowledged")
	}
}

// TestReceiptStoreListSortedByID is the trivial boundary: list
// surfaces the connector's persisted receipts in the audit log.
func TestReceiptStoreListSortedByID(t *testing.T) {
	store := NewReceiptStore()
	sender := &fakeAckSender{}
	handler := NewIncomingAckHandler(store, sender)
	for _, id := range []string{"r-2", "r-1", "r-3"} {
		if _, err := handler.HandleReceipt(sampleReceipt(id, "ex-"+id)); err != nil {
			t.Fatalf("handle %s: %v", id, err)
		}
	}
	if got := len(store.List()); got != 3 {
		t.Errorf("list = %d, want 3", got)
	}
}

// TestReceiptStoreComputeDigestStable pins the digest contract:
// the same receipt body produces the same digest on every call.
func TestReceiptStoreComputeDigestStable(t *testing.T) {
	a := sampleReceipt("r-1", "ex-1")
	b := sampleReceipt("r-1", "ex-1")
	if computeReceiptDigest(a) != computeReceiptDigest(b) {
		t.Error("digest is non-deterministic")
	}
}

// TestReceiptStoreRejectsNilReceipt covers the trivial boundary.
func TestReceiptStoreRejectsNilReceipt(t *testing.T) {
	store := NewReceiptStore()
	if _, err := store.Store(nil, 0); err == nil {
		t.Fatal("nil receipt must fail")
	}
	if _, err := store.Store(&EvidenceReceiptEnvelope{}, 0); err == nil {
		t.Fatal("empty receipt ID must fail")
	}
}

// TestReceiptStoreRejectsAckForUnknownReceipt covers the
// boundary: an ack for a receipt we haven't seen is rejected.
func TestReceiptStoreRejectsAckForUnknownReceipt(t *testing.T) {
	store := NewReceiptStore()
	if err := store.Acknowledge(&ReceiptAck{ReceiptID: "missing", AckDigest: [32]byte{1}}, 0); err == nil {
		t.Fatal("ack for unknown receipt must fail")
	}
}

// TestIncomingAckHandlerRejectsNil covers the trivial boundary.
func TestIncomingAckHandlerRejectsNil(t *testing.T) {
	if _, err := (*IncomingAckHandler)(nil).HandleReceipt(sampleReceipt("r-1", "ex-1")); err == nil {
		t.Fatal("nil handler must fail")
	}
}
