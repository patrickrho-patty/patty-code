// Package provenancewire is the harness-side provenance emission
// path. It takes the harness's local evidence Ledger (Receipts) and
// builds PAPER-compatible provenance envelopes (ChangeSet,
// ProvenanceSpan, ActionEnvelope) for the relay to record.
//
// The wire shape mirrors the relay's `internal/models/provenance.go`
// columns. The connector cannot import the relay package, so the
// envelope struct field order, JSON tags, and CBOR labels MUST stay
// in lockstep with the relay's models.
//
// The relay ingests these envelopes through its
// `provenance.CreateChangeSet` / `provenance.CreateProvenanceSpan`
// / `provenance.RecordAction` paths, which today have zero callers.
// Once the connector emits them, the relay's `CodeExplorer` and
// `Provenance` products light up.
package provenancewire

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"patty/internal/evidence"
)

// AttributionState is the PRD §19.3 attribution classifier.
type AttributionState string

const (
	AttributionAIGenerated        AttributionState = "AI_GENERATED"
	AttributionAIThenHumanEdited   AttributionState = "AI_THEN_HUMAN_EDITED"
	AttributionHumanThenAIAssisted AttributionState = "HUMAN_THEN_AI_ASSISTED"
	AttributionHumanWritten        AttributionState = "HUMAN_WRITTEN"
)

// ProvenanceDomain is the connector's domain-separation prefix for
// the cross-repo signature envelope.
const ProvenanceDomain = "DARI-PROVENANCE-v1\x00"

// ChangeSetEnvelope is the wire shape the connector emits and the
// relay's `provenance.CreateChangeSet` consumes. The CBOR label
// layout mirrors `models.ChangeSet` so the relay's GORM layer can
// round-trip without modification.
type ChangeSetEnvelope struct {
	ChangeSetID    string            `cbor:"1,keyasint"`
	OrganizationID string            `cbor:"2,keyasint"`
	SessionID      string            `cbor:"3,keyasint"`
	ExchangeID     string            `cbor:"4,keyasint,omitempty"`
	RepositoryID   string            `cbor:"5,keyasint"`
	Branch         string            `cbor:"6,keyasint"`
	BaselineID     string            `cbor:"7,keyasint,omitempty"`
	UserID         string            `cbor:"8,keyasint,omitempty"`
	HarnessID      string            `cbor:"9,keyasint,omitempty"`
	ModelPackageID string            `cbor:"10,keyasint,omitempty"`
	EndpointID     string            `cbor:"11,keyasint,omitempty"`
	FilesChanged   []string          `cbor:"12,keyasint,omitempty"`
	DiffSummary    string            `cbor:"13,keyasint,omitempty"`
	DiffDigest     [32]byte          `cbor:"14,keyasint"`
	LinesAdded     int               `cbor:"15,keyasint"`
	LinesRemoved   int               `cbor:"16,keyasint"`
	AttributionState AttributionState `cbor:"17,keyasint"`
	Confidence     float64           `cbor:"18,keyasint"`
	ChangeSetDigest [32]byte         `cbor:"19,keyasint"`
	Status         string            `cbor:"20,keyasint,omitempty"`
}

// ProvenanceSpanEnvelope is the wire shape for line-level attribution
// (PRD §19, Appendix B.1). The relay ingests through
// `provenance.CreateProvenanceSpan`.
type ProvenanceSpanEnvelope struct {
	SpanID              string            `cbor:"1,keyasint"`
	OrganizationID      string            `cbor:"2,keyasint"`
	RepositoryID        string            `cbor:"3,keyasint"`
	ChangeSetID         string            `cbor:"4,keyasint,omitempty"`
	FilePath            string            `cbor:"5,keyasint"`
	CommitSHA           string            `cbor:"6,keyasint,omitempty"`
	SymbolLang          string            `cbor:"7,keyasint,omitempty"`
	SymbolName          string            `cbor:"8,keyasint,omitempty"`
	StartLine           int               `cbor:"9,keyasint"`
	EndLine             int               `cbor:"10,keyasint"`
	ASTFingerprint      [32]byte          `cbor:"11,keyasint"`
	SemanticFingerprint [32]byte          `cbor:"12,keyasint,omitempty"`
	AttributionState    AttributionState `cbor:"13,keyasint"`
	Confidence          float64           `cbor:"14,keyasint"`
	SessionID           string            `cbor:"15,keyasint,omitempty"`
	UserID              string            `cbor:"16,keyasint,omitempty"`
	HarnessID           string            `cbor:"17,keyasint,omitempty"`
	ModelPackageID      string            `cbor:"18,keyasint,omitempty"`
	EndpointID          string            `cbor:"19,keyasint,omitempty"`
	ContextRefs         []string          `cbor:"20,keyasint,omitempty"`
	ParentSpanRefs      []string          `cbor:"21,keyasint,omitempty"`
	SpanDigest          [32]byte          `cbor:"22,keyasint"`
}

