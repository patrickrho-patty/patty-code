package dariproto

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// LeaseRequest is the connector's input to a LEASE_ISSUE handshake. The
// relay returns a freshly signed `Lease` it scopes to the harness, user,
// session, and policy epoch. Every field is bound by the relay's
// COSE-Sign1 signature; the connector never modifies any field after
// issuance.
type LeaseRequest struct {
	SubjectPeerID      string
	UserID             string
	SessionID          string
	PolicyEpochID      string
	AllowedModels      []string
	RepositoryScope    []map[string]string
	FilePathReadScope  []string
	FilePathWriteScope []string
	ToolClasses        []string
	TokenBudget        int64
	Validity           time.Duration
}

// LeaseClient manages the harness's currently-held capability lease. The
// client is the single point that *presents* a lease to exchanges, *renews*
// it before expiry, and fails closed when the lease is missing, expired,
// revoked, or bound to a stale subject/session/policy epoch.
//
// The lease is presented per exchange (DARI §22, §26). The connector
// holds the lease in memory + persists the COSE-Sign1 bytes to disk so
// reconnect-after-restart can resume sessions without re-issuing.
type LeaseClient struct {
	mu              sync.Mutex
	verifier        *LeaseVerifier
	issuerPublicKey ed25519.PublicKey
	issuerID        string
	current         *Lease
	// autoRenewBefore is the lead time before NotAfter at which the
	// client proactively renews. Default 5 minutes.
	autoRenewBefore time.Duration
	// nowFn is overridable for tests.
	nowFn func() time.Time

	// metrics: surfaced for A1 (quota awareness) and audit logging.
	lastRenewedAtUnixMs  atomic.Int64
	lastVerifiedAtUnixMs atomic.Int64
	renewFailureCount    atomic.Int64
}

// NewLeaseClient wires the issuer trust bundle and the verifier. The
// issuer's public key is the same one the relay uses to sign leases;
// the connector MUST extract it from the AUTH_PROOF trust bundle rather
// than reading it from a config file (that path is for the harness
// identity, not the relay's policy issuer).
func NewLeaseClient(issuerPub ed25519.PublicKey, issuerID string) *LeaseClient {
	return &LeaseClient{
		verifier:        NewLeaseVerifier(issuerPub, issuerID),
		issuerPublicKey: append(ed25519.PublicKey(nil), issuerPub...),
		issuerID:        issuerID,
		autoRenewBefore: 5 * time.Minute,
		nowFn:           time.Now,
	}
}

// WithAutoRenewBefore overrides the proactive renewal lead time. Tests
// set it to a sub-second value to assert the renewal path without
// waiting for real expiry.
func (c *LeaseClient) WithAutoRenewBefore(d time.Duration) *LeaseClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoRenewBefore = d
	return c
}

// WithNowFunc overrides the time source. Tests use this to drive expiry
// and renewal without sleeping.
func (c *LeaseClient) WithNowFunc(fn func() time.Time) *LeaseClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nowFn = fn
	return c
}

// Acquire stores a freshly-issued lease as the client's current lease.
// The caller (the connector's DARI listener) supplies the lease the
// relay returned; Acquire validates it under the issuer trust bundle
// before storing. A3 requires the harness to *fail-closed* on missing
// credentials, so a faulty lease at session start terminates the
// session before any AI_OPEN is sent.
func (c *LeaseClient) Acquire(subjectPeerID, sessionID string, lease *Lease) error {
	if lease == nil {
		return ErrLeaseInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	nowMs := c.nowFn().UnixMilli()
	if err := c.verifier.Verify(lease, subjectPeerID, sessionID, nowMs); err != nil {
		return fmt.Errorf("dari: lease acquire verification failed: %w", err)
	}
	c.current = lease
	c.lastRenewedAtUnixMs.Store(nowMs)
	c.lastVerifiedAtUnixMs.Store(nowMs)
	return nil
}

// IssuerPublicKey returns the policy issuer's public key this client
// verifies leases (and session grants) under.
func (c *LeaseClient) IssuerPublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), c.issuerPublicKey...)
}

// Current returns the cached lease or nil if none is held. The connector
// uses this to attach the lease to each governance-gated exchange.
func (c *LeaseClient) Current() *Lease {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// Drop clears the held lease without verifying a replacement. The
// transport layer calls this when the relay pushes LEASE_REVOKE:
// every subsequent AuthorizeExchange fails closed until a fresh
// lease arrives.
func (c *LeaseClient) Drop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = nil
}

