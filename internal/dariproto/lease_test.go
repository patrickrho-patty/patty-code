package dariproto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// leaseIssuerFixture stands up a CP/relay-shaped lease issuer used by the
// lease lifecycle tests. The issuer is a real Ed25519 key + CBOR-shape
// signer so the connector's verification path runs against the same byte
// contract the relay's `policy.IssueCapabilityLease` produces.
type leaseIssuerFixture struct {
	issuerID  string
	priv      ed25519.PrivateKey
	publicKey ed25519.PublicKey
}

func newLeaseIssuerFixture(t *testing.T) leaseIssuerFixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("issuer key: %v", err)
	}
	return leaseIssuerFixture{
		issuerID:  "pccp-policy",
		priv:      priv,
		publicKey: pub,
	}
}

// IssueLease signs a CapabilityLease body using the relay's exact signing
// shape: SHA-256 of the canonical body (id|subject|user|session|epoch|not-
// before|not-after), COSE-Sign1 wrapped, hex-encoded. The relay's
// `policy.IssueCapabilityLease` writes the same string into
// `lease.CPSignature`, so a connector that verifies it under a known
// issuer key is exercising the live path.
func (f leaseIssuerFixture) IssueLease(t *testing.T, body LeaseBody) *Lease {
	t.Helper()
	lease := &Lease{
		Version:            1,
		Issuer:             f.issuerID,
		LeaseID:            body.LeaseID,
		SubjectPeerID:      body.SubjectPeerID,
		UserID:             body.UserID,
		SessionID:          body.SessionID,
		PolicyEpochID:      body.PolicyEpochID,
		AllowedModels:      body.AllowedModels,
		RepositoryScope:    body.RepositoryScope,
		FilePathReadScope:  body.FilePathReadScope,
		FilePathWriteScope: body.FilePathWriteScope,
		ToolClasses:        body.ToolClasses,
		TokenBudget:        body.TokenBudget,
		NotBeforeUnixMs:    body.NotBeforeUnixMs,
		NotAfterUnixMs:     body.NotAfterUnixMs,
		LeaseSequence:      body.LeaseSequence,
		IssuedAtUnixMs:     body.IssuedAtUnixMs,
		Status:             "active",
	}
	lease.Signature = f.signBody(t, lease)
	return lease
}

func (f leaseIssuerFixture) signBody(t *testing.T, lease *Lease) string {
	t.Helper()
	body := lease.SigningBytes()
	encoded, err := CreateCOSESign1(body, f.priv, []byte("pccp-policy"))
	if err != nil {
		t.Fatalf("sign lease: %v", err)
	}
	return hex.EncodeToString(encoded)
}

// TestLeaseVerifierAcceptsIssuerSignature pins the byte contract: the
// connector verifies the lease under the issuer's public key, the subject
// peer matches the authenticated harness, and the validity window covers
// the current time. Anything else is rejected.
func TestLeaseVerifierAcceptsIssuerSignature(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-test-1",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-test-1",
		PolicyEpochID:   "epoch-2026-01",
		AllowedModels:   []string{"patty-code-standard"},
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	verifier := NewLeaseVerifier(issuer.publicKey, "pccp-policy")
	if err := verifier.Verify(lease, "hrn:patty:test", "ses-test-1", now); err != nil {
		t.Fatalf("verifier rejected a structurally valid lease: %v", err)
	}
}

// TestLeaseVerifierRejectsExpiredLease exercises the fail-closed path the
// connector must take when a capacity lease elapses mid-session. The relay
// revokes or 410s the lease; the connector's verifier reports `LeaseExpired`
// before any AI_OPEN or renew handshake.
func TestLeaseVerifierRejectsExpiredLease(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-expired",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-test-2",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now - 120_000,
		NotAfterUnixMs:  now - 60_000, // already expired
		IssuedAtUnixMs:  now - 120_000,
		LeaseSequence:   1,
	})
	verifier := NewLeaseVerifier(issuer.publicKey, "pccp-policy")
	if err := verifier.Verify(lease, "hrn:patty:test", "ses-test-2", now); err == nil {
		t.Fatal("expired lease must fail verification")
	} else if !IsLeaseExpired(err) {
		t.Fatalf("expected LeaseExpired sentinel, got %v", err)
	}
}

// TestLeaseVerifierRejectsSubjectMismatch guards against a malicious harness
// re-using another peer's lease. A3 requires the issuer-signed subject bind
// to match the authenticated harness identity.
func TestLeaseVerifierRejectsSubjectMismatch(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-mismatch",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-test-3",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	verifier := NewLeaseVerifier(issuer.publicKey, "pccp-policy")
	err := verifier.Verify(lease, "hrn:patty:attacker", "ses-test-3", now)
	if err == nil {
		t.Fatal("subject-mismatch lease must fail verification")
	}
	if !IsLeaseSubjectMismatch(err) {
		t.Fatalf("expected LeaseSubjectMismatch sentinel, got %v", err)
	}
}