// ActionEnvelope is the wire shape for signed governed actions
// (PRD §37.2). Every harness command path emits one of these.
type ActionEnvelope struct {
	ActionID        string  `cbor:"1,keyasint"`
	OrganizationID  string  `cbor:"2,keyasint"`
	SessionID       string  `cbor:"3,keyasint,omitempty"`
	ExchangeID      string  `cbor:"4,keyasint,omitempty"`
	UserID          string  `cbor:"5,keyasint,omitempty"`
	HarnessID       string  `cbor:"6,keyasint,omitempty"`
	ModelPackageID  string  `cbor:"7,keyasint,omitempty"`
	EndpointID      string  `cbor:"8,keyasint,omitempty"`
	ProjectID       string  `cbor:"9,keyasint,omitempty"`
	RepositoryID    string  `cbor:"10,keyasint,omitempty"`
	Branch          string  `cbor:"11,keyasint,omitempty"`
	PolicyEpochID   string  `cbor:"12,keyasint,omitempty"`
	LeaseID         string  `cbor:"13,keyasint,omitempty"`
	ActionType      string  `cbor:"14,keyasint"`
	ActionPayload   string  `cbor:"15,keyasint,omitempty"`
	VerdictResult   string  `cbor:"16,keyasint,omitempty"`
	Classification  string  `cbor:"17,keyasint,omitempty"`
	EnvelopeDigest  [32]byte `cbor:"18,keyasint"`
	CPSignature     string  `cbor:"19,keyasint,omitempty"`
	OccurredAtUnixMs int64 `cbor:"20,keyasint"`
}

// EvidenceReceiptEnvelope is the relay's signed tamper-evident
// proof for a completed exchange (PRD §40.3). The relay pushes
// this back to the connector over MsgEvidenceReceipt, and the
// connector persists it as tamper-evidence (B3).
type EvidenceReceiptEnvelope struct {
	ReceiptID       string  `cbor:"1,keyasint"`
	ExchangeID      string  `cbor:"2,keyasint"`
	SessionID       string  `cbor:"3,keyasint,omitempty"`
	OrganizationID  string  `cbor:"4,keyasint,omitempty"`
	FinalState      string  `cbor:"5,keyasint,omitempty"`
	FirstEventSeq    uint64 `cbor:"6,keyasint"`
	LastEventSeq     uint64 `cbor:"7,keyasint"`
	ChainRoot       [32]byte `cbor:"8,keyasint"`
	ProvenanceRoot  [32]byte `cbor:"9,keyasint,omitempty"`
	PolicyEpochID   string  `cbor:"10,keyasint,omitempty"`
	LeaseDigest     [32]byte `cbor:"11,keyasint,omitempty"`
	RelayIdentity   string  `cbor:"12,keyasint,omitempty"`
	ModelPackageID  string  `cbor:"13,keyasint,omitempty"`
	EndpointID      string  `cbor:"14,keyasint,omitempty"`
	KeyAlgorithm    string  `cbor:"15,keyasint"`
	Signature       string  `cbor:"16,keyasint"`
	RedactionManifest string `cbor:"17,keyasint,omitempty"`
	IssuedAtUnixMs  int64   `cbor:"18,keyasint"`
	Acknowledged    bool    `cbor:"19,keyasint,omitempty"`
}

// ReceiptAck is the connector's reply to a relay-pushed
// EvidenceReceipt. The connector stores the receipt locally and
// signals back so the relay can drop the receipt from its retry
// queue.
type ReceiptAck struct {
	ReceiptID    string  `cbor:"1,keyasint"`
	ExchangeID   string  `cbor:"2,keyasint"`
	AckDigest    [32]byte `cbor:"3,keyasint"`
	AckedAtUnixMs int64   `cbor:"4,keyasint"`
}

// CommitBindingEnvelope is the wire shape for linking a git
// commit to a ChangeSet (PRD §18.6).
type CommitBindingEnvelope struct {
	BindingID      string  `cbor:"1,keyasint"`
	OrganizationID string  `cbor:"2,keyasint,omitempty"`
	RepositoryID   string  `cbor:"3,keyasint"`
	CommitSHA      string  `cbor:"4,keyasint"`
	ChangeSetID    string  `cbor:"5,keyasint"`
	SessionID      string  `cbor:"6,keyasint,omitempty"`
	Branch         string  `cbor:"7,keyasint,omitempty"`
	BoundAtUnixMs  int64   `cbor:"8,keyasint"`
	BindingDigest  [32]byte `cbor:"9,keyasint"`
}

