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
	"errors"
	"fmt"
	"sync"
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
// cross-repo digest stays stable. The format matches the relay's
// `internal/provenance/service.go::computeChangeSetDigest` body:
// `sessionID|repositoryID|branch|filesChanged|diffSummary|modelPackageID|endpointID`.
func (c *ChangeSetEnvelope) SignBytes() []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		c.SessionID, c.RepositoryID, c.Branch, joinStrings(c.FilesChanged, ","),
		c.DiffSummary, c.ModelPackageID, c.EndpointID)
	return []byte(data)
}

// Digest computes the ChangeSet's content-addressed digest and
// stores it back into the envelope. This digest is what the
// connector pushes into the envelope's ChangeSetDigest field.
func (c *ChangeSetEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte("DARI-PROVENANCE-CHANGESET-v1\x00"))
	h.Write([]byte("digest"))
	h.Write(c.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	c.ChangeSetDigest = d
	return d
}

// SignBytes returns the canonical signing bytes for the span. The
// ASTFingerprint binds the span to a specific code region; the
// SemanticFingerprint binds it to a logical symbol so the span
// survives file rename/move (PRD §19.4). The format matches the
// relay's `computeSpanDigest` body:
// `repositoryID|filePath|commitSHA|startLine-endLine|attributionState|sessionID`.
func (s *ProvenanceSpanEnvelope) SignBytes() []byte {
	data := fmt.Sprintf("%s|%s|%s|%d-%d|%s|%s",
		s.RepositoryID, s.FilePath, s.CommitSHA,
		s.StartLine, s.EndLine, string(s.AttributionState), s.SessionID)
	return []byte(data)
}

// Digest computes and stores the span's content-addressed digest.
func (s *ProvenanceSpanEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte("DARI-PROVENANCE-SPAN-v1\x00"))
	h.Write([]byte("digest"))
	h.Write(s.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	s.SpanDigest = d
	return d
}

// SignBytes returns the canonical signing bytes for the action.
// The format matches the relay's `computeEnvelopeDigest` body:
// `actionID|organizationID|sessionID|exchangeID|userID|harnessID|
// modelPackageID|endpointID|actionType|actionPayload|occurredAtUnixMs`.
func (a *ActionEnvelope) SignBytes() []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d",
		a.ActionID, a.OrganizationID, a.SessionID, a.ExchangeID,
		a.UserID, a.HarnessID, a.ModelPackageID, a.EndpointID,
		a.ActionType, a.ActionPayload, a.OccurredAtUnixMs)
	return []byte(data)
}

// Digest computes and stores the action's content-addressed digest.
func (a *ActionEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte("DARI-PROVENANCE-ACTION-v1\x00"))
	h.Write([]byte("digest"))
	h.Write(a.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	a.EnvelopeDigest = d
	return d
}

// SignBytes returns the canonical signing bytes for the commit
// binding. The binding digest is what the relay stores.
func (c *CommitBindingEnvelope) SignBytes() []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%d",
		c.RepositoryID, c.CommitSHA, c.ChangeSetID, c.Branch, c.BoundAtUnixMs)
	return []byte(data)
}

// Digest computes and stores the binding's content-addressed digest.
func (c *CommitBindingEnvelope) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte("DARI-PROVENANCE-COMMIT-BINDING-v1\x00"))
	h.Write([]byte("digest"))
	h.Write(c.SignBytes())
	var d [32]byte
	copy(d[:], h.Sum(nil))
	c.BindingDigest = d
	return d
}

// joinStrings joins a slice of strings with the supplied separator.
// The connector's `filesChanged` field is rendered as a
// comma-separated string to match the relay's `FilesChanged`
// column (a JSON array stored as a string in the GORM model).
func joinStrings(values []string, sep string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
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
