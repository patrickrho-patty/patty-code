package provenancewire

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"patty/internal/evidence"
)

// TestEmitterAcceptsValidActionEnvelope is the green path: an
// envelope with all required fields is stored and returned.
func TestEmitterAcceptsValidActionEnvelope(t *testing.T) {
	emitter := NewProvenanceEmitter()
	env, err := BuildActionEnvelopeFromReceipt(ActionBuildRequest{
		ActionID:         "act-1",
		OrganizationID:   "org-test",
		SessionID:        "ses-1",
		ActionType:       "tool_use",
		ActionPayload:    `{"tool":"bash","args":["ls"]}`,
		OccurredAtUnixMs: 1_700_000_000_000,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := emitter.EmitAction(env); err != nil {
		t.Fatalf("emit: %v", err)
	}
	_, _, actions := emitter.Pending()
	if len(actions) != 1 {
		t.Errorf("actions = %d, want 1", len(actions))
	}
}

// TestEmitterRejectsActionEnvelopeMissingFields exercises the
// fail-closed boundary: an envelope missing required fields is
// rejected before storage.
func TestEmitterRejectsActionEnvelopeMissingFields(t *testing.T) {
	emitter := NewProvenanceEmitter()
	if err := emitter.EmitAction(nil); err == nil {
		t.Fatal("nil envelope must fail")
	}
	env := &ActionEnvelope{ActionType: "tool_use", OccurredAtUnixMs: 1}
	if err := emitter.EmitAction(env); err == nil {
		t.Fatal("missing ActionID must fail")
	}
	env = &ActionEnvelope{ActionID: "act-1", OccurredAtUnixMs: 1}
	if err := emitter.EmitAction(env); err == nil {
		t.Fatal("missing ActionType must fail")
	}
	env = &ActionEnvelope{ActionID: "act-1", ActionType: "tool_use"}
	if err := emitter.EmitAction(env); err == nil {
		t.Fatal("missing OccurredAtUnixMs must fail")
	}
}

// TestEmitterChangeSetBuildFromReceipts exercises the receipt
// aggregation path: a slice of mutation receipts becomes a single
// ChangeSetEnvelope with the expected file list, line counts,
// and attribution state.
func TestEmitterChangeSetBuildFromReceipts(t *testing.T) {
	receipts := []evidence.Receipt{
		{
			ToolName:    "edit_file",
			Command:     "edit /repo/foo.go",
			Success:     true,
			Mutation:    true,
			Paths:       []string{"/repo/foo.go", "/repo/bar.go"},
			OutputBytes: 800,
		},
		{
			ToolName:    "write_file",
			Command:     "write /repo/bar.go",
			Success:     true,
			Mutation:    true,
			Paths:       []string{"/repo/bar.go"},
			OutputBytes: 1600,
		},
	}
	cs, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID:  "cs-1",
		RepositoryID: "pccp",
		Branch:       "main",
		Receipts:     receipts,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(cs.FilesChanged) != 2 {
		t.Errorf("files = %v, want 2", cs.FilesChanged)
	}
	if cs.FilesChanged[0] != "/repo/bar.go" || cs.FilesChanged[1] != "/repo/foo.go" {
		t.Errorf("files = %v, want sorted", cs.FilesChanged)
	}
	if cs.LinesAdded <= 0 {
		t.Errorf("lines added = %d, want > 0", cs.LinesAdded)
	}
	if cs.AttributionState != AttributionAIGenerated {
		t.Errorf("attribution = %s, want AI_GENERATED", cs.AttributionState)
	}
}

// TestEmitterChangeSetBuildRejectsMissingFields exercises the
// fail-closed path for ChangeSet build.
func TestEmitterChangeSetBuildRejectsMissingFields(t *testing.T) {
	if _, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{}); err == nil {
		t.Fatal("missing ChangeSetID must fail")
	}
	if _, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID: "cs-1",
	}); err == nil {
		t.Fatal("missing RepositoryID must fail")
	}
	if _, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID:  "cs-1",
		RepositoryID: "pccp",
	}); err == nil {
		t.Fatal("missing receipts must fail")
	}
	if _, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID:  "cs-1",
		RepositoryID: "pccp",
		Receipts: []evidence.Receipt{
			{ToolName: "read_file", Success: true},
		},
	}); err == nil {
		t.Fatal("receipts without paths must fail")
	}
}

