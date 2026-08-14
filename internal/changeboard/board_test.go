package changeboard

import (
	"strings"
	"sync"
	"testing"
)

func sampleSubmission(id string) *Submission {
	return &Submission{
		SubmissionID:   id,
		OrganizationID: "org-test",
		RepositoryID:   "pccp",
		Branch:         "main",
		CommitSHA:      "abc123",
		RiskClass:      RiskCrypto,
		Description:    "add ECDSA signature verification",
		DescriptionKo:  "ECDSA 서명 검증 추가",
		Submitter:      "alice",
	}
}

// TestSubmitAndApproveFlow covers the D2 green path: a
// high-risk change is submitted, the relay pushes the admin's
// approval back, and the submission transitions to APPROVED.
func TestSubmitAndApproveFlow(t *testing.T) {
	b := NewBoard()
	sub, err := b.Submit(sampleSubmission("sub-1"))
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !sub.IsPending() {
		t.Error("submission must start pending")
	}
	if sub.IsApproved() {
		t.Error("submission must not start approved")
	}
	if err := b.Approve("sub-1", "reviewer-bob", "LGTM", 1_700_000_000_000); err != nil {
		t.Fatalf("approve: %v", err)
	}
	got, ok := b.Get("sub-1")
	if !ok {
		t.Fatal("submission must be retrievable")
	}
	if !got.IsApproved() {
		t.Error("submission must be approved after Approve")
	}
	if got.ReviewerID != "reviewer-bob" {
		t.Errorf("reviewer id = %s, want reviewer-bob", got.ReviewerID)
	}
}

// TestSubmitIsIdempotentByFingerprint covers the D2 idempotency
// boundary: re-submitting the same change returns the same
// submission record.
func TestSubmitIsIdempotentByFingerprint(t *testing.T) {
	b := NewBoard()
	first, err := b.Submit(sampleSubmission("sub-1"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := b.Submit(sampleSubmission("sub-2"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Error("idempotent submit must return the same record")
	}
	if b.Count() != 1 {
		t.Errorf("count = %d, want 1", b.Count())
	}
}

// TestSubmitRejectsMissingFields covers the trivial boundary:
// missing IDs / risk classes fail closed.
func TestSubmitRejectsMissingFields(t *testing.T) {
	b := NewBoard()
	if _, err := b.Submit(nil); err == nil {
		t.Fatal("nil submission must fail")
	}
	if _, err := b.Submit(&Submission{}); err == nil {
		t.Fatal("empty submission must fail")
	}
	if _, err := b.Submit(&Submission{SubmissionID: "x"}); err == nil {
		t.Fatal("missing RiskClass must fail")
	}
}

// TestApproveRejectsUnknownSubmission covers the trivial
// boundary: the harness can't approve a submission it never
// submitted.
func TestApproveRejectsUnknownSubmission(t *testing.T) {
	b := NewBoard()
	if err := b.Approve("missing", "bob", "LGTM", 0); err == nil {
		t.Fatal("approve missing must fail")
	}
	if err := b.Reject("missing", "bob", "no", 0); err == nil {
		t.Fatal("reject missing must fail")
	}
}

// TestIsExpired covers the time-boundary: a submission past
// NotAfter is expired.
func TestIsExpired(t *testing.T) {
	now := int64(1_700_000_000_000)
	sub := sampleSubmission("sub-1")
	sub.NotAfter = now + 100
	if sub.IsExpired(now + 50) {
		t.Error("before NotAfter must not be expired")
	}
	if !sub.IsExpired(now + 200) {
		t.Error("past NotAfter must be expired")
	}
	if sub.IsExpired(0) {
		t.Error("NotAfter=0 must mean no expiry")
	}
}

// TestHighRiskClassRequired covers the high-risk classification.
func TestHighRiskClassRequired(t *testing.T) {
	high := []RiskClass{RiskCrypto, RiskPayment, RiskPIIAdjacent,
		RiskNewDependency, RiskNewNetwork, RiskNewMCPServer}
	for _, c := range high {
		if !HighRiskClassRequired(c) {
			t.Errorf("%s must require change board", c)
		}
	}
	low := []RiskClass{RiskLow, RiskMedium}
	for _, c := range low {
		if HighRiskClassRequired(c) {
			t.Errorf("%s must NOT require change board", c)
		}
	}
}

// TestFingerprintStableAcrossSubmissions covers the audit invariant:
// two submissions with the same logical change share the same
// fingerprint.
func TestFingerprintStableAcrossSubmissions(t *testing.T) {
	a := sampleSubmission("sub-1")
	b := sampleSubmission("sub-2")
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("same logical change must share fingerprint")
	}
	c := sampleSubmission("sub-3")
	c.Branch = "feature"
	if a.Fingerprint() == c.Fingerprint() {
		t.Error("different branch must produce different fingerprint")
	}
}

// TestListPendingReturnsOnlyPending covers the operator-facing
// surface: pending submissions are surfaced in the IDE status bar.
func TestListPendingReturnsOnlyPending(t *testing.T) {
	b := NewBoard()
	for i, id := range []string{"sub-1", "sub-2", "sub-3"} {
		sub := sampleSubmission(id)
		sub.CommitSHA = "sha-" + id // distinct fingerprints
		sub.Branch = "branch-" + id
		_, _ = b.Submit(sub)
		_ = i
	}
	if err := b.Approve("sub-2", "reviewer-bob", "ok", 0); err != nil {
		t.Fatalf("approve: %v", err)
	}
	pending := b.ListPending()
	if len(pending) != 2 {
		t.Errorf("pending count = %d, want 2", len(pending))
	}
	for _, s := range pending {
		if s.SubmissionID == "sub-2" {
			t.Error("approved submission must not appear in pending list")
		}
	}
}

// TestRejectFlow covers the D2 rejection path: the admin pushes a
// rejection back, the submission transitions to REJECTED.
func TestRejectFlow(t *testing.T) {
	b := NewBoard()
	_, _ = b.Submit(sampleSubmission("sub-1"))
	if err := b.Reject("sub-1", "reviewer-bob", "needs more tests", 0); err != nil {
		t.Fatalf("reject: %v", err)
	}
	got, _ := b.Get("sub-1")
	if got.Status != StatusRejected {
		t.Errorf("status = %s, want REJECTED", got.Status)
	}
	if !strings.Contains(got.ReviewerComment, "more tests") {
		t.Errorf("reviewer comment lost")
	}
}

// TestBoardConcurrentSubmit covers the lock boundary.
func TestBoardConcurrentSubmit(t *testing.T) {
	b := NewBoard()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sub := sampleSubmission("sub-" + string(rune('a'+i%26)))
			sub.Branch = "main"
			_, _ = b.Submit(sub)
		}(i)
	}
	wg.Wait()
	if b.Count() > 50 {
		t.Errorf("count = %d, want <= 50", b.Count())
	}
}
