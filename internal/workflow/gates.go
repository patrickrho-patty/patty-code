// Package workflow gates the harness's command path on the
// enterprise workflow policies the relay pushes via the policy
// epoch. The connector-side guard mirrors the relay's
// `internal/korean/service.go` so the harness fails closed before
// any workflow-gated action reaches the relay.
//
// Feature plan D:
//   - D1 mandatory policy-acknowledgement gate
//   - D2 change-control-board submission
//   - D3 change-freeze compliance
//   - D4 coding-standard & architecture packs
//   - D5 forced harness version & release-ring
//   - D6 emergency model recall
package workflow

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Severity mirrors the korean service's severity scale.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityAdvisory Severity = "advisory"
	SeverityBlock    Severity = "block"
)

// ChangeFreeze is the connector's cached view of an active
// freeze on the target repository (PRD §33.13, PAPER §38.4).
type ChangeFreeze struct {
	OrganizationID string
	Reason         string
	ReasonKo       string
	AffectedRepos  []string
	AllowedActions []string
	InitiatedBy    string
	StartedAt      time.Time
	NotAfter       time.Time
}

// AllowsAction reports whether the freeze permits the supplied
// action. The default action allow-list is empty, which means
// only `read`/`review`/`test` are allowed (the freeze semantics
// from PRD §33.13).
func (f *ChangeFreeze) AllowsAction(action string) bool {
	if f == nil {
		return true
	}
	for _, a := range f.AllowedActions {
		if a == action || a == "*" {
			return true
		}
	}
	switch action {
	case "read", "review", "test":
		return true
	}
	return false
}

// Affects reports whether the freeze covers the supplied
// repository.
func (f *ChangeFreeze) Affects(repoID string) bool {
	if f == nil {
		return false
	}
	for _, r := range f.AffectedRepos {
		if r == repoID {
			return true
		}
	}
	return false
}

// AcknowledgementRequirement is the connector's cached view of
// an outstanding policy acknowledgement (PRD §33.6, D1). The
// harness MUST block configured high-risk workflows until the
// developer acknowledges.
type AcknowledgementRequirement struct {
	OrganizationID string
	PolicyEpochID  string
	Summary        string
	SummaryKo      string
	Blocking       bool
	NotAfter       time.Time
}

// ModelRecall is the connector's cached view of an active model
// recall (PRD §33.9, D6). The harness MUST refuse to dispatch
// to a recalled model.
type ModelRecall struct {
	OrganizationID string
	RecalledModel  string
	Replacement    string
	Reason         string
	NotAfter       time.Time
}

// VersionRequirement is the connector's cached view of the
// harness's release-ring + minimum-version requirement (PRD
// §33.10, D5). The harness refuses to start if the local build
// is below the minimum or on a blocked ring.
type VersionRequirement struct {
	OrganizationID string
	MinVersion     string
	BlockedRings   []string
	ReleaseRing    string
	CurrentVersion string
	CurrentRing    string
}

// IsSatisfied reports whether the harness's local build meets
// the relay's requirement.
func (r *VersionRequirement) IsSatisfied() bool {
	if r == nil {
		return true
	}
	// SemVer comparison: split on dots. The harness's build is
	// below the requirement iff any segment is smaller.
	if r.CurrentVersion == "" || r.MinVersion == "" {
		// Without a comparison primitive, fall back to a string
		// inequality check. The harness persists this in the audit
		// log so operators can manually verify.
		return r.CurrentVersion == r.MinVersion
	}
	if r.CurrentVersion == r.MinVersion {
		return true
	}
	return compareVersions(r.CurrentVersion, r.MinVersion) >= 0
}

// IsBlockedRing reports whether the harness's current ring is on
// the relay's blocked list.
func (r *VersionRequirement) IsBlockedRing() bool {
	if r == nil {
		return false
	}
	for _, ring := range r.BlockedRings {
		if ring == r.CurrentRing {
			return true
		}
	}
	return false
}

// CodingStandard is the connector's cached view of an org coding
// standard (PRD §33.11, D4). The harness uses it to surface
// plain-Korean explanations when a rule blocks work.
type CodingStandard struct {
	OrganizationID string
	RuleID         string
	Severity       Severity
	Description    string
	DescriptionKo  string
	BlockPattern   string
}