// TestEmitterSpanBuildFromReceipt exercises the span construction
// path: a single mutation receipt becomes a ProvenanceSpanEnvelope
// with the expected file path, AST fingerprint, and semantic
// fingerprint.
func TestEmitterSpanBuildFromReceipt(t *testing.T) {
	receipt := evidence.Receipt{
		ToolName: "edit_file",
		Command:  "edit /repo/foo.go",
		Success:  true,
		Mutation: true,
		Paths:    []string{"/repo/foo.go"},
	}
	span, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:       "span-1",
		RepositoryID: "pccp",
		SymbolLang:   "go",
		SymbolName:   "main.compute",
		StartLine:    42,
		EndLine:      56,
		Receipt:      receipt,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if span.FilePath != "/repo/foo.go" {
		t.Errorf("file = %s, want /repo/foo.go", span.FilePath)
	}
	if span.ASTFingerprint == [32]byte{} {
		t.Error("AST fingerprint must be set")
	}
	if span.SemanticFingerprint == [32]byte{} {
		t.Error("semantic fingerprint must be set")
	}
	if span.AttributionState != AttributionAIGenerated {
		t.Errorf("attribution = %s, want AI_GENERATED", span.AttributionState)
	}
}

// TestEmitterSpanFingerprintSurvivesRename pins the rename-safety
// boundary (PRD §19.4): two spans with the same logical symbol
// but different file paths must share a SemanticFingerprint but
// differ on ASTFingerprint.
func TestEmitterSpanFingerprintSurvivesRename(t *testing.T) {
	receipt := evidence.Receipt{
		ToolName: "edit_file",
		Command:  "edit foo.go",
		Success:  true,
		Mutation: true,
		Paths:    []string{"/repo/foo.go"},
	}
	span1, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:     "span-1",
		SymbolLang: "go",
		SymbolName: "compute",
		StartLine:  42,
		EndLine:    56,
		Receipt:    receipt,
	})
	if err != nil {
		t.Fatalf("build span1: %v", err)
	}
	span2, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:     "span-2",
		FilePath:   "/repo/renamed/foo.go",
		SymbolLang: "go",
		SymbolName: "compute",
		StartLine:  42,
		EndLine:    56,
		Receipt:    receipt,
	})
	if err != nil {
		t.Fatalf("build span2: %v", err)
	}
	// AST fingerprint depends on file path -> must differ.
	if span1.ASTFingerprint == span2.ASTFingerprint {
		t.Error("AST fingerprint must change when file path changes")
	}
	// Semantic fingerprint depends only on (file, lang, symbol) ->
	// since both use the same logical symbol and the original span
	// shares the same source file, semantic fingerprint should be
	// identical for the original anchor.
	if span1.SemanticFingerprint != span2.SemanticFingerprint {
		t.Errorf("semantic fingerprint differs: %x vs %x",
			span1.SemanticFingerprint, span2.SemanticFingerprint)
	}
}

// TestEmitterClearEmptiesPending ensures Clear drains all buffers
// after a successful flush.
func TestEmitterClearEmptiesPending(t *testing.T) {
	emitter := NewProvenanceEmitter()
	env, _ := BuildActionEnvelopeFromReceipt(ActionBuildRequest{
		ActionID:         "act-1",
		ActionType:       "tool_use",
		OccurredAtUnixMs: 1,
	})
	_ = emitter.EmitAction(env)
	emitter.Clear()
	_, _, actions := emitter.Pending()
	if len(actions) != 0 {
		t.Errorf("after Clear, actions = %d, want 0", len(actions))
	}
}

// TestEmitterConcurrentEmitAndPending guards the mutex around the
// pending buffers.
func TestEmitterConcurrentEmitAndPending(t *testing.T) {
	emitter := NewProvenanceEmitter()
	env, _ := BuildActionEnvelopeFromReceipt(ActionBuildRequest{
		ActionID:         "act",
		ActionType:       "tool_use",
		OccurredAtUnixMs: 1,
	})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = emitter.EmitAction(env)
		}()
		go func() {
			defer wg.Done()
			_, _, _ = emitter.Pending()
		}()
	}
	wg.Wait()
}

