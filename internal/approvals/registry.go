// Package approvals is the connector-side tool/MCP/network/secret
// approval gate (harness feature plan C3 + C4). The harness
// consults a cached view of the relay's tool registry + per-org
// approval state before any tool/MCP call or outbound network.
//
// The relay's tool registry surfaces every registered tool + its
// approval status (REQUIRE_REVIEW | APPROVED | BLOCKED). The
// connector caches the latest snapshot and consults it on every
// tool invocation; the harness fails closed when the registry
// says BLOCKED, prompts when it says REQUIRE_REVIEW, and emits an
// audit event for every APPROVED call.
package approvals

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ApprovalStatus is the per-tool approval verdict the relay
// pushes via the policy epoch.
type ApprovalStatus string

const (
	StatusApproved      ApprovalStatus = "APPROVED"
	StatusRequireReview ApprovalStatus = "REQUIRE_REVIEW"
	StatusBlocked       ApprovalStatus = "BLOCKED"
	StatusPending       ApprovalStatus = "PENDING"
)

// ToolRegistration is the connector's cached view of one
// registered tool.
type ToolRegistration struct {
	ToolID           string
	DisplayName      string
	Version          string
	Category         string
	Status           ApprovalStatus
	Description      string
	DescriptionKo    string
	OrganizationID   string
	ApprovedAtUnixMs int64
	NotAfterUnixMs   int64
}

// IsActive reports whether the registration is in force at the
// supplied unix-ms timestamp.
func (t *ToolRegistration) IsActive(nowMs int64) bool {
	if t == nil {
		return false
	}
	if t.NotAfterUnixMs > 0 && nowMs >= t.NotAfterUnixMs {
		return false
	}
	return true
}

// NetworkGrant is the connector's cached view of an outbound
// network grant (PRD §17.4).
type NetworkGrant struct {
	OrganizationID string
	HostPattern    string
	Port           int
	IssuedAt       time.Time
	NotAfter       time.Time
	TokenBudget    int64
	TokensConsumed int64
}

// AllowsHost reports whether the grant covers the supplied host.
func (n *NetworkGrant) AllowsHost(host string) bool {
	if n == nil {
		return false
	}
	return matchGlob(n.HostPattern, host)
}

// matchGlob is a tiny glob matcher used for network-grant host
// patterns. It supports "*" (any sequence) and "?" (any single
// char). Anything else is treated as a literal.
func matchGlob(pattern, host string) bool {
	pi, hi := 0, 0
	star := -1
	match := 0
	for hi < len(host) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == host[hi]) {
			pi++
			hi++
			continue
		}
		if pi < len(pattern) && pattern[pi] == '*' {
			star = pi
			match = hi
			pi++
			continue
		}
		if star != -1 {
			pi = star + 1
			match++
			hi = match
			continue
		}
		return false
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// SecretGrant is the connector's cached view of a secret broker
// grant (PRD §17.5).
type SecretGrant struct {
	OrganizationID string
	SecretID       string
	IssuedAt       time.Time
	NotAfter       time.Time
	MaxReads       int64
	ReadsIssued    int64
}

// IsExhausted reports whether the grant has spent its read quota.
func (s *SecretGrant) IsExhausted() bool {
	if s == nil {
		return true
	}
	return s.MaxReads > 0 && s.ReadsIssued >= s.MaxReads
}

// Registry is the connector-side cached view of the relay's
// approval state.
type Registry struct {
	mu                sync.RWMutex
	tools             map[string]*ToolRegistration
	networks          []*NetworkGrant
	secrets           map[string]*SecretGrant
	lastRefreshMs     int64
	toolAllowCount    int64
	toolDenyCount     int64
	networkAllowCount int64
	networkDenyCount  int64
	secretAllowCount  int64
	secretDenyCount   int64
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:   make(map[string]*ToolRegistration),
		secrets: make(map[string]*SecretGrant),
	}
}

// SetTools replaces the tool registry snapshot.
func (r *Registry) SetTools(tools []*ToolRegistration, nowMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = make(map[string]*ToolRegistration, len(tools))
	for _, t := range tools {
		r.tools[t.ToolID] = t
	}
	r.lastRefreshMs = nowMs
}

// SetNetworkGrants replaces the network-grant snapshot.
func (r *Registry) SetNetworkGrants(grants []*NetworkGrant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.networks = append([]*NetworkGrant(nil), grants...)
}

// SetSecretGrants replaces the secret-grant snapshot.
func (r *Registry) SetSecretGrants(grants []*SecretGrant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets = make(map[string]*SecretGrant, len(grants))
	for _, g := range grants {
		r.secrets[g.SecretID] = g
	}
}

// incrementDeny bumps the appropriate counter under the write
// lock. Called from Check* methods after they have released the
// read lock.
func (r *Registry) incrementDeny(kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch kind {
	case "tool":
		r.toolDenyCount++
	case "network":
		r.networkDenyCount++
	case "secret":
		r.secretDenyCount++
	}
}

// incrementAllow bumps the appropriate counter under the write
// lock.
func (r *Registry) incrementAllow(kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch kind {
	case "tool":
		r.toolAllowCount++
	case "network":
		r.networkAllowCount++
	case "secret":
		r.secretAllowCount++
	}
}

// ToolDecision is the verdict the harness consults before a tool
// invocation.
type ToolDecision struct {
	Allow     bool
	Reason    string
	ReasonKo  string
	BlockedBy string
}

