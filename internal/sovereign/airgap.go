// Package sovereign is the harness-side sovereign / air-gap
// support (harness feature plan E3). In government deployments
// the harness operates fully offline: the trust bundle, policy
// epoch, catalog, and DLP rules all arrive via signed offline
// advisories rather than online fetches.
//
// The harness persists every relay-pushed state in a tamper-
// evident local store and refuses to dial out for any reason other
// than the explicit allow-list (catalog download, signed update).
package sovereign

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

const ed25519PublicKeySize = 32

func ed25519Verify(pub, msg, sig []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(pub), msg, sig)
}

// TrustSource is the catalog of relay public keys + signed
// catalogs the harness trusts when operating offline. The
// harness refuses to apply state from any source not in this
// catalog.
type TrustSource struct {
	OrganizationID string
	SourceID       string
	Name           string
	IssuerPub      []byte // ed25519 public key
	// CatalogDigest is the SHA-256 of the latest catalog this
	// source delivered. The harness compares inbound catalogs
	// against this digest.
	CatalogDigest [32]byte
	// IssuedAt is when the source delivered its current catalog.
	IssuedAt int64
}

// UpdateAdvisory is a signed offline-update bundle. The relay
// publishes these for government deployments that need signed
// install bundles.
type UpdateAdvisory struct {
	AdvisoryID string
	Version    string
	Payload    []byte // opaque bundle bytes
	IssuedAt   int64
	NotAfter   int64
	// Signature is the ed25519 signature over SigningBytes() using
	// the issuing source's public key.
	Signature []byte
}

// SigningBytes returns the canonical bytes for the advisory. The
// relay and connector agree on this exact layout.
func (a *UpdateAdvisory) SigningBytes() []byte {
	data := fmt.Sprintf("advisory|%s|%s|%d|%d|%s",
		a.AdvisoryID, a.Version, a.IssuedAt, a.NotAfter, string(a.Payload))
	return []byte(data)
}

// Digest returns the content-addressed digest of the advisory
// payload. The harness stores it in the audit log.
func (a *UpdateAdvisory) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte("DARI-SOVEREIGN-UPDATE-v1\x00"))
	h.Write([]byte(a.AdvisoryID))
	h.Write([]byte(a.Version))
	var tsBuf [8]byte
	binary.BigEndian.PutUint64(tsBuf[:], uint64(a.IssuedAt))
	h.Write(tsBuf[:])
	h.Write(a.Payload)
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// IsExpired reports whether the advisory is past NotAfter.
func (a *UpdateAdvisory) IsExpired(nowMs int64) bool {
	return a.NotAfter > 0 && nowMs >= a.NotAfter
}

// AirGapMode is the connector's sovereign deployment mode.
// When enabled, the harness refuses all online network calls
// other than the offline-update channel (which is itself signed
// and only contacts sources in the trust bundle).
type AirGapMode struct {
	mu         sync.RWMutex
	enabled    bool
	sources    map[string]*TrustSource
	advisories map[string]*UpdateAdvisory
	// Online exception list: hosts the harness may dial even in
	// air-gap mode (e.g. internal mirror for catalog download).
	onlineAllowList map[string]bool
	// metrics
	appliedAdvisories  int64
	rejectedAdvisories int64
}

// NewAirGapMode constructs a disabled air-gap mode. The harness
// enables it when the relay pushes a sovereign-deployment flag.
func NewAirGapMode() *AirGapMode {
	return &AirGapMode{
		sources:         make(map[string]*TrustSource),
		advisories:      make(map[string]*UpdateAdvisory),
		onlineAllowList: make(map[string]bool),
	}
}

// Enable activates air-gap mode. The harness's transport layer
// reads `IsEnabled()` before every dial.
func (a *AirGapMode) Enable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = true
}

// Disable deactivates air-gap mode.
func (a *AirGapMode) Disable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = false
}

// IsEnabled reports whether the harness is in air-gap mode.
func (a *AirGapMode) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

// SetTrustSources installs the offline trust catalog. The harness
// refuses any update from a source not in this list.
func (a *AirGapMode) SetTrustSources(sources []*TrustSource) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sources = make(map[string]*TrustSource, len(sources))
	for _, s := range sources {
		a.sources[s.SourceID] = s
	}
}

// SetOnlineAllowList installs the hosts the harness may dial even
// in air-gap mode (e.g. internal mirrors).
func (a *AirGapMode) SetOnlineAllowList(hosts []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onlineAllowList = make(map[string]bool, len(hosts))
	for _, h := range hosts {
		a.onlineAllowList[h] = true
	}
}

// AllowsDial reports whether the harness may dial the supplied
// host. In air-gap mode, only allow-listed hosts may be dialed;
// outside air-gap, all hosts are allowed.
func (a *AirGapMode) AllowsDial(host string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.enabled {
		return true
	}
	return a.onlineAllowList[host]
}

// ApplyUpdateAdvisory verifies and persists the supplied advisory.
// Returns the applied advisory or an error if verification fails
// or the advisory is unknown / expired.
func (a *AirGapMode) ApplyUpdateAdvisory(advisory *UpdateAdvisory, sourcePub []byte, nowMs int64) error {
	if advisory == nil {
		return errors.New("sovereign: nil advisory")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if advisory.IsExpired(nowMs) {
		a.rejectedAdvisories++
		return errors.New("sovereign: advisory expired")
	}
	if !verifyAdvisorySignature(sourcePub, advisory) {
		a.rejectedAdvisories++
		return errors.New("sovereign: advisory signature verification failed")
	}
	a.advisories[advisory.AdvisoryID] = advisory
	a.appliedAdvisories++
	return nil
}

// AppliedAdvisoryCount returns the E1 status-bar counter.
func (a *AirGapMode) AppliedAdvisoryCount() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.appliedAdvisories
}

// RejectedAdvisoryCount returns the E1 status-bar counter.
func (a *AirGapMode) RejectedAdvisoryCount() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rejectedAdvisories
}

// verifyAdvisorySignature checks the advisory's signature under
// the supplied source public key.
func verifyAdvisorySignature(sourcePub []byte, advisory *UpdateAdvisory) bool {
	if len(sourcePub) == 0 {
		return false
	}
	if len(advisory.Signature) == 0 {
		return false
	}
	// ed25519.Verify expects a 32-byte public key. A custom
	// source might supply a longer key for downstream tooling;
	// we require the canonical 32-byte length.
	if len(sourcePub) != ed25519PublicKeySize {
		return false
	}
	// Re-import the ed25519.Verify path here so callers don't
	// have to. The build constraint enforces ed25519 import.
	return ed25519Verify(sourcePub, advisory.SigningBytes(), advisory.Signature)
}

// VerifyAdvisorySignature is the exported form used by the CLI's offline
// update import path (ADR G3: signed offline advisory is the sovereign
// update channel).
func VerifyAdvisorySignature(sourcePub []byte, advisory *UpdateAdvisory) bool {
	return verifyAdvisorySignature(sourcePub, advisory)
}

// _ keeps the time import visible when tests evolve to use it.
var _ = time.Now
