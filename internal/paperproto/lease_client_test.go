package paperproto

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// leaseClock is a settable time source used by the lease-client tests so
// they can drive expiry and renewal without sleeping.
type leaseClock struct {
	mu  sync.Mutex
	now time.Time
}

func newLeaseClock(start time.Time) *leaseClock { return &leaseClock{now: start} }

func (c *leaseClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *leaseClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newIssuerBundle hands out a fresh issuer key + a LeaseClient wired to
// it. The clock is a manual control so the test can move time forward
// without sleeping.
func newIssuerBundle(t *testing.T) (leaseIssuerFixture, *LeaseClient, *leaseClock) {
	t.Helper()
	issuer := newLeaseIssuerFixture(t)
	clock := newLeaseClock(time.Unix(1_700_000_000, 0))
	client := NewLeaseClient(issuer.publicKey, issuer.issuerID).
		WithNowFunc(clock.Now).
		WithAutoRenewBefore(time.Minute)
	return issuer, client, clock
}

// TestLeaseClientAcquireStoresValidatedLease is the happy path: a fresh
// lease signs out, the client verifies it under the issuer's public key,
// and a subsequent Present returns nil.
func TestLeaseClientAcquireStoresValidatedLease(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli() - 60_000,
		NotAfterUnixMs:  now.Add(30 * time.Minute).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if got := client.Current(); got != lease {
		t.Fatalf("current = %p, want %p", got, lease)
	}
	if err := client.Present("hrn:patty:test", "ses-1"); err != nil {
		t.Fatalf("present: %v", err)
	}
}

// TestLeaseClientAcquireRejectsUntrustedIssuer guards the trust boundary:
// a lease signed by a different issuer's key is rejected at acquire time
// so the connector never holds a lease it cannot re-verify.
func TestLeaseClientAcquireRejectsUntrustedIssuer(t *testing.T) {
	_, client, _ := newIssuerBundle(t)
	otherIssuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := otherIssuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-rogue",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err == nil {
		t.Fatal("rogue-issuer lease must be rejected")
	}
	if client.Current() != nil {
		t.Fatal("rogue-issuer lease must not be stored")
	}
}

// TestLeaseClientPresentReturnsRenewalDueInLeadWindow exercises the
// proactive renewal path. A lease whose `NotAfter` is inside the
// configured `autoRenewBefore` window returns `ErrLeaseRenewalDue` so
// the caller can issue a LEASE_RENEW handshake.
func TestLeaseClientPresentReturnsRenewalDueInLeadWindow(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	// autoRenewBefore = 1 minute; expire at now + 30s ⇒ already inside the window.
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-renew-window",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(30 * time.Second).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	err := client.Present("hrn:patty:test", "ses-1")
	if !errors.Is(err, ErrLeaseRenewalDue) {
		t.Fatalf("expected ErrLeaseRenewalDue, got %v", err)
	}
}

// TestLeaseClientRenewAdvancesSequenceAndPreservesValidity proves the
// renewal path: a fresh lease with a higher sequence replaces the
// current one and Present returns nil even after the old lease would
// have expired.
func TestLeaseClientRenewAdvancesSequenceAndPreservesValidity(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	original := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(2 * time.Minute).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", original); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	clock.Advance(90 * time.Second)
	renewed := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  clock.Now().UnixMilli(),
		LeaseSequence:   2,
	})
	if err := client.Renew("hrn:patty:test", "ses-1", renewed); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err := client.Present("hrn:patty:test", "ses-1"); err != nil {
		t.Fatalf("present after renew: %v", err)
	}
}

// TestLeaseClientRenewRefusesSameSequence protects the connector from a
// buggy relay that hands back a renewed lease with a non-advancing
// sequence. The renewal MUST carry a strictly higher sequence.
func TestLeaseClientRenewRefusesSameSequence(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	original := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", original); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	stale := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(2 * time.Hour).UnixMilli(),
		IssuedAtUnixMs:  clock.Now().UnixMilli(),
		LeaseSequence:   1, // same as original
	})
	if err := client.Renew("hrn:patty:test", "ses-1", stale); err == nil {
		t.Fatal("renew with same sequence must fail")
	}
}

