package dariproto

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// epochClock is a settable time source used by the policy-epoch tests so
// the test can drive expiry without sleeping.
type epochClock struct {
	mu  sync.Mutex
	now time.Time
}

func newEpochClock(start time.Time) *epochClock { return &epochClock{now: start} }

func (c *epochClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *epochClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// sampleEpoch returns a fresh PolicyEpoch at the supplied time. Tests
// fork this and adjust fields to exercise failure paths.
func sampleEpoch(now time.Time, id string, seq uint64) *PolicyEpoch {
	return &PolicyEpoch{
		EpochID:           id,
		IssuedAtUnixMs:    now.UnixMilli(),
		NotBeforeUnixMs:   now.UnixMilli(),
		NotAfterUnixMs:    now.Add(time.Hour).UnixMilli(),
		MonotonicSequence: seq,
		IssuerKeyThumbprint: [32]byte{0x01, 0x02, 0x03},
		Digest:             [32]byte{0x10, 0x20, 0x30},
	}
}

// TestPolicyEpochBindAndVerifyHappyPath is the green path: the
// connector binds to the active epoch, every subsequent exchange
// verifies under the same epoch ID, and the connector never raises
// the mismatch sentinel.
func TestPolicyEpochBindAndVerifyHappyPath(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	epoch := sampleEpoch(clock.Now(), "epoch-2026-01", 1)
	if err := client.Bind(epoch); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := client.Verify("epoch-2026-01"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

// TestPolicyEpochVerifyRejectsMismatch is the security boundary: a
// lease or catalog entry pinned to a different epoch must fail verify
// before any AI_OPEN reaches the relay.
func TestPolicyEpochVerifyRejectsMismatch(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	err := client.Verify("epoch-2026-02")
	if !IsPolicyEpochMismatch(err) {
		t.Fatalf("expected mismatch sentinel, got %v", err)
	}
}

// TestPolicyEpochVerifyRejectsExpired pins the expired-epoch fail-closed
// boundary: the connector surfaces ErrPolicyEpochExpired when the bound
// epoch is past NotAfter, so a hub that missed the rebind cannot
// dispatch stale exchanges.
func TestPolicyEpochVerifyRejectsExpired(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	clock.Advance(2 * time.Hour)
	err := client.Verify("epoch-2026-01")
	if !IsPolicyEpochExpired(err) {
		t.Fatalf("expected expired sentinel, got %v", err)
	}
}

// TestPolicyEpochVerifyRejectsUnboundConnector guards the "no epoch
// bound" boundary: the connector must refuse to dispatch before the
// relay's POLICY message has been processed.
func TestPolicyEpochVerifyRejectsUnboundConnector(t *testing.T) {
	client := NewPolicyEpochClient()
	err := client.Verify("epoch-2026-01")
	if !IsPolicyEpochUnbound(err) {
		t.Fatalf("expected unbound sentinel, got %v", err)
	}
}

// TestPolicyEpochRebindRejectsLowerSequence pins the rollback/replay
// protection: a fresh epoch with a non-strictly-increasing sequence
// is rejected. A buggy relay cannot silently downgrade the connector.
func TestPolicyEpochRebindRejectsLowerSequence(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 5)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	clock.Advance(time.Minute)
	rollback := sampleEpoch(clock.Now(), "epoch-2026-01", 4) // lower sequence
	if err := client.Rebind(rollback); err == nil {
		t.Fatal("rollback epoch must be rejected")
	}
}

// TestPolicyEpochRebindRejectsSameOrOlderIssuedAt confirms the rebind
// path also checks the issued-at timestamp. A relay that hands back
// an epoch with a strictly-incrementing sequence but a stale issued-at
// timestamp is rejected.
func TestPolicyEpochRebindRejectsSameOrOlderIssuedAt(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 5)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	clock.Advance(time.Minute)
	older := sampleEpoch(clock.Now().Add(-2*time.Hour), "epoch-2026-02", 6)
	if err := client.Rebind(older); err == nil {
		t.Fatal("older issued-at must be rejected")
	}
}

// TestPolicyEpochRebindAdvancesSequence covers the green-path rebind.
func TestPolicyEpochRebindAdvancesSequence(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 5)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	clock.Advance(time.Minute)
	advanced := sampleEpoch(clock.Now(), "epoch-2026-02", 6)
	if err := client.Rebind(advanced); err != nil {
		t.Fatalf("rebind: %v", err)
	}
	if got := client.Current(); got != advanced {
		t.Fatalf("current = %p, want %p", got, advanced)
	}
	if err := client.Verify("epoch-2026-02"); err != nil {
		t.Fatalf("verify advanced: %v", err)
	}
}

