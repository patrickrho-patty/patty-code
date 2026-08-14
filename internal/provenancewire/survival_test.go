package provenancewire

import (
	"strings"
	"testing"

	"patty/internal/evidence"
)

// TestSpanSurvivesFileRename pins the B2.3 survival boundary
// (PRD §19.4): when a file moves from path A to path B, the
// SemanticFingerprint (bound to (lang, symbol)) must remain
// stable so the relay's `provenance.LookupCodeSpan` re-resolves
// the span against the new path. The ASTFingerprint (bound to
// (file, line range)) MUST change because the physical location
// changed.
func TestSpanSurvivesFileRename(t *testing.T) {
	receipt := evidence.Receipt{
		ToolName: "edit_file",
		Command:  "edit /repo/foo.go",
		Success:  true,
		Mutation: true,
		Paths:    []string{"/repo/foo.go"},
	}
	spanA, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:     "sp-rename",
		RepositoryID: "pccp",
		FilePath:    "/repo/foo.go",
		SymbolLang: "go",
		SymbolName: "compute",
		StartLine:  42,
		EndLine:    56,
		Receipt:    receipt,
	})
	if err != nil {
		t.Fatalf("build spanA: %v", err)
	}
	// The relay stores spanA at /repo/foo.go; the developer moves
	// the file. The harness re-emits a span at the new path with
	// the same logical symbol.
	spanB, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:     "sp-rename",
		RepositoryID: "pccp",
		FilePath:    "/repo/subdir/foo.go", // renamed/moved
		SymbolLang: "go",
		SymbolName: "compute", // same symbol
		StartLine:  42,
		EndLine:    56,
		Receipt:    receipt,
	})
	if err != nil {
		t.Fatalf("build spanB: %v", err)
	}
	// Semantic fingerprint MUST be stable across the rename.
	if spanA.SemanticFingerprint != spanB.SemanticFingerprint {
		t.Errorf("semantic fingerprint must survive rename: %x vs %x",
			spanA.SemanticFingerprint, spanB.SemanticFingerprint)
	}
	// AST fingerprint MUST change because the file path changed.
	if spanA.ASTFingerprint == spanB.ASTFingerprint {
		t.Error("AST fingerprint must change when file path changes")
	}
	// SpanDigest must also differ because it incorporates the
	// AST fingerprint.
	if spanA.SpanDigest == spanB.SpanDigest {
		t.Error("span digest must differ across renames")
	}
}

// TestSemanticFingerprintIgnoresFilePath pins the language-symbol
// binding: two spans with the same (lang, symbol) but different
// paths must produce identical SemanticFingerprint values
// regardless of file location.
func TestSemanticFingerprintIgnoresFilePath(t *testing.T) {
	receipt := evidence.Receipt{
		ToolName: "edit_file", Mutation: true, Paths: []string{"a"}, Success: true,
	}
	a, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:     "sp-a",
		RepositoryID: "pccp",
		FilePath:    "/repo/a.go",
		SymbolLang: "go",
		SymbolName: "compute",
		StartLine:  1,
		EndLine:    10,
		Receipt:    receipt,
	})
	if err != nil {
		t.Fatalf("build a: %v", err)
	}
	b, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID:     "sp-b",
		RepositoryID: "pccp",
		FilePath:    "/elsewhere/b.go",
		SymbolLang: "go",
		SymbolName: "compute",
		StartLine:  1,
		EndLine:    10,
		Receipt:    receipt,
	})
	if err != nil {
		t.Fatalf("build b: %v", err)
	}
	if a.SemanticFingerprint != b.SemanticFingerprint {
		t.Errorf("semantic fingerprint must be identical: %x vs %x",
			a.SemanticFingerprint, b.SemanticFingerprint)
	}
}