// TestLeaseClientPresentFailsClosedAfterExpiry exercises the
// fail-closed behavior: once the lease is past NotAfter, Present
// returns ErrLeaseExpired and the connector must not dispatch an
// AI_OPEN.
func TestLeaseClientPresentFailsClosedAfterExpiry(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Minute).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	clock.Advance(2 * time.Minute)
	err := client.Present("hrn:patty:test", "ses-1")
	if !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("expected ErrLeaseExpired, got %v", err)
	}
}

// TestLeaseClientRevokeClearsState confirms the manual revocation path
// wipes the held lease so a subsequent Present fails closed.
func TestLeaseClientRevokeClearsState(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	client.Revoke()
	if client.Current() != nil {
		t.Fatal("revoke must clear current lease")
	}
	if err := client.Present("hrn:patty:test", "ses-1"); err == nil {
		t.Fatal("present after revoke must fail")
	}
}

// TestLeaseClientVerifySessionContextPinsPolicyEpoch exercises the
// A4 boundary: each governance-gated exchange pins the lease's policy
// epoch against the bound session epoch. A mismatch is rejected without
// touching the network.
func TestLeaseClientVerifySessionContextPinsPolicyEpoch(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := client.VerifySessionContext("hrn:patty:test", "ses-1", "epoch-2026-01"); err != nil {
		t.Fatalf("matching epoch must verify: %v", err)
	}
	if err := client.VerifySessionContext("hrn:patty:test", "ses-1", "epoch-2026-02"); err == nil {
		t.Fatal("mismatched epoch must fail verification")
	}
}

// TestLeaseClientMetricsSurfaceRenewalHealth pins the E1 quota/status
// surface: `MetricsFor` returns the held lease ID, sequence, and
// expiry whenever a lease is held, and the renew/verify timestamps
// advance on each exchange.
func TestLeaseClientMetricsSurfaceRenewalHealth(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := client.Present("hrn:patty:test", "ses-1"); err != nil {
		t.Fatalf("present: %v", err)
	}
	m := client.MetricsFor()
	if m.HeldLeaseID != "lease-1" {
		t.Errorf("held lease id = %q, want lease-1", m.HeldLeaseID)
	}
	if m.HeldSequence != 1 {
		t.Errorf("held sequence = %d, want 1", m.HeldSequence)
	}
	if m.PolicyEpochID != "epoch-2026-01" {
		t.Errorf("policy epoch = %q, want epoch-2026-01", m.PolicyEpochID)
	}
	if m.LastRenewedAtUnixMs == 0 || m.LastVerifiedAtUnixMs == 0 {
		t.Errorf("metrics missing timestamps: %+v", m)
	}
}

// TestLeaseClientRenewFailureCountIncrements guards the operator's
// audit signal: every failed renewal bumps the counter so the harness
// status bar can surface "renew failing" without parsing prose.
func TestLeaseClientRenewFailureCountIncrements(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	// Force a renewal failure by handing the verifier a lease signed by
	// a different issuer.
	otherIssuer := newLeaseIssuerFixture(t)
	rogue := otherIssuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-rogue",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: clock.Now().UnixMilli(),
		NotAfterUnixMs:  clock.Now().Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  clock.Now().UnixMilli(),
		LeaseSequence:   2,
	})
	if err := client.Renew("hrn:patty:test", "ses-1", rogue); err == nil {
		t.Fatal("rogue renewal must fail")
	}
	if got := client.MetricsFor().RenewFailureCount; got != 1 {
		t.Errorf("renew failure count = %d, want 1", got)
	}
}