// TestPolicyEpochBindRejectsInvalidBody exercises the wire-shape gate.
// The connector must reject a malformed epoch (empty ID, not-before in
// the future) without poisoning the bound state.
func TestPolicyEpochBindRejectsInvalidBody(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(&PolicyEpoch{EpochID: "", NotAfterUnixMs: clock.Now().Add(time.Hour).UnixMilli()}); err == nil {
		t.Fatal("empty epoch id must fail")
	}
	if err := client.Bind(&PolicyEpoch{EpochID: "epoch", NotBeforeUnixMs: clock.Now().Add(time.Hour).UnixMilli(), NotAfterUnixMs: clock.Now().Add(2 * time.Hour).UnixMilli()}); err == nil {
		t.Fatal("future not-before must fail")
	}
	// Re-enable the green path; the previous failures should not have
	// poisoned the bound state.
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err != nil {
		t.Fatalf("clean bind after invalid: %v", err)
	}
}

// TestPolicyEpochIsStaleReportsExpired covers the proactive-rebind
// driver: the connector checks IsStale between exchanges to schedule
// a Rebind before the next AI_OPEN.
func TestPolicyEpochIsStaleReportsExpired(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if client.IsStale() {
		t.Fatal("freshly bound epoch must not be stale")
	}
	clock.Advance(2 * time.Hour)
	if !client.IsStale() {
		t.Fatal("past-NotAfter epoch must be stale")
	}
}

// TestPolicyEpochMetricsSurfaceFailureCount covers the E1 status bar:
// the metrics expose bind/rebind failure counts so the operator can
// see when the relay is handing the connector bad epochs.
func TestPolicyEpochMetricsSurfaceFailureCount(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(&PolicyEpoch{}); err == nil {
		t.Fatal("empty epoch must fail")
	}
	if err := client.Rebind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err != nil {
		t.Fatalf("first rebind: %v", err)
	}
	clock.Advance(time.Minute)
	if err := client.Rebind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err == nil {
		t.Fatal("rebind with same sequence must fail")
	}
	m := client.MetricsFor()
	if m.BindFailureCount != 1 {
		t.Errorf("bind failure count = %d, want 1", m.BindFailureCount)
	}
	if m.RebindFailureCount != 1 {
		t.Errorf("rebind failure count = %d, want 1", m.RebindFailureCount)
	}
}

// TestPolicyEpochConcurrentBindAndVerify is the concurrency guard: the
// mutex around the bound epoch must serialize concurrent Verify/Rebind
// calls so a mid-rebind Verify never observes a torn state. The test
// pumps Rebinds with strictly-increasing sequences; Verify runs in
// parallel. Concurrent renewals that race to a higher sequence are
// expected to fail (the connector keeps the highest sequence), so the
// test only fails on Verify errors that are not epoch-mismatch.
func TestPolicyEpochConcurrentBindAndVerify(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	var wg sync.WaitGroup
	verifyErrors := atomic.Int64{}
	for i := 0; i < 50; i++ {
		wg.Add(2)
		seq := uint64(i + 2)
		go func(seq uint64) {
			defer wg.Done()
			clock.Advance(time.Minute)
			epoch := sampleEpoch(clock.Now(), "epoch-2026", seq)
			// A concurrent rebind with a lower sequence is expected to
			// fail; the sequencer drops it. We don't count those as
			// failures because the connector keeps the highest
			// sequence, which is the intended behavior.
			_ = client.Rebind(epoch)
		}(seq)
		go func() {
			defer wg.Done()
			if err := client.Verify("epoch-2026-01"); err != nil && !IsPolicyEpochMismatch(err) {
				verifyErrors.Add(1)
			}
		}()
	}
	wg.Wait()
	if verifyErrors.Load() != 0 {
		t.Errorf("concurrent Verify produced %d unexpected failures", verifyErrors.Load())
	}
}

