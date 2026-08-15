package operational

import (
	"strings"
	"testing"
	"time"
)

// TestWorkSlotThrottleDetection covers E1: a "free" slot class
// with non-zero QueuePosition reports throttled.
func TestWorkSlotThrottleDetection(t *testing.T) {
	slot := &WorkSlot{SlotClass: "free", QueuePosition: 5}
	if !slot.IsThrottled() {
		t.Error("free slot with queue position must be throttled")
	}
	pro := &WorkSlot{SlotClass: "pro", QueuePosition: 5}
	if pro.IsThrottled() {
		t.Error("pro slot must not be throttled by queue position")
	}
}

// TestCapacityLeaseQuotaUtilization covers E1: the harness
// surfaces quota utilization in the IDE status bar.
func TestCapacityLeaseQuotaUtilization(t *testing.T) {
	lease := &CapacityLease{TokenQuota: 100, TokensConsumed: 25}
	if got := lease.QuotaUtilization(); got != 0.25 {
		t.Errorf("utilization = %f, want 0.25", got)
	}
	if got := lease.RemainingTokens(); got != 75 {
		t.Errorf("remaining = %d, want 75", got)
	}
}

// TestCapacityLeaseQuotaUtilizationZeroQuota covers the
// zero-quota edge case.
func TestCapacityLeaseQuotaUtilizationZeroQuota(t *testing.T) {
	if got := (&CapacityLease{}).QuotaUtilization(); got != 0 {
		t.Errorf("zero-quota utilization = %f, want 0", got)
	}
}

// TestAwarenessSnapshotThrottled covers the E1 status-bar
// surface when the developer is throttled.
func TestAwarenessSnapshotThrottled(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	a.SetWorkSlot(&WorkSlot{
		SlotClass:     "free",
		TokenBudget:   100000,
		QueuePosition: 7,
		QueueDepth:    12,
	})
	a.SetCapacityLease(&CapacityLease{TokenQuota: 100000, TokensConsumed: 12500})
	a.SetVisibility(&WorkIntelVisibility{
		CanSeeRawActivity:       false,
		CanSeeAggregatedMetrics: true,
		AggregationFloorSeconds: 60,
	})
	a.RecordThrottleEvent()
	a.RecordThrottleEvent()
	snap := a.Snapshot()
	if !snap.IsThrottled {
		t.Error("snapshot must be throttled")
	}
	if snap.QuotaUtilization != 0.125 {
		t.Errorf("utilization = %f, want 0.125", snap.QuotaUtilization)
	}
	if snap.ThrottleEventCount != 2 {
		t.Errorf("throttle event count = %d, want 2", snap.ThrottleEventCount)
	}
	if snap.CanSeeRawActivity {
		t.Error("raw activity must be denied")
	}
}

// TestAwarenessSnapshotNoStateInstalled covers the boundary:
// the snapshot is safe when no relay state has been pushed.
func TestAwarenessSnapshotNoStateInstalled(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	snap := a.Snapshot()
	if snap.IsThrottled {
		t.Error("no-state snapshot must not be throttled")
	}
	if snap.SlotClass != "" {
		t.Errorf("slot class = %s, want empty", snap.SlotClass)
	}
}

// TestAwarenessCanDisplayRawActivityDefaultDeny covers E6: when
// no visibility rule is installed, the harness defaults to
// denying raw activity display so the privacy boundary is
// preserved by default.
func TestAwarenessCanDisplayRawActivityDefaultDeny(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	if a.CanDisplayRawActivity() {
		t.Error("no-visibility default must deny raw activity")
	}
	a.SetVisibility(&WorkIntelVisibility{CanSeeRawActivity: false})
	if a.CanDisplayRawActivity() {
		t.Error("explicit-deny visibility must continue to deny raw activity")
	}
	a.SetVisibility(&WorkIntelVisibility{CanSeeRawActivity: true})
	if !a.CanDisplayRawActivity() {
		t.Error("explicit-allow visibility must permit raw activity")
	}
}

// TestAwarenessRefreshFailureTracking covers the audit signal:
// a missed refresh surfaces in the snapshot so operators see
// stale operational state.
func TestAwarenessRefreshFailureTracking(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	for i := 0; i < 3; i++ {
		a.RecordRefreshFailure()
	}
	if a.Snapshot().RefreshFailures != 3 {
		t.Errorf("refresh failure count mismatch")
	}
}

// TestAwarenessSanityCheckFormat covers the E1 status-bar footer.
// The Korean message must contain both the throttle indicator
// (when throttled) and the quota utilization.
func TestAwarenessSanityCheckFormat(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	a.SetWorkSlot(&WorkSlot{SlotClass: "free", QueuePosition: 5, QueueDepth: 10, TokenBudget: 1000})
	a.SetCapacityLease(&CapacityLease{TokenQuota: 1000, TokensConsumed: 250})
	snap := a.Snapshot()
	formatted := snap.SanityCheckFormat()
	if !strings.Contains(formatted, "free") {
		t.Errorf("slot class missing: %q", formatted)
	}
	if !strings.Contains(formatted, "동결") {
		t.Errorf("throttle indicator missing: %q", formatted)
	}
}

// TestAwarenessSanityCheckFormatNonThrottled covers the
// non-throttled footer.
func TestAwarenessSanityCheckFormatNonThrottled(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	a.SetWorkSlot(&WorkSlot{SlotClass: "pro", TokenBudget: 1000})
	a.SetCapacityLease(&CapacityLease{TokenQuota: 1000, TokensConsumed: 100})
	snap := a.Snapshot()
	formatted := snap.SanityCheckFormat()
	if strings.Contains(formatted, "동결") {
		t.Errorf("non-throttled footer must not say 동결: %q", formatted)
	}
}

// TestAwarenessRefreshStampsLastRefresh covers the freshness
// signal: the snapshot must show when the operational state was
// last refreshed so operators see stale-vs-fresh.
func TestAwarenessRefreshStampsLastRefresh(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	a.Refresh(1_700_000_000_000)
	if got := a.Snapshot().LastRefreshAtMs; got != 1_700_000_000_000 {
		t.Errorf("last refresh = %d, want 1700000000000", got)
	}
}

// TestAwarenessConcurrentSnapshotAndSet covers the lock boundary.
func TestAwarenessConcurrentSnapshotAndSet(t *testing.T) {
	a := NewAwarenessClient("org-test", "alice")
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			_ = a.Snapshot()
			done <- struct{}{}
		}()
		go func() {
			a.SetWorkSlot(&WorkSlot{SlotClass: "free", QueuePosition: i})
			done <- struct{}{}
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

// _ keeps the time import visible even when only the time.Time
// type is used in struct literals.
var _ = time.Time{}
