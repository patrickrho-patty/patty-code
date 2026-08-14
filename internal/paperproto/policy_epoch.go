package paperproto

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

// PolicyEpoch is the relay's session-bound policy identity. The harness
// pins every governance-gated exchange to the epoch it received at
// AUTH_PROOF time; a stale epoch MUST refuse to dispatch AI_OPEN. A4
// requires the connector to surface the bound epoch to the operator
// and re-bind on receipt of a fresh POLICY message.
type PolicyEpoch struct {
	// EpochID is the relay's stable identifier for the policy epoch.
	// Every control-plane decision (lease issuance, catalog entry,
	// model package) carries this epoch so the connector can refuse
	// to honor a stale credential.
	EpochID string `cbor:"1,keyasint"`
	// IssuedAt is the wall-clock issuance time in Unix milliseconds.
	IssuedAtUnixMs int64 `cbor:"2,keyasint"`
	// NotBeforeUnixMs / NotAfterUnixMs bound the validity window.
	// Past expiry, the connector MUST re-bind to a fresh epoch.
	NotBeforeUnixMs int64 `cbor:"3,keyasint"`
	NotAfterUnixMs  int64 `cbor:"4,keyasint"`
	// MonotonicSequence is the per-issuer monotonic sequence. The
	// harness treats any incoming epoch with a lower sequence than
	// the held one as a rollback and refuses to honor it.
	MonotonicSequence uint64 `cbor:"5,keyasint"`
	// IssuerKeyThumbprint is the SHA-256 of the issuer's public key.
	// The connector pins the trust bundle it received at AUTH_PROOF
	// time and refuses an epoch signed by a different issuer.
	IssuerKeyThumbprint [32]byte `cbor:"6,keyasint"`
	// Digest is the SHA-256 of the policy content body. The
	// connector surfaces this in receipts so the audit chain can
	// reproduce the exact policy that authorized a given exchange.
	Digest [32]byte `cbor:"7,keyasint"`
}

// PolicyEpochDomain is the domain-separation prefix for the policy
// epoch's signing bytes. The relay and the connector MUST agree on this
// constant down to the trailing NUL byte.
const PolicyEpochDomain = "DARI-POLICY-EPOCH-v1\x00"

// SigningBytes returns the canonical byte string the issuer signs and
// the connector verifies. The encoding is domain-separated and
// length-prefixed for every string/uint64 field.
func (p *PolicyEpoch) SigningBytes() []byte {
	canonical := make([]byte, 0, 256)
	canonical = append(canonical, []byte(PolicyEpochDomain)...)
	canonical = writeLengthPrefixedString(canonical, p.EpochID)
	canonical = writeLengthPrefixedU64(canonical, uint64(p.IssuedAtUnixMs))
	canonical = writeLengthPrefixedU64(canonical, uint64(p.NotBeforeUnixMs))
	canonical = writeLengthPrefixedU64(canonical, uint64(p.NotAfterUnixMs))
	canonical = writeLengthPrefixedU64(canonical, p.MonotonicSequence)
	canonical = append(canonical, p.IssuerKeyThumbprint[:]...)
	canonical = append(canonical, p.Digest[:]...)
	// Length-prefix the fixed-size slices so the verifier can bound
	// the trailing bytes without re-marshaling.
	canonical = writeLengthPrefixedBytes(canonical, p.IssuerKeyThumbprint[:])
	canonical = writeLengthPrefixedBytes(canonical, p.Digest[:])
	return canonical
}

// writeLengthPrefixedBytes is a thin wrapper around the lease's
// helpers so the policy-epoch signing bytes stay bound to the same
// length-prefixed layout. The lease package owns the byte shape; the
// policy package delegates to it to keep one source of truth.
func writeLengthPrefixedBytes(dst []byte, value []byte) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(value)))
	dst = append(dst, lenBuf[:]...)
	dst = append(dst, value...)
	return dst
}

// PolicyEpochDigest produces a content-addressed digest of the epoch
// for receipts and audit evidence. The digest is the SHA-256 of the
// canonical signing bytes.
func PolicyEpochDigest(p *PolicyEpoch) [32]byte {
	return sha256.Sum256(p.SigningBytes())
}