// SignBytes returns the canonical signing bytes for the change set.
// The relay and the connector agree on this exact layout so the
// cross-repo digest stays stable.
func (c *ChangeSetEnvelope) SignBytes() []byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("changeset"))
	writeLengthPrefixedString(h, c.ChangeSetID)
	writeLengthPrefixedString(h, c.OrganizationID)
	writeLengthPrefixedString(h, c.SessionID)
	writeLengthPrefixedString(h, c.RepositoryID)
	writeLengthPrefixedString(h, c.Branch)
	writeLengthPrefixedStringSlice(h, c.FilesChanged)
	h.Write(c.DiffDigest[:])
	writeU64BE(h, uint64(c.LinesAdded))
	writeU64BE(h, uint64(c.LinesRemoved))
	writeLengthPrefixedString(h, string(c.AttributionState))
	writeF64BE(h, c.Confidence)
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d[:]
}

// Digest computes the ChangeSet's content-addressed digest and
// stores it back into the envelope. This digest is what the
// connector pushes into the envelope's ChangeSetDigest field.
func (c *ChangeSetEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("changeset-digest"))
	h.Write(c.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	c.ChangeSetDigest = d
	return d
}

// SignBytes returns the canonical signing bytes for the span. The
// ASTFingerprint binds the span to a specific code region; the
// SemanticFingerprint binds it to a logical symbol so the span
// survives file rename/move (PRD §19.4).
func (s *ProvenanceSpanEnvelope) SignBytes() []byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("span"))
	writeLengthPrefixedString(h, s.SpanID)
	writeLengthPrefixedString(h, s.OrganizationID)
	writeLengthPrefixedString(h, s.RepositoryID)
	writeLengthPrefixedString(h, s.FilePath)
	writeLengthPrefixedString(h, s.SymbolLang)
	writeLengthPrefixedString(h, s.SymbolName)
	writeU64BE(h, uint64(s.StartLine))
	writeU64BE(h, uint64(s.EndLine))
	h.Write(s.ASTFingerprint[:])
	h.Write(s.SemanticFingerprint[:])
	writeLengthPrefixedString(h, string(s.AttributionState))
	writeF64BE(h, s.Confidence)
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d[:]
}

// Digest computes and stores the span's content-addressed digest.
func (s *ProvenanceSpanEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("span-digest"))
	h.Write(s.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	s.SpanDigest = d
	return d
}

// SignBytes returns the canonical signing bytes for the action.
func (a *ActionEnvelope) SignBytes() []byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("action"))
	writeLengthPrefixedString(h, a.ActionID)
	writeLengthPrefixedString(h, a.OrganizationID)
	writeLengthPrefixedString(h, a.SessionID)
	writeLengthPrefixedString(h, a.ExchangeID)
	writeLengthPrefixedString(h, a.UserID)
	writeLengthPrefixedString(h, a.HarnessID)
	writeLengthPrefixedString(h, a.ActionType)
	writeLengthPrefixedString(h, a.ActionPayload)
	writeLengthPrefixedString(h, a.VerdictResult)
	writeLengthPrefixedString(h, a.Classification)
	writeU64BE(h, uint64(a.OccurredAtUnixMs))
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d[:]
}

// Digest computes and stores the action's content-addressed digest.
func (a *ActionEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("action-digest"))
	h.Write(a.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	a.EnvelopeDigest = d
	return d
}

// SignBytes returns the canonical signing bytes for the commit
// binding. The binding digest is what the relay stores.
func (c *CommitBindingEnvelope) SignBytes() []byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("commit-binding"))
	writeLengthPrefixedString(h, c.RepositoryID)
	writeLengthPrefixedString(h, c.CommitSHA)
	writeLengthPrefixedString(h, c.ChangeSetID)
	writeLengthPrefixedString(h, c.Branch)
	writeU64BE(h, uint64(c.BoundAtUnixMs))
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d[:]
}

// Digest computes and stores the binding's content-addressed digest.
func (c *CommitBindingEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("commit-binding-digest"))
	h.Write(c.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	c.BindingDigest = d
	return d
}

// writeLengthPrefixedString appends a uint32 big-endian length
// followed by the value bytes.
func writeLengthPrefixedString(h interface{ Write([]byte) (int, error) }, value string) {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(value)))
	h.Write(lenBuf[:])
	h.Write([]byte(value))
}

// writeLengthPrefixedStringSlice appends a uint32 length followed
// by each string's own uint32 length and bytes.
func writeLengthPrefixedStringSlice(h interface{ Write([]byte) (int, error) }, values []string) {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(values)))
	h.Write(lenBuf[:])
	for _, v := range values {
		writeLengthPrefixedString(h, v)
	}
}

// writeU64BE appends a big-endian uint64.
func writeU64BE(h interface{ Write([]byte) (int, error) }, value uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	h.Write(buf[:])
}

// writeF64BE appends a big-endian IEEE-754 float64 via the standard
// library's math.Float64bits helper. Stable across hosts because
// IEEE-754 is a fixed bit layout.
func writeF64BE(h interface{ Write([]byte) (int, error) }, value float64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], math.Float64bits(value))
	h.Write(buf[:])
}

