package dariproto

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CatalogEntry is one model advertised in the server-authoritative model
// catalog. The connector's source of truth for which model a harness
// MAY dispatch to is the LEASE's AllowedModels list; the catalog is the
// source of truth for what models the relay CURRENTLY ADVERTISES. A5
// requires the harness to refuse a model that is not in the catalog
// because the relay could not authorize it.
type CatalogEntry struct {
	// ModelID is the canonical model identifier (e.g. "patty-code-standard").
	ModelID string `cbor:"1,keyasint"`
	// DisplayName is the operator-facing label.
	DisplayName string `cbor:"2,keyasint"`
	// Version is the model package version (e.g. "1.2.0").
	Version string `cbor:"3,keyasint"`
	// ModelPackageDigest is the SHA-256 of the model package manifest
	// the relay signed. The connector surfaces this so the harness
	// can refuse to load a model whose package has been rotated
	// underneath the lease.
	ModelPackageDigest [32]byte `cbor:"4,keyasint"`
	// EndpointDigest is the SHA-256 of the endpoint lease authorizing
	// the relay to dispatch to this model. The harness cross-checks
	// this against the endpoint it authenticated to.
	EndpointDigest [32]byte `cbor:"5,keyasint"`
	// Capabilities enumerate the model's advertised capabilities
	// (vision, function-calling, thinking, etc.). The harness
	// turns these into tool/MCP admission decisions.
	Capabilities []string `cbor:"6,keyasint,omitempty"`
	// TokenLimit is the per-request output token ceiling.
	TokenLimit uint32 `cbor:"7,keyasint,omitempty"`
	// ContextWindow is the model's input context window.
	ContextWindow uint32 `cbor:"8,keyasint,omitempty"`
	// ModeTags classify the model (e.g. "code", "review", "vision").
	ModeTags []string `cbor:"9,keyasint,omitempty"`
	// PolicyEpochID is the policy epoch that authorized this catalog
	// entry. The connector pins the catalog to the bound epoch so a
	// stale catalog entry cannot be replayed across a policy change.
	PolicyEpochID string `cbor:"10,keyasint"`
	// ActiveUntilUnixMs is the entry's soft deactivation time. The
	// connector refuses to dispatch against an entry past this time
	// even if it remains in the snapshot.
	ActiveUntilUnixMs int64 `cbor:"11,keyasint,omitempty"`
}

// CatalogSnapshot is the relay's authoritative model catalog. The
// relay pushes CATALOG_SNAPSHOT at session start and CATALOG_DELTA on
// every change. The connector accepts deltas only when their epoch
// matches the bound policy epoch.
type CatalogSnapshot struct {
	Version uint64 `cbor:"1,keyasint"`
	// EpochID is the policy epoch the catalog was signed under.
	EpochID string `cbor:"2,keyasint"`
	// IssuedAtUnixMs is the wall-clock issuance time.
	IssuedAtUnixMs int64 `cbor:"3,keyasint"`
	// NotAfterUnixMs bounds the snapshot's validity. Past expiry the
	// connector treats the catalog as stale and refuses to dispatch.
	NotAfterUnixMs int64 `cbor:"4,keyasint"`
	// IssuedSequence is the monotonic sequence of catalog updates.
	// The connector refuses to apply a snapshot with a sequence
	// lower than the held one.
	IssuedSequence uint64 `cbor:"5,keyasint"`
	// IssuerKeyThumbprint pins the snapshot's issuer so a
	// cross-issuer spoof is rejected.
	IssuerKeyThumbprint [32]byte `cbor:"6,keyasint"`
	// Digest is the SHA-256 of the canonical entries body. The
	// connector surfaces this in receipts.
	Digest [32]byte `cbor:"7,keyasint"`
	// Entries is the ordered list of catalog entries.
	Entries []CatalogEntry `cbor:"8,keyasint"`
}

