package provenancewire

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
	"time"
)

func TestVerifyEvidenceReceiptSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	env := &EvidenceReceiptEnvelope{
		ReceiptID: "rcpt-x", ExchangeID: "exch-x",
		FinalState: "COMPLETED", RelayIdentity: "relay-1",
		PolicyEpochID: "ep-1", ModelPackageID: "pmp-1",
		IssuedAtUnixMs: 1700000000000,
	}
	var chain [32]byte
	copy(chain[:], "chainroot")
	env.ChainRoot = chain

	issuedAt := time.UnixMilli(env.IssuedAtUnixMs).UTC().Format(time.RFC3339)
	data := env.ExchangeID + "|" + env.FinalState + "|" + hex.EncodeToString(env.ChainRoot[:]) + "|" +
		env.RelayIdentity + "|" + env.PolicyEpochID + "|" + env.ModelPackageID + "|" + issuedAt
	env.Signature = hex.EncodeToString(ed25519.Sign(priv, []byte(data)))

	if err := VerifyEvidenceReceiptSignature(env, priv.Public().(ed25519.PublicKey)); err != nil {
		t.Fatalf("valid receipt must verify: %v", err)
	}
	other, _, _ := ed25519.GenerateKey(nil)
	if err := VerifyEvidenceReceiptSignature(env, other); err == nil {
		t.Fatal("wrong key must fail")
	}
	env.Signature = hex.EncodeToString([]byte("garbage"))
	if err := VerifyEvidenceReceiptSignature(env, priv.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("garbage signature must fail")
	}
}

func TestDiskReceiptStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s1, err := NewDiskReceiptStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	env := &EvidenceReceiptEnvelope{ReceiptID: "rcpt-disk", ExchangeID: "exch-d", IssuedAtUnixMs: 1}
	if _, err := s1.Store(env, 1); err != nil {
		t.Fatal(err)
	}
	// A fresh store over the same dir replays it (restart survival).
	s2, err := NewDiskReceiptStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.Get("rcpt-disk"); !ok {
		t.Fatal("receipt must survive restart via disk store")
	}
}
