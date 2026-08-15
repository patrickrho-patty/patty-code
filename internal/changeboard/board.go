// Package changeboard is the connector-side change-control-board
// submission (harness feature plan D2). The harness auto-submits
// high-risk changes (crypto/payment/PII-adjacent, new MCP, new
// network destination, new dependency) to PCCP's change-control
// queue and blocks merge until approved.
//
// The submission is idempotent: a re-submission of the same
// change produces the same submission ID. The harness consults
// the queue status before allowing the merge.
package changeboard

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RiskClass is the sensitivity tier the harness assigns to a
// change. High-risk changes MUST go through the change board.
type RiskClass string

const (
	RiskLow           RiskClass = "LOW"
	RiskMedium        RiskClass = "MEDIUM"
	RiskHigh          RiskClass = "HIGH"
	RiskCrypto        RiskClass = "CRYPTO"
	RiskPayment       RiskClass = "PAYMENT"
	RiskPIIAdjacent   RiskClass = "PII_ADJACENT"
	RiskNewDependency RiskClass = "NEW_DEPENDENCY"
	RiskNewNetwork    RiskClass = "NEW_NETWORK"
	RiskNewMCPServer  RiskClass = "NEW_MCP_SERVER"
)

// SubmissionStatus is the lifecycle state of a change-board
// submission.
type SubmissionStatus string

const (
	StatusPending    SubmissionStatus = "PENDING"
	StatusApproved   SubmissionStatus = "APPROVED"
	StatusRejected   SubmissionStatus = "REJECTED"
	StatusSuperseded SubmissionStatus = "SUPERSEDED"
)

// Submission is a single high-risk change submitted to the
// change-control board. The harness consults SubmissionStatus
// before allowing a merge.
type Submission struct {
	SubmissionID    string
	OrganizationID  string
	RepositoryID    string
	Branch          string
	CommitSHA       string
	RiskClass       RiskClass
	Description     string
	DescriptionKo   string
	Submitter       string
	SubmittedAt     int64 // unix-ms
	NotAfter        int64 // unix-ms; 0 = no expiry
	Status          SubmissionStatus
	ReviewerID      string
	ReviewedAt      int64
	ReviewerComment string
}

// IsApproved reports whether the submission is approved for merge.
func (s *Submission) IsApproved() bool {
	return s != nil && s.Status == StatusApproved
}

// IsPending reports whether the submission is still awaiting
// review.
func (s *Submission) IsPending() bool {
	return s != nil && s.Status == StatusPending
}

// IsExpired reports whether the submission is past NotAfter.
func (s *Submission) IsExpired(nowMs int64) bool {
	return s != nil && s.NotAfter > 0 && nowMs >= s.NotAfter
}

// Fingerprint is the content-addressed fingerprint the harness
// uses to dedupe submissions. Two submissions with the same
// fingerprint represent the same logical change.
func (s *Submission) Fingerprint() [32]byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%s",
		s.RepositoryID, s.Branch, s.CommitSHA, s.RiskClass, s.Description)
	h := sha256.New()
	h.Write([]byte("DARI-CHANGEBOARD-v1\x00"))
	h.Write([]byte(data))
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// Board is the harness-side change-control board queue. The
// harness uses it to record every high-risk change submission
// and consult it before allowing merges.
type Board struct {
	mu            sync.RWMutex
	submissions   map[string]*Submission // keyed by SubmissionID
	byFingerprint map[[32]byte]*Submission
	nowFn         func() int64
}

// NewBoard constructs an empty board.
func NewBoard() *Board {
	return &Board{
		submissions:   make(map[string]*Submission),
		byFingerprint: make(map[[32]byte]*Submission),
		nowFn:         time.Now().UnixMilli,
	}
}

// WithNowFunc overrides the time source. Tests use this to drive
// expiry deterministically.
func (b *Board) WithNowFunc(fn func() int64) *Board {
	b.nowFn = fn
	return b
}

// Submit records the submission. If a submission with the same
// fingerprint already exists, the existing record is returned
// unchanged (idempotency).
func (b *Board) Submit(s *Submission) (*Submission, error) {
	if s == nil {
		return nil, errors.New("changeboard: nil submission")
	}
	if s.SubmissionID == "" {
		return nil, errors.New("changeboard: submission missing ID")
	}
	if s.RiskClass == "" {
		return nil, errors.New("changeboard: submission missing RiskClass")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	fp := s.Fingerprint()
	if existing, ok := b.byFingerprint[fp]; ok {
		return existing, nil
	}
	s.Status = StatusPending
	s.SubmittedAt = b.nowFn()
	b.submissions[s.SubmissionID] = s
	b.byFingerprint[fp] = s
	return s, nil
}

// Get returns the submission by ID, if present.
func (b *Board) Get(submissionID string) (*Submission, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.submissions[submissionID]
	return s, ok
}

// FindByFingerprint returns the submission with the supplied
// fingerprint, if present.
func (b *Board) FindByFingerprint(fp [32]byte) (*Submission, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.byFingerprint[fp]
	return s, ok
}

// Approve marks the submission approved. The harness's
// merge pipeline invokes this when the relay pushes the admin's
// approval back over PAPER.
func (b *Board) Approve(submissionID, reviewerID, comment string, nowMs int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.submissions[submissionID]
	if !ok {
		return fmt.Errorf("changeboard: submission %s not found", submissionID)
	}
	s.Status = StatusApproved
	s.ReviewerID = reviewerID
	s.ReviewedAt = nowMs
	s.ReviewerComment = comment
	return nil
}

// Reject marks the submission rejected.
func (b *Board) Reject(submissionID, reviewerID, comment string, nowMs int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.submissions[submissionID]
	if !ok {
		return fmt.Errorf("changeboard: submission %s not found", submissionID)
	}
	s.Status = StatusRejected
	s.ReviewerID = reviewerID
	s.ReviewedAt = nowMs
	s.ReviewerComment = comment
	return nil
}

// ListPending returns all pending submissions. The harness surfaces
// these in the IDE status bar so the operator sees what's
// outstanding.
func (b *Board) ListPending() []*Submission {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []*Submission
	for _, s := range b.submissions {
		if s.Status == StatusPending {
			out = append(out, s)
		}
	}
	return out
}

// Count returns the total number of submissions.
func (b *Board) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.submissions)
}

// HighRiskClassRequired reports whether the supplied RiskClass
// requires a change-board submission. The harness consults this
// before every mutation.
func HighRiskClassRequired(c RiskClass) bool {
	switch c {
	case RiskCrypto, RiskPayment, RiskPIIAdjacent,
		RiskNewDependency, RiskNewNetwork, RiskNewMCPServer:
		return true
	}
	return false
}

// _ keeps the binary import visible when tests evolve.
var _ = binary.BigEndian