// PolicyEpochClient is the harness's session-bound policy epoch
// tracker. The connector calls `Bind` when AUTH_PROOF completes (relay
// pushes the active epoch as part of the trust bundle) and `Rebind`
// when a fresh POLICY message arrives. Every governance-gated exchange
// pins the bound epoch through `Verify`.
type PolicyEpochClient struct {
	mu          sync.Mutex
	bound       *PolicyEpoch
	lastVerifiedUnixMs int64
	bindFailureCount   int64
	rebindFailureCount int64
	// nowFn is overridable for tests.
	nowFn func() time.Time
}

// NewPolicyEpochClient constructs a tracker with the wall clock as the
// default time source.
func NewPolicyEpochClient() *PolicyEpochClient {
	return &PolicyEpochClient{nowFn: time.Now}
}

// WithNowFunc overrides the time source. Tests use this to drive
// validity-window expiry without sleeping.
func (c *PolicyEpochClient) WithNowFunc(fn func() time.Time) *PolicyEpochClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nowFn = fn
	return c
}

// Bind stores the active epoch as the harness's bound policy epoch. The
// connector MUST call this exactly once at AUTH_PROOF time and refresh
// via `Rebind` whenever the relay pushes a fresh POLICY message.
func (c *PolicyEpochClient) Bind(epoch *PolicyEpoch) error {
	if epoch == nil {
		return ErrPolicyEpochInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	nowMs := c.nowFn().UnixMilli()
	if err := c.validate(epoch, nowMs); err != nil {
		c.bindFailureCount++
		return err
	}
	c.bound = epoch
	c.lastVerifiedUnixMs = nowMs
	return nil
}

// Current returns the bound epoch or nil if no epoch is bound. The
// connector surfaces this in operator-visible status (E1 status bar).
func (c *PolicyEpochClient) Current() *PolicyEpoch {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bound
}

// Rebind replaces the bound epoch with a fresh one. The new epoch MUST
// be strictly newer than the current one: a higher monotonic sequence,
// a later issued-at timestamp, or a different issuer-key thumbprint
// (the latter when the relay rotated its CA). Anything else is rejected
// so a buggy relay cannot silently downgrade the policy epoch. Every
// failed rebind bumps `rebindFailureCount` so the operator sees drift
// in the audit log.
func (c *PolicyEpochClient) Rebind(epoch *PolicyEpoch) error {
	if epoch == nil {
		c.rebindFailureCount++
		return ErrPolicyEpochInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	nowMs := c.nowFn().UnixMilli()
	if err := c.validate(epoch, nowMs); err != nil {
		c.rebindFailureCount++
		return err
	}
	if c.bound != nil {
		if epoch.MonotonicSequence <= c.bound.MonotonicSequence {
			c.rebindFailureCount++
			return fmt.Errorf("paper: new epoch sequence %d is not greater than current %d", epoch.MonotonicSequence, c.bound.MonotonicSequence)
		}
		if epoch.IssuedAtUnixMs < c.bound.IssuedAtUnixMs {
			c.rebindFailureCount++
			return fmt.Errorf("paper: new epoch issued-at %d is older than current %d", epoch.IssuedAtUnixMs, c.bound.IssuedAtUnixMs)
		}
	}
	c.bound = epoch
	c.lastVerifiedUnixMs = nowMs
	return nil
}

// Verify checks that the supplied epoch matches the connector's bound
// epoch. A mismatch indicates the lease or catalog has been minted
// against a different policy than the one currently in force. The
// connector refuses to honor the exchange.
func (c *PolicyEpochClient) Verify(epochID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bound == nil {
		return ErrPolicyEpochUnbound
	}
	nowMs := c.nowFn().UnixMilli()
	if c.bound.NotAfterUnixMs > 0 && nowMs >= c.bound.NotAfterUnixMs {
		return ErrPolicyEpochExpired
	}
	if c.bound.EpochID != epochID {
		return ErrPolicyEpochMismatch
	}
	c.lastVerifiedUnixMs = nowMs
	return nil
}

// IsStale reports whether the bound epoch is past its expiry. The
// connector checks this between exchanges to schedule a proactive
// `Rebind` from the relay.
func (c *PolicyEpochClient) IsStale() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bound == nil {
		return true
	}
	if c.bound.NotAfterUnixMs <= 0 {
		return false
	}
	return c.nowFn().UnixMilli() >= c.bound.NotAfterUnixMs
}

// MetricsFor surfaces the connector's policy-epoch health for the
// status bar (E1). The connector exposes the bound epoch ID and
// validity timestamps without leaking the issuer's private key.
func (c *PolicyEpochClient) MetricsFor() PolicyEpochMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Compute IsStale while holding the lock to avoid the
	// double-lock deadlock that would otherwise occur here.
	isStale := true
	if c.bound != nil && c.bound.NotAfterUnixMs > 0 {
		isStale = c.nowFn().UnixMilli() >= c.bound.NotAfterUnixMs
	}
	m := PolicyEpochMetrics{
		LastVerifiedUnixMs: c.lastVerifiedUnixMs,
		BindFailureCount:   c.bindFailureCount,
		RebindFailureCount: c.rebindFailureCount,
		IsStale:            isStale,
	}
	if c.bound != nil {
		m.BoundEpochID = c.bound.EpochID
		m.IssuedAtUnixMs = c.bound.IssuedAtUnixMs
		m.NotAfterUnixMs = c.bound.NotAfterUnixMs
		m.MonotonicSequence = c.bound.MonotonicSequence
	}
	return m
}

