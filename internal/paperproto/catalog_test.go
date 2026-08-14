package paperproto

import (
	"crypto/sha256"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// catalogClock is a settable time source for the catalog tests so the
// test can drive snapshot expiry without sleeping.
type catalogClock struct {
	mu  sync.Mutex
	now time.Time
}

func newCatalogClock(start time.Time) *catalogClock { return &catalogClock{now: start} }

func (c *catalogClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *catalogClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// sampleCatalog returns a fresh CatalogSnapshot at the supplied time.
// Tests fork this and adjust fields to exercise failure paths.
func sampleCatalog(now time.Time, version uint64, epoch string, seq uint64) *CatalogSnapshot {
	snap := &CatalogSnapshot{
		Version:       version,
		EpochID:       epoch,
		IssuedAtUnixMs: now.UnixMilli(),
		NotAfterUnixMs: now.Add(time.Hour).UnixMilli(),
		IssuedSequence: seq,
		IssuerKeyThumbprint: [32]byte{0x01, 0x02, 0x03},
		Entries: []CatalogEntry{
			{
				ModelID:           "patty-code-standard",
				DisplayName:       "Patty Code Standard",
				Version:           "1.0.0",
				ModelPackageDigest: [32]byte{0x10},
				EndpointDigest:     [32]byte{0x20},
				Capabilities:      []string{"code", "review"},
				TokenLimit:        8192,
				ContextWindow:     262144,
				PolicyEpochID:     epoch,
				ActiveUntilUnixMs: now.Add(30 * 24 * time.Hour).UnixMilli(),
			},
		},
	}
	snap.Digest = CatalogDigest(snap)
	return snap
}

// TestCatalogApplyAndFindRoundTrip is the green path: a fresh snapshot
// applies, the connector resolves a model by ID, and the entry's
// metadata round-trips.
func TestCatalogApplyAndFindRoundTrip(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	snap := sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1)
	if err := client.ApplySnapshot(snap, "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	entry, err := client.FindModel("patty-code-standard")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if entry.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", entry.Version)
	}
	if entry.ContextWindow != 262144 {
		t.Errorf("context window = %d, want 262144", entry.ContextWindow)
	}
}

// TestCatalogApplyRejectsSequenceRollback pins the rollback guard: a
// snapshot with a sequence <= the held one is rejected.
func TestCatalogApplyRejectsSequenceRollback(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 5), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	rollback := sampleCatalog(clock.Now(), 0, "epoch-2026-01", 4)
	if err := client.ApplySnapshot(rollback, "epoch-2026-01"); err == nil {
		t.Fatal("rollback snapshot must fail")
	}
}

// TestCatalogApplyRejectsEpochMismatch pins the relay/connector
// agreement on the policy epoch. A snapshot whose EpochID does not
// match the bound epoch is rejected.
func TestCatalogApplyRejectsEpochMismatch(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	snap := sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1)
	if err := client.ApplySnapshot(snap, "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	wrong := sampleCatalog(clock.Now(), 2, "epoch-2026-02", 2)
	if err := client.ApplySnapshot(wrong, "epoch-2026-01"); err == nil {
		t.Fatal("epoch-mismatch snapshot must fail")
	}
}

// TestCatalogApplyRejectsExpired guards the time-bound fail-closed
// path: a snapshot whose NotAfter is already in the past is rejected.
func TestCatalogApplyRejectsExpired(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	now := clock.Now()
	snap := sampleCatalog(now, 1, "epoch-2026-01", 1)
	snap.NotAfterUnixMs = now.Add(-time.Hour).UnixMilli()
	snap.Digest = CatalogDigest(snap)
	if err := client.ApplySnapshot(snap, "epoch-2026-01"); err == nil {
		t.Fatal("expired snapshot must fail")
	}
}

// TestCatalogApplyRejectsDigestMismatch guards the audit chain: a
// snapshot whose computed digest does not match the embedded digest
// is rejected. A buggy relay cannot silently substitute entries.
func TestCatalogApplyRejectsDigestMismatch(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	snap := sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1)
	snap.Digest = [32]byte{} // eliminate the digest
	if err := client.ApplySnapshot(snap, "epoch-2026-01"); err == nil {
		t.Fatal("digest-mismatch snapshot must fail")
	}
}