// Renew validates a fresh lease returned by the relay and updates the
// cached copy. The new lease MUST carry a higher `LeaseSequence` than the
// current lease; otherwise the connector refuses to clobber a fresh
// receipt with a stale one (a buggy relay could otherwise silently
// downgrade the lease's expiry).
func (c *LeaseClient) Renew(subjectPeerID, sessionID string, lease *Lease) error {
	if lease == nil {
		return ErrLeaseInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	nowMs := c.nowFn().UnixMilli()
	if err := c.verifier.Verify(lease, subjectPeerID, sessionID, nowMs); err != nil {
		c.renewFailureCount.Add(1)
		return fmt.Errorf("dari: lease renew verification failed: %w", err)
	}
	if c.current != nil && lease.LeaseSequence <= c.current.LeaseSequence {
		return fmt.Errorf("dari: renewed lease sequence %d is not greater than current %d", lease.LeaseSequence, c.current.LeaseSequence)
	}
	c.current = lease
	c.lastRenewedAtUnixMs.Store(nowMs)
	c.lastVerifiedAtUnixMs.Store(nowMs)
	return nil
}

// Revoke clears the held lease. The connector calls this when the relay
// reports `MsgClose` with a transport-level session termination, or
// when the operator surfaces a manual revocation through the harness
// UI. After Revoke, Present fails closed until Acquire runs again.
func (c *LeaseClient) Revoke() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = nil
}

// Present validates the held lease against the connected subject/session
// before the connector dispatches an AI_OPEN. Present returns nil when
// the lease is valid; otherwise it returns a sentinel error so the
// caller can decide whether to attempt a renewal or fail closed.
//
// The renewal lead time is read from `autoRenewBefore`; if the lease is
// inside that window, Present returns ErrLeaseRenewalDue so the caller
// can issue a LEASE_RENEW handshake before the next exchange.
func (c *LeaseClient) Present(subjectPeerID, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return ErrLeaseInvalid
	}
	now := c.nowFn()
	nowMs := now.UnixMilli()
	if err := c.verifier.Verify(c.current, subjectPeerID, sessionID, nowMs); err != nil {
		return err
	}
	c.lastVerifiedAtUnixMs.Store(nowMs)
	notAfter := time.Unix(c.current.NotAfterUnixMs/1000, (c.current.NotAfterUnixMs%1000)*int64(time.Millisecond))
	if notAfter.Sub(now) <= c.autoRenewBefore {
		return ErrLeaseRenewalDue
	}
	return nil
}

// NeedsRenewal reports whether the held lease is within the configured
// renewal lead time. The connector queries this between exchanges to
// trigger a proactive renewal on the next opportunity.
func (c *LeaseClient) NeedsRenewal() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return false
	}
	now := c.nowFn()
	notAfter := time.Unix(c.current.NotAfterUnixMs/1000, (c.current.NotAfterUnixMs%1000)*int64(time.Millisecond))
	return notAfter.Sub(now) <= c.autoRenewBefore
}

// ErrLeaseRenewalDue is returned by Present when the lease is within the
// configured auto-renewal lead time. The connector upgrades the call
// site to a LEASE_RENEW handshake rather than a protocol failure.
var ErrLeaseRenewalDue = errors.New("dari: lease renewal due")

// MetricsFor surfaces the connector's lease health for the quota/status
// surfaces (E1). The values are returned as a snapshot; concurrent
// updates may shift the counters by a unit between reads.
func (c *LeaseClient) MetricsFor() LeaseMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Compute NeedsRenewal while holding the lock so we don't
	// recursively take it (deadlock). The local helper does the
	// same work as the public NeedsRenewal method.
	needsRenewal := false
	if c.current != nil {
		now := c.nowFn()
		notAfter := time.Unix(c.current.NotAfterUnixMs/1000, (c.current.NotAfterUnixMs%1000)*int64(time.Millisecond))
		needsRenewal = notAfter.Sub(now) <= c.autoRenewBefore
	}
	m := LeaseMetrics{
		IssuerID:             c.issuerID,
		LastRenewedAtUnixMs:  c.lastRenewedAtUnixMs.Load(),
		LastVerifiedAtUnixMs: c.lastVerifiedAtUnixMs.Load(),
		RenewFailureCount:    c.renewFailureCount.Load(),
		NeedsRenewal:         needsRenewal,
	}
	if c.current != nil {
		m.HeldLeaseID = c.current.LeaseID
		m.HeldSequence = c.current.LeaseSequence
		m.NotAfterUnixMs = c.current.NotAfterUnixMs
		m.PolicyEpochID = c.current.PolicyEpochID
	}
	return m
}