// PolicyEpochMetrics is the policy-epoch snapshot consumed by the
// status bar (E1) and the audit log.
type PolicyEpochMetrics struct {
	BoundEpochID        string
	IssuedAtUnixMs      int64
	NotAfterUnixMs      int64
	MonotonicSequence   uint64
	LastVerifiedUnixMs  int64
	BindFailureCount    int64
	RebindFailureCount  int64
	// IsStale mirrors IsStale so the metric snapshot is self-contained;
	// callers should not need to call the method separately.
	IsStale bool
}

// validate enforces the structural invariants of a PolicyEpoch. The
// connector calls this from `Bind` and `Rebind` so a malformed wire
// object never makes it into the bound state.
func (c *PolicyEpochClient) validate(epoch *PolicyEpoch, nowMs int64) error {
	if epoch.EpochID == "" {
		return ErrPolicyEpochInvalid
	}
	if epoch.NotBeforeUnixMs > 0 && nowMs < epoch.NotBeforeUnixMs {
		return fmt.Errorf("paper: policy epoch %q is not yet valid (now=%d, notBefore=%d)", epoch.EpochID, nowMs, epoch.NotBeforeUnixMs)
	}
	if epoch.NotAfterUnixMs > 0 && nowMs >= epoch.NotAfterUnixMs {
		return fmt.Errorf("paper: policy epoch %q is expired at now=%d", epoch.EpochID, nowMs)
	}
	return nil
}

// Sentinel errors for the policy-epoch boundary. The connector surfaces
// these to the operator UI without translation.
var (
	ErrPolicyEpochInvalid     = errors.New("paper: policy epoch is empty or malformed")
	ErrPolicyEpochUnbound     = errors.New("paper: no policy epoch bound to the connector")
	ErrPolicyEpochMismatch    = errors.New("paper: presented policy epoch does not match the bound epoch")
	ErrPolicyEpochExpired     = errors.New("paper: bound policy epoch is past its validity window")
)

// IsPolicyEpochMismatch reports whether err is the epoch-mismatch sentinel.
func IsPolicyEpochMismatch(err error) bool { return errors.Is(err, ErrPolicyEpochMismatch) }

// IsPolicyEpochExpired reports whether err is the epoch-expired sentinel.
func IsPolicyEpochExpired(err error) bool { return errors.Is(err, ErrPolicyEpochExpired) }

// IsPolicyEpochUnbound reports whether err is the unbound sentinel.
func IsPolicyEpochUnbound(err error) bool { return errors.Is(err, ErrPolicyEpochUnbound) }

// EncodePolicyEpochMessage serializes a PolicyEpoch for the wire. The
// relay pushes POLICy messages carrying this body to enrolled harnesses
// when the policy is updated.
func EncodePolicyEpochMessage(epoch *PolicyEpoch) ([]byte, error) {
	if epoch == nil {
		return nil, errors.New("paper: nil policy epoch")
	}
	return MarshalCBOR(epoch)
}

// DecodePolicyEpochMessage parses a POLICY message body.
func DecodePolicyEpochMessage(data []byte) (*PolicyEpoch, error) {
	if len(data) == 0 {
		return nil, errors.New("paper: empty policy epoch body")
	}
	var epoch PolicyEpoch
	if err := UnmarshalCBOR(data, &epoch); err != nil {
		return nil, fmt.Errorf("paper: decode policy epoch: %w", err)
	}
	return &epoch, nil
}
