package textutil

import (
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestNormalizeNFC(t *testing.T) {
	// NFD representation of "안녕하세요" (decomposed Jamo)
	nfd := norm.NFD.String("안녕하세요")
	if IsNFCNormalized(nfd) {
		t.Fatal("expected NFD string to return false for IsNFCNormalized")
	}

	normalized := NormalizeNFC(nfd)
	if !IsNFCNormalized(normalized) {
		t.Error("NormalizeNFC failed to compose NFD to NFC")
	}
	if normalized != "안녕하세요" {
		t.Errorf("got %q, want %q", normalized, "안녕하세요")
	}

	// NFC pass-through
	nfc := "안녕하세요"
	if got := NormalizeNFC(nfc); got != nfc {
		t.Errorf("NFC pass-through modified string: got %q", got)
	}

	// Mixed Latin + NFD Korean + symbols
	mixedNFD := "Hello " + norm.NFD.String("세계") + " 123!"
	mixedNFC := NormalizeNFC(mixedNFD)
	if mixedNFC != "Hello 세계 123!" {
		t.Errorf("got %q, want %q", mixedNFC, "Hello 세계 123!")
	}

	// Empty string
	if got := NormalizeNFC(""); got != "" {
		t.Errorf("empty string normalized to %q", got)
	}
}
