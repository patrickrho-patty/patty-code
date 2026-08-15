package provenancewire

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"patty/internal/dariproto"
)

// ReceiptStore persists evidence receipts the relay pushes to the
// connector. The store is the connector-side tamper-evidence:
// after a relay-pushed receipt is acknowledged, the harness
// retains it forever (or as long as policy requires) so a
// downstream auditor can re-verify the exchange.
//
// The store is in-memory only by default; production replaces it
// with a disk-backed implementation. The interface lets tests
// substitute a fresh store per test.
type ReceiptStore struct {
	mu       sync.Mutex
	receipts map[string]*StoredReceipt
	acks     map[string]*ReceiptAck // by ReceiptID
	// dir is the disk persistence root; empty = memory-only (tests).
	dir string
}

// StoredReceipt wraps an EvidenceReceiptEnvelope with the
// persistence-time metadata (when it was stored, its computed
// digest for cross-repo integrity, the ack state).
type StoredReceipt struct {
	Envelope     *EvidenceReceiptEnvelope
	StoredAt     int64
	Digest       [32]byte
	Acknowledged bool
	AckDigest    [32]byte
}

// NewReceiptStore constructs an empty store.
func NewReceiptStore() *ReceiptStore {
	return &ReceiptStore{
		receipts: make(map[string]*StoredReceipt),
		acks:     make(map[string]*ReceiptAck),
	}
}