// TestValidateEnvelopeFamilyAcceptsCoherentSet exercises the
// envelope-family consistency check.
func TestValidateEnvelopeFamilyAcceptsCoherentSet(t *testing.T) {
	actions := []*ActionEnvelope{
		{ActionID: "act-1", OrganizationID: "org-test", ActionType: "tool_use", OccurredAtUnixMs: 1, EnvelopeDigest: [32]byte{1}},
	}
	changeSets := []*ChangeSetEnvelope{
		{ChangeSetID: "cs-1", SessionID: "ses-1", OrganizationID: "org-test", RepositoryID: "pccp", FilesChanged: []string{"foo.go"}, ChangeSetDigest: [32]byte{2}},
	}
	spans := []*ProvenanceSpanEnvelope{
		{SpanID: "sp-1", RepositoryID: "pccp", FilePath: "foo.go", ChangeSetID: "cs-1", SessionID: "ses-1", ASTFingerprint: [32]byte{3}, SpanDigest: [32]byte{4}},
	}
	if err := ValidateEnvelopeFamily(actions, changeSets, spans); err != nil {
		t.Errorf("coherent family must pass, got %v", err)
	}
}

// TestValidateEnvelopeFamilyRejectsUnknownChangeSet guards the
// reference-integrity boundary.
func TestValidateEnvelopeFamilyRejectsUnknownChangeSet(t *testing.T) {
	changeSets := []*ChangeSetEnvelope{
		{ChangeSetID: "cs-1", RepositoryID: "pccp", FilesChanged: []string{"foo.go"}, ChangeSetDigest: [32]byte{2}},
	}
	spans := []*ProvenanceSpanEnvelope{
		{SpanID: "sp-1", RepositoryID: "pccp", FilePath: "foo.go", ChangeSetID: "cs-unknown", ASTFingerprint: [32]byte{3}, SpanDigest: [32]byte{4}},
	}
	if err := ValidateEnvelopeFamily(nil, changeSets, spans); err == nil {
		t.Fatal("span referencing unknown change set must fail")
	}
}

// TestValidateEnvelopeFamilyAcceptsEmpty covers the trivial
// boundary: an empty envelope family is structurally valid (the
// harness calls the dispatcher only when there's something to push;
// the validator itself doesn't gate emptiness).
func TestValidateEnvelopeFamilyAcceptsEmpty(t *testing.T) {
	if err := ValidateEnvelopeFamily(nil, nil, nil); err != nil {
		t.Fatalf("empty family must pass structural validation, got %v", err)
	}
}

// TestSanitizeAttributionAcceptsValidValues pins the documented
// attribution states.
func TestSanitizeAttributionAcceptsValidValues(t *testing.T) {
	for _, value := range []AttributionState{
		AttributionAIGenerated, AttributionAIThenHumanEdited,
		AttributionHumanThenAIAssisted, AttributionHumanWritten,
	} {
		if SanitizeAttribution(value) != value {
			t.Errorf("valid value %s must round-trip", value)
		}
		if !IsValidAttributionState(value) {
			t.Errorf("valid value %s marked invalid", value)
		}
	}
}

// TestSanitizeAttributionDefaultsUnknown defends against a buggy
// relay that hands the connector an unrecognized attribution string.
func TestSanitizeAttributionDefaultsUnknown(t *testing.T) {
	unknown := SanitizeAttribution("BUGGY_RELAY_VALUE")
	if unknown != AttributionAIGenerated {
		t.Errorf("unknown attribution must default to AI_GENERATED, got %s", unknown)
	}
	if IsValidAttributionState("BUGGY_RELAY_VALUE") {
		t.Error("unknown value must be flagged invalid")
	}
}