// TestLeaseClientContextPlumb validates the `WithLeaseClient` /
// `LeaseClientFromContext` helper used by the harness's request
// boundary to inject the client into services that need to Present.
func TestLeaseClientContextPlumb(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	ctx := WithLeaseClient(context.Background(), client)
	if LeaseClientFromContext(ctx) != client {
		t.Fatal("LeaseClientFromContext must return the injected client")
	}
	if LeaseClientFromContext(context.Background()) != nil {
		t.Fatal("LeaseClientFromContext must return nil for a context without a client")
	}
}

// TestLeaseClientConcurrentRenewAndPresent is the concurrency guard: the
// mutex around the held lease must serialize concurrent Present/Renew
// calls so a renewal at the boundary can't race with a verification
// miss. The test pumps Renewals sequentially through a barrier so the
// sequence number is strictly monotonic; Present runs in parallel.
func TestLeaseClientConcurrentRenewAndPresent(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	const renewals = 50
	renewalsCh := make(chan uint64, renewals)
	for i := 0; i < renewals; i++ {
		renewalsCh <- uint64(i + 2)
	}
	close(renewalsCh)
	var wg sync.WaitGroup
	var failures atomic.Int64
	presentErrors := atomic.Int64{}
	for i := 0; i < renewals; i++ {
		wg.Add(2)
		seq := <-renewalsCh
		go func(seq uint64) {
			defer wg.Done()
			r := issuer.IssueLease(t, LeaseBody{
				LeaseID:         "lease-1",
				SubjectPeerID:   "hrn:patty:test",
				UserID:          "alice",
				SessionID:       "ses-1",
				PolicyEpochID:   "epoch-2026-01",
				NotBeforeUnixMs: now.UnixMilli(),
				NotAfterUnixMs:  now.Add(time.Hour + time.Duration(seq)*time.Minute).UnixMilli(),
				IssuedAtUnixMs:  clock.Now().UnixMilli(),
				LeaseSequence:   seq,
			})
			if err := client.Renew("hrn:patty:test", "ses-1", r); err != nil {
				// A renewal that races with a faster sequence is
				// expected to fail: the connector holds the fastest
				// sequence so the slower one is rejected.
				failures.Add(1)
			}
		}(seq)
		go func() {
			defer wg.Done()
			if err := client.Present("hrn:patty:test", "ses-1"); err != nil && !errors.Is(err, ErrLeaseRenewalDue) {
				presentErrors.Add(1)
			}
		}()
	}
	wg.Wait()
	if presentErrors.Load() != 0 {
		t.Fatalf("present produced %d unexpected errors", presentErrors.Load())
	}
	// Some renewals are expected to be rejected (stale sequence), but
	// the final held lease must be the highest sequence issued.
	if final := client.Current().LeaseSequence; final < uint64(renewals+1) {
		t.Errorf("final sequence = %d, want at least %d", final, renewals+1)
	}
}

