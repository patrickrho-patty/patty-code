package paperproto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// LeaseBody is the input the connector hands to the lease issuer/renewer
// helper. Every field is signed by the issuer; any field the relay omits
// must remain at the zero value so the verifier's signing-bytes contract
// stays stable.
type LeaseBody struct {
	LeaseID         string
	SubjectPeerID   string
	UserID          string
	SessionID       string
	PolicyEpochID   string
	AllowedModels   []string
	RepositoryScope []map[string]string
	FilePathReadScope []string
	FilePathWriteScope []string
	ToolClasses     []string
	TokenBudget     int64
	NotBeforeUnixMs int64
	NotAfterUnixMs  int64
	IssuedAtUnixMs  int64
	LeaseSequence   uint64
}

// Lease is the connector-side decoded view of a relay-issued capability
// lease. The relay's `policy.IssueCapabilityLease` writes the same fields
// into `models.CapabilityLease`; the connector receives the COSE-Sign1
// signature alongside the body and uses it to authorize each
// governance-gated exchange.
type Lease struct {
	Version        uint16 `cbor:"1,keyasint"`
	Issuer         string `cbor:"2,keyasint"`
	LeaseID        string `cbor:"3,keyasint"`
	SubjectPeerID  string `cbor:"4,keyasint"`
	UserID         string `cbor:"5,keyasint"`
	SessionID      string `cbor:"6,keyasint,omitempty"`
	PolicyEpochID  string `cbor:"7,keyasint"`
	AllowedModels  []string `cbor:"8,keyasint,omitempty"`
	RepositoryScope []map[string]string `cbor:"9,keyasint,omitempty"`
	FilePathReadScope []string `cbor:"10,keyasint,omitempty"`
	FilePathWriteScope []string `cbor:"11,keyasint,omitempty"`
	ToolClasses    []string `cbor:"12,keyasint,omitempty"`
	TokenBudget    int64  `cbor:"13,keyasint,omitempty"`
	NotBeforeUnixMs int64 `cbor:"14,keyasint"`
	NotAfterUnixMs  int64 `cbor:"15,keyasint"`
	LeaseSequence  uint64 `cbor:"16,keyasint"`
	IssuedAtUnixMs int64 `cbor:"17,keyasint,omitempty"`
	Status         string `cbor:"18,keyasint,omitempty"`
	// Signature is the hex-encoded COSE-Sign1 wrapping the canonical
	// lease body. The connector MUST verify it before relying on any
	// field of the lease.
	Signature string `cbor:"19,keyasint,omitempty"`
}

// LeaseDomain is the constant domain-separation prefix applied to the
// lease signing bytes. It must match the relay's `policy.IssueCapabilityLease`
// body-format (`leaseID|subject|user|session|epoch|notBefore|notAfter`)
// exactly. The connector mirrors the exact byte layout so the relay's
// pre-existing signature stays valid.
const LeaseDomain = "DARI-CAPABILITY-LEASE-v1\x00"

// NewLeaseVerifier builds a LeaseVerifier that pins the issuer identity
// and subject-key thumbprint. The connector constructs one of these from
// the trust bundle pushed by the relay at AUTH_PROOF time, so a lease
// issued by a peer that is not in the bundle is rejected before any
// governance-gated exchange.
func NewLeaseVerifier(issuerPub ed25519.PublicKey, issuerID string) *LeaseVerifier {
	return &LeaseVerifier{
		issuerPub: append(ed25519.PublicKey(nil), issuerPub...),
		issuerID:  issuerID,
	}
}

// LeaseVerifier validates a relay-issued lease against the issuer's
// public key and the connector's authenticated identity. The verifier
// never silently downgrades an error; every failure returns a sentinel
// the caller can surface to operators (UI banner, audit log, etc.).
type LeaseVerifier struct {
	issuerPub ed25519.PublicKey
	issuerID  string
}