// TestCatalogFindModelReturnsNotFoundSentinel guards the A5
// fail-closed boundary: a model the relay's catalog does not list
// must be rejected before AI_OPEN.
func TestCatalogFindModelReturnsNotFoundSentinel(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	_, err := client.FindModel("patty-code-pro")
	if !IsCatalogModelNotFound(err) {
		t.Errorf("expected ErrCatalogModelNotFound, got %v", err)
	}
}

// TestCatalogFindModelReturnsStaleSentinel pins the fail-closed
// post-expiry path: a query against an expired catalog must surface
// ErrCatalogStale so the harness knows to refresh.
func TestCatalogFindModelReturnsStaleSentinel(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	clock.Advance(2 * time.Hour)
	_, err := client.FindModel("patty-code-standard")
	if !IsCatalogStale(err) {
		t.Errorf("expected ErrCatalogStale, got %v", err)
	}
}

// TestCatalogFindModelReturnsEntryExpiredSentinel: a model entry
// whose ActiveUntil has elapsed is rejected even if the snapshot
// itself is still valid.
func TestCatalogFindModelReturnsEntryExpiredSentinel(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	snap := sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1)
	snap.Entries[0].ActiveUntilUnixMs = clock.Now().Add(time.Minute).UnixMilli()
	snap.Digest = CatalogDigest(snap)
	if err := client.ApplySnapshot(snap, "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	clock.Advance(2 * time.Minute)
	_, err := client.FindModel("patty-code-standard")
	if !IsCatalogEntryExpired(err) {
		t.Errorf("expected ErrCatalogEntryExpired, got %v", err)
	}
}

// TestCatalogFindModelReturnsUnboundSentinel guards the pre-catalog
// boundary: a connector that hasn't received a snapshot must refuse
// to dispatch against any model.
func TestCatalogFindModelReturnsUnboundSentinel(t *testing.T) {
	client := NewCatalogClient()
	_, err := client.FindModel("patty-code-standard")
	if err == nil {
		t.Fatal("expected unbound error")
	}
}

// TestCatalogApplyDeltaAddsEntry exercises the delta shape: a
// delta adds models without resetting the snapshot.
func TestCatalogApplyDeltaAddsEntry(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	delta := &CatalogDelta{
		Version:        1,
		EpochID:        "epoch-2026-01",
		IssuedAtUnixMs: clock.Now().UnixMilli(),
		IssuedSequence: 2,
		Added: []CatalogEntry{
			{
				ModelID:           "patty-code-pro",
				Version:           "1.0.0",
				ModelPackageDigest: [32]byte{0x30},
				EndpointDigest:     [32]byte{0x40},
				PolicyEpochID:     "epoch-2026-01",
				ActiveUntilUnixMs:  clock.Now().Add(30 * 24 * time.Hour).UnixMilli(),
			},
		},
	}
	if err := client.ApplyDelta(delta, "epoch-2026-01"); err != nil {
		t.Fatalf("apply delta: %v", err)
	}
	if _, err := client.FindModel("patty-code-pro"); err != nil {
		t.Errorf("expected new model to be findable, got %v", err)
	}
	if got := client.Current().IssuedSequence; got != 2 {
		t.Errorf("IssuedSequence = %d, want 2", got)
	}
}

// TestCatalogApplyDeltaRemovesEntry: a delta removes a model by ID.
func TestCatalogApplyDeltaRemovesEntry(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	delta := &CatalogDelta{
		Version:        1,
		EpochID:        "epoch-2026-01",
		IssuedAtUnixMs: clock.Now().UnixMilli(),
		IssuedSequence: 2,
		Removed:        []string{"patty-code-standard"},
	}
	if err := client.ApplyDelta(delta, "epoch-2026-01"); err != nil {
		t.Fatalf("apply delta: %v", err)
	}
	if _, err := client.FindModel("patty-code-standard"); !IsCatalogModelNotFound(err) {
		t.Errorf("expected model not found after removal, got %v", err)
	}
}

