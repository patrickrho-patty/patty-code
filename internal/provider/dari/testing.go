package dari

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"patty/internal/dariproto"
	"patty/internal/provider"
)

// testLeaseIssuer signs a fresh lease for the connector's auth subject.
// The relay's `policy.IssueCapabilityLease` performs the same Ed25519
// signing path, so the bytes the connector verifies under this issuer
// are byte-for-byte a relay-issued lease.
type testLeaseIssuer struct {
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	id   string
}

func newTestLeaseIssuer(t *testing.T) *testLeaseIssuer {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("issuer key: %v", err)
	}
	return &testLeaseIssuer{pub: pub, priv: priv, id: "pccp-policy"}
}

func (i *testLeaseIssuer) Lease(t *testing.T, body dariproto.LeaseBody) *dariproto.Lease {
	lease := &dariproto.Lease{
		Version:         1,
		Issuer:          i.id,
		LeaseID:         body.LeaseID,
		SubjectPeerID:   body.SubjectPeerID,
		UserID:          body.UserID,
		SessionID:       body.SessionID,
		PolicyEpochID:   body.PolicyEpochID,
		AllowedModels:   body.AllowedModels,
		ToolClasses:     body.ToolClasses,
		TokenBudget:     body.TokenBudget,
		NotBeforeUnixMs: body.NotBeforeUnixMs,
		NotAfterUnixMs:  body.NotAfterUnixMs,
		IssuedAtUnixMs:  body.IssuedAtUnixMs,
		LeaseSequence:   body.LeaseSequence,
		Status:          "active",
	}
	bodyBytes := lease.SigningBytes()
	encoded, err := dariproto.CreateCOSESign1(bodyBytes, i.priv, []byte(i.id))
	if err != nil {
		if t != nil {
			t.Fatalf("sign lease: %v", err)
		}
		panic("sign lease: " + err.Error())
	}
	lease.Signature = hexEncode(encoded)
	return lease
}

func hexEncode(b []byte) string {
	const hexchars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[2*i] = hexchars[v>>4]
		out[2*i+1] = hexchars[v&0xf]
	}
	return string(out)
}

// testConfig is the harness-only factory input used by the lease-wiring
// tests. Each field controls a single axis of the connector's
// authorization surface so the test can pin the failure mode without
// reading the full Config struct.
type testConfig struct {
	RelayAddr            string
	Model                string
	LeaseIssuer          *testLeaseIssuer
	LeaseSubject         string
	LeaseSession         string
	LeaseEpoch           string
	LeaseAt              time.Time
	LeaseFor             time.Duration
	LeaseSubjectOverride string
	LeaseModels          []string
	SessionPolicyEpoch   string
	// AdvanceTime overrides the lease client's clock. The test uses
	// this to drive expiry without sleeping.
	AdvanceTime func() time.Time
}

// NewForTest constructs a Provider from a test-only config. The
// provider's connection is a stub (no real dial) so the test can
// observe the lease-validation path without touching a DARI relay.
func NewForTest(cfg *testConfig) *Provider {
	leaseClient := newLeaseClientForTest(cfg)
	// The session's bound policy epoch may differ from the lease's
	// epoch. The default is to bind to the lease's epoch; tests that
	// want a mismatch set SessionPolicyEpoch explicitly.
	boundEpoch := cfg.LeaseEpoch
	if cfg.SessionPolicyEpoch != "" {
		boundEpoch = cfg.SessionPolicyEpoch
	}
	p := &Provider{
		name:            "paper-test",
		model:           cfg.Model,
		relayAddr:       cfg.RelayAddr,
		leaseClient:     leaseClient,
		subjectPeerID:   cfg.LeaseSubject,
		sessionID:       cfg.LeaseSession,
		policyEpoch:     boundEpoch,
		nowFn:           time.Now,
		autoRenewBefore: time.Minute,
	}
	if cfg.LeaseIssuer != nil {
		if err := leaseClient.Acquire(cfg.LeaseSubject, cfg.LeaseSession, signedLeaseForTest(cfg)); err != nil {
			// Test setup failure surfaces here so the test fails
			// immediately rather than later when Stream() runs.
			panic("lease acquire failed in test setup: " + err.Error())
		}
	}
	return p
}

// acquireLease constructs and acquires a signed lease for the connector.
// It mirrors the NewForTest's lease-acquisition path but returns the
// lease so the test can inspect intermediate state if needed.
func acquireLease(t *testing.T, cfg *testConfig, client *dariproto.LeaseClient) {
	t.Helper()
	if err := client.Acquire(cfg.LeaseSubject, cfg.LeaseSession, signedLeaseForTest(cfg)); err != nil {
		t.Fatalf("lease acquire failed in test setup: %v", err)
	}
}

// newLeaseClientForTest wires a lease client with the issuer's pub key
// and a fixed clock so the test can drive expiry deterministically.
func newLeaseClientForTest(cfg *testConfig) *dariproto.LeaseClient {
	if cfg.LeaseIssuer == nil {
		return dariproto.NewLeaseClient(ed25519.PublicKey{}, "")
	}
	client := dariproto.NewLeaseClient(cfg.LeaseIssuer.pub, cfg.LeaseIssuer.id).
		WithAutoRenewBefore(time.Minute)
	if cfg.AdvanceTime != nil {
		client.WithNowFunc(cfg.AdvanceTime)
	}
	return client
}

// signedLeaseForTest builds the lease body the connector holds for the
// duration of the test. The body mirrors the relay's
// `policy.IssueCapabilityLease` signing input.
func signedLeaseForTest(cfg *testConfig) *dariproto.Lease {
	now := cfg.LeaseAt
	if now.IsZero() {
		now = time.Now()
	}
	subject := cfg.LeaseSubject
	if cfg.LeaseSubjectOverride != "" {
		subject = cfg.LeaseSubjectOverride
	}
	body := dariproto.LeaseBody{
		LeaseID:         "lease-test",
		SubjectPeerID:   subject,
		UserID:          "alice",
		SessionID:       cfg.LeaseSession,
		PolicyEpochID:   cfg.LeaseEpoch,
		AllowedModels:   cfg.LeaseModels,
		NotBeforeUnixMs: now.UnixMilli(),
		NotAfterUnixMs:  now.Add(cfg.LeaseFor).UnixMilli(),
		IssuedAtUnixMs:  now.UnixMilli(),
		LeaseSequence:   1,
	}
	return cfg.LeaseIssuer.Lease(nil, body)
}

// testRequestContext returns a minimal context for the test. The
// provider's Stream() signature takes a context.Context; the test does
// not exercise cancellation, so a plain context.Background() is enough.
func testRequestContext(t *testing.T) context.Context {
	return context.Background()
}

// stubRequest is the test request shape that drives the provider's
// fail-closed checks. The provider's Stream signature requires a
// provider.Request; the test only needs the Model field populated.
func stubRequest(model string) provider.Request {
	return provider.Request{
		Messages:  []provider.Message{{Role: provider.RoleUser, Content: "hello"}},
		MaxTokens: 16,
	}
}

// _ keeps the time import in use when only the duration appears in
// the test request shape.
var _ = testLeaseIssuer{}