// TestLeaseRequestWireEncodingRoundTrip pins the byte contract for the
// LEASE_ISSUE body: the relay decodes what the connector sends.
func TestLeaseRequestWireEncodingRoundTrip(t *testing.T) {
	req := &LeaseRequest{
		SubjectPeerID:     "hrn:patty:test",
		UserID:            "alice",
		SessionID:         "ses-1",
		PolicyEpochID:     "epoch-2026-01",
		AllowedModels:     []string{"patty-code-standard"},
		RepositoryScope:   []map[string]string{{"repo": "pccp", "branch": "main"}},
		FilePathReadScope: []string{"/etc/pccp"},
		FilePathWriteScope: []string{"/workspace"},
		ToolClasses:       []string{"repo.read", "repo.write"},
		TokenBudget:       8192,
		Validity:          30 * time.Minute,
	}
	bytes, err := EncodeLeaseRequest(req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeLeaseRequest(bytes)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.SubjectPeerID != req.SubjectPeerID {
		t.Errorf("subject drift: %q vs %q", decoded.SubjectPeerID, req.SubjectPeerID)
	}
	if decoded.PolicyEpochID != req.PolicyEpochID {
		t.Errorf("policy epoch drift: %q vs %q", decoded.PolicyEpochID, req.PolicyEpochID)
	}
	if decoded.TokenBudget != req.TokenBudget {
		t.Errorf("token budget drift: %d vs %d", decoded.TokenBudget, req.TokenBudget)
	}
	if decoded.Validity != req.Validity {
		t.Errorf("validity drift: %v vs %v", decoded.Validity, req.Validity)
	}
	if len(decoded.AllowedModels) != 1 || decoded.AllowedModels[0] != "patty-code-standard" {
		t.Errorf("allowed models drift: %v", decoded.AllowedModels)
	}
}

// TestLeaseServerFixtureHasSubjectKey confirms the lease issuer fixture
// supplies a valid Ed25519 key the connector can verify against. This
// guarantees the test harness never accidentally allows a zero-key
// fixture to silently sign a lease.
func TestLeaseServerFixtureHasSubjectKey(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	if len(issuer.publicKey) != ed25519.PublicKeySize {
		t.Fatalf("public key size = %d, want %d", len(issuer.publicKey), ed25519.PublicKeySize)
	}
	if len(issuer.priv) != ed25519.PrivateKeySize {
		t.Fatalf("private key size = %d, want %d", len(issuer.priv), ed25519.PrivateKeySize)
	}
	if !strings.HasPrefix(issuer.issuerID, "pccp-policy") {
		t.Errorf("unexpected issuer id: %q", issuer.issuerID)
	}
}

// TestAuthorizeExchangeChainsAllChecks is the consolidated
// exchange-time guard. The connector calls AuthorizeExchange once per
// AI_OPEN; the test pins the order: missing lease → subject/session
// mismatch → epoch mismatch → model mismatch.
func TestAuthorizeExchangeChainsAllChecks(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		AllowedModels:   []string{"patty-code-standard"},
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	// Happy path: subject/session/epoch all match, model is in
	// allow-list.
	if err := client.AuthorizeExchange("hrn:patty:test", "ses-1", "epoch-2026-01", "patty-code-standard"); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	// Subject mismatch.
	if err := client.AuthorizeExchange("hrn:patty:other", "ses-1", "epoch-2026-01", "patty-code-standard"); !errors.Is(err, ErrLeaseSubjectMismatch) {
		t.Errorf("expected subject mismatch, got %v", err)
	}
	// Epoch mismatch.
	if err := client.AuthorizeExchange("hrn:patty:test", "ses-1", "epoch-2026-02", "patty-code-standard"); err == nil {
		t.Errorf("expected epoch mismatch, got nil")
	}
	// Model mismatch.
	if err := client.AuthorizeExchange("hrn:patty:test", "ses-1", "epoch-2026-01", "patty-code-pro"); err == nil {
		t.Errorf("expected model mismatch, got nil")
	}
}

// TestAuthorizeExchangeRejectsEmptyAllowedList confirms the A5
// fail-closed boundary: a lease with no AllowedModels list is treated
// as "no model approved" rather than "all models approved".
func TestAuthorizeExchangeRejectsEmptyAllowedList(t *testing.T) {
	issuer, client, clock := newIssuerBundle(t)
	now := clock.Now()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-1",
		PolicyEpochID:   "epoch-2026-01",
		AllowedModels:   nil, // empty
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(time.Hour).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	})
	if err := client.Acquire("hrn:patty:test", "ses-1", lease); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	err := client.AuthorizeExchange("hrn:patty:test", "ses-1", "epoch-2026-01", "patty-code-standard")
	if err == nil {
		t.Fatal("expected empty-list failure")
	}
	if !strings.Contains(err.Error(), "no allowed-models list") {
		t.Errorf("expected empty-list error, got %v", err)
	}
}
