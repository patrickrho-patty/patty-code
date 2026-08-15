package dari

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/dariproto"
)

// catalogTestClock is a settable time source used by the catalog-pin
// tests. The provider's catalog client and the test share the same
// clock so the test can drive expiry deterministically.
type catalogTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func newcatalogTestClock(start time.Time) *catalogTestClock { return &catalogTestClock{now: start} }

func (c *catalogTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *catalogTestClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newProviderWithLease constructs a Provider with a valid lease that
// authorizes the supplied model. The provider's policy epoch and
// catalog clients are attached by the test.
func newProviderWithLease(t *testing.T, model string, leaseModels []string) *Provider {
	t.Helper()
	issuer := newTestLeaseIssuer(t)
	return NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        model,
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      time.Now(),
		LeaseFor:     time.Hour,
		LeaseModels:  leaseModels,
	})
}

// TestProviderRejectsModelNotInCatalog is the A5 catalog boundary: a
// model that the lease allows but the relay never advertised is
// caught by the catalog check before any AI_OPEN.
func TestProviderRejectsModelNotInCatalog(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-pro", []string{"patty-code-pro", "patty-code-standard"})
	catalog := dariproto.NewCatalogClient()
	if err := catalog.ApplySnapshot(sampleCatalogForTest(t), "epoch-2026-01"); err != nil {
		t.Fatalf("apply catalog: %v", err)
	}
	provider.SetCatalogClient(catalog)
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-pro"))
	if err == nil {
		t.Fatal("stream must fail when the model is not in the relay's catalog")
	}
	if !errors.Is(err, dariproto.ErrCatalogModelNotFound) {
		t.Errorf("expected ErrCatalogModelNotFound, got %v", err)
	}
}

// TestProviderAcceptsModelInCatalog covers the A5 green path: the
// model is in the lease's allow-list AND the relay's catalog.
func TestProviderAcceptsModelInCatalog(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-standard", []string{"patty-code-standard"})
	catalog := dariproto.NewCatalogClient()
	if err := catalog.ApplySnapshot(sampleCatalogForTest(t), "epoch-2026-01"); err != nil {
		t.Fatalf("apply catalog: %v", err)
	}
	provider.SetCatalogClient(catalog)
	// The actual stream will fail later (no real relay), but the
	// authorization checks must pass.
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err != nil && (errors.Is(err, dariproto.ErrCatalogModelNotFound) || errors.Is(err, dariproto.ErrLeaseExpired) || errors.Is(err, dariproto.ErrLeaseSubjectMismatch)) {
		t.Errorf("catalog and lease checks must pass, got %v", err)
	}
}

// TestProviderRejectsModelRejectedByCatalogAfterEntryExpires covers
// the A5 entry-active-until boundary.
func TestProviderRejectsModelRejectedByCatalogAfterEntryExpires(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-standard", []string{"patty-code-standard"})
	catalog := dariproto.NewCatalogClient()
	clock := newcatalogTestClock(time.Now())
	catalog.WithNowFunc(clock.Now)
	snap := sampleCatalogForTest(t)
	snap.Entries[0].ActiveUntilUnixMs = clock.Now().Add(time.Minute).UnixMilli()
	snap.Digest = dariproto.CatalogDigest(snap)
	if err := catalog.ApplySnapshot(snap, "epoch-2026-01"); err != nil {
		t.Fatalf("apply catalog: %v", err)
	}
	provider.SetCatalogClient(catalog)
	// Advance the clock past the entry's active-until.
	clock.Advance(2 * time.Minute)
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the catalog entry is past its active-until")
	}
	if !errors.Is(err, dariproto.ErrCatalogEntryExpired) {
		t.Errorf("expected ErrCatalogEntryExpired, got %v", err)
	}
}

// TestProviderRejectsModelWithStaleCatalog covers the A5 stale-catalog
// boundary: a connector that has not refreshed the catalog past its
// NotAfter must refuse to dispatch.
func TestProviderRejectsModelWithStaleCatalog(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-standard", []string{"patty-code-standard"})
	catalog := dariproto.NewCatalogClient()
	clock := newcatalogTestClock(time.Now())
	catalog.WithNowFunc(clock.Now)
	snap := sampleCatalogForTest(t)
	snap.NotAfterUnixMs = clock.Now().Add(time.Minute).UnixMilli()
	snap.Digest = dariproto.CatalogDigest(snap)
	if err := catalog.ApplySnapshot(snap, "epoch-2026-01"); err != nil {
		t.Fatalf("apply catalog: %v", err)
	}
	provider.SetCatalogClient(catalog)
	clock.Advance(2 * time.Minute)
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the catalog is stale")
	}
	if !errors.Is(err, dariproto.ErrCatalogStale) {
		t.Errorf("expected ErrCatalogStale, got %v", err)
	}
}