// TestPolicyEpochSigningBytesBoundAllFields exercises the binding
// invariant: every field must hit the signing bytes so a relay or
// connector drift that drops a field cannot silently downgrade the
// policy.
func TestPolicyEpochSigningBytesBoundAllFields(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	primary := sampleEpoch(now, "epoch-2026-01", 1).SigningBytes()
	mutations := []struct {
		name string
		fn   func(*PolicyEpoch)
	}{
		{"epoch_id", func(e *PolicyEpoch) { e.EpochID = "epoch-other" }},
		{"issued_at", func(e *PolicyEpoch) { e.IssuedAtUnixMs += 1 }},
		{"not_before", func(e *PolicyEpoch) { e.NotBeforeUnixMs += 1 }},
		{"not_after", func(e *PolicyEpoch) { e.NotAfterUnixMs += 1 }},
		{"sequence", func(e *PolicyEpoch) { e.MonotonicSequence += 1 }},
		{"issuer_key", func(e *PolicyEpoch) { e.IssuerKeyThumbprint[0] ^= 0xff }},
		{"digest", func(e *PolicyEpoch) { e.Digest[0] ^= 0xff }},
	}
	for _, m := range mutations {
		clone := sampleEpoch(now, "epoch-2026-01", 1)
		m.fn(clone)
		if bytesEqual(clone.SigningBytes(), primary) {
			t.Errorf("signing bytes unchanged after %s mutation", m.name)
		}
	}
}

// TestPolicyEpochWireRoundTrip pins the byte contract: the relay
// decodes what the connector sends and vice versa.
func TestPolicyEpochWireRoundTrip(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	epoch := sampleEpoch(now, "epoch-2026-01", 1)
	bytes, err := EncodePolicyEpochMessage(epoch)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodePolicyEpochMessage(bytes)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.EpochID != epoch.EpochID {
		t.Errorf("epoch id drift: %q vs %q", decoded.EpochID, epoch.EpochID)
	}
	if decoded.MonotonicSequence != epoch.MonotonicSequence {
		t.Errorf("sequence drift: %d vs %d", decoded.MonotonicSequence, epoch.MonotonicSequence)
	}
	if decoded.IssuerKeyThumbprint != epoch.IssuerKeyThumbprint {
		t.Errorf("issuer thumbprint drift")
	}
}

// TestPolicyEpochDigestIsDeterministic guards the audit chain: the
// content digest must be stable across runs so receipts reproduce.
func TestPolicyEpochDigestIsDeterministic(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	epoch := sampleEpoch(now, "epoch-2026-01", 1)
	a := PolicyEpochDigest(epoch)
	b := PolicyEpochDigest(epoch)
	if a != b {
		t.Fatal("digest is non-deterministic")
	}
}

// TestPolicyEpochRebindIncrementsFailureCount pins the audit signal:
// every failed Rebind bumps the counter so the operator can see when
// the relay is handing back epochs that fail validation.
func TestPolicyEpochRebindIncrementsFailureCount(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Rebind(&PolicyEpoch{}); err == nil {
		t.Fatal("empty rebind must fail")
	}
	if got := client.MetricsFor().RebindFailureCount; got != 1 {
		t.Errorf("rebind failure count = %d, want 1", got)
	}
}

// TestPolicyEpochVerifyErrorTypes exercises the sentinel set: each
// failure mode returns the documented sentinel so the harness UI can
// surface the exact reason without parsing prose.
func TestPolicyEpochVerifyErrorTypes(t *testing.T) {
	clock := newEpochClock(time.Unix(1_700_000_000, 0))
	client := NewPolicyEpochClient().WithNowFunc(clock.Now)
	if err := client.Verify("epoch"); !IsPolicyEpochUnbound(err) {
		t.Errorf("unbound sentinel mismatch: %v", err)
	}
	if err := client.Bind(sampleEpoch(clock.Now(), "epoch-2026-01", 1)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := client.Verify("epoch-other"); !IsPolicyEpochMismatch(err) {
		t.Errorf("mismatch sentinel mismatch: %v", err)
	}
	clock.Advance(2 * time.Hour)
	if err := client.Verify("epoch-2026-01"); !IsPolicyEpochExpired(err) {
		t.Errorf("expired sentinel mismatch: %v", err)
	}
}

// TestPolicyEpochDecodingFailsEmpty guards the wire-shape gate.
func TestPolicyEpochDecodingFailsEmpty(t *testing.T) {
	if _, err := DecodePolicyEpochMessage(nil); err == nil {
		t.Fatal("empty body must fail")
	}
	if _, err := DecodePolicyEpochMessage([]byte{0xff}); err == nil {
		t.Fatal("invalid body must fail")
	}
}

// _ guards the time import when the test helpers evolve.
var _ = errors.New