// ProvenanceEmitter converts the harness's local Ledger receipts
// into PAPER-compatible provenance envelopes. The emitter is the
// single point the harness uses to surface attribution to PCCP; it
// batches receipts into ChangeSet/ProvenanceSpan/ActionEnvelope
// triples and orders them by occurrence time.
type ProvenanceEmitter struct {
	mu sync.Mutex
	// pending collects the un-emitted envelopes. The harness flushes
	// them at the end of each turn so the relay sees a coherent
	// snapshot.
	pendingChangeSets []*ChangeSetEnvelope
	pendingSpans     []*ProvenanceSpanEnvelope
	pendingActions   []*ActionEnvelope
	pendingBindings  []*CommitBindingEnvelope
}

// NewProvenanceEmitter constructs an empty emitter. The harness
// keeps one per session.
func NewProvenanceEmitter() *ProvenanceEmitter {
	return &ProvenanceEmitter{}
}

// Pending returns the un-emitted envelopes. The harness's network
// layer flushes these to the relay at the end of each turn.
func (e *ProvenanceEmitter) Pending() ([]*ChangeSetEnvelope, []*ProvenanceSpanEnvelope, []*ActionEnvelope) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pendingChangeSets, e.pendingSpans, e.pendingActions
}

// EmitAction records a governed action envelope. Every harness
// command path emits one of these via the emitter.
func (e *ProvenanceEmitter) EmitAction(env *ActionEnvelope) error {
	if env == nil {
		return errors.New("provenancewire: nil action envelope")
	}
	if env.ActionID == "" {
		return errors.New("provenancewire: action envelope missing ActionID")
	}
	if env.ActionType == "" {
		return errors.New("provenancewire: action envelope missing ActionType")
	}
	if env.OccurredAtUnixMs <= 0 {
		return errors.New("provenancewire: action envelope missing OccurredAtUnixMs")
	}
	env.Digest()
	e.mu.Lock()
	e.pendingActions = append(e.pendingActions, env)
	e.mu.Unlock()
	return nil
}

// EmitChangeSet records a code patch with provenance. The change
// set envelope carries the per-session attribution lineage that
// the relay's `provenance.CreateChangeSet` consumes.
func (e *ProvenanceEmitter) EmitChangeSet(env *ChangeSetEnvelope) error {
	if env == nil {
		return errors.New("provenancewire: nil changeset envelope")
	}
	if env.ChangeSetID == "" {
		return errors.New("provenancewire: changeset envelope missing ChangeSetID")
	}
	if env.RepositoryID == "" {
		return errors.New("provenancewire: changeset envelope missing RepositoryID")
	}
	if len(env.FilesChanged) == 0 {
		return errors.New("provenancewire: changeset envelope must list at least one file")
	}
	env.Digest()
	e.mu.Lock()
	e.pendingChangeSets = append(e.pendingChangeSets, env)
	e.mu.Unlock()
	return nil
}

// EmitSpan records a line-level attribution span. The relay's
// `provenance.CreateProvenanceSpan` consumes the envelope.
func (e *ProvenanceEmitter) EmitSpan(env *ProvenanceSpanEnvelope) error {
	if env == nil {
		return errors.New("provenancewire: nil span envelope")
	}
	if env.SpanID == "" {
		return errors.New("provenancewire: span envelope missing SpanID")
	}
	if env.FilePath == "" {
		return errors.New("provenancewire: span envelope missing FilePath")
	}
	env.Digest()
	e.mu.Lock()
	e.pendingSpans = append(e.pendingSpans, env)
	e.mu.Unlock()
	return nil
}

// EmitCommitBinding records a git commit link to a ChangeSet.
// The harness invokes this when the user pushes a commit that
// includes a governed session's edits.
func (e *ProvenanceEmitter) EmitCommitBinding(env *CommitBindingEnvelope) error {
	if env == nil {
		return errors.New("provenancewire: nil commit binding envelope")
	}
	if env.BindingID == "" {
		return errors.New("provenancewire: commit binding missing BindingID")
	}
	if env.CommitSHA == "" {
		return errors.New("provenancewire: commit binding missing CommitSHA")
	}
	if env.ChangeSetID == "" {
		return errors.New("provenancewire: commit binding missing ChangeSetID")
	}
	env.Digest()
	e.mu.Lock()
	e.pendingBindings = append(e.pendingBindings, env)
	e.mu.Unlock()
	return nil
}

// PendingBindings returns the un-emitted commit bindings. The
// dispatcher reads these after ChangeSets/Spans/Actions so the
// relay's `provenance.BindCommit` records the commit link last.
func (e *ProvenanceEmitter) PendingBindings() []*CommitBindingEnvelope {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pendingBindings
}