// GatesClient is the harness-side cached view of the relay's
// enterprise workflow state. The harness queries this on every
// high-risk command dispatch; the relay pushes refreshes via the
// policy epoch.
type GatesClient struct {
	mu             sync.RWMutex
	organizationID string
	policyEpochID  string
	freeze         *ChangeFreeze
	acks           []AcknowledgementRequirement
	recalls        []ModelRecall
	versionReq     *VersionRequirement
	standards      []CodingStandard
	// The harness's local identity.
	harnessVersion string
	harnessRing    string
	// metrics: surfaced in the E1 status bar.
	AckedAt              map[string]int64 // ruleID -> unix-ms
	BlockedDispatchCount int64
}

// NewGatesClient constructs an empty gates client. The harness
// calls `SetEpoch` / `SetFreeze` / etc. when the relay pushes
// refreshed state via the policy epoch.
func NewGatesClient(organizationID, harnessVersion, harnessRing string) *GatesClient {
	return &GatesClient{
		organizationID: organizationID,
		harnessVersion: harnessVersion,
		harnessRing:    harnessRing,
		AckedAt:        make(map[string]int64),
	}
}

// SetEpoch updates the bound policy epoch. The harness consults
// this when the relay pushes a fresh epoch.
func (g *GatesClient) SetEpoch(epochID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.policyEpochID = epochID
}

// SetFreeze installs or clears the active change-freeze.
func (g *GatesClient) SetFreeze(freeze *ChangeFreeze) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.freeze = freeze
}

// SetAcknowledgements installs the outstanding acknowledgement
// requirements. The harness must block until each is acked.
func (g *GatesClient) SetAcknowledgements(acks []AcknowledgementRequirement) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.acks = append([]AcknowledgementRequirement(nil), acks...)
}

// AcknowledgePolicy marks the supplied acknowledgement rule as
// acked at the supplied unix-ms timestamp. The harness invokes
// this after the developer dismisses the policy gate in the IDE.
func (g *GatesClient) AcknowledgePolicy(ruleID string, nowMs int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, a := range g.acks {
		if a.PolicyEpochID == ruleID {
			g.AckedAt[ruleID] = nowMs
			return nil
		}
	}
	return fmt.Errorf("workflow: unknown acknowledgement %s", ruleID)
}

// SetRecalls installs or clears the active model recalls.
func (g *GatesClient) SetRecalls(recalls []ModelRecall) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.recalls = append([]ModelRecall(nil), recalls...)
}

// SetVersionRequirement installs or clears the harness version
// requirement.
func (g *GatesClient) SetVersionRequirement(req *VersionRequirement) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.versionReq = req
}

// SetStandards installs or clears the coding standards.
func (g *GatesClient) SetStandards(standards []CodingStandard) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.standards = append([]CodingStandard(nil), standards...)
}

// GateDecision is the verdict the harness consults before a
// high-risk dispatch. Allow means the harness may proceed;
// Block means the dispatch MUST be refused with the supplied
// Korean explanation surfaced in the IDE.
type GateDecision struct {
	Allow         bool
	Reason        string
	ReasonKo      string
	BlockedBy     string // rule ID / freeze / recall / version
	KoreanMessage string
}

// CheckDispatch is the single entry point the harness calls
// before every high-risk command (file write, dependency add, MCP
// call, etc.). The method returns a GateDecision; Allow=true
// means proceed, Allow=false means the harness MUST refuse to
// dispatch and surface ReasonKo in the IDE.
//
// The action is a stable string the relay and the harness agree
// on, e.g. "file_write", "tool_use", "mcp_call", "network_dial".
func (g *GatesClient) CheckDispatch(action, repoID, modelID string) GateDecision {
	// Compute the decision under the read lock; release the read
	// lock before bumping the counter under the write lock. This
	// avoids the manual lock-switch pattern that can deadlock
	// under contention.
	g.mu.RLock()
	dec := g.evaluate(action, repoID, modelID)
	g.mu.RUnlock()
	if !dec.Allow {
		g.mu.Lock()
		g.BlockedDispatchCount++
		g.mu.Unlock()
	}
	return dec
}

