// Package operational is the harness-side operational awareness
// layer. The connector caches the relay's capacity-lease,
// work-slot, and queue state so the harness UI can show the
// developer WHY they're throttled (PRD \u00a710C.3, \u00a710C.7).
//
// The relay's `internal/publiccloud/service.go` issues
// WorkSlot/CapacityLease records. The connector caches the latest
// values and re-fetches on every flush of the harness's turn
// loop. The connector also tracks the employment-decision
// guardrails (PRD \u00a726, \u00a727): the client respects the same
// privacy boundary as the console and never surfaces raw-activity
// surveillance to managers.
package operational

import (
	"fmt"
	"sync"
	"time"
)

// WorkSlot is the harness's cached view of the developer's work
// slot class. The class controls the queue priority and capacity
// caps the relay enforces on the harness's behalf.
type WorkSlot struct {
	OrganizationID string
	SlotClass      string // free, pro, enterprise, government
	TokenBudget    int64  // tokens per turn
	QueuePosition  int    // current position in the relay's queue
	QueueDepth     int    // total items ahead in the queue
	UpdatedAt      time.Time
}

// IsThrottled reports whether the harness is currently behind a
// throttling queue. A slot class of "free" with QueuePosition > 0
// indicates the relay is rate-limiting the developer.
func (w *WorkSlot) IsThrottled() bool {
	if w == nil {
		return false
	}
	return w.SlotClass == "free" && w.QueuePosition > 0
}

// CapacityLease is the harness's cached view of the active
// capacity lease. The lease bounds the harness's per-turn
// resource consumption.
type CapacityLease struct {
	OrganizationID  string
	LeaseID         string
	IssuedAt        time.Time
	NotAfter        time.Time
	TokenQuota      int64
	TokensConsumed  int64
	ContextQuota    int64
	ContextConsumed int64
	RequestQuota    int64
	RequestsIssued  int64
}

// RemainingTokens returns the tokens the harness can still spend
// this turn.
func (c *CapacityLease) RemainingTokens() int64 {
	if c == nil {
		return 0
	}
	return c.TokenQuota - c.TokensConsumed
}

// QuotaUtilization returns the fraction (0..1) of the harness's
// per-turn token budget consumed. Operators see this in the IDE
// status bar.
func (c *CapacityLease) QuotaUtilization() float64 {
	if c == nil || c.TokenQuota == 0 {
		return 0
	}
	return float64(c.TokensConsumed) / float64(c.TokenQuota)
}

// WorkIntelVisibility is the harness-side view of the
// employment-decision guardrails (PRD \u00a726, \u00a727). The
// client respects the same privacy boundary as the console and
// surfaces aggregated/signed signals only.
type WorkIntelVisibility struct {
	OrganizationID string
	// CanSeeRawActivity reports whether the local user has
	// permission to see raw activity. The harness UI uses this
	// to hide the raw stream and show only aggregated metrics
	// when the user is a manager without raw-activity access.
	CanSeeRawActivity bool
	// CanSeeAggregatedMetrics reports whether the local user has
	// permission to see aggregated metrics. Most operators
	// (engineers, leads) have this; managers without
	// raw-activity access still see this.
	CanSeeAggregatedMetrics bool
	// AggregationFloor is the minimum aggregation window for
	// any signal the harness surfaces. Raw signals below this
	// floor are suppressed.
	AggregationFloorSeconds int
}

// AwarenessClient is the harness's cached view of the operational
// state. The harness refreshes the client on every flush of the
// turn loop. The client surfaces an E1 status-bar snapshot.
type AwarenessClient struct {
	mu              sync.RWMutex
	organizationID string
	userID         string
	slot           *WorkSlot
	capacity       *CapacityLease
	visibility     *WorkIntelVisibility
	// metrics: surfaced in the E1 status bar.
	lastRefreshAtMs   int64
	refreshFailures   int64
	throttleEventCount int64
}

// NewAwarenessClient constructs an empty client. The harness
// calls SetWorkSlot / SetCapacityLease / SetVisibility when the
// relay pushes refreshed state.
func NewAwarenessClient(organizationID, userID string) *AwarenessClient {
	return &AwarenessClient{
		organizationID: organizationID,
		userID:         userID,
	}
}

