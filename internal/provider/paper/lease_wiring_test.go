package paper

import (
	"errors"
	"strings"
	"testing"
	"time"

	"patty/internal/paperproto"
	"patty/internal/provider"
)

// testLeaseIssuer, hexEncode, and newTestLeaseIssuer live in testing.go
// alongside the test helpers they share.

// TestProviderRejectsStreamWithoutLease is the fail-closed boundary: the
// connector must refuse to dispatch an AI_OPEN when no lease is held,
// because the relay's governance path requires a valid lease per
// exchange.
func TestProviderRejectsStreamWithoutLease(t *testing.T) {
	provider := NewForTest(&testConfig{
		RelayAddr: "relay.example.com:8444",
		Model:     "patty-code-standard",
	})
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when no lease is held")
	}
	if !strings.Contains(err.Error(), "lease") {
		t.Errorf("expected error to mention lease, got %v", err)
	}
}

// TestProviderRejectsStreamWithExpiredLease guards the time-bound
// failure: a lease whose NotAfter is in the past must fail-closed in
// the connector before any AI_OPEN reaches the relay. The test acquires
// a valid lease at `t0`, then advances the lease-client clock past
// `NotAfter` so the next Present surfaces ErrLeaseExpired.
func TestProviderRejectsStreamWithExpiredLease(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	now := time.Now()
	// Replace the lease client's clock: at acquire time return now;
	// on Present return now + 2h (past the lease's expiry).
	advanced := false
	clock := func() time.Time {
		if !advanced {
			advanced = true
			return now
		}
		return now.Add(2 * time.Hour)
	}
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      now,
		LeaseFor:     time.Hour, // 1h validity
		AdvanceTime:  clock,
	})
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the held lease is expired")
	}
	if !errors.Is(err, paperproto.ErrLeaseExpired) {
		t.Errorf("expected ErrLeaseExpired sentinel, got %v", err)
	}
}

// TestProviderRejectsStreamWithMismatchedSubject pins the subject-binding
// boundary: a lease whose SubjectPeerID does not match the connector's
// authenticated harness identity MUST be rejected. The test acquires a
// valid lease for the harness, then re-binds the harness to a different
// subject peer ID (simulating a re-auth), and confirms the connector
// fails-closed on the next exchange.
func TestProviderRejectsStreamWithMismatchedSubject(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      time.Now(),
		LeaseFor:     time.Hour,
	})
	// Re-bind to a different harness identity; the held lease's
	// SubjectPeerID no longer matches.
	provider.SetSessionContext("hrn:patty:other", "ses-1", "epoch-2026-01")
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the lease subject does not match")
	}
	if !errors.Is(err, paperproto.ErrLeaseSubjectMismatch) {
		t.Errorf("expected ErrLeaseSubjectMismatch sentinel, got %v", err)
	}
}

// TestProviderRequiresPolicyEpochBinding enforces the A4 cross-feature
// invariant: every governance-gated exchange pins the lease's policy
// epoch to the session's bound epoch. A connector that boots without a
// pinned epoch must refuse to dispatch.
func TestProviderRequiresPolicyEpochBinding(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	provider := NewForTest(&testConfig{
		RelayAddr:          "relay.example.com:8444",
		Model:              "patty-code-standard",
		LeaseIssuer:        issuer,
		LeaseSubject:       "hrn:patty:test",
		LeaseSession:       "ses-1",
		LeaseEpoch:         "epoch-2026-01",
		LeaseAt:            time.Now(),
		LeaseFor:           time.Hour,
		SessionPolicyEpoch: "epoch-2026-02", // session has rotated
	})
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the lease epoch disagrees with the session epoch")
	}
	if !strings.Contains(err.Error(), "policy epoch") {
		t.Errorf("expected policy-epoch-mismatch error, got %v", err)
	}
}

// TestProviderRejectsModelNotInLeaseAllowedModels is the A5 binding: the
// model in the request must be a member of the lease's AllowedModels.
// A connector that asks for an undisclosed model cannot route it.
func TestProviderRejectsModelNotInLeaseAllowedModels(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      time.Now(),
		LeaseFor:     time.Hour,
		LeaseModels:  []string{"patty-code-pro"}, // request asks for a different model
	})
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the requested model is not in the lease")
	}
	if !strings.Contains(err.Error(), "not in lease") {
		t.Errorf("expected model-not-in-lease error, got %v", err)
	}
}