// CheckTool exercises C3: the harness consults the registry
// before invoking a tool.
func (r *Registry) CheckTool(toolID string, nowMs int64) ToolDecision {
	r.mu.RLock()
	t, ok := r.tools[toolID]
	r.mu.RUnlock()
	if !ok {
		r.incrementDeny("tool")
		return ToolDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("tool %s is not registered with PCCP", toolID),
			ReasonKo:  fmt.Sprintf("도구 %s은(는) PCCP에 등록되지 않았습니다", toolID),
			BlockedBy: "unregistered-tool",
		}
	}
	if !t.IsActive(nowMs) {
		r.incrementDeny("tool")
		return ToolDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("tool %s registration expired", toolID),
			ReasonKo:  fmt.Sprintf("도구 %s 등록이 만료되었습니다", toolID),
			BlockedBy: "expired-registration",
		}
	}
	switch t.Status {
	case StatusApproved:
		r.incrementAllow("tool")
		return ToolDecision{Allow: true}
	case StatusRequireReview:
		r.incrementDeny("tool")
		return ToolDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("tool %s requires admin review", toolID),
			ReasonKo:  fmt.Sprintf("도구 %s은(는) 관리자 승인이 필요합니다", toolID),
			BlockedBy: "require-review",
		}
	case StatusBlocked:
		r.incrementDeny("tool")
		return ToolDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("tool %s is blocked by admin policy", toolID),
			ReasonKo:  fmt.Sprintf("도구 %s은(는) 관리 정책에 의해 차단되었습니다", toolID),
			BlockedBy: "blocked-tool",
		}
	default:
		r.incrementDeny("tool")
		return ToolDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("tool %s is pending admin review", toolID),
			ReasonKo:  fmt.Sprintf("도구 %s은(는) 관리자 검토 대기 중입니다", toolID),
			BlockedBy: "pending-review",
		}
	}
}

// NetworkDecision is the verdict for an outbound network dial.
type NetworkDecision struct {
	Allow     bool
	Reason    string
	ReasonKo  string
	BlockedBy string
	GrantID   string
}

// CheckNetwork exercises C4: the harness consults the network-grant
// cache before dialing a non-loopback address.
func (r *Registry) CheckNetwork(host string) NetworkDecision {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return NetworkDecision{Allow: true, Reason: "loopback"}
	}
	r.mu.RLock()
	var matched *NetworkGrant
	for _, g := range r.networks {
		if g.AllowsHost(host) {
			matched = g
			break
		}
	}
	r.mu.RUnlock()
	if matched == nil {
		r.incrementDeny("network")
		return NetworkDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("no network grant covers host %s", host),
			ReasonKo:  fmt.Sprintf("호스트 %s을(를) 허용하는 네트워크 그랜트가 없습니다", host),
			BlockedBy: "no-grant",
		}
	}
	if matched.TokenBudget > 0 && matched.TokensConsumed >= matched.TokenBudget {
		r.incrementDeny("network")
		return NetworkDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("network grant %s exhausted for host %s", matched.HostPattern, host),
			ReasonKo:  fmt.Sprintf("네트워크 그랜트 %s이(가) 호스트 %s에 대해 소진되었습니다", matched.HostPattern, host),
			BlockedBy: "exhausted-grant",
			GrantID:   matched.HostPattern,
		}
	}
	r.incrementAllow("network")
	return NetworkDecision{Allow: true, GrantID: matched.HostPattern}
}

// SecretDecision is the verdict for a secret read.
type SecretDecision struct {
	Allow     bool
	Reason    string
	ReasonKo  string
	BlockedBy string
	GrantID   string
}

// CheckSecret exercises C4 (secret broker).
func (r *Registry) CheckSecret(secretID string) SecretDecision {
	r.mu.RLock()
	g, ok := r.secrets[secretID]
	r.mu.RUnlock()
	if !ok {
		r.incrementDeny("secret")
		return SecretDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("no secret grant issued for %s", secretID),
			ReasonKo:  fmt.Sprintf("%s에 발급된 시크릿 그랜트가 없습니다", secretID),
			BlockedBy: "no-grant",
		}
	}
	if g.IsExhausted() {
		r.incrementDeny("secret")
		return SecretDecision{
			Allow:     false,
			Reason:    fmt.Sprintf("secret grant for %s exhausted", secretID),
			ReasonKo:  fmt.Sprintf("%s에 대한 시크릿 그랜트가 소진되었습니다", secretID),
			BlockedBy: "exhausted-grant",
			GrantID:   secretID,
		}
	}
	r.incrementAllow("secret")
	return SecretDecision{Allow: true, GrantID: secretID}
}

// Counter getters. They are named differently from the fields to
// avoid the field/method shadowing that Go disallows.
func (r *Registry) ToolAllowCountValue() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.toolAllowCount
}

func (r *Registry) ToolDenyCountValue() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.toolDenyCount
}

func (r *Registry) NetworkAllowCountValue() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.networkAllowCount
}

func (r *Registry) NetworkDenyCountValue() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.networkDenyCount
}

func (r *Registry) SecretAllowCountValue() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.secretAllowCount
}

func (r *Registry) SecretDenyCountValue() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.secretDenyCount
}

// LastRefreshMs returns the timestamp of the most recent refresh.
func (r *Registry) LastRefreshMs() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastRefreshMs
}

// ErrRegistryMissingClient is the trivial boundary for misuse.
var ErrRegistryMissingClient = errors.New("approvals: registry not configured")
