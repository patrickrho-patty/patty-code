// Package sandbox is the connector-side sandbox-baseline policy
// (harness feature plan E4). For sensitive repositories, the
// relay's sandbox definition requires remote-sandbox execution as
// the enforced baseline; local execution is an explicit, audited
// exception.
//
// The connector consults this policy before every shell/agent
// execution. Local execution against a sensitive repo is refused
// unless the developer explicitly opts in (and the opt-in is
// recorded as evidence).
package sandbox

import (
	"errors"
	"fmt"
	"sync"
)

// RiskClass is the sandbox risk tier the relay assigns to a
// repository. Sensitive repos get remote-only execution.
type RiskClass string

const (
	RiskLow       RiskClass = "LOW"
	RiskMedium    RiskClass = "MEDIUM"
	RiskHigh      RiskClass = "HIGH"
	RiskSensitive RiskClass = "SENSITIVE"
)

// SandboxMode is the execution mode the policy mandates.
type SandboxMode string

const (
	ModeRemoteOnly    SandboxMode = "REMOTE_ONLY"
	ModeLocalAllowed  SandboxMode = "LOCAL_ALLOWED"
	ModeLocalRequired SandboxMode = "LOCAL_REQUIRED" // offline / air-gap
)

// Policy is the cached view of the relay's sandbox definition for
// a repository. The harness consults this before every shell/agent
// execution.
type Policy struct {
	OrganizationID string
	RepositoryID   string
	RiskClass      RiskClass
	Mode           SandboxMode
	// MaxLocalExecutionsPerDay bounds local-execution count so a
	// buggy harness can't accidentally bypass the sandbox by
	// counting on opt-in.
	MaxLocalExecutionsPerDay int
	IssuedAtUnixMs           int64
}

// RequiresRemote reports whether the policy mandates remote-only
// execution. SENSITIVE repos always require remote.
func (p *Policy) RequiresRemote() bool {
	if p == nil {
		return false
	}
	return p.Mode == ModeRemoteOnly || p.RiskClass == RiskSensitive
}

// AllowsLocal reports whether the policy permits local execution
// at all. A ModeLocalRequired harness (air-gap mode) ALWAYS
// allows local; otherwise only LOCAL_ALLOWED does.
func (p *Policy) AllowsLocal() bool {
	if p == nil {
		return true
	}
	switch p.Mode {
	case ModeLocalAllowed, ModeLocalRequired:
		return true
	}
	return false
}

// PolicyStore is the harness-side cached view of the relay's
// per-repo sandbox policies. The harness calls Set when the relay
// pushes refreshed state.
type PolicyStore struct {
	mu       sync.RWMutex
	policies map[string]*Policy // key: repoID
	// LocalExecutionBudget tracks today's local-execution count
	// per repo. Reset at midnight UTC.
	LocalExecutionBudget map[string]*LocalBudget
}

// LocalBudget is the harness-side counter for local-execution
// usage on a single repository within the current day.
type LocalBudget struct {
	RepoID  string
	DayUnix int64 // days since epoch
	Used    int
}

// NewPolicyStore constructs an empty store.
func NewPolicyStore() *PolicyStore {
	return &PolicyStore{
		policies:             make(map[string]*Policy),
		LocalExecutionBudget: make(map[string]*LocalBudget),
	}
}

// Set installs or replaces the policy for the supplied repository.
func (s *PolicyStore) Set(p *Policy) {
	if p == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[p.RepositoryID] = p
}

// Get returns the policy for the supplied repository.
func (s *PolicyStore) Get(repoID string) *Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policies[repoID]
}

// ExecutionDecision is the verdict the harness consults before
// every shell/agent execution.
type ExecutionDecision struct {
	Allow         bool
	Reason        string
	ReasonKo      string
	Mode          SandboxMode
	BlockByPolicy bool
}

// CheckExecution exercises E4: the harness consults the policy
// before dispatching a local execution. The decision respects the
// policy's RiskClass and Mode. SENSITIVE repos always require
// remote unless the developer explicitly opts in (LocalOptIn=true).
func (s *PolicyStore) CheckExecution(repoID string, localOptIn bool) ExecutionDecision {
	policy := s.Get(repoID)
	if policy == nil {
		// No policy installed yet: the harness fails closed for
		// unknown repos. The relay pushes the policy at session
		// start; until then, local execution is blocked.
		return ExecutionDecision{
			Allow:         false,
			Reason:        fmt.Sprintf("no sandbox policy for repo %s", repoID),
			ReasonKo:      fmt.Sprintf("%s 저장소에 샌드박스 정책이 없습니다", repoID),
			BlockByPolicy: true,
		}
	}
	if !policy.RequiresRemote() {
		return ExecutionDecision{
			Allow: true,
			Mode:  policy.Mode,
		}
	}
	// Sensitive/RemoteOnly: needs either explicit opt-in or
	// already-exhausted daily local budget.
	if policy.RequiresRemote() && !localOptIn {
		return ExecutionDecision{
			Allow:         false,
			Reason:        fmt.Sprintf("repo %s requires remote execution; no local opt-in", repoID),
			ReasonKo:      fmt.Sprintf("%s 저장소는 원격 실행이 필요합니다 (로컬 동의 없음)", repoID),
			Mode:          ModeRemoteOnly,
			BlockByPolicy: true,
		}
	}
	if s.LocalBudgetExceeded(repoID, policy) {
		return ExecutionDecision{
			Allow:         false,
			Reason:        fmt.Sprintf("repo %s local-execution budget exhausted", repoID),
			ReasonKo:      fmt.Sprintf("%s 저장소의 로컬 실행 한도가 소진되었습니다", repoID),
			Mode:          ModeRemoteOnly,
			BlockByPolicy: true,
		}
	}
	return ExecutionDecision{
		Allow: true,
		Mode:  ModeLocalAllowed,
	}
}

// RecordLocalExecution bumps the per-day local-execution counter.
// Returns the new count.
func (s *PolicyStore) RecordLocalExecution(repoID string, nowMs int64) int {
	day := nowMs / 86_400_000
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.LocalExecutionBudget[repoID]
	if b == nil || b.DayUnix != day {
		b = &LocalBudget{RepoID: repoID, DayUnix: day, Used: 0}
	}
	b.Used++
	s.LocalExecutionBudget[repoID] = b
	return b.Used
}

// LocalBudgetExceeded reports whether the supplied policy's
// MaxLocalExecutionsPerDay budget has been spent.
func (s *PolicyStore) LocalBudgetExceeded(repoID string, policy *Policy) bool {
	if policy == nil || policy.MaxLocalExecutionsPerDay <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.LocalExecutionBudget[repoID]
	if b == nil {
		return false
	}
	return b.Used >= policy.MaxLocalExecutionsPerDay
}

// LocalExecutionCount returns the current-day local-execution
// count for the supplied repository. Surfaced in the E1 status
// bar.
func (s *PolicyStore) LocalExecutionCount(repoID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.LocalExecutionBudget[repoID]
	if b == nil {
		return 0
	}
	return b.Used
}

// ErrPolicyStoreMissingClient covers the trivial boundary for
// misuse.
var ErrPolicyStoreMissingClient = errors.New("sandbox: policy store not configured")
