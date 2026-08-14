package dariproto

import (
	"bytes"
	"testing"
)

// compatibility_test.go pins the legacy `paper/1` compatibility
// surface byte-for-byte (master plan Task 21). If anyone changes the
// preface bytes or the legacy ALPN literal, this file fails: the
// values are protocol-frozen compatibility artifacts, not naming.

func TestLegacyPaper1PrefaceFrozen(t *testing.T) {
	want := []byte{0x50, 0x41, 0x50, 0x45, 0x52, 0x00, 0x01, 0x0A}
	if !bytes.Equal(LegacyPaper1Preface, want) {
		t.Fatalf("legacy preface bytes changed: got %x want %x", LegacyPaper1Preface, want)
	}
	if len(LegacyPaper1Preface) != 8 {
		t.Fatalf("legacy preface must stay 8 bytes, got %d", len(LegacyPaper1Preface))
	}
}

func TestLegacyPaper1ALPNLiteral(t *testing.T) {
	if LegacyPaper1ALPN != "paper/1" {
		t.Fatalf("legacy ALPN literal changed: %q", LegacyPaper1ALPN)
	}
}

func TestALPNOfferOrderDARIFirst(t *testing.T) {
	protos := ALPNProtocols()
	if len(protos) != 2 || protos[0] != DARIProtocol || protos[1] != LegacyPaper1ALPN {
		t.Fatalf("ALPN offer must be [dari/1, paper/1], got %v", protos)
	}
}