// LeaseMetrics is a snapshot of the lease client's health, used by the
// harness status bar (E1) and the audit log.
type LeaseMetrics struct {
	IssuerID             string
	HeldLeaseID          string
	HeldSequence         uint64
	NotAfterUnixMs       int64
	PolicyEpochID        string
	LastRenewedAtUnixMs  int64
	LastVerifiedAtUnixMs int64
	RenewFailureCount    int64
	// NeedsRenewal mirrors NeedsRenewal so the metric snapshot is
	// self-contained; callers should not need to call the method
	// separately.
	NeedsRenewal bool
}

// VerifySessionContext is the exchange-time helper that pins the
// subject/session/policy epoch binding. The connector calls it on every
// governance-gated exchange so a stale lease cannot be replayed across
// a re-authenticated identity. VerifySessionContext updates the same
// `lastVerifiedAtUnixMs` metric `Present` updates so the operator sees
// a single, consistent timestamp.
func (c *LeaseClient) VerifySessionContext(subjectPeerID, sessionID, expectedPolicyEpoch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return ErrLeaseInvalid
	}
	if expectedPolicyEpoch != "" && c.current.PolicyEpochID != "" && c.current.PolicyEpochID != expectedPolicyEpoch {
		return fmt.Errorf("dari: lease policy epoch %q does not match bound session epoch %q", c.current.PolicyEpochID, expectedPolicyEpoch)
	}
	nowMs := c.nowFn().UnixMilli()
	if err := c.verifier.Verify(c.current, subjectPeerID, sessionID, nowMs); err != nil {
		return err
	}
	c.lastVerifiedAtUnixMs.Store(nowMs)
	return nil
}

// AuthorizeExchange is the single entry point the connector uses
// before dispatching an AI_OPEN. It chains the A3 (subject/session),
// A4 (policy epoch), renewal-window, and A5 (allowed-models) checks
// in the documented order. The connector calls this once per
// exchange; the previous separate Present + VerifySessionContext +
// manual model membership check is folded into one method so the
// boundary is the lease client's responsibility, not the connector's.
func (c *LeaseClient) AuthorizeExchange(subjectPeerID, sessionID, expectedPolicyEpoch, model string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return ErrLeaseInvalid
	}
	nowMs := c.nowFn().UnixMilli()
	if err := c.verifier.Verify(c.current, subjectPeerID, sessionID, nowMs); err != nil {
		return err
	}
	if expectedPolicyEpoch != "" && c.current.PolicyEpochID != "" && c.current.PolicyEpochID != expectedPolicyEpoch {
		return fmt.Errorf("dari: lease policy epoch %q does not match bound session epoch %q", c.current.PolicyEpochID, expectedPolicyEpoch)
	}
	// Check the renewal window after the structural checks so a
	// missing/expired/tampered lease still surfaces its own sentinel
	// even if the renewal window would also fire.
	notAfter := time.Unix(c.current.NotAfterUnixMs/1000, (c.current.NotAfterUnixMs%1000)*int64(time.Millisecond))
	if notAfter.Sub(c.nowFn()) <= c.autoRenewBefore {
		return ErrLeaseRenewalDue
	}
	if len(c.current.AllowedModels) == 0 {
		return fmt.Errorf("dari: lease carries no allowed-models list; refusing to dispatch to %q", model)
	}
	allowed := false
	for _, m := range c.current.AllowedModels {
		if m == model {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("dari: requested model %q is not in lease's allowed models", model)
	}
	c.lastVerifiedAtUnixMs.Store(nowMs)
	return nil
}

// EncodeLeaseRequest builds the canonical CBOR body the connector sends
// in a LEASE_ISSUE control record. The relay decodes it back into a
// `policy.IssueLeaseRequest`; the field set MUST match the relay's
// decoder or the connector's request is silently dropped.
func EncodeLeaseRequest(req *LeaseRequest) ([]byte, error) {
	if req == nil {
		return nil, errors.New("dari: nil lease request")
	}
	msg := &leaseRequestWire{
		SubjectPeerID:      req.SubjectPeerID,
		UserID:             req.UserID,
		SessionID:          req.SessionID,
		PolicyEpochID:      req.PolicyEpochID,
		AllowedModels:      req.AllowedModels,
		RepositoryScope:    req.RepositoryScope,
		FilePathReadScope:  req.FilePathReadScope,
		FilePathWriteScope: req.FilePathWriteScope,
		ToolClasses:        req.ToolClasses,
		TokenBudget:        req.TokenBudget,
		ValidityMs:         req.Validity.Milliseconds(),
	}
	return MarshalCBOR(msg)
}