// TestSemanticFingerprintDistinguishesLanguage pins the language
// boundary: the same symbol name across languages must produce
// different SemanticFingerprint values.
func TestSemanticFingerprintDistinguishesLanguage(t *testing.T) {
	receipt := evidence.Receipt{
		ToolName: "edit_file", Mutation: true, Paths: []string{"a"}, Success: true,
	}
	goSpan, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID: "sp-go", RepositoryID: "pccp", FilePath: "/x/foo.go",
		SymbolLang: "go", SymbolName: "compute", StartLine: 1, EndLine: 5, Receipt: receipt,
	})
	if err != nil {
		t.Fatalf("build go: %v", err)
	}
	pySpan, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID: "sp-py", RepositoryID: "pccp", FilePath: "/x/foo.py",
		SymbolLang: "python", SymbolName: "compute", StartLine: 1, EndLine: 5, Receipt: receipt,
	})
	if err != nil {
		t.Fatalf("build python: %v", err)
	}
	if goSpan.SemanticFingerprint == pySpan.SemanticFingerprint {
		t.Error("semantic fingerprint must distinguish languages")
	}
}

// TestASTFingerprintDistinguishesLineRange pins the line-range
// binding: a span moved to a different line range must produce a
// different ASTFingerprint.
func TestASTFingerprintDistinguishesLineRange(t *testing.T) {
	receipt := evidence.Receipt{
		ToolName: "edit_file", Mutation: true, Paths: []string{"a"}, Success: true,
	}
	a, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID: "sp-a", RepositoryID: "pccp", FilePath: "/x/a.go",
		SymbolLang: "go", SymbolName: "compute", StartLine: 1, EndLine: 10, Receipt: receipt,
	})
	if err != nil {
		t.Fatalf("build a: %v", err)
	}
	b, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID: "sp-b", RepositoryID: "pccp", FilePath: "/x/a.go",
		SymbolLang: "go", SymbolName: "compute", StartLine: 20, EndLine: 30, Receipt: receipt,
	})
	if err != nil {
		t.Fatalf("build b: %v", err)
	}
	if a.ASTFingerprint == b.ASTFingerprint {
		t.Error("AST fingerprint must distinguish line ranges")
	}
}

// TestAttributionChainRoundTrip pins the B4 chain-readability
// boundary: the connector constructs an AttributionChain from a
// span and reproduces a ReplayPlan that captures the prompt +
// model + files.
func TestAttributionChainRoundTrip(t *testing.T) {
	receipt := evidence.Receipt{
		ToolName: "edit_file", Mutation: true, Paths: []string{"foo.go"}, Success: true,
	}
	span, err := BuildSpanEnvelopeFromReceipt(SpanBuildRequest{
		SpanID: "sp-chain", RepositoryID: "pccp", FilePath: "foo.go",
		SymbolLang: "go", SymbolName: "compute", StartLine: 1, EndLine: 10, Receipt: receipt,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	chain := &AttributionChain{
		ExchangeID:     "ex-1",
		SessionID:      "ses-1",
		UserID:         "alice",
		HarnessID:      "harness-1",
		ModelPackageID: "pkg-1",
		EndpointID:     "endpoint-1",
		PolicyEpochID:  "epoch-1",
		LeaseID:        "lease-1",
		Prompts:        []string{"edit compute to add caching"},
		Tools:          []string{"edit_file", "read_file"},
		Files:          []string{"foo.go"},
		Commits:        []string{"abc123"},
		Spans:          []SpanLookupKey{{FilePath: span.FilePath, StartLine: 1, EndLine: 10}},
	}
	if !strings.Contains(chain.String(), "exchange=ex-1") {
		t.Errorf("chain string = %q, want exchange=ex-1", chain.String())
	}
	plan := replayFromChain(chain)
	if plan == nil {
		t.Fatal("plan must not be nil")
	}
	if plan.ExchangeID != "ex-1" {
		t.Errorf("plan exchange = %s, want ex-1", plan.ExchangeID)
	}
	if plan.ModelPackageID != "pkg-1" {
		t.Errorf("plan model = %s, want pkg-1", plan.ModelPackageID)
	}
	if !strings.Contains(plan.Prompt, "caching") {
		t.Errorf("plan prompt missing")
	}
	if !strings.Contains(plan.String(), "model=pkg-1") {
		t.Errorf("plan string = %q, want model=pkg-1", plan.String())
	}
}

// TestReplayFromNilChain is the trivial boundary.
func TestReplayFromNilChain(t *testing.T) {
	if plan := replayFromChain(nil); plan != nil {
		t.Errorf("nil chain must yield nil plan, got %v", plan)
	}
}