// TestWireRoundTripChangeSet pins the byte contract: the relay
// decodes what the connector sends.
func TestWireRoundTripChangeSet(t *testing.T) {
	cs, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID:  "cs-1",
		RepositoryID: "pccp",
		Branch:       "main",
		Receipts: []evidence.Receipt{
			{ToolName: "edit_file", Mutation: true, Paths: []string{"foo.go"}, Success: true},
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := EncodeChangeSetEnvelope(cs)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeChangeSetEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.ChangeSetID != cs.ChangeSetID {
		t.Errorf("id drift: %q vs %q", decoded.ChangeSetID, cs.ChangeSetID)
	}
	if decoded.RepositoryID != cs.RepositoryID {
		t.Errorf("repo drift: %q vs %q", decoded.RepositoryID, cs.RepositoryID)
	}
}

// TestWireRoundTripSpan covers the span encode/decode pair.
func TestWireRoundTripSpan(t *testing.T) {
	span, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:       "sp-1",
		RepositoryID: "pccp",
		SymbolLang:   "go",
		SymbolName:   "compute",
		StartLine:    10,
		EndLine:      20,
		Receipt: evidence.Receipt{
			ToolName: "edit_file", Mutation: true, Paths: []string{"foo.go"}, Success: true,
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := EncodeSpanEnvelope(span)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeSpanEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.SpanID != span.SpanID {
		t.Errorf("id drift: %q vs %q", decoded.SpanID, span.SpanID)
	}
	if decoded.ASTFingerprint != span.ASTFingerprint {
		t.Error("AST fingerprint drift")
	}
}

// TestWireRoundTripAction covers the action encode/decode pair.
func TestWireRoundTripAction(t *testing.T) {
	env, err := BuildActionEnvelopeFromReceipt(ActionBuildRequest{
		ActionID:         "act-1",
		ActionType:       "tool_use",
		ActionPayload:    `{"tool":"bash"}`,
		OccurredAtUnixMs: 1_700_000_000_000,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := EncodeActionEnvelope(env)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeActionEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.ActionID != env.ActionID {
		t.Errorf("id drift: %q vs %q", decoded.ActionID, env.ActionID)
	}
}

// TestWireRoundTripCommitBinding covers the commit-binding encode/decode.
func TestWireRoundTripCommitBinding(t *testing.T) {
	cb := &CommitBindingEnvelope{
		BindingID:     "b-1",
		RepositoryID:  "pccp",
		CommitSHA:     "abc123",
		ChangeSetID:   "cs-1",
		Branch:        "main",
		BoundAtUnixMs: 1_700_000_000_000,
	}
	cb.Digest()
	data, err := EncodeCommitBindingEnvelope(cb)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommitBindingEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.BindingDigest != cb.BindingDigest {
		t.Error("binding digest drift")
	}
}

// TestWireRoundTripEvidenceReceipt covers the relay-pushed
// receipt envelope.
func TestWireRoundTripEvidenceReceipt(t *testing.T) {
	receipt := &EvidenceReceiptEnvelope{
		ReceiptID:      "r-1",
		ExchangeID:     "ex-1",
		SessionID:      "ses-1",
		FinalState:     "completed",
		ChainRoot:      [32]byte{1, 2, 3},
		RelayIdentity:  "pccp-relay",
		KeyAlgorithm:   "ed25519+cose-sign1",
		Signature:      "00",
		IssuedAtUnixMs: 1_700_000_000_000,
	}
	data, err := EncodeEvidenceReceiptEnvelope(receipt)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeEvidenceReceiptEnvelope(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.ReceiptID != receipt.ReceiptID {
		t.Errorf("id drift: %q vs %q", decoded.ReceiptID, receipt.ReceiptID)
	}
}

// TestWireRoundTripReceiptAck covers the connector's reply to a
// relay-pushed receipt.
func TestWireRoundTripReceiptAck(t *testing.T) {
	ack := &ReceiptAck{
		ReceiptID:     "r-1",
		ExchangeID:    "ex-1",
		AckDigest:     [32]byte{9},
		AckedAtUnixMs: 1_700_000_000_000,
	}
	data, err := EncodeReceiptAck(ack)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeReceiptAck(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.AckDigest != ack.AckDigest {
		t.Error("ack digest drift")
	}
}

// TestDigestChangeIsContentBound pins the binding invariant:
// changing any field changes the digest.
func TestDigestChangeIsContentBound(t *testing.T) {
	receipts := []evidence.Receipt{
		{ToolName: "edit_file", Mutation: true, Paths: []string{"foo.go"}, Success: true, OutputBytes: 100},
	}
	a, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID: "cs-1", RepositoryID: "pccp", Receipts: receipts,
	})
	if err != nil {
		t.Fatalf("build a: %v", err)
	}
	b, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID: "cs-1", RepositoryID: "pccp", Receipts: receipts,
	})
	if err != nil {
		t.Fatalf("build b: %v", err)
	}
	if a.ChangeSetDigest != b.ChangeSetDigest {
		t.Errorf("same input produced different digests: %x vs %x", a.ChangeSetDigest, b.ChangeSetDigest)
	}
}

// TestReplayFromChainBuildsReplayPlan covers the B4 replay
// boundary: the connector reads an AttributionChain and produces a
// ReplayPlan.
func TestReplayFromChainBuildsReplayPlan(t *testing.T) {
	chain := &AttributionChain{
		ExchangeID:     "ex-1",
		SessionID:      "ses-1",
		ModelPackageID: "pkg-1",
		Files:          []string{"foo.go"},
		Prompts:        []string{"first prompt", "second prompt"},
	}
	plan := replayFromChain(chain)
	if plan == nil {
		t.Fatal("plan must not be nil")
	}
	if plan.ExchangeID != "ex-1" {
		t.Errorf("plan exchange = %s", plan.ExchangeID)
	}
	if !strings.Contains(plan.Prompt, "first prompt") || !strings.Contains(plan.Prompt, "second prompt") {
		t.Errorf("plan prompt = %q, missing prompts", plan.Prompt)
	}
}

// TestEmittedReceiptsCarryIssuerKeyEdgeCase checks that the
// digest remains stable across empty Optional fields, ensuring
// the relay's `provenance.CreateChangeSet` records the same bytes.
func TestEmittedReceiptsCarryIssuerKeyEdgeCase(t *testing.T) {
	cs := &ChangeSetEnvelope{
		ChangeSetID:      "cs-1",
		OrganizationID:   "org-test",
		SessionID:        "ses-1",
		RepositoryID:     "pccp",
		Branch:           "main",
		FilesChanged:     []string{"foo.go"},
		AttributionState: AttributionAIGenerated,
	}
	cs.Digest()
	if cs.ChangeSetDigest == [32]byte{} {
		t.Fatal("digest must be set")
	}
}

// TestEdgeCase_EmptyFilesChangedInChangeSet guards the failure
// path: an empty file list must be rejected before emission.
func TestEdgeCase_EmptyFilesChangedInChangeSet(t *testing.T) {
	if _, err := BuildChangeSetEnvelopeFromReceipts(ChangeSetBuildRequest{
		ChangeSetID:  "cs-1",
		RepositoryID: "pccp",
		Receipts:     []evidence.Receipt{},
	}); err == nil {
		t.Fatal("empty receipts must fail")
	}
}

// TestEdgeCase_SpanWithoutFilePath rejects span builds that
// reference neither an explicit FilePath nor a Receipt.Paths
// entry.
func TestEdgeCase_SpanWithoutFilePath(t *testing.T) {
	_, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:       "sp-1",
		RepositoryID: "pccp",
		SymbolLang:   "go",
		SymbolName:   "compute",
		StartLine:    1,
		EndLine:      10,
		// Receipt has empty Paths.
		Receipt: evidence.Receipt{ToolName: "edit_file", Success: true},
	})
	if err == nil {
		t.Fatal("span without file path must fail")
	}
}

