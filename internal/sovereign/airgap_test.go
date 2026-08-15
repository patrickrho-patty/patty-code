package sovereign

import (
	"crypto/ed25519"
	"strings"
	"testing"
)

func sampleSource() (*TrustSource, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	return &TrustSource{
		OrganizationID: "org-test",
		SourceID:       "src-1",
		Name:           "Government Mirror",
		IssuerPub:      pub,
	}, priv
}

func sampleAdvisory(priv ed25519.PrivateKey) *UpdateAdvisory {
	a := &UpdateAdvisory{
		AdvisoryID: "adv-1",
		Version:    "1.2.0",
		Payload:    []byte("payload-bytes"),
		IssuedAt:   1_700_000_000_000,
		NotAfter:   2_000_000_000_000,
	}
	a.Signature = ed25519.Sign(priv, a.SigningBytes())
	return a
}

// TestAirGapModeDefaultsDisabled covers the trivial boundary: a
// fresh mode is disabled.
func TestAirGapModeDefaultsDisabled(t *testing.T) {
	m := NewAirGapMode()
	if m.IsEnabled() {
		t.Error("fresh mode must be disabled")
	}
}

// TestAirGapModeEnableDisable covers the toggle round-trip.
func TestAirGapModeEnableDisable(t *testing.T) {
	m := NewAirGapMode()
	m.Enable()
	if !m.IsEnabled() {
		t.Error("enable must flip to enabled")
	}
	m.Disable()
	if m.IsEnabled() {
		t.Error("disable must flip to disabled")
	}
}

// TestAirGapModeAllowsDialDisabled covers the green path: outside
// air-gap, all hosts are allowed.
func TestAirGapModeAllowsDialDisabled(t *testing.T) {
	m := NewAirGapMode()
	if !m.AllowsDial("any.example.com") {
		t.Error("disabled mode must allow any host")
	}
}

// TestAirGapModeAllowsDialAllowList covers the air-gap allow-list
// path.
func TestAirGapModeAllowsDialAllowList(t *testing.T) {
	m := NewAirGapMode()
	m.Enable()
	m.SetOnlineAllowList([]string{"mirror.example.com", "internal.corp"})
	if !m.AllowsDial("mirror.example.com") {
		t.Error("allow-list host must pass")
	}
	if m.AllowsDial("evil.example.com") {
		t.Error("non-allow-list host must fail in air-gap mode")
	}
}

// TestAirGapModeAppliesSignedAdvisory covers the green path: a
// properly-signed advisory lands.
func TestAirGapModeAppliesSignedAdvisory(t *testing.T) {
	src, priv := sampleSource()
	m := NewAirGapMode()
	m.SetTrustSources([]*TrustSource{src})

	advisory := sampleAdvisory(priv)
	if err := m.ApplyUpdateAdvisory(advisory, src.IssuerPub, 1_700_000_000_500); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if m.AppliedAdvisoryCount() != 1 {
		t.Errorf("applied = %d, want 1", m.AppliedAdvisoryCount())
	}
}

// TestAirGapModeRejectsTamperedAdvisory covers the trust
// boundary: an advisory signed by a different source is rejected.
func TestAirGapModeRejectsTamperedAdvisory(t *testing.T) {
	src, priv := sampleSource()
	_, otherPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("other key: %v", err)
	}
	_ = priv

	m := NewAirGapMode()
	m.SetTrustSources([]*TrustSource{src})
	advisory := sampleAdvisory(otherPriv) // wrong source
	if err := m.ApplyUpdateAdvisory(advisory, src.IssuerPub, 1_700_000_000_500); err == nil {
		t.Fatal("rogue-source advisory must fail")
	}
	if m.RejectedAdvisoryCount() != 1 {
		t.Errorf("rejected count = %d, want 1", m.RejectedAdvisoryCount())
	}
}

// TestAirGapModeRejectsExpiredAdvisory covers the E3 expiry
// boundary: a past-NotAfter advisory is rejected.
func TestAirGapModeRejectsExpiredAdvisory(t *testing.T) {
	src, priv := sampleSource()
	m := NewAirGapMode()
	m.SetTrustSources([]*TrustSource{src})
	advisory := sampleAdvisory(priv)
	advisory.NotAfter = 1_700_000_000_000 // expired before our apply time
	err := m.ApplyUpdateAdvisory(advisory, src.IssuerPub, 1_700_000_000_500)
	if err == nil {
		t.Fatal("expired advisory must fail")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got %v", err)
	}
}

// TestAirGapModeRejectsNil covers the trivial boundary.
func TestAirGapModeRejectsNil(t *testing.T) {
	m := NewAirGapMode()
	if err := m.ApplyUpdateAdvisory(nil, nil, 0); err == nil {
		t.Fatal("nil advisory must fail")
	}
}

// TestUpdateAdvisoryDigestIsDeterministic covers the audit-chain
// invariant.
func TestUpdateAdvisoryDigestIsDeterministic(t *testing.T) {
	_, priv := sampleSource()
	a := sampleAdvisory(priv)
	if a.Digest() != a.Digest() {
		t.Fatal("digest is non-deterministic")
	}
}

// TestUpdateAdvisoryIsExpired covers the expiry boundary.
func TestUpdateAdvisoryIsExpired(t *testing.T) {
	_, priv := sampleSource()
	a := sampleAdvisory(priv)
	if a.IsExpired(a.NotAfter - 1) {
		t.Error("advisory just before NotAfter must not be expired")
	}
	if !a.IsExpired(a.NotAfter + 1) {
		t.Error("advisory past NotAfter must be expired")
	}
	if a.IsExpired(0) {
		// NotAfter=0 means "no expiry"
		t.Error("NotAfter=0 must mean no expiry")
	}
}

// TestUpdateAdvisorySigningBytesBoundAllFields pins the binding
// invariant.
func TestUpdateAdvisorySigningBytesBoundAllFields(t *testing.T) {
	_, priv := sampleSource()
	primary := sampleAdvisory(priv).SigningBytes()
	mutations := []struct {
		name string
		fn   func(*UpdateAdvisory)
	}{
		{"id", func(a *UpdateAdvisory) { a.AdvisoryID = "adv-2" }},
		{"version", func(a *UpdateAdvisory) { a.Version = "2.0.0" }},
		{"issued_at", func(a *UpdateAdvisory) { a.IssuedAt = 1 }},
		{"not_after", func(a *UpdateAdvisory) { a.NotAfter = 2 }},
		{"payload", func(a *UpdateAdvisory) { a.Payload = []byte("p") }},
	}
	for _, m := range mutations {
		clone := sampleAdvisory(priv)
		m.fn(clone)
		if string(clone.SigningBytes()) == string(primary) {
			t.Errorf("signing bytes unchanged after %s mutation", m.name)
		}
	}
}