// TestLeaseVerifierRejectsTamperedBody confirms the issuer signature binds
// the entire lease body. Any field re-order, scope widening, or expiry
// extension breaks the verifier.
func TestLeaseVerifierRejectsTamperedBody(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-tamper",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-test-4",
		PolicyEpochID:   "epoch-2026-01",
		AllowedModels:   []string{"patty-code-standard"},
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	// Wide the scope to an administration tool the harness should never have.
	lease.AllowedModels = append(lease.AllowedModels, "shell.admin")
	verifier := NewLeaseVerifier(issuer.publicKey, "pccp-policy")
	err := verifier.Verify(lease, "hrn:patty:test", "ses-test-4", now)
	if err == nil {
		t.Fatal("broadened lease body verified")
	}
	if !IsLeaseSignatureInvalid(err) {
		t.Fatalf("expected LeaseSignatureInvalid sentinel, got %v", err)
	}
}

// TestLeaseVerifierRejectsRevokedStatus gates the relay's `RevokeCapabilityLease`
// outcome. A lease with `Status: revoked` must fail verification even if its
// signature and validity window are otherwise intact.
func TestLeaseVerifierRejectsRevokedStatus(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-revoked",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-test-5",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	lease.Status = "revoked"
	verifier := NewLeaseVerifier(issuer.publicKey, "pccp-policy")
	err := verifier.Verify(lease, "hrn:patty:test", "ses-test-5", now)
	if err == nil {
		t.Fatal("revoked lease must fail verification")
	}
	if !IsLeaseRevoked(err) {
		t.Fatalf("expected LeaseRevoked sentinel, got %v", err)
	}
}

// TestLeaseBuilderAdvancesSequenceAndSignature guards the renewal path: a
// fresh lease with the same leaseID but a higher `LeaseSequence` and a
// later `NotAfterUnixMs` must verify.
func TestLeaseBuilderAdvancesSequenceAndSignature(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	original := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-renew",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses-test-6",
		PolicyEpochID:   "epoch-2026-01",
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	renewed := issuer.IssueLease(t, LeaseBody{
		LeaseID:         original.LeaseID,
		SubjectPeerID:   original.SubjectPeerID,
		UserID:          original.UserID,
		SessionID:       original.SessionID,
		PolicyEpochID:   original.PolicyEpochID,
		NotBeforeUnixMs: now,
		NotAfterUnixMs:  now + 5*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   original.LeaseSequence + 1,
	})
	if renewed.LeaseSequence != original.LeaseSequence+1 {
		t.Fatalf("renewed sequence = %d, want %d", renewed.LeaseSequence, original.LeaseSequence+1)
	}
	if renewed.Signature == original.Signature {
		t.Fatal("renewed signature must differ from original")
	}
	verifier := NewLeaseVerifier(issuer.publicKey, "pccp-policy")
	if err := verifier.Verify(renewed, "hrn:patty:test", "ses-test-6", now); err != nil {
		t.Fatalf("renewed lease must verify: %v", err)
	}
}

// TestLeaseSigningBytesAreBoundToAllFields guards against a relay/connector
// drift that drops a field from the signed body. The signing input must
// change when to-every field changes; otherwise a verifier that doesn't
// recheck the field could let a malicious lease pass.
func TestLeaseSigningBytesAreBoundToAllFields(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	base := LeaseBody{
		LeaseID:         "lease-binding",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses",
		PolicyEpochID:   "epoch",
		NotBeforeUnixMs: now,
		NotAfterUnixMs:  now + 60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	}
	lease := &Lease{
		Version:         1,
		Issuer:          issuer.issuerID,
		LeaseID:         base.LeaseID,
		SubjectPeerID:   base.SubjectPeerID,
		UserID:          base.UserID,
		SessionID:       base.SessionID,
		PolicyEpochID:   base.PolicyEpochID,
		NotBeforeUnixMs: base.NotBeforeUnixMs,
		NotAfterUnixMs:  base.NotAfterUnixMs,
		IssuedAtUnixMs:  base.IssuedAtUnixMs,
		LeaseSequence:   base.LeaseSequence,
		Status:          "active",
	}
	primary := lease.SigningBytes()
	mutations := []struct {
		name string
		fn   func(*Lease)
	}{
		{"not_after", func(l *Lease) { l.NotAfterUnixMs += 1 }},
		{"not_before", func(l *Lease) { l.NotBeforeUnixMs += 1 }},
		{"session", func(l *Lease) { l.SessionID = "ses-other" }},
		{"policy_epoch", func(l *Lease) { l.PolicyEpochID = "epoch-2" }},
		{"subject", func(l *Lease) { l.SubjectPeerID = "hrn:patty:other" }},
		{"tool_classes", func(l *Lease) { l.ToolClasses = []string{"shell"} }},
		{"file_read", func(l *Lease) { l.FilePathReadScope = []string{"/etc"} }},
		{"file_write", func(l *Lease) { l.FilePathWriteScope = []string{"/etc"} }},
		{"repo", func(l *Lease) { l.RepositoryScope = []map[string]string{{"repo": "x"}} }},
		{"token_budget", func(l *Lease) { l.TokenBudget = 1 }},
		{"sequence", func(l *Lease) { l.LeaseSequence += 1 }},
	}
	for _, m := range mutations {
		clone := *lease
		m.fn(&clone)
		if h := sha256.Sum256(clone.SigningBytes()); h == sha256.Sum256(primary) {
			t.Errorf("signing bytes unchanged after %s mutation", m.name)
		}
	}
}