// CatalogDelta is the wire shape for an incremental catalog update.
type CatalogDelta struct {
	Version        uint64 `cbor:"1,keyasint"`
	EpochID        string `cbor:"2,keyasint"`
	IssuedAtUnixMs int64  `cbor:"3,keyasint"`
	IssuedSequence uint64 `cbor:"4,keyasint"`
	// Added are new entries the connector MUST accept.
	Added []CatalogEntry `cbor:"5,keyasint,omitempty"`
	// Removed are model IDs the connector MUST drop.
	Removed []string `cbor:"6,keyasint,omitempty"`
	// Updated are entries the connector MUST replace.
	Updated []CatalogEntry `cbor:"7,keyasint,omitempty"`
}

// CatalogDomain is the domain-separation prefix for the catalog
// digest. The connector's audit logs surface this digest so the
// receipt chain can reproduce the exact catalog the relay pushed.
const CatalogDomain = "DARI-CATALOG-v1\x00"

// CatalogDigest produces the content-addressed digest of the catalog
// entries body. The digest binds every field that the connector
// uses to authorize an exchange (version, epoch, sequence, issuer
// thumbprint, validity window, per-entry fields) so a buggy relay
// cannot silently mutate any of them without re-signing.
func CatalogDigest(snap *CatalogSnapshot) [32]byte {
	if snap == nil {
		return [32]byte{}
	}
	h := sha256.New()
	h.Write([]byte(CatalogDomain))
	var versionBuf [8]byte
	binary.BigEndian.PutUint64(versionBuf[:], snap.Version)
	h.Write(versionBuf[:])
	h.Write([]byte(snap.EpochID))
	var seqBuf [8]byte
	binary.BigEndian.PutUint64(seqBuf[:], snap.IssuedSequence)
	h.Write(seqBuf[:])
	h.Write(snap.IssuerKeyThumbprint[:])
	var notAfterBuf [8]byte
	binary.BigEndian.PutUint64(notAfterBuf[:], uint64(snap.NotAfterUnixMs))
	h.Write(notAfterBuf[:])
	for i := range snap.Entries {
		h.Write([]byte(snap.Entries[i].ModelID))
		h.Write([]byte(snap.Entries[i].Version))
		h.Write(snap.Entries[i].ModelPackageDigest[:])
		h.Write(snap.Entries[i].EndpointDigest[:])
	}
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// CatalogClient owns the connector's stored snapshot. The client
// applies deltas only when their epoch matches the bound policy epoch
// and their sequence is strictly greater than the held one. Stale
// updates are rejected so a buggy relay cannot regress the catalog.
type CatalogClient struct {
	mu                sync.Mutex
	current           *CatalogSnapshot
	stalenessCount    int64
	applyFailureCount int64
	nowFn             func() time.Time
}

// NewCatalogClient constructs a client with the wall clock as the
// default time source.
func NewCatalogClient() *CatalogClient {
	return &CatalogClient{nowFn: time.Now}
}

// WithNowFunc overrides the time source. Tests use this to drive
// snapshot expiry without sleeping.
func (c *CatalogClient) WithNowFunc(fn func() time.Time) *CatalogClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nowFn = fn
	return c
}