// TestProviderRejectsEmptyAllowedModelsList is the A5 fail-closed
// boundary: a relay that returns a lease with an empty AllowList must
// not be treated as "all models allowed" by the connector. The
// harness refuses to dispatch so a misconfigured or adversarial relay
// cannot silently grant access to undisclosed models.
func TestProviderRejectsEmptyAllowedModelsList(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      time.Now(),
		LeaseFor:     time.Hour,
		LeaseModels:  nil, // empty list
	})
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the lease carries no allowed-models list")
	}
	if !strings.Contains(err.Error(), "no allowed-models list") {
		t.Errorf("expected empty-list error, got %v", err)
	}
}

// TestProviderRequiresLeaseRenewalBeforeExpiry confirms the connector
// surfaces a renewal prompt rather than failing closed when the lease
// is within the auto-renewal lead window. The renewal handshake must
// run before the next AI_OPEN.
func TestProviderRequiresLeaseRenewalBeforeExpiry(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	now := time.Now()
	// At acquire return now; on Present return now + 31s (inside the
	// 60s auto-renew window).
	advanced := false
	clock := func() time.Time {
		if !advanced {
			advanced = true
			return now
		}
		return now.Add(31 * time.Second)
	}
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      now,
		LeaseFor:     35 * time.Second, // 35s total; 4s left after clock advance
		AdvanceTime:  clock,
		LeaseModels:  []string{"patty-code-standard"},
	})
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail when the lease is in the renewal window")
	}
	if !errors.Is(err, paperproto.ErrLeaseRenewalDue) {
		t.Errorf("expected ErrLeaseRenewalDue sentinel, got %v", err)
	}
}

// TestProviderAcceptsValidLease is the green path: the connector holds
// a valid lease bound to the configured subject/session/epoch, and the
// requested model is in the lease's AllowList. The stream then fails
// with a connection-level error because no real relay is wired up, but
// the lease validation must NOT short-circuit the call.
func TestProviderAcceptsValidLease(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      time.Now(),
		LeaseFor:     time.Hour,
		LeaseModels:  []string{"patty-code-standard"},
	})
	// The lease validation has to pass; the connection dial is
	// expected to fail because we don't have a real relay. The error
	// must NOT mention "lease".
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err != nil && (strings.Contains(err.Error(), "lease") || errors.Is(err, paperproto.ErrLeaseExpired)) {
		t.Errorf("lease validation must pass with valid lease, got %v", err)
	}
	// The actual error is a dial failure. We just check that the
	// authorization checks are not the short-circuit.
}

// TestProviderLeaseMetricsExposesHeldLease covers the E1 quota/status
// surface: the connector reports the held lease's ID and sequence
// without exposing private key material.
func TestProviderLeaseMetricsExposesHeldLease(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      time.Now(),
		LeaseFor:     time.Hour,
		LeaseModels:  []string{"patty-code-standard"},
	})
	m := provider.LeaseMetrics()
	if m.HeldLeaseID != "lease-test" {
		t.Errorf("held lease id = %q, want lease-test", m.HeldLeaseID)
	}
	if m.HeldSequence != 1 {
		t.Errorf("held sequence = %d, want 1", m.HeldSequence)
	}
	if m.PolicyEpochID != "epoch-2026-01" {
		t.Errorf("policy epoch = %q, want epoch-2026-01", m.PolicyEpochID)
	}
}

// TestProviderSetSessionContextUpdatesBinding verifies the operator
// utility that re-binds the provider to a different session/epoch.
// After the rebind, the lease validation must honor the new context.
func TestProviderSetSessionContextUpdatesBinding(t *testing.T) {
	issuer := newTestLeaseIssuer(t)
	provider := NewForTest(&testConfig{
		RelayAddr:    "relay.example.com:8444",
		Model:        "patty-code-standard",
		LeaseIssuer:  issuer,
		LeaseSubject: "hrn:patty:test",
		LeaseSession: "ses-1",
		LeaseEpoch:   "epoch-2026-01",
		LeaseAt:      time.Now(),
		LeaseFor:     time.Hour,
		LeaseModels:  []string{"patty-code-standard"},
	})
	provider.SetSessionContext("hrn:patty:other", "ses-2", "epoch-2026-02")
	_, err := provider.Stream(testRequestContext(t), stubRequest("patty-code-standard"))
	if err == nil {
		t.Fatal("stream must fail after SetSessionContext to a different subject")
	}
	if !errors.Is(err, paperproto.ErrLeaseSubjectMismatch) {
		t.Errorf("expected ErrLeaseSubjectMismatch after rebind, got %v", err)
	}
}

// _ guards the provider import so it stays in the test file when the
// validateLease path is modified.
var _ = provider.New