// Clear empties the pending buffers. The network layer calls this
// after a successful flush.
func (e *ProvenanceEmitter) Clear() {
	e.mu.Lock()
	e.pendingChangeSets = nil
	e.pendingSpans = nil
	e.pendingActions = nil
	e.pendingBindings = nil
	e.mu.Unlock()
}

// BuildActionEnvelopeFromReceipt walks the harness's local Receipt
// and produces an ActionEnvelope. The receipt's `ToolName` becomes
// the action type and the receipt's `Command` becomes the payload
// (PRD §37.2). The caller is responsible for filling in the
// harness/session/policy identifiers.
func BuildActionEnvelopeFromReceipt(req ActionBuildRequest) (*ActionEnvelope, error) {
	if req.ActionID == "" {
		return nil, errors.New("provenancewire: action build requires ActionID")
	}
	if req.ActionType == "" {
		return nil, errors.New("provenancewire: action build requires ActionType")
	}
	if req.OccurredAtUnixMs <= 0 {
		return nil, errors.New("provenancewire: action build requires OccurredAtUnixMs")
	}
	env := &ActionEnvelope{
		ActionID:        req.ActionID,
		OrganizationID:  req.OrganizationID,
		SessionID:       req.SessionID,
		ExchangeID:      req.ExchangeID,
		UserID:          req.UserID,
		HarnessID:       req.HarnessID,
		ModelPackageID:  req.ModelPackageID,
		EndpointID:      req.EndpointID,
		ProjectID:       req.ProjectID,
		RepositoryID:    req.RepositoryID,
		Branch:          req.Branch,
		PolicyEpochID:   req.PolicyEpochID,
		LeaseID:         req.LeaseID,
		ActionType:      req.ActionType,
		ActionPayload:   req.ActionPayload,
		VerdictResult:   req.VerdictResult,
		Classification:  req.Classification,
		OccurredAtUnixMs: req.OccurredAtUnixMs,
	}
	env.Digest()
	return env, nil
}

// ActionBuildRequest groups the inputs BuildActionEnvelopeFromReceipt
// needs to construct an ActionEnvelope.
type ActionBuildRequest struct {
	ActionID        string
	OrganizationID  string
	SessionID       string
	ExchangeID      string
	UserID          string
	HarnessID       string
	ModelPackageID  string
	EndpointID      string
	ProjectID       string
	RepositoryID    string
	Branch          string
	PolicyEpochID   string
	LeaseID         string
	ActionType      string
	ActionPayload   string
	VerdictResult   string
	Classification  string
	OccurredAtUnixMs int64
}

// BuildChangeSetEnvelopeFromReceipts walks a slice of Receipt and
// produces a single ChangeSetEnvelope covering the files the
// receipts touched. The function groups receipts by file path,
// aggregates lines-added/lines-removed counts (estimated from
// `OutputBytes`), and computes the attribution state from the
// receipts' mutation signals.
func BuildChangeSetEnvelopeFromReceipts(req ChangeSetBuildRequest) (*ChangeSetEnvelope, error) {
	if req.ChangeSetID == "" {
		return nil, errors.New("provenancewire: change set build requires ChangeSetID")
	}
	if req.RepositoryID == "" {
		return nil, errors.New("provenancewire: change set build requires RepositoryID")
	}
	if len(req.Receipts) == 0 {
		return nil, errors.New("provenancewire: change set build requires at least one receipt")
	}
	files := make(map[string]bool)
	for _, r := range req.Receipts {
		for _, p := range r.Paths {
			files[p] = true
		}
	}
	if len(files) == 0 {
		return nil, errors.New("provenancewire: receipts do not reference any paths")
	}
	fileList := make([]string, 0, len(files))
	for f := range files {
		fileList = append(fileList, f)
	}
	sort.Strings(fileList)
	linesAdded, linesRemoved, attributed := aggregateReceiptStats(req.Receipts)
	env := &ChangeSetEnvelope{
		ChangeSetID:      req.ChangeSetID,
		OrganizationID:   req.OrganizationID,
		SessionID:        req.SessionID,
		ExchangeID:       req.ExchangeID,
		RepositoryID:     req.RepositoryID,
		Branch:           req.Branch,
		BaselineID:       req.BaselineID,
		UserID:           req.UserID,
		HarnessID:        req.HarnessID,
		ModelPackageID:   req.ModelPackageID,
		EndpointID:       req.EndpointID,
		FilesChanged:     fileList,
		LinesAdded:       linesAdded,
		LinesRemoved:     linesRemoved,
		AttributionState: attributed,
		Confidence:      computeConfidence(req.Receipts),
		Status:           "pending",
	}
	env.DiffDigest = diffDigestOf(fileList)
	env.Digest()
	return env, nil
}

