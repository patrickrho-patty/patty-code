package dariproto

// legacy_paper1.go is the ONLY home of the legacy `paper/1`
// compatibility surface (master plan Task 21). The DARI client
// prefers `dari/1`; `paper/1` — and the historical wire preface
// bytes — are retained so a DARI connector can still reach a
// not-yet-migrated relay. Active client paths reference these named
// legacy constants; no other file may hardcode the legacy literals.
//
// Compatibility profile (bounded):
//   - ALPN: offer `dari/1` first, `paper/1` second.
//   - Preface: the 8-byte connection preface still spells the
//     historical name. Changing it would break every deployed
//     transport; it stays frozen and is asserted byte-for-byte in
//     compatibility_test.go.

// LegacyPaper1ALPN is the historical ALPN identifier. Offered only as
// the fallback protocol identifier during TLS negotiation.
const LegacyPaper1ALPN = "paper/1"

// DARIProtocol is the canonical ALPN identifier for DARI.
const DARIProtocol = "dari/1"

// LegacyPaper1Preface is the frozen 8-byte connection preface
// ("P-A-P-E-R", version 1, frame-kind 0x0A) every DARI and legacy
// transport writes and expects. The bytes are historical; they are
// kept for wire compatibility and pinned by the compatibility test.
var LegacyPaper1Preface = []byte{0x50, 0x41, 0x50, 0x45, 0x52, 0x00, 0x01, 0x0A}

// ALPNProtocols returns the ALPN offer list: DARI first, legacy
// `paper/1` as the fallback so a DARI connector can reach a
// not-yet-migrated relay.
func ALPNProtocols() []string { return []string{DARIProtocol, LegacyPaper1ALPN} }

// CompatEnabled reports whether the legacy profile is available in
// this build. The legacy surface is compile-time bounded: this
// constant exists so callers can gate behavior, and flipping it to
// false removes `paper/1` from the offer list in a follow-up.
const CompatEnabled = true