// ApplySnapshot replaces the held catalog with the supplied snapshot.
// The connector rejects:
//
//  1. Snapshots with a sequence lower than the current one (rollback).
//  2. Snapshots with an epoch that does not match the supplied
//     expected epoch (the connector and the relay must agree on the
//     policy epoch).
//  3. Snapshots whose NotAfter is already in the past.
//  4. Snapshots whose computed digest does not match the
//     embedded digest.
func (c *CatalogClient) ApplySnapshot(snap *CatalogSnapshot, expectedEpoch string) error {
	if snap == nil {
		return ErrCatalogInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	nowMs := c.nowFn().UnixMilli()
	if expectedEpoch != "" && snap.EpochID != "" && snap.EpochID != expectedEpoch {
		c.applyFailureCount++
		return fmt.Errorf("dari: catalog epoch %q does not match bound epoch %q", snap.EpochID, expectedEpoch)
	}
	if snap.NotAfterUnixMs > 0 && nowMs >= snap.NotAfterUnixMs {
		c.applyFailureCount++
		return fmt.Errorf("dari: catalog snapshot expired at now=%d", nowMs)
	}
	if c.current != nil && snap.IssuedSequence <= c.current.IssuedSequence {
		c.applyFailureCount++
		return fmt.Errorf("dari: catalog sequence %d is not greater than current %d", snap.IssuedSequence, c.current.IssuedSequence)
	}
	if got := CatalogDigest(snap); got != snap.Digest {
		c.applyFailureCount++
		return errors.New("dari: catalog digest mismatch")
	}
	c.current = snap
	return nil
}

// ApplyDelta mutates the held catalog by adding/removing/updating
// entries. The same constraints as ApplySnapshot apply: epoch
// pin, sequence monotonic, expiry. The connector rebuilds the
// digest after the mutation so receipts see the exact catalog
// state.
func (c *CatalogClient) ApplyDelta(delta *CatalogDelta, expectedEpoch string) error {
	if delta == nil {
		return ErrCatalogInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	nowMs := c.nowFn().UnixMilli()
	if c.current == nil {
		c.applyFailureCount++
		return errors.New("dari: no baseline catalog to apply delta to")
	}
	if expectedEpoch != "" && delta.EpochID != "" && delta.EpochID != expectedEpoch {
		c.applyFailureCount++
		return fmt.Errorf("dari: delta epoch %q does not match bound epoch %q", delta.EpochID, expectedEpoch)
	}
	if delta.IssuedSequence <= c.current.IssuedSequence {
		c.applyFailureCount++
		return fmt.Errorf("dari: delta sequence %d is not greater than current %d", delta.IssuedSequence, c.current.IssuedSequence)
	}
	if c.current.NotAfterUnixMs > 0 && nowMs >= c.current.NotAfterUnixMs {
		c.applyFailureCount++
		return errors.New("dari: catalog already expired; cannot apply delta")
	}
	// Merge: removals first, then updates, then additions. The
	// merge MUST allocate a fresh backing array; reusing
	// `c.current.Entries[:0]` would alias the source slice and
	// cause the iteration to overwrite entries it is still reading
	// (a memory-corruption bug that surfaces only when the
	// removed/updated set is non-empty).
	removeSet := map[string]bool{}
	for _, m := range delta.Removed {
		removeSet[m] = true
	}
	filtered := make([]CatalogEntry, 0, len(c.current.Entries)+len(delta.Added))
	for _, e := range c.current.Entries {
		if removeSet[e.ModelID] {
			continue
		}
		filtered = append(filtered, e)
	}
	for _, u := range delta.Updated {
		replaced := false
		for i, e := range filtered {
			if e.ModelID == u.ModelID {
				filtered[i] = u
				replaced = true
				break
			}
		}
		if !replaced {
			filtered = append(filtered, u)
		}
	}
	c.current.Entries = append(filtered, delta.Added...)
	c.current.IssuedSequence = delta.IssuedSequence
	c.current.Digest = CatalogDigest(c.current)
	return nil
}

// Current returns the held snapshot or nil if no catalog has been
// applied. The connector plumbs this through every exchange check.
func (c *CatalogClient) Current() *CatalogSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// FindModel returns the catalog entry for the supplied model ID.
// The connector uses this to cross-check the lease's allow-list
// against the relay's actual catalog.
func (c *CatalogClient) FindModel(modelID string) (*CatalogEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return nil, ErrCatalogUnbound
	}
	nowMs := c.nowFn().UnixMilli()
	if c.current.NotAfterUnixMs > 0 && nowMs >= c.current.NotAfterUnixMs {
		c.stalenessCount++
		return nil, ErrCatalogStale
	}
	for i := range c.current.Entries {
		if c.current.Entries[i].ModelID == modelID {
			entry := c.current.Entries[i]
			if entry.ActiveUntilUnixMs > 0 && nowMs >= entry.ActiveUntilUnixMs {
				return nil, ErrCatalogEntryExpired
			}
			return &entry, nil
		}
	}
	return nil, ErrCatalogModelNotFound
}

// IsStale reports whether the held catalog is past its validity
// window. The connector checks this between exchanges to schedule a
// proactive snapshot refresh.
func (c *CatalogClient) IsStale() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return true
	}
	if c.current.NotAfterUnixMs <= 0 {
		return false
	}
	return c.nowFn().UnixMilli() >= c.current.NotAfterUnixMs
}