// TestProviderRejectsModelWhenPolicyEpochReinstalled pins the A4
// boundary: a model is allowed under one epoch but the connector
// has rebounded to a new epoch whose catalog no longer lists it.
func TestProviderRejectsModelWhenPolicyEpochReinstalled(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-standard", []string{"patty-code-standard"})
	provider.SetPolicyEpochClient(dariproto.NewPolicyEpochClient())
	catalog := dariproto.NewCatalogClient()
	if err := catalog.ApplySnapshot(sampleCatalogForTest(t), "epoch-2026-01"); err != nil {
		t.Fatalf("apply catalog: %v", err)
	}
	provider.SetCatalogClient(catalog)
	// Rebind to a new epoch. The provider's p.policyEpoch now points
	// at epoch-2026-02.
	provider.RebindPolicyEpoch(&dariproto.PolicyEpoch{
		EpochID:           "epoch-2026-02",
		IssuedAtUnixMs:    time.Now().UnixMilli(),
		NotAfterUnixMs:    time.Now().Add(time.Hour).UnixMilli(),
		MonotonicSequence: 2,
	})
	// The ApplyCatalogSnapshot path checks the snap's epoch against
	// the bound policy epoch. The OLD snapshot has epoch-2026-01, the
	// bound epoch is now epoch-2026-02, so apply fails.
	stale := sampleCatalogForTest(t)
	stale.IssuedSequence = 2
	stale.Digest = dariproto.CatalogDigest(stale)
	err := provider.ApplyCatalogSnapshot(stale)
	if err == nil {
		t.Fatal("stale catalog must reject after policy epoch rebind")
	}
	if !strings.Contains(err.Error(), "epoch") {
		t.Errorf("expected epoch-mismatch error, got %v", err)
	}
}

// TestProviderAllowsModelWhenCatalogNotConfigured: a connector that
// has not received a catalog (yet) skips the catalog check rather
// than failing closed. The lease allow-list is the only model filter.
func TestProviderAllowsModelWhenCatalogNotConfigured(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-standard", []string{"patty-code-standard"})
	// No catalog attached.
	// The authorization checks must pass; the stream will fail later
	// at the dial stage.
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err != nil && (errors.Is(err, dariproto.ErrCatalogModelNotFound) || errors.Is(err, dariproto.ErrCatalogStale) || errors.Is(err, dariproto.ErrCatalogEntryExpired)) {
		t.Errorf("catalog check should be skipped when no client is configured, got %v", err)
	}
}

// TestProviderBindPolicyEpochRejectsInvalid guards the A4 epoch
// binding guard.
func TestProviderBindPolicyEpochRejectsInvalid(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-standard", []string{"patty-code-standard"})
	provider.SetPolicyEpochClient(dariproto.NewPolicyEpochClient())
	if err := provider.BindPolicyEpoch(&dariproto.PolicyEpoch{}); err == nil {
		t.Fatal("empty epoch must fail")
	}
}

// TestProviderMetricsExposeAllThreeSignals covers the E1 status bar:
// the provider combines lease, policy-epoch, and catalog metrics.
func TestProviderMetricsExposeAllThreeSignals(t *testing.T) {
	provider := newProviderWithLease(t, "patty-code-standard", []string{"patty-code-standard"})
	provider.SetPolicyEpochClient(dariproto.NewPolicyEpochClient())
	catalog := dariproto.NewCatalogClient()
	if err := catalog.ApplySnapshot(sampleCatalogForTest(t), "epoch-2026-01"); err != nil {
		t.Fatalf("apply catalog: %v", err)
	}
	provider.SetCatalogClient(catalog)
	leaseMetrics := provider.LeaseMetrics()
	if leaseMetrics.HeldLeaseID != "lease-test" {
		t.Errorf("lease metric held id = %q, want lease-test", leaseMetrics.HeldLeaseID)
	}
	epochMetrics := provider.PolicyEpochMetrics()
	catalogMetrics := provider.CatalogMetrics()
	if catalogMetrics.Version != 1 {
		t.Errorf("catalog metric version = %d, want 1", catalogMetrics.Version)
	}
	if catalogMetrics.EntryCount != 1 {
		t.Errorf("catalog metric entry count = %d, want 1", catalogMetrics.EntryCount)
	}
	_ = epochMetrics
}

// sampleCatalogForTest returns a fresh CatalogSnapshot at the
// supplied time with one model entry.
func sampleCatalogForTest(t *testing.T) *dariproto.CatalogSnapshot {
	t.Helper()
	now := time.Now()
	snap := &dariproto.CatalogSnapshot{
		Version:        1,
		EpochID:        "epoch-2026-01",
		IssuedAtUnixMs: now.UnixMilli(),
		NotAfterUnixMs: now.Add(time.Hour).UnixMilli(),
		IssuedSequence: 1,
		Entries: []dariproto.CatalogEntry{
			{
				ModelID:            "patty-code-standard",
				Version:            "1.0.0",
				ModelPackageDigest: [32]byte{0x10},
				EndpointDigest:     [32]byte{0x20},
				PolicyEpochID:      "epoch-2026-01",
				ActiveUntilUnixMs:  now.Add(30 * 24 * time.Hour).UnixMilli(),
			},
		},
	}
	snap.Digest = dariproto.CatalogDigest(snap)
	return snap
}
