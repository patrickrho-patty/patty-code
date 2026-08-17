package textutil

import "golang.org/x/text/unicode/norm"

// NormalizeNFC converts s to Unicode Canonical Composition (NFC).
// If s is already in NFC or empty, it returns s directly without allocation.
func NormalizeNFC(s string) string {
	if s == "" || norm.NFC.IsNormalString(s) {
		return s
	}
	return norm.NFC.String(s)
}

// IsNFCNormalized reports whether s is in Unicode Canonical Composition (NFC).
func IsNFCNormalized(s string) bool {
	return norm.NFC.IsNormalString(s)
}
