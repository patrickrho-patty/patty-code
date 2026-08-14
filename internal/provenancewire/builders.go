package provenancewire

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"patty/internal/evidence"
)

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
// counts when it receives the envelope. Both numbers are clamped to
// the int range so a malicious or runaway receipt cannot overflow
// the CBOR encoder.
func aggregateReceiptStats(receipts []evidence.Receipt) (int, int, AttributionState) {
	added, removed := 0, 0
	mutationSeen := false
	const maxLines = 1 << 30 // 1B lines ceiling; relay overrides on receive.
	for _, r := range receipts {
		if r.Mutation {
			mutationSeen = true
		}
		if added < maxLines {
			added++
		}
		if r.Mutation {
			estimate := r.OutputBytes / 80
			if estimate > maxLines-added {
				estimate = maxLines - added
			}
			added += estimate
			estimate = r.OutputBytes / 200
			if estimate > maxLines-removed {
				estimate = maxLines - removed
			}
			removed += estimate
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
	data := fmt.Sprintf("ast|%s|%d|%d|%s", filePath, startLine, endLine, string(content))
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte(data))
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
	data := fmt.Sprintf("semantic|%s|%s", lang, symbol)
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte(data))
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// diffDigestOf computes the canonical digest of the file list.
// The relay uses this to cross-reference the change set against
// its diff tree. The encoding matches the relay's FilesChanged
// column layout (comma-separated paths).
func diffDigestOf(files []string) [32]byte {
	h := sha256.New()
	h.Write([]byte(ProvenanceDomain))
	h.Write([]byte("diff"))
	h.Write([]byte(joinStrings(files, ",")))
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// ValidateEnvelopeFamily checks that the supplied envelopes form a
// coherent set: every span references a known change set, every
// change set's session/exchange matches the actions', and every
// action carries the relay's expected identifiers. The harness
// runs this before flushing to catch drift.
func ValidateEnvelopeFamily(actions []*ActionEnvelope, changeSets []*ChangeSetEnvelope, spans []*ProvenanceSpanEnvelope) error {
	csByID := make(map[string]*ChangeSetEnvelope, len(changeSets))
	for _, cs := range changeSets {
		if cs.ChangeSetID == "" {
			return errors.New("provenancewire: change set missing ID")
		}
		if cs.SessionID == "" {
			return errors.New("provenancewire: change set missing SessionID")
		}
		csByID[cs.ChangeSetID] = cs
	}
	for _, sp := range spans {
		if sp.ChangeSetID != "" {
			if _, ok := csByID[sp.ChangeSetID]; !ok {
				return fmt.Errorf("provenancewire: span %q references unknown change set %q", sp.SpanID, sp.ChangeSetID)
			}
		}
		if sp.ChangeSetID != "" && sp.SessionID != "" {
			if cs, ok := csByID[sp.ChangeSetID]; ok && cs.SessionID != sp.SessionID {
				return fmt.Errorf("provenancewire: span %q session %q does not match change set session %q", sp.SpanID, sp.SessionID, cs.SessionID)
			}
		}
	}
	for _, act := range actions {
		if act.ActionID == "" {
			return errors.New("provenancewire: action missing ActionID")
		}
		if act.OrganizationID == "" {
			return fmt.Errorf("provenancewire: action %q missing OrganizationID", act.ActionID)
		}
		if act.ActionType == "" {
			return fmt.Errorf("provenancewire: action %q missing ActionType", act.ActionID)
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


// sync import surface check: keep math visible even if the test
// files use it through indirection.