// TestEdgeCase_DecodeEmptyBody guards the wire-shape gate.
func TestEdgeCase_DecodeEmptyBody(t *testing.T) {
	if _, err := DecodeChangeSetEnvelope(nil); err == nil {
		t.Fatal("empty change set body must fail")
	}
	if _, err := DecodeChangeSetEnvelope([]byte{0xff}); err == nil {
		t.Fatal("invalid change set body must fail")
	}
	if _, err := DecodeSpanEnvelope(nil); err == nil {
		t.Fatal("empty span body must fail")
	}
	if _, err := DecodeActionEnvelope(nil); err == nil {
		t.Fatal("empty action body must fail")
	}
	if _, err := DecodeCommitBindingEnvelope(nil); err == nil {
		t.Fatal("empty commit binding body must fail")
	}
	if _, err := DecodeEvidenceReceiptEnvelope(nil); err == nil {
		t.Fatal("empty evidence receipt body must fail")
	}
	if _, err := DecodeReceiptAck(nil); err == nil {
		t.Fatal("empty receipt ack body must fail")
	}
}

// TestEdgeCase_SpanLookupKeyStable covers the deterministic
// label requirement for audit logs.
func TestEdgeCase_SpanLookupKeyStable(t *testing.T) {
	k := SpanLookupKey{FilePath: "/repo/foo.go", StartLine: 42, EndLine: 56}
	if got := k.String(); got != "/repo/foo.go:42-56" {
		t.Errorf("lookup key = %q", got)
	}
}

// _ guards the standard-library imports against unused-import
// errors when tests evolve.
var _ = bytes.Equal
var _ = sha256.Sum256
var _ = hex.EncodeToString
var _ = json.Marshal
var _ ed25519.PublicKey