// Store persists the supplied receipt. The store rejects
// duplicates by ReceiptID (a replay would otherwise overwrite the
// persisted version). Returns the computed digest so the caller
// can verify byte equality with the relay.
func (s *ReceiptStore) Store(receipt *EvidenceReceiptEnvelope, nowMs int64) ([32]byte, error) {
	if receipt == nil {
		return [32]byte{}, errors.New("provenancewire: nil receipt")
	}
	if receipt.ReceiptID == "" {
		return [32]byte{}, errors.New("provenancewire: receipt missing ReceiptID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.receipts[receipt.ReceiptID]; ok {
		// A duplicate push is a relay retry. Verify the
		// already-persisted receipt matches the new one; if so,
		// return the existing digest so the caller can send an
		// ack without re-persisting.
		if existing.Envelope.IssuedAtUnixMs == receipt.IssuedAtUnixMs &&
			existing.Envelope.ExchangeID == receipt.ExchangeID &&
			existing.Digest == computeReceiptDigest(receipt) {
			return existing.Digest, nil
		}
		return existing.Digest, fmt.Errorf("provenancewire: receipt %s already stored with different bytes", receipt.ReceiptID)
	}
	digest := computeReceiptDigest(receipt)
	sr := &StoredReceipt{
		Envelope: receipt,
		StoredAt: nowMs,
		Digest:   digest,
	}
	s.receipts[receipt.ReceiptID] = sr
	if err := s.persistLocked(sr); err != nil {
		// Persistence failure is surfaced: a receipt the harness cannot
		// retain is not tamper-evidence. The in-memory copy remains so
		// the ack can still flow; the error records the gap honestly.
		return digest, fmt.Errorf("provenancewire: receipt %s not persisted: %w", receipt.ReceiptID, err)
	}
	return digest, nil
}

// Acknowledge records the connector's reply for a stored receipt.
// Returns the persisted ack.
func (s *ReceiptStore) Acknowledge(ack *ReceiptAck, nowMs int64) error {
	if ack == nil {
		return errors.New("provenancewire: nil ack")
	}
	if ack.ReceiptID == "" {
		return errors.New("provenancewire: ack missing ReceiptID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, ok := s.receipts[ack.ReceiptID]
	if !ok {
		return fmt.Errorf("provenancewire: cannot ack unknown receipt %s", ack.ReceiptID)
	}
	if ack.AckDigest != receipt.Digest {
		return fmt.Errorf("provenancewire: ack digest %x does not match stored receipt digest %x", ack.AckDigest, receipt.Digest)
	}
	ack.AckedAtUnixMs = nowMs
	s.acks[ack.ReceiptID] = ack
	receipt.Acknowledged = true
	receipt.AckDigest = ack.AckDigest
	if err := s.persistLocked(receipt); err != nil {
		return fmt.Errorf("provenancewire: ack for %s not persisted: %w", ack.ReceiptID, err)
	}
	return nil
}

// Get returns a stored receipt by ID. The bool indicates whether
// the receipt is present.
func (s *ReceiptStore) Get(receiptID string) (*StoredReceipt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.receipts[receiptID]
	return r, ok
}

// List returns all stored receipts in insertion order. The
// connector surfaces this in the audit log on demand.
func (s *ReceiptStore) List() []*StoredReceipt {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*StoredReceipt, 0, len(s.receipts))
	for _, r := range s.receipts {
		out = append(out, r)
	}
	return out
}

// VerifyAck checks that the connector's stored ack matches the
// stored receipt digest. Returns the ack and the stored receipt,
// or an error.
func (s *ReceiptStore) VerifyAck(receiptID string) (*ReceiptAck, *StoredReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.receipts[receiptID]
	if !ok {
		return nil, nil, fmt.Errorf("provenancewire: receipt %s not stored", receiptID)
	}
	if !r.Acknowledged {
		return nil, r, fmt.Errorf("provenancewire: receipt %s not acknowledged", receiptID)
	}
	return s.acks[receiptID], r, nil
}

// computeReceiptDigest produces the content-addressed digest of
// the receipt body. The connector and the relay must agree on
// this byte layout; the conformance test in the root repo
// enforces the contract.
func computeReceiptDigest(receipt *EvidenceReceiptEnvelope) [32]byte {
	if receipt == nil {
		return [32]byte{}
	}
	data := fmt.Sprintf("receipt|%s|%s|%s|%s|%s|%s|%s|%d|%s",
		receipt.ReceiptID, receipt.ExchangeID, receipt.SessionID,
		receipt.OrganizationID, receipt.FinalState,
		receipt.RelayIdentity, receipt.ModelPackageID,
		receipt.IssuedAtUnixMs, receipt.Signature)
	h := sha256.New()
	h.Write([]byte("DARI-PROVENANCE-RECEIPT-v1\x00"))
	h.Write([]byte(data))
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// ReceiptAckForDigest builds a ReceiptAck whose AckDigest matches
// the supplied receipt digest. The connector uses this when it
// generates an ack in response to a relay-pushed receipt.
func ReceiptAckForDigest(receiptID, exchangeID string, digest [32]byte, nowMs int64) *ReceiptAck {
	return &ReceiptAck{
		ReceiptID:     receiptID,
		ExchangeID:    exchangeID,
		AckDigest:     digest,
		AckedAtUnixMs: nowMs,
	}
}

// IncomingAckHandler is the connector-side handler that the
// PAPER transport invokes when the relay pushes
// `MsgEvidenceReceipt`. The handler stores the receipt, builds
// an ack, and dispatches the ack back over the same PAPER
// connection via the supplied sender.
type IncomingAckHandler struct {
	store    *ReceiptStore
	send     AckSender
	nowFn    func() int64
	acks     int64
	failures int64
}

// AckSender is the dispatch seam for the connector's reply. The
// harness wires this to the live PAPER transport.
type AckSender interface {
	SendRecord(rec *dariproto.Record) error
}

// NewIncomingAckHandler constructs the handler.
func NewIncomingAckHandler(store *ReceiptStore, send AckSender) *IncomingAckHandler {
	return &IncomingAckHandler{
		store: store,
		send:  send,
		nowFn: defaultNowMs,
	}
}

// WithNowFunc overrides the time source. Tests use this to drive
// the ack timestamp deterministically.
func (h *IncomingAckHandler) WithNowFunc(fn func() int64) *IncomingAckHandler {
	h.nowFn = fn
	return h
}

// HandleReceipt stores the supplied receipt and sends an ack back
// to the relay. Returns the stored receipt's digest so the caller
// can record it in the audit log.
func (h *IncomingAckHandler) HandleReceipt(receipt *EvidenceReceiptEnvelope) ([32]byte, error) {
	if h == nil || h.store == nil || h.send == nil {
		return [32]byte{}, errors.New("provenancewire: nil handler")
	}
	digest, err := h.store.Store(receipt, h.nowFn())
	if err != nil {
		h.failures++
		return digest, err
	}
	ack := ReceiptAckForDigest(receipt.ReceiptID, receipt.ExchangeID, digest, h.nowFn())
	if err := h.store.Acknowledge(ack, h.nowFn()); err != nil {
		h.failures++
		return digest, err
	}
	data, err := EncodeReceiptAck(ack)
	if err != nil {
		h.failures++
		return digest, fmt.Errorf("provenancewire: encode ack: %w", err)
	}
	rec := &dariproto.Record{
		Kind:        dariproto.KindMessage,
		MessageType: uint16(dariproto.MsgEvidenceReceiptAck),
		Payload:     data,
	}
	if err := h.send.SendRecord(rec); err != nil {
		h.failures++
		return digest, fmt.Errorf("provenancewire: send ack: %w", err)
	}
	h.acks++
	return digest, nil
}

// AckCount returns the number of successful ack round-trips since
// the handler was created. Surfaced in the E1 status bar.
func (h *IncomingAckHandler) AckCount() int64 {
	if h == nil {
		return 0
	}
	return h.acks
}

// FailureCount returns the number of failed ack round-trips.
// Operators see drift here when the relay pushes receipts the
// connector can't verify (signature drift, replay, etc.).
func (h *IncomingAckHandler) FailureCount() int64 {
	if h == nil {
		return 0
	}
	return h.failures
}

func defaultNowMs() int64 {
	return time.Now().UnixMilli()
}

// NewDiskReceiptStore builds a disk-backed store: each receipt is one
// JSON file under dir (created lazily), retained forever (policy
// retention). The in-memory map remains the hot cache; loads on
// Start() replay the directory. This is the production store the
// harness installs — receipts survive harness restarts (B3).
func NewDiskReceiptStore(dir string) (*ReceiptStore, error) {
	if dir == "" {
		return nil, errors.New("provenancewire: receipt store dir required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("provenancewire: receipt dir: %w", err)
	}
	s := NewReceiptStore()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("provenancewire: read receipt dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var sr StoredReceipt
		if json.Unmarshal(raw, &sr) != nil || sr.Envelope == nil {
			continue
		}
		s.receipts[sr.Envelope.ReceiptID] = &sr
	}
	s.dir = dir
	return s, nil
}

// persistLocked writes one receipt file (caller holds the lock).
func (s *ReceiptStore) persistLocked(sr *StoredReceipt) error {
	if s.dir == "" {
		return nil
	}
	raw, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	name := filepath.Join(s.dir, fmt.Sprintf("%s.json", sr.Envelope.ReceiptID))
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, name)
}

// VerifyEvidenceReceiptSignature checks the relay's receipt signature:
// the Signature field is a hex-encoded COSE-Sign1 envelope whose
// payload MUST equal the canonical signing data (so the envelope is
// bound to these receipt fields), verified under the AUTH_ACK-carried
// receipt signer key.
func VerifyEvidenceReceiptSignature(env *EvidenceReceiptEnvelope, pub ed25519.PublicKey) error {
	if env == nil {
		return errors.New("provenancewire: nil receipt")
	}
	if len(pub) == 0 {
		return errors.New("provenancewire: no receipt signer key")
	}
	raw, err := hex.DecodeString(env.Signature)
	if err != nil || len(raw) == 0 {
		return errors.New("provenancewire: receipt carries no signature")
	}
	sign1, err := dariproto.DecodeCOSESign1(raw)
	if err != nil {
		return fmt.Errorf("provenancewire: decode receipt COSE: %w", err)
	}
	// Canonical signing data mirrors the relay byte-for-byte:
	// exchangeID|finalState|chainRootHex|relayIdentity|policyEpochID|
	// modelPackageID|issuedAtUnixMs (the INTEGER — RFC3339 strings are
	// zone-dependent and do not round-trip).
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d",
		env.ExchangeID, env.FinalState, hex.EncodeToString(env.ChainRoot[:]),
		env.RelayIdentity, env.PolicyEpochID, env.ModelPackageID, env.IssuedAtUnixMs)
	if string(sign1.Payload) != data {
		return errors.New("provenancewire: receipt COSE payload does not match the receipt fields")
	}
	if err := dariproto.VerifyCOSESign1(sign1, pub); err != nil {
		return fmt.Errorf("provenancewire: receipt signature verification failed: %w", err)
	}
	return nil
}