// evaluate is the read-locked half of CheckDispatch. It walks
// the gate precedence: version, ring, recall, freeze, ack.
func (g *GatesClient) evaluate(action, repoID, modelID string) GateDecision {
	// D5: forced harness version + release ring.
	if g.versionReq != nil && !g.versionReq.IsSatisfied() {
		return GateDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("harness version %s below required %s", g.versionReq.CurrentVersion, g.versionReq.MinVersion),
			ReasonKo:  fmt.Sprintf("허브 버전 %s이(가) 최소 요구 버전 %s보다 낮습니다", g.versionReq.CurrentVersion, g.versionReq.MinVersion),
			BlockedBy: "version-requirement",
		}
	}
	if g.versionReq != nil && g.versionReq.IsBlockedRing() {
		return GateDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("release ring %s is blocked", g.versionReq.CurrentRing),
			ReasonKo:  fmt.Sprintf("릴리스 링 %s은(는) 차단되었습니다", g.versionReq.CurrentRing),
			BlockedBy: "blocked-ring",
		}
	}
	// D6: model recall.
	for _, r := range g.recalls {
		if r.RecalledModel == modelID {
			return GateDecision{
				Allow:     false,
				Reason:    fmt.Sprintf("model %s has been recalled: %s", modelID, r.Reason),
				ReasonKo:  fmt.Sprintf("모델 %s은(는) 회수되었습니다. 대체 모델: %s", modelID, r.Replacement),
				BlockedBy: "model-recall",
			}
		}
	}
	// D3: change-freeze compliance.
	if g.freeze != nil && g.freeze.Affects(repoID) && !g.freeze.AllowsAction(action) {
		return GateDecision{
			Allow:         false,
			Reason:        fmt.Sprintf("action %s blocked by change-freeze on %s", action, repoID),
			ReasonKo:      fmt.Sprintf("%s 저장소에 변경 동결이 적용되었습니다. 사유: %s", repoID, g.freeze.ReasonKo),
			BlockedBy:     "change-freeze",
			KoreanMessage: g.freeze.ReasonKo,
		}
	}
	// D1: mandatory policy acknowledgement.
	for _, ack := range g.acks {
		if ack.Blocking && g.AckedAt[ack.PolicyEpochID] == 0 {
			return GateDecision{
				Allow:     false,
				Reason:    fmt.Sprintf("policy epoch %s requires acknowledgement", ack.PolicyEpochID),
				ReasonKo:  ack.SummaryKo,
				BlockedBy: "policy-acknowledgement",
			}
		}
	}
	return GateDecision{Allow: true}
}

// CheckCodingStandard exercises D4: the harness consults the
// cached standards before accepting a write. The harness passes
// the file path + content; the method returns the first matching
// standard's explanation (if any). An empty BlockPattern is
// treated as a configuration error and never matches (a rule
// without a pattern would block everything, which is rarely
// intentional — admin UI should reject empty patterns).
func (g *GatesClient) CheckCodingStandard(filePath, content string) *CodingStandard {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, s := range g.standards {
		if s.BlockPattern == "" {
			continue
		}
		if !strings.Contains(filePath, s.BlockPattern) && !strings.Contains(content, s.BlockPattern) {
			continue
		}
		s := s
		return &s
	}
	return nil
}

// BlockedCount returns the E1 status-bar metric for blocked
// dispatches.
func (g *GatesClient) BlockedCount() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.BlockedDispatchCount
}

// compareVersions performs a SemVer-style dotted integer
// comparison. Returns -1 if a < b, 0 if equal, 1 if a > b. Missing
// segments on one side are treated as the implicit zero on the
// other side, so "1.0" < "1.0.1" because the shorter string has
// 0 on the third segment where the longer has 1.
func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(as) {
			fmt.Sscanf(as[i], "%d", &ai)
		}
		if i < len(bs) {
			fmt.Sscanf(bs[i], "%d", &bi)
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	// If one side has more segments and they were all zero so far,
	// the longer side is greater.
	if len(as) != len(bs) {
		if len(as) > len(bs) {
			return 1
		}
		return -1
	}
	return 0
}

// GateError wraps a GateDecision so callers can return it from
// helper functions and pattern-match on the BlockedBy field.
type GateError struct {
	Decision GateDecision
}

func (e *GateError) Error() string {
	if e.Decision.ReasonKo != "" {
		return e.Decision.ReasonKo
	}
	return e.Decision.Reason
}

// ErrGateMissingClient covers the trivial boundary: a nil
// GatesClient signals a misconfigured harness.
var ErrGateMissingClient = errors.New("workflow: gates client not configured")