// TestCatalogApplyDeltaUpdatesEntry: a delta replaces an existing
// entry's metadata.
func TestCatalogApplyDeltaUpdatesEntry(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	delta := &CatalogDelta{
		Version:        1,
		EpochID:        "epoch-2026-01",
		IssuedAtUnixMs: clock.Now().UnixMilli(),
		IssuedSequence: 2,
		Updated: []CatalogEntry{
			{
				ModelID:           "patty-code-standard",
				Version:           "1.0.1",
				ModelPackageDigest: [32]byte{0x99},
				EndpointDigest:     [32]byte{0x20},
				PolicyEpochID:     "epoch-2026-01",
				ActiveUntilUnixMs:  clock.Now().Add(30 * 24 * time.Hour).UnixMilli(),
			},
		},
	}
	if err := client.ApplyDelta(delta, "epoch-2026-01"); err != nil {
		t.Fatalf("apply delta: %v", err)
	}
	entry, err := client.FindModel("patty-code-standard")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if entry.Version != "1.0.1" {
		t.Errorf("version = %q, want 1.0.1", entry.Version)
	}
}

// TestCatalogApplyDeltaWithoutBaselineFailsClosed: a delta with no
// prior snapshot is rejected. The connector must have a baseline
// before applying incremental updates.
func TestCatalogApplyDeltaWithoutBaselineFailsClosed(t *testing.T) {
	client := NewCatalogClient()
	delta := &CatalogDelta{
		Version:        1,
		EpochID:        "epoch-2026-01",
		IssuedAtUnixMs: time.Now().UnixMilli(),
		IssuedSequence: 1,
	}
	if err := client.ApplyDelta(delta, "epoch-2026-01"); err == nil {
		t.Fatal("delta without baseline must fail")
	}
}

// TestCatalogApplyDeltaRejectsLowerSequence pins the monotonic
// sequence guard.
func TestCatalogApplyDeltaRejectsLowerSequence(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 5), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	delta := &CatalogDelta{
		Version:        1,
		EpochID:        "epoch-2026-01",
		IssuedAtUnixMs: clock.Now().UnixMilli(),
		IssuedSequence: 4,
	}
	if err := client.ApplyDelta(delta, "epoch-2026-01"); err == nil {
		t.Fatal("delta with lower sequence must fail")
	}
}

// TestCatalogIsStaleDrivesRefresh covers the proactive refresh path.
func TestCatalogIsStaleDrivesRefresh(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if client.IsStale() {
		t.Fatal("fresh snapshot must not be stale")
	}
	clock.Advance(2 * time.Hour)
	if !client.IsStale() {
		t.Fatal("past NotAfter snapshot must be stale")
	}
}

// TestCatalogMetricsSurfaceFailureCount exercises the E1 status bar.
func TestCatalogMetricsSurfaceFailureCount(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(&CatalogSnapshot{}, "epoch-2026-01"); err == nil {
		t.Fatal("empty snapshot must fail")
	}
	if got := client.MetricsFor().ApplyFailureCount; got != 1 {
		t.Errorf("apply failure count = %d, want 1", got)
	}
}