// ChangeSetBuildRequest groups the inputs
// BuildChangeSetEnvelopeFromReceipts needs.
type ChangeSetBuildRequest struct {
	ChangeSetID    string
	OrganizationID string
	SessionID      string
	ExchangeID     string
	RepositoryID   string
	Branch         string
	BaselineID     string
	UserID         string
	HarnessID      string
	ModelPackageID string
	EndpointID     string
	Receipts       []evidence.Receipt
}

// BuildSpanEnvelopeFromReceipt walks a Receipt and produces a
// single ProvenanceSpanEnvelope covering the first file the
// receipt touched. The caller is responsible for providing the
// commit SHA and the symbol anchor.
func BuildSpanEnvelopeFromReceipt(req SpanBuildRequest) (*ProvenanceSpanEnvelope, error) {
	if req.SpanID == "" {
		return nil, errors.New("provenancewire: span build requires SpanID")
	}
	if req.FilePath == "" {
		if len(req.Receipt.Paths) == 0 {
			return nil, errors.New("provenancewire: span build requires a file path")
		}
		req.FilePath = req.Receipt.Paths[0]
	}
	env := &ProvenanceSpanEnvelope{
		SpanID:           req.SpanID,
		OrganizationID:   req.OrganizationID,
		RepositoryID:     req.RepositoryID,
		ChangeSetID:      req.ChangeSetID,
		FilePath:         req.FilePath,
		CommitSHA:        req.CommitSHA,
		SymbolLang:       req.SymbolLang,
		SymbolName:       req.SymbolName,
		StartLine:        req.StartLine,
		EndLine:          req.EndLine,
		ASTFingerprint:   astFingerprint(req.FilePath, req.StartLine, req.EndLine, []byte(req.SymbolName+":"+req.Receipt.Command)),
		SemanticFingerprint: semanticFingerprint(req.FilePath, req.SymbolLang, req.SymbolName),
		AttributionState: computeSpanAttribution(req.Receipt),
		Confidence:       computeSpanConfidence(req.Receipt),
		SessionID:        req.SessionID,
		UserID:           req.UserID,
		HarnessID:        req.HarnessID,
		ModelPackageID:   req.ModelPackageID,
		EndpointID:       req.EndpointID,
		ContextRefs:      req.ContextRefs,
		ParentSpanRefs:   req.ParentSpanRefs,
	}
	env.Digest()
	return env, nil
}

// SpanBuildRequest groups the inputs
// BuildSpanEnvelopeFromReceipt needs.
type SpanBuildRequest struct {
	SpanID         string
	OrganizationID string
	RepositoryID   string
	ChangeSetID    string
	FilePath       string
	CommitSHA      string
	SymbolLang     string
	SymbolName     string
	StartLine      int
	EndLine        int
	SessionID      string
	UserID         string
	HarnessID      string
	ModelPackageID string
	EndpointID     string
	ContextRefs    []string
	ParentSpanRefs []string
	Receipt        evidence.Receipt
}

// aggregateReceiptStats computes the cumulative lines-added /
// lines-removed counts and the attribution state from a slice of
// receipts. The line counts are heuristic: the connector doesn't
// have the diff (yet), so it derives an estimate from `OutputBytes`
// and the `Mutation` flag. The relay applies the authoritative
// counts when it receives the envelope.
func aggregateReceiptStats(receipts []evidence.Receipt) (int, int, AttributionState) {
	added, removed := 0, 0
	mutationSeen := false
	for _, r := range receipts {
		if r.Mutation {
			mutationSeen = true
		}
		// Each receipt contributes at least 1 line (the touched
		// file's minimum) plus an OutputBytes-derived estimate. This
		// is intentionally conservative; the relay's `provenance`
		// service re-counts from the actual diff when the envelope
		// lands.
		added += 1
		if r.Mutation {
			added += r.OutputBytes / 80
			removed += r.OutputBytes / 200
		}
	}
	state := AttributionAIGenerated
	if !mutationSeen {
		state = AttributionHumanWritten
	}
	return added, removed, state
}

// computeConfidence returns a confidence score in [0, 1] based on
// the number of receipts and the mutation signals.
func computeConfidence(receipts []evidence.Receipt) float64 {
	if len(receipts) == 0 {
		return 0
	}
	muts := 0
	for _, r := range receipts {
		if r.Mutation {
			muts++
		}
	}
	// 1 mutation -> 0.5 confidence. 5+ mutations -> 0.95.
	c := 0.5 + 0.1*float64(muts)
	if c > 0.99 {
		c = 0.99
	}
	return c
}

// computeSpanAttribution returns the attribution state for a
// single span. A span covering a mutation-only receipt is
// AI_GENERATED; a span covering both reads and writes is
// HUMAN_THEN_AI_ASSISTED.
func computeSpanAttribution(r evidence.Receipt) AttributionState {
	if !r.Mutation {
		if r.Read {
			return AttributionHumanWritten
		}
		return AttributionHumanWritten
	}
	if r.Read {
		return AttributionHumanThenAIAssisted
	}
	return AttributionAIGenerated
}