// Verify returns nil if the lease is structurally valid, signed by the
// configured issuer, bound to the same subject/session as the authenticated
// harness, and unexpired at the supplied nowMs. Any failure returns a
// sentinel error the caller can match with IsLeaseExpired etc.
func (v *LeaseVerifier) Verify(lease *Lease, subjectPeerID, sessionID string, nowMs int64) error {
	if lease == nil {
		return ErrLeaseInvalid
	}
	if lease.Issuer != "" && v.issuerID != "" && lease.Issuer != v.issuerID {
		return fmt.Errorf("paper: lease issuer %q is not in trust bundle (expected %q)", lease.Issuer, v.issuerID)
	}
	if strings.EqualFold(lease.Status, "revoked") {
		return ErrLeaseRevoked
	}
	if strings.EqualFold(lease.Status, "expired") {
		return ErrLeaseExpired
	}
	if lease.SubjectPeerID != "" && lease.SubjectPeerID != subjectPeerID {
		return ErrLeaseSubjectMismatch
	}
	if sessionID != "" && lease.SessionID != "" && lease.SessionID != sessionID {
		return fmt.Errorf("paper: lease session %q does not match connection session %q", lease.SessionID, sessionID)
	}
	if lease.NotBeforeUnixMs > 0 && nowMs < lease.NotBeforeUnixMs {
		return fmt.Errorf("paper: lease not yet valid (now=%d, notBefore=%d)", nowMs, lease.NotBeforeUnixMs)
	}
	if lease.NotAfterUnixMs > 0 && nowMs >= lease.NotAfterUnixMs {
		return ErrLeaseExpired
	}
	if err := v.verifySignature(lease); err != nil {
		return fmt.Errorf("%w: %v", ErrLeaseSignatureInvalid, err)
	}
	return nil
}

// verifySignature decrypts the COSE-Sign1 envelope, recomputes the
// canonical signing bytes from the lease fields, and verifies the Ed25519
// signature under the configured issuer key. A drift in the field set
// or the byte layout fails verification.
func (v *LeaseVerifier) verifySignature(lease *Lease) error {
	if lease.Signature == "" {
		return errors.New("missing COSE-Sign1 signature")
	}
	raw, err := hex.DecodeString(lease.Signature)
	if err != nil {
		return fmt.Errorf("decode signature hex: %w", err)
	}
	sign1, err := DecodeCOSESign1(raw)
	if err != nil {
		return fmt.Errorf("decode COSE-Sign1: %w", err)
	}
	if !bytesEqual(sign1.Payload, lease.SigningBytes()) {
		return errors.New("COSE-Sign1 payload does not match presented lease body")
	}
	if err := VerifyCOSESign1(sign1, v.issuerPub); err != nil {
		return err
	}
	return nil
}

// SigningBytes produces the canonical byte string the issuer signs and
// the verifier recomputes. The bytes are domain-separated, length-prefixed
// for every string/slice/uint64 field, and big-endian for numeric values.
// Field order matches the relay's existing `policy.IssueCapabilityLease`
// signing body (id|subject|user|session|epoch|notBefore|notAfter) so the
// connector honors every relay already running.
func (l *Lease) SigningBytes() []byte {
	canonical := make([]byte, 0, 256)
	canonical = append(canonical, []byte(LeaseDomain)...)
	canonical = writeLengthPrefixedString(canonical, l.LeaseID)
	canonical = writeLengthPrefixedString(canonical, l.SubjectPeerID)
	canonical = writeLengthPrefixedString(canonical, l.UserID)
	canonical = writeLengthPrefixedString(canonical, l.SessionID)
	canonical = writeLengthPrefixedString(canonical, l.PolicyEpochID)
	// Scope fields. Each is bound so a relay/connector drift that
	// quietly drops a field (e.g. tool_classes) cannot silently upgrade
	// the lease's authority.
	canonical = writeLengthPrefixedStringSlice(canonical, l.AllowedModels)
	canonical = writeLengthPrefixedStringSlice(canonical, l.FilePathReadScope)
	canonical = writeLengthPrefixedStringSlice(canonical, l.FilePathWriteScope)
	canonical = writeLengthPrefixedStringSlice(canonical, l.ToolClasses)
	canonical = writeLengthPrefixedRepoScope(canonical, l.RepositoryScope)
	canonical = writeLengthPrefixedU64(canonical, uint64(l.TokenBudget))
	canonical = writeLengthPrefixedU64(canonical, uint64(l.NotBeforeUnixMs))
	canonical = writeLengthPrefixedU64(canonical, uint64(l.NotAfterUnixMs))
	canonical = writeLengthPrefixedU64(canonical, l.LeaseSequence)
	canonical = writeLengthPrefixedU64(canonical, uint64(l.IssuedAtUnixMs))
	return canonical
}

// writeLengthPrefixedStringSlice binds a string slice before signing.
// Each element is prefixed with its own uint32 length so a verifier can
// bound the trailing bytes without re-marshaling.
func writeLengthPrefixedStringSlice(dst []byte, values []string) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(values)))
	dst = append(dst, lenBuf[:]...)
	for _, v := range values {
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(v)))
		dst = append(dst, lenBuf[:]...)
		dst = append(dst, v...)
	}
	return dst
}