// DecodeLeaseRequest parses a CBOR LEASE_ISSUE body the connector
// received from the relay.
func DecodeLeaseRequest(data []byte) (*LeaseRequest, error) {
	if len(data) == 0 {
		return nil, errors.New("dari: empty lease request body")
	}
	var msg leaseRequestWire
	if err := UnmarshalCBOR(data, &msg); err != nil {
		return nil, fmt.Errorf("dari: decode lease request: %w", err)
	}
	return &LeaseRequest{
		SubjectPeerID:      msg.SubjectPeerID,
		UserID:             msg.UserID,
		SessionID:          msg.SessionID,
		PolicyEpochID:      msg.PolicyEpochID,
		AllowedModels:      msg.AllowedModels,
		RepositoryScope:    msg.RepositoryScope,
		FilePathReadScope:  msg.FilePathReadScope,
		FilePathWriteScope: msg.FilePathWriteScope,
		ToolClasses:        msg.ToolClasses,
		TokenBudget:        msg.TokenBudget,
		Validity:           time.Duration(msg.ValidityMs) * time.Millisecond,
	}, nil
}

// EncodeLeaseResponse returns the wire shape the relay sends back after
// signing a lease. The connector decodes it into a `Lease` and stores
// it through `LeaseClient.Acquire`.
func EncodeLeaseResponse(lease *Lease) ([]byte, error) {
	if lease == nil {
		return nil, errors.New("dari: nil lease")
	}
	return MarshalCBOR(lease)
}

// DecodeLeaseResponse parses the relay's signed lease body.
func DecodeLeaseResponse(data []byte) (*Lease, error) {
	if len(data) == 0 {
		return nil, errors.New("dari: empty lease response body")
	}
	var lease Lease
	if err := UnmarshalCBOR(data, &lease); err != nil {
		return nil, fmt.Errorf("dari: decode lease response: %w", err)
	}
	return &lease, nil
}

// leaseRequestWire is the on-wire LEASE_ISSUE body. The relay's
// `policy.IssueLeaseRequest` defines the canonical field names; this
// struct mirrors them so the connector's encoder is byte-for-byte the
// JSON the relay `policy.IssueCapabilityLease` decodes.
type leaseRequestWire struct {
	SubjectPeerID      string              `cbor:"1,keyasint"`
	UserID             string              `cbor:"2,keyasint"`
	SessionID          string              `cbor:"3,keyasint,omitempty"`
	PolicyEpochID      string              `cbor:"4,keyasint"`
	AllowedModels      []string            `cbor:"5,keyasint,omitempty"`
	RepositoryScope    []map[string]string `cbor:"6,keyasint,omitempty"`
	FilePathReadScope  []string            `cbor:"7,keyasint,omitempty"`
	FilePathWriteScope []string            `cbor:"8,keyasint,omitempty"`
	ToolClasses        []string            `cbor:"9,keyasint,omitempty"`
	TokenBudget        int64               `cbor:"10,keyasint,omitempty"`
	ValidityMs         int64               `cbor:"11,keyasint,omitempty"`
}

// contextKey is a private type to avoid accidental collision with other
// context-keyed values in the connector.
type contextKey int

const (
	leaseClientContextKey contextKey = iota
)

// WithLeaseClient returns a new context carrying the lease client. The
// connector plumbs the client through the request boundary so
// governance-gated services can call `Present` without re-deriving the
// issuer identity.
func WithLeaseClient(ctx context.Context, client *LeaseClient) context.Context {
	return context.WithValue(ctx, leaseClientContextKey, client)
}

// LeaseClientFromContext returns the lease client attached to ctx, or
// nil if none was attached. The helper is the inversion of
// `WithLeaseClient` for the harness's request-handling seams.
func LeaseClientFromContext(ctx context.Context) *LeaseClient {
	client, _ := ctx.Value(leaseClientContextKey).(*LeaseClient)
	return client
}