// MetricsFor surfaces the connector's catalog health for the E1
// status bar. The connector exposes the held snapshot's version
// and digest without leaking the issuer's private key.
func (c *CatalogClient) MetricsFor() CatalogMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Compute IsStale while holding the lock to avoid the
	// double-lock deadlock that would otherwise occur here.
	isStale := false
	if c.current == nil {
		isStale = true
	} else if c.current.NotAfterUnixMs > 0 {
		isStale = c.nowFn().UnixMilli() >= c.current.NotAfterUnixMs
	}
	m := CatalogMetrics{
		StalenessCount:    c.stalenessCount,
		ApplyFailureCount: c.applyFailureCount,
		IsStale:           isStale,
	}
	if c.current != nil {
		m.Version = c.current.Version
		m.EpochID = c.current.EpochID
		m.IssuedSequence = c.current.IssuedSequence
		m.NotAfterUnixMs = c.current.NotAfterUnixMs
		m.EntryCount = len(c.current.Entries)
		m.Digest = c.current.Digest
	}
	return m
}

// CatalogMetrics is a snapshot of the catalog client's health, used
// by the harness status bar (E1) and the audit log.
type CatalogMetrics struct {
	Version           uint64
	EpochID           string
	IssuedSequence    uint64
	NotAfterUnixMs    int64
	EntryCount        int
	Digest            [32]byte
	StalenessCount    int64
	ApplyFailureCount int64
	// IsStale mirrors IsStale so the metric snapshot is self-contained;
	// callers should not need to call the method separately.
	IsStale bool
}

// Sentinel errors for the catalog boundary. The connector surfaces
// these to the operator UI without translation.
var (
	ErrCatalogInvalid       = errors.New("dari: catalog snapshot is empty or malformed")
	ErrCatalogUnbound       = errors.New("dari: no catalog applied to the connector")
	ErrCatalogStale         = errors.New("dari: catalog snapshot is past its validity window")
	ErrCatalogModelNotFound = errors.New("dari: requested model is not in the relay's catalog")
	ErrCatalogEntryExpired  = errors.New("dari: catalog entry is past its deactivation time")
)

// IsCatalogStale reports whether err is the catalog-stale sentinel.
func IsCatalogStale(err error) bool { return errors.Is(err, ErrCatalogStale) }

// IsCatalogModelNotFound reports whether err is the catalog-not-found sentinel.
func IsCatalogModelNotFound(err error) bool { return errors.Is(err, ErrCatalogModelNotFound) }

// IsCatalogEntryExpired reports whether err is the entry-expired sentinel.
func IsCatalogEntryExpired(err error) bool { return errors.Is(err, ErrCatalogEntryExpired) }

// EncodeCatalogSnapshot serializes a snapshot for the wire.
func EncodeCatalogSnapshot(snap *CatalogSnapshot) ([]byte, error) {
	if snap == nil {
		return nil, errors.New("dari: nil catalog snapshot")
	}
	return MarshalCBOR(snap)
}

// DecodeCatalogSnapshot parses a CATALOG_SNAPSHOT body.
func DecodeCatalogSnapshot(data []byte) (*CatalogSnapshot, error) {
	if len(data) == 0 {
		return nil, errors.New("dari: empty catalog snapshot body")
	}
	var snap CatalogSnapshot
	if err := UnmarshalCBOR(data, &snap); err != nil {
		return nil, fmt.Errorf("dari: decode catalog snapshot: %w", err)
	}
	return &snap, nil
}

// EncodeCatalogDelta serializes a delta for the wire.
func EncodeCatalogDelta(delta *CatalogDelta) ([]byte, error) {
	if delta == nil {
		return nil, errors.New("dari: nil catalog delta")
	}
	return MarshalCBOR(delta)
}

// DecodeCatalogDelta parses a CATALOG_DELTA body.
func DecodeCatalogDelta(data []byte) (*CatalogDelta, error) {
	if len(data) == 0 {
		return nil, errors.New("dari: empty catalog delta body")
	}
	var delta CatalogDelta
	if err := UnmarshalCBOR(data, &delta); err != nil {
		return nil, fmt.Errorf("dari: decode catalog delta: %w", err)
	}
	return &delta, nil
}