// writeLengthPrefixedRepoScope binds a list of {repo,branch,…} maps.
// Each map is encoded as a length-prefixed sequence of length-prefixed
// key/value pairs. CBOR-encoding-then-binding would diverge if either
// side changes the map encoding; the relay and connector agree on this
// bespoke byte layout, then the issuer re-signs every byte.
func writeLengthPrefixedRepoScope(dst []byte, scopes []map[string]string) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(scopes)))
	dst = append(dst, lenBuf[:]...)
	for _, scope := range scopes {
		// Order keys deterministically so the signed bytes are stable.
		keys := make([]string, 0, len(scope))
		for k := range scope {
			keys = append(keys, k)
		}
		sortStrings(keys)
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(keys)))
		dst = append(dst, lenBuf[:]...)
		for _, k := range keys {
			binary.BigEndian.PutUint32(lenBuf[:], uint32(len(k)))
			dst = append(dst, lenBuf[:]...)
			dst = append(dst, k...)
			v := scope[k]
			binary.BigEndian.PutUint32(lenBuf[:], uint32(len(v)))
			dst = append(dst, lenBuf[:]...)
			dst = append(dst, v...)
		}
	}
	return dst
}

// sortStrings is a small stable-by-length-preserving sort that the
// connector uses to keep map-key iteration deterministic. The standard
// library's `sort.Strings` would sufficed but the import is avoided
// here to keep the lease file's dependency surface narrow.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j-1] > values[j]; j-- {
			values[j-1], values[j] = values[j], values[j-1]
		}
	}
}

// writeLengthPrefixedString appends a uint32 big-endian length followed
// by the value bytes to the slice and returns the new slice (so the
// caller can keep chaining). An empty string is still prefixed with a
// zero length so the domain separation is preserved.
func writeLengthPrefixedString(dst []byte, value string) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(value)))
	dst = append(dst, lenBuf[:]...)
	dst = append(dst, value...)
	return dst
}

// writeLengthPrefixedU64 appends a uint32 big-endian length followed by the
// 8-byte big-endian uint64 value. The length prefix lets the verifier
// bound the trailing bytes without re-marshaling.
func writeLengthPrefixedU64(dst []byte, value uint64) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], 8)
	dst = append(dst, lenBuf[:]...)
	var valBuf [8]byte
	binary.BigEndian.PutUint64(valBuf[:], value)
	dst = append(dst, valBuf[:]...)
	return dst
}

// Sentinel errors. The connector surfaces these to the UI/audit without
// translation so operators can "is the lease expired vs. revoked vs.
// invalid" without parsing prose.
var (
	ErrLeaseInvalid         = errors.New("paper: lease is empty or malformed")
	ErrLeaseExpired         = errors.New("paper: lease expired")
	ErrLeaseRevoked         = errors.New("paper: lease revoked")
	ErrLeaseSubjectMismatch = errors.New("paper: lease subject peer does not match authenticated harness")
	ErrLeaseSignatureInvalid = errors.New("paper: lease signature verification failed")
)

// IsLeaseExpired reports whether err is the lease-expiry sentinel.
func IsLeaseExpired(err error) bool { return errors.Is(err, ErrLeaseExpired) }

// IsLeaseRevoked reports whether err is the lease-revoked sentinel.
func IsLeaseRevoked(err error) bool { return errors.Is(err, ErrLeaseRevoked) }

// IsLeaseSubjectMismatch reports whether err is the subject-mismatch sentinel.
func IsLeaseSubjectMismatch(err error) bool { return errors.Is(err, ErrLeaseSubjectMismatch) }

// IsLeaseSignatureInvalid reports whether err is the signature-invalid sentinel.
func IsLeaseSignatureInvalid(err error) bool { return errors.Is(err, ErrLeaseSignatureInvalid) }

// bytesEqual is a small helper for byte-slice equality. The standard
// library's `bytes.Equal` is fine, but keeping the comparison local
// avoids an extra import for one call site.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// LeaseDigest produces a content-addressed digest of the lease body used
// to identify it in receipts, evidence chains, and operator audit logs.
// The digest is the SHA-256 of the canonical signing bytes (the same
// bytes the issuer signed), so a tampered lease produces a different
// digest and breaks receipt verification.
func LeaseDigest(lease *Lease) [32]byte {
	return sha256.Sum256(lease.SigningBytes())
}