// TestLeaseCBORRoundtrip is the byte-contract guard: the connector decodes
// what the relay signed. Both sides MUST marshal/unmarshal the same fields
// in the same order; a missing label or rearranged field would silently
// change the signature domain.
func TestLeaseCBORRoundtrip(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-roundtrip",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses",
		PolicyEpochID:   "epoch",
		AllowedModels:   []string{"patty-code-standard"},
		RepositoryScope: []map[string]string{{"repo": "pccp", "branch": "main"}},
		ToolClasses:     []string{"repo.read", "repo.write"},
		TokenBudget:     8192,
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	bytes, err := MarshalCBOR(lease)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Lease
	if err := UnmarshalCBOR(bytes, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.LeaseID != lease.LeaseID {
		t.Errorf("lease id drift: %q vs %q", decoded.LeaseID, lease.LeaseID)
	}
	if len(decoded.AllowedModels) != 1 || decoded.AllowedModels[0] != "patty-code-standard" {
		t.Errorf("allowed models drift: %v", decoded.AllowedModels)
	}
	if decoded.TokenBudget != 8192 {
		t.Errorf("token budget drift: %d", decoded.TokenBudget)
	}
	if decoded.Signature != lease.Signature {
		t.Errorf("signature drift: %q vs %q", decoded.Signature, lease.Signature)
	}
}

// TestLeaseVerifierRejectsUnknownIssuer exercises the trust-bundle boundary:
// the connector must not accept a lease whose issuer is not in its
// configured TrustBundle. A3's authority to "fail-closed" means a
// wrong-key presentation is rejected, not silently trusted.
func TestLeaseVerifierRejectsUnknownIssuer(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	notTheIssuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-unknown-issuer",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses",
		PolicyEpochID:   "epoch",
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	verifier := NewLeaseVerifier(notTheIssuer.publicKey, "some-other-issuer")
	err := verifier.Verify(lease, "hrn:patty:test", "ses", now)
	if err == nil {
		t.Fatal("lease signed by an unknown issuer must fail verification")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("expected issuer-mismatch error, got %v", err)
	}
}

// TestLeaseVerifierRejectsEmptyIssuerWhenBundleConfigured is the
// fail-closed boundary: the verifier MUST refuse a lease whose Issuer
// field is empty when the connector has pinned an issuer ID. An empty
// issuer would otherwise bypass the trust bundle entirely.
func TestLeaseVerifierRejectsEmptyIssuerWhenBundleConfigured(t *testing.T) {
	issuer := newLeaseIssuerFixture(t)
	now := time.Now().UnixMilli()
	lease := issuer.IssueLease(t, LeaseBody{
		LeaseID:         "lease-empty-issuer",
		SubjectPeerID:   "hrn:patty:test",
		UserID:          "alice",
		SessionID:       "ses",
		PolicyEpochID:   "epoch",
		NotBeforeUnixMs: now - 60_000,
		NotAfterUnixMs:  now + 60*60_000,
		IssuedAtUnixMs:  now,
		LeaseSequence:   1,
	})
	lease.Issuer = "" // simulate a relay that omits the field
	verifier := NewLeaseVerifier(issuer.publicKey, issuer.issuerID)
	err := verifier.Verify(lease, "hrn:patty:test", "ses", now)
	if err == nil {
		t.Fatal("lease with empty issuer must be rejected when the verifier has a pinned issuer")
	}
}
