package dari

import (
	"os"
	"path/filepath"
	"testing"
)

// provenance_edit_test.go pins the B1 production wiring: a real
// harness-side file edit surfaces as an attribution span plus a
// sealed turn change set on the session emitter.

func TestRecordAIEditEmitsSpanAndSealsChangeSet(t *testing.T) {
	p := &Provider{}
	p.SetProvenanceContext("repo-x", "main")

	dir := t.TempDir()
	path := filepath.Join(dir, "edited.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p.RecordAIEdit(path, "write_file")

	em := p.ProvenanceEmitter()
	css, spans, _ := em.Pending()
	if len(css) != 0 {
		t.Fatalf("change set must only appear at flush time, got %d", len(css))
	}
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	s := spans[0]
	if s.FilePath != path || s.RepositoryID != "repo-x" {
		t.Fatalf("span = %+v", s)
	}
	if s.AttributionState != "AI_GENERATED" {
		t.Fatalf("attribution = %q", s.AttributionState)
	}
	if s.EndLine < 3 {
		t.Fatalf("EndLine = %d, want >= 3 lines", s.EndLine)
	}
	if s.SpanDigest == [32]byte{} {
		t.Fatal("span digest must be computed")
	}

	p.sealTurnChangeSet(em)
	css, _, _ = em.Pending()
	if len(css) != 1 {
		t.Fatalf("sealed change sets = %d, want 1", len(css))
	}
	cs := css[0]
	if cs.RepositoryID != "repo-x" || cs.Branch != "main" || len(cs.FilesChanged) != 1 || cs.FilesChanged[0] != path {
		t.Fatalf("changeset = %+v", cs)
	}
	if cs.ChangeSetDigest == [32]byte{} || cs.DiffDigest == [32]byte{} {
		t.Fatal("change set digests must be computed")
	}
}

func TestRecordAIEditWithoutRepoIdentityIsNoop(t *testing.T) {
	p := &Provider{} // no SetProvenanceContext
	p.RecordAIEdit("/tmp/whatever.go", "write_file")
	if _, spans, _ := p.ProvenanceEmitter().Pending(); len(spans) != 0 {
		t.Fatalf("spans = %d, want 0 without repo identity", len(spans))
	}
}

func TestSealTurnChangeSetResetsAccumulator(t *testing.T) {
	p := &Provider{}
	p.SetProvenanceContext("repo-x", "main")
	p.provTurnPaths = map[string]bool{"a.go": true}
	em := p.ProvenanceEmitter()
	p.sealTurnChangeSet(em)
	if p.provTurnPaths != nil {
		t.Fatal("accumulator must reset after seal")
	}
	p.sealTurnChangeSet(em)
	if css, _, _ := em.Pending(); len(css) != 1 {
		t.Fatalf("second seal must not re-emit, change sets = %d", len(css))
	}
}