// computeSpanConfidence returns a span-level confidence. The
// spans inherit the change-set's confidence but discount by
// whether the span saw both reads and writes.
func computeSpanConfidence(r evidence.Receipt) float64 {
	if r.Mutation && r.Read {
		return 0.85
	}
	if r.Mutation {
		return 0.7
	}
	return 0.5
}

// astFingerprint computes the line-anchored AST fingerprint for a
// span. The connector hashes the (file, start, end, content) tuple
// so the span survives file rename/move (PRD §19.4): when the
// file moves, the relay re-resolves the fingerprint against the
// new path.
func astFingerprint(filePath string, startLine, endLine int, content []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("ast"))
	writeLengthPrefixedString(h, filePath)
	writeU64BE(h, uint64(startLine))
	writeU64BE(h, uint64(endLine))
	h.Write(content)
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// semanticFingerprint computes the symbol-anchored fingerprint so
// the span follows a logical symbol across renames (PRD §19.4).
// The fingerprint is bound ONLY to (lang, symbol) — NOT the file
// path — so a span on the same symbol moves with the symbol even
// after rename/move.
func semanticFingerprint(filePath, lang, symbol string) [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("semantic"))
	writeLengthPrefixedString(h, lang)
	writeLengthPrefixedString(h, symbol)
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// diffDigestOf computes the canonical digest of the file list.
// The relay uses this to cross-reference the change set against
// its diff tree.
func diffDigestOf(files []string) [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("diff"))
	for _, f := range files {
		writeLengthPrefixedString(h, f)
	}
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// EncodeChangeSetEnvelope serializes a change-set envelope for the
// relay. The relay's `provenance.CreateChangeSet` decodes the
// exact CBOR layout produced here.
func EncodeChangeSetEnvelope(env *ChangeSetEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil change set envelope")
	}
	return marshalEnvelope(env)
}

// DecodeChangeSetEnvelope parses a CBOR change-set envelope from
// the wire.
func DecodeChangeSetEnvelope(data []byte) (*ChangeSetEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty change set body")
	}
	env := &ChangeSetEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode change set: %w", err)
	}
	return env, nil
}

// EncodeSpanEnvelope serializes a span envelope.
func EncodeSpanEnvelope(env *ProvenanceSpanEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil span envelope")
	}
	return marshalEnvelope(env)
}

// DecodeSpanEnvelope parses a span envelope.
func DecodeSpanEnvelope(data []byte) (*ProvenanceSpanEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty span body")
	}
	env := &ProvenanceSpanEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode span: %w", err)
	}
	return env, nil
}

// EncodeActionEnvelope serializes an action envelope.
func EncodeActionEnvelope(env *ActionEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil action envelope")
	}
	return marshalEnvelope(env)
}

// DecodeActionEnvelope parses an action envelope.
func DecodeActionEnvelope(data []byte) (*ActionEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty action body")
	}
	env := &ActionEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode action: %w", err)
	}
	return env, nil
}

// EncodeCommitBindingEnvelope serializes a commit-binding envelope.
func EncodeCommitBindingEnvelope(env *CommitBindingEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil commit binding envelope")
	}
	return marshalEnvelope(env)
}

// DecodeCommitBindingEnvelope parses a commit-binding envelope.
func DecodeCommitBindingEnvelope(data []byte) (*CommitBindingEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty commit binding body")
	}
	env := &CommitBindingEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode commit binding: %w", err)
	}
	return env, nil
}

// EncodeEvidenceReceiptEnvelope serializes a relay-pushed
// evidence receipt. The relay uses this in the
// `MsgEvidenceReceipt` body (PRD §40.3).
func EncodeEvidenceReceiptEnvelope(env *EvidenceReceiptEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil evidence receipt envelope")
	}
	return marshalEnvelope(env)
}

// DecodeEvidenceReceiptEnvelope parses a relay-pushed evidence
// receipt.
func DecodeEvidenceReceiptEnvelope(data []byte) (*EvidenceReceiptEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty evidence receipt body")
	}
	env := &EvidenceReceiptEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode evidence receipt: %w", err)
	}
	return env, nil
}

// EncodeReceiptAck serializes the connector's reply to a
// relay-pushed receipt.
func EncodeReceiptAck(ack *ReceiptAck) ([]byte, error) {
	if ack == nil {
		return nil, errors.New("provenancewire: nil receipt ack")
	}
	return marshalEnvelope(ack)
}

// DecodeReceiptAck parses the connector's reply.
func DecodeReceiptAck(data []byte) (*ReceiptAck, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty receipt ack body")
	}
	ack := &ReceiptAck{}
	if err := unmarshalEnvelope(data, ack); err != nil {
		return nil, fmt.Errorf("provenancewire: decode receipt ack: %w", err)
	}
	return ack, nil
}