// SetWorkSlot installs the harness's cached work-slot state. The
// caller MUST call Refresh() to stamp lastRefreshAtMs; the Set*
// methods don't update it so the relay-pushed refresh and the
// connector's poll cycle are decoupled.
func (a *AwarenessClient) SetWorkSlot(slot *WorkSlot) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.slot = slot
}

// SetCapacityLease installs the cached capacity lease.
func (a *AwarenessClient) SetCapacityLease(lease *CapacityLease) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.capacity = lease
}

// SetVisibility installs the cached work-intel visibility rules.
func (a *AwarenessClient) SetVisibility(v *WorkIntelVisibility) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.visibility = v
}

// Refresh records a successful state refresh. The harness invokes
// this when it pulls fresh operational state from the relay. The
// timestamp drives the freshness signal surfaced in the E1
// status bar (stale-state detection).
func (a *AwarenessClient) Refresh(nowMs int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastRefreshAtMs = nowMs
}

// RecordRefreshFailure surfaces a failed refresh to the audit log.
func (a *AwarenessClient) RecordRefreshFailure() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.refreshFailures++
}

// RecordThrottleEvent surfaces a throttling transition so the
// operator sees when the harness moved into/out of the queue.
func (a *AwarenessClient) RecordThrottleEvent() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.throttleEventCount++
}

// Snapshot is the E1 status-bar view of the operational state.
type Snapshot struct {
	OrganizationID      string
	UserID             string
	SlotClass           string
	TokenBudget         int64
	TokensConsumed      int64
	QuotaUtilization    float64
	QueuePosition       int
	QueueDepth          int
	IsThrottled         bool
	LeaseNotAfter       time.Time
	CanSeeRawActivity   bool
	LastRefreshAtMs     int64
	RefreshFailures     int64
	ThrottleEventCount  int64
}

// Snapshot returns a thread-safe copy of the current state.
func (a *AwarenessClient) Snapshot() Snapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	snap := Snapshot{
		OrganizationID:     a.organizationID,
		UserID:            a.userID,
		LastRefreshAtMs:    a.lastRefreshAtMs,
		RefreshFailures:    a.refreshFailures,
		ThrottleEventCount: a.throttleEventCount,
	}
	if a.slot != nil {
		snap.SlotClass = a.slot.SlotClass
		snap.TokenBudget = a.slot.TokenBudget
		snap.QueuePosition = a.slot.QueuePosition
		snap.QueueDepth = a.slot.QueueDepth
		snap.IsThrottled = a.slot.IsThrottled()
	}
	if a.capacity != nil {
		snap.TokensConsumed = a.capacity.TokensConsumed
		snap.QuotaUtilization = a.capacity.QuotaUtilization()
		snap.LeaseNotAfter = a.capacity.NotAfter
	}
	if a.visibility != nil {
		snap.CanSeeRawActivity = a.visibility.CanSeeRawActivity
	}
	return snap
}

// CanDisplayRawActivity is the E6 boundary check: the harness UI
// shows raw activity only when both the visibility rule and the
// local user have permission. The default-deny path (false when
// no visibility is installed) prevents accidental raw-stream
// exposure when the relay hasn't pushed a visibility rule yet.
func (a *AwarenessClient) CanDisplayRawActivity() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.visibility == nil {
		return false
	}
	return a.visibility.CanSeeRawActivity
}

// SanityCheckFormat formats the snapshot as a one-line status
// string for the harness UI footer.
func (s Snapshot) SanityCheckFormat() string {
	if s.IsThrottled {
		return fmt.Sprintf("스롯 %s | 토큰 %d/%d (%.0f%%) | 큐 %d/%d | 동결",
			s.SlotClass, s.TokensConsumed, s.TokenBudget, s.QuotaUtilization*100,
			s.QueuePosition, s.QueueDepth)
	}
	return fmt.Sprintf("슬롯 %s | 토큰 %d/%d (%.0f%%) | 큐 %d/%d",
		s.SlotClass, s.TokensConsumed, s.TokenBudget, s.QuotaUtilization*100,
		s.QueuePosition, s.QueueDepth)
}