// TestCatalogConcurrentApplyAndFind is the concurrency guard: the
// mutex around the snapshot must serialize concurrent Find/Apply
// calls so a mid-find apply never observes a torn state.
func TestCatalogConcurrentApplyAndFind(t *testing.T) {
	clock := newCatalogClock(time.Unix(1_700_000_000, 0))
	client := NewCatalogClient().WithNowFunc(clock.Now)
	if err := client.ApplySnapshot(sampleCatalog(clock.Now(), 1, "epoch-2026-01", 1), "epoch-2026-01"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var wg sync.WaitGroup
	findErrors := atomic.Int64{}
	for i := 0; i < 50; i++ {
		wg.Add(2)
		seq := uint64(i + 2)
		go func(seq uint64) {
			defer wg.Done()
			clock.Advance(time.Millisecond)
			snap := sampleCatalog(clock.Now(), 1, "epoch-2026-01", seq)
			_ = client.ApplySnapshot(snap, "epoch-2026-01")
		}(seq)
		go func() {
			defer wg.Done()
			if _, err := client.FindModel("patty-code-standard"); err != nil && !IsCatalogStale(err) {
				findErrors.Add(1)
			}
		}()
	}
	wg.Wait()
	if findErrors.Load() != 0 {
		t.Errorf("FindModel produced %d unexpected errors", findErrors.Load())
	}
}

// TestCatalogDigestIsDeterministic guards the audit chain: the content
// digest must be stable across runs so receipts reproduce.
func TestCatalogDigestIsDeterministic(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	snap := sampleCatalog(now, 1, "epoch-2026-01", 1)
	a := CatalogDigest(snap)
	b := CatalogDigest(snap)
	if a != b {
		t.Fatal("digest is non-deterministic")
	}
}

// TestCatalogDigestChangesWithEntryUpdate pins the binding invariant.
// Adding or changing an entry must change the digest.
func TestCatalogDigestChangesWithEntryUpdate(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	primary := sampleCatalog(now, 1, "epoch-2026-01", 1)
	modified := sampleCatalog(now, 1, "epoch-2026-01", 1)
	modified.Entries[0].Version = "1.0.1"
	modified.Entries[0].ModelPackageDigest = [32]byte{0xff}
	if CatalogDigest(primary) == CatalogDigest(modified) {
		t.Fatal("digest unchanged after entry update")
	}
}

// TestCatalogWireEncodingRoundTrip pins the byte contract: the
// connector decodes what the relay sends.
func TestCatalogWireEncodingRoundTrip(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	snap := sampleCatalog(now, 1, "epoch-2026-01", 1)
	bytes, err := EncodeCatalogSnapshot(snap)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCatalogSnapshot(bytes)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Version != snap.Version {
		t.Errorf("version drift: %d vs %d", decoded.Version, snap.Version)
	}
	if decoded.EpochID != snap.EpochID {
		t.Errorf("epoch drift: %q vs %q", decoded.EpochID, snap.EpochID)
	}
	if len(decoded.Entries) != len(snap.Entries) {
		t.Errorf("entry count drift: %d vs %d", len(decoded.Entries), len(snap.Entries))
	}
}

// TestCatalogDeltaWireEncodingRoundTrip pins the delta byte contract.
func TestCatalogDeltaWireEncodingRoundTrip(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	delta := &CatalogDelta{
		Version:        1,
		EpochID:        "epoch-2026-01",
		IssuedAtUnixMs: now.UnixMilli(),
		IssuedSequence: 2,
		Added: []CatalogEntry{
			{
				ModelID:           "patty-code-pro",
				Version:           "1.0.0",
				ModelPackageDigest: [32]byte{0xaa},
				EndpointDigest:     [32]byte{0xbb},
				PolicyEpochID:     "epoch-2026-01",
				ActiveUntilUnixMs:  now.Add(30 * 24 * time.Hour).UnixMilli(),
			},
		},
		Removed: []string{"patty-code-legacy"},
	}
	bytes, err := EncodeCatalogDelta(delta)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCatalogDelta(bytes)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Added) != 1 || decoded.Added[0].ModelID != "patty-code-pro" {
		t.Errorf("added drift: %v", decoded.Added)
	}
	if len(decoded.Removed) != 1 || decoded.Removed[0] != "patty-code-legacy" {
		t.Errorf("removed drift: %v", decoded.Removed)
	}
}

// TestCatalogDecodingEmptyFails guards the wire-shape gate.
func TestCatalogDecodingEmptyFails(t *testing.T) {
	if _, err := DecodeCatalogSnapshot(nil); err == nil {
		t.Fatal("empty snapshot must fail")
	}
	if _, err := DecodeCatalogSnapshot([]byte{0xff}); err == nil {
		t.Fatal("invalid snapshot must fail")
	}
	if _, err := DecodeCatalogDelta(nil); err == nil {
		t.Fatal("empty delta must fail")
	}
}

// _ guards the hashed import.
var _ = sha256.Sum256