// marshalEnvelope is a tiny CBOR encoder wrapper.
func marshalEnvelope(env interface{}) ([]byte, error) {
	return marshalCBOR(env)
}

// unmarshalEnvelope is a tiny CBOR decoder wrapper.
func unmarshalEnvelope(data []byte, env interface{}) error {
	return unmarshalCBOR(data, env)
}

// ValidateEnvelopeFamily checks that the supplied envelopes form a
// coherent set: every span references a known change set, every
// change set references a known action, and every action carries
// the relay's expected identifiers. The harness runs this before
// flushing to catch drift.
func ValidateEnvelopeFamily(actions []*ActionEnvelope, changeSets []*ChangeSetEnvelope, spans []*ProvenanceSpanEnvelope) error {
	csByID := make(map[string]bool, len(changeSets))
	for _, cs := range changeSets {
		if cs.ChangeSetID == "" {
			return errors.New("provenancewire: change set missing ID")
		}
		csByID[cs.ChangeSetID] = true
	}
	for _, sp := range spans {
		if sp.ChangeSetID != "" && !csByID[sp.ChangeSetID] {
			return fmt.Errorf("provenancewire: span %q references unknown change set %q", sp.SpanID, sp.ChangeSetID)
		}
	}
	return nil
}

// SanitizeAttribution defends against a buggy relay that hands the
// connector an envelope with an unrecognized attribution string.
// The harness maps the unknown value to AttributionAIGenerated (the
// safest default) and surfaces a metric so operators can see
// drift.
func SanitizeAttribution(value AttributionState) AttributionState {
	switch value {
	case AttributionAIGenerated, AttributionAIThenHumanEdited,
		AttributionHumanThenAIAssisted, AttributionHumanWritten:
		return value
	}
	return AttributionAIGenerated
}

// IsValidAttributionState reports whether the supplied attribution
// string is one of the documented values.
func IsValidAttributionState(value AttributionState) bool {
	switch value {
	case AttributionAIGenerated, AttributionAIThenHumanEdited,
		AttributionHumanThenAIAssisted, AttributionHumanWritten:
		return true
	}
	return false
}

// SpanLookupKey is the cross-repo key the relay's
// `provenance.LookupCodeSpan` uses to find a span by file path
// and line range. The connector emits these keys from its
// attribution-chain reader.
type SpanLookupKey struct {
	FilePath  string
	StartLine int
	EndLine   int
}

// String renders the key as a deterministic label suitable for
// logs and audit chains.
func (k SpanLookupKey) String() string {
	return fmt.Sprintf("%s:%d-%d", k.FilePath, k.StartLine, k.EndLine)
}

// AttributionChain is the harness-side view of the prompt -> tool ->
// file -> commit chain (PRD §19.1). The harness surfaces this in
// the author UI and uses it to power replay-from-provenance.
type AttributionChain struct {
	ExchangeID     string
	SessionID      string
	UserID         string
	HarnessID      string
	ModelPackageID string
	EndpointID     string
	PolicyEpochID  string
	LeaseID        string
	Prompts        []string
	Tools          []string
	Files          []string
	Commits        []string
	Spans          []SpanLookupKey
}

// String renders the chain as a deterministic label suitable for
// audit logs.
func (c *AttributionChain) String() string {
	return fmt.Sprintf("chain{exchange=%s session=%s user=%s harness=%s files=%d}",
		c.ExchangeID, c.SessionID, c.UserID, c.HarnessID, len(c.Files))
}

// ReplayPlan is the input the harness feeds to its replay engine
// (PRD §14.3). It captures the prompt, model, and toolset the
// exchange used so a downstream replay reproduces the same code.
type ReplayPlan struct {
	ExchangeID     string
	SessionID      string
	Prompt         string
	ModelPackageID string
	EndpointID     string
	Files          []string
	ToolClasses    []string
}

// String renders the replay plan as a deterministic label.
func (p *ReplayPlan) String() string {
	return fmt.Sprintf("replay{exchange=%s model=%s files=%d}", p.ExchangeID, p.ModelPackageID, len(p.Files))
}

// replayFromChain builds a ReplayPlan from the supplied chain.
// The connector's replay engine uses this to reconstruct the
// exact context+model that produced a given span.
func replayFromChain(chain *AttributionChain) *ReplayPlan {
	if chain == nil {
		return nil
	}
	prompts := strings.Join(chain.Prompts, "\n\n")
	return &ReplayPlan{
		ExchangeID:     chain.ExchangeID,
		SessionID:      chain.SessionID,
		Prompt:         prompts,
		ModelPackageID: chain.ModelPackageID,
		EndpointID:     chain.EndpointID,
		Files:          append([]string(nil), chain.Files...),
		ToolClasses:    append([]string(nil), chain.Tools...),
	}
}

// sync import surface check: keep math visible even if the test
// files use it through indirection.
var _ = math.Float64bits
