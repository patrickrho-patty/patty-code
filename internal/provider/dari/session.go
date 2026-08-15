package dari

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"patty/internal/dariproto"
)

// session.go implements the connector's half of the session-governance
// handshake (harness feature plans A3/A4/A5, e2e):
//
//	SESSION_OPEN → POLICY_EPOCH → CATALOG_SNAPSHOT → LEASE_ISSUE → SESSION_GRANT
//
// The relay drives the sequence after AUTH_ACK; this code sends the
// SESSION_OPEN, consumes the pushes, verifies the issued lease under
// the issuer key the AUTH_ACK payload carried, and installs the
// clients the provider's fail-closed dispatch checks consult.

// authAckInfo is the AUTH_ACK payload body the relay sends.
type authAckInfo struct {
	Status         string `json:"status"`
	RelayID        string `json:"relay_id"`
	PolicyIssuer   string `json:"policy_issuer"`
	PolicyIssuerPK string `json:"policy_issuer_pk"`
}

// openSession runs the governance setup on a freshly authenticated
// connection. It is idempotent per connection (the relay runs setup on
// every SESSION_OPEN).
func (p *Provider) openSession(conn *dariproto.TransportConn) error {
	sessionID := p.ensureSessionIDLocked()
	userID := p.userID
	if userID == "" {
		userID = "user-" + p.subjectPeerID
	}
	body, err := json.Marshal(map[string]string{
		"session_id": sessionID,
		"user_id":    userID,
		"model":      p.model,
	})
	if err != nil {
		return fmt.Errorf("dari: marshal SESSION_OPEN: %w", err)
	}
	if err := conn.SendMessage(dariproto.MsgSessionOpen, nil, body, 0, 0); err != nil {
		return fmt.Errorf("dari: send SESSION_OPEN: %w", err)
	}

	// Consume the relay's setup pushes until SESSION_GRANT. The relay
	// sends POLICY_EPOCH, CATALOG_SNAPSHOT, LEASE_ISSUE, SESSION_GRANT
	// in that order; tolerate re-pushes (epoch change mid-setup).
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return errors.New("dari: session setup timed out waiting for SESSION_GRANT")
		}
		rec, err := conn.RecvRecord()
		if err != nil {
			return fmt.Errorf("dari: session setup recv: %w", err)
		}
		msgType := dariproto.MessageType(rec.MessageType)
		switch msgType {
		case dariproto.MsgPolicyEpochPush:
			epoch, err := dariproto.DecodePolicyEpochMessage(rec.Payload)
			if err != nil {
				return fmt.Errorf("dari: decode POLICY_EPOCH: %w", err)
			}
			if p.policyEpochClient == nil {
				p.policyEpochClient = dariproto.NewPolicyEpochClient()
			}
			if err := p.policyEpochClient.Bind(epoch); err != nil {
				return fmt.Errorf("dari: bind POLICY_EPOCH: %w", err)
			}
			p.policyEpoch = epoch.EpochID

		case dariproto.MsgCatalogSnapshot:
			snap, err := dariproto.DecodeCatalogSnapshot(rec.Payload)
			if err != nil {
				return fmt.Errorf("dari: decode CATALOG_SNAPSHOT: %w", err)
			}
			if p.catalogClient == nil {
				p.catalogClient = dariproto.NewCatalogClient()
			}
			if err := p.catalogClient.ApplySnapshot(snap, p.policyEpoch); err != nil {
				// A snapshot for a different epoch than we just bound is a
				// relay-side incoherence — fail closed rather than adopt it.
				return fmt.Errorf("dari: apply CATALOG_SNAPSHOT: %w", err)
			}

		case dariproto.MsgLeaseIssue:
			lease, err := dariproto.DecodeLeaseResponse(rec.Payload)
			if err != nil {
				return fmt.Errorf("dari: decode LEASE_ISSUE: %w", err)
			}
			if p.leaseClient == nil {
				return errors.New("dari: LEASE_ISSUE received but no lease client is configured (missing AUTH_ACK issuer key)")
			}
			if err := p.leaseClient.Acquire(p.subjectPeerID, sessionID, lease); err != nil {
				return fmt.Errorf("dari: acquire lease: %w", err)
			}

		case dariproto.MsgSessionGrant:
			var grant struct {
				SessionID string `json:"session_id"`
				GrantHex  string `json:"grant_hex"`
			}
			_ = json.Unmarshal(rec.Payload, &grant)
			// The signed DARI Authorization Grant (Task 7): verify it
			// under the AUTH_ACK policy issuer key and retain it as the
			// session's authority object.
			if grant.GrantHex != "" && p.leaseClient != nil {
				raw, derr := hex.DecodeString(grant.GrantHex)
				if derr != nil {
					return fmt.Errorf("dari: session grant hex: %w", derr)
				}
				env, derr2 := dariproto.DecodeAuthorizationGrant(raw, p.leaseClient.IssuerPublicKey())
				if derr2 != nil {
					return fmt.Errorf("dari: session grant verification failed: %w", derr2)
				}
				p.sessionGrant = env
			}
			return nil

		case dariproto.MsgClose:
			var errMsg map[string]string
			json.Unmarshal(rec.Payload, &errMsg)
			return fmt.Errorf("dari: relay rejected session: %s", errMsg["error"])

		case dariproto.MsgPing:
			if err := conn.SendControl(dariproto.MsgPong, nil, []byte("pong")); err != nil {
				return fmt.Errorf("dari: pong during setup: %w", err)
			}

		default:
			// Unknown extension during setup — skip, the relay may
			// interleave advisory pushes.
		}
	}
}

// ensureSessionID returns the provider's session ID, generating and
// persisting one on first call.
func (p *Provider) ensureSessionID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ensureSessionIDLocked()
}

// installGovernanceClientsFromAuthAck builds the lease/epoch/catalog
// clients from the AUTH_ACK payload's policy-issuer public key. The
// provider keeps externally-installed clients (tests, offline
// installs) untouched.
func (p *Provider) installGovernanceClientsFromAuthAck(payload []byte) error {
	if len(payload) == 0 {
		return nil // dev relay without governance payload; dispatch stays fail-closed
	}
	var info authAckInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		// Plain "authenticated" body from an older relay — not an error.
		return nil
	}
	if info.PolicyIssuerPK == "" {
		return nil
	}
	pubBytes, err := hex.DecodeString(info.PolicyIssuerPK)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("dari: invalid policy issuer public key in AUTH_ACK")
	}
	p.policyIssuerKey = ed25519.PublicKey(pubBytes)
	if p.leaseClient == nil {
		p.leaseClient = dariproto.NewLeaseClient(ed25519.PublicKey(pubBytes), info.PolicyIssuer)
		p.leaseClient.WithAutoRenewBefore(p.autoRenewBefore)
	}
	// E5: directive verification key — the admin dispatcher verifies
	// signed admin commands under the same issuer.
	if p.adminDisp != nil {
		p.adminDisp.SetIssuerPubKey(p.policyIssuerKey)
	}
	return nil
}

// connAckSender adapts the live DARI transport to the provenancewire
// AckSender seam so evidence-receipt acks ride the authenticated
// connection.
type connAckSender struct {
	conn *dariproto.TransportConn
}

func (s connAckSender) SendRecord(rec *dariproto.Record) error {
	if s.conn == nil {
		return errors.New("dari: no live connection for ack")
	}
	return s.conn.SendRecord(rec)
}

// SessionID returns the open session's identifier (generating one if
// none exists yet).
func (p *Provider) SessionID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ensureSessionIDLocked()
}

func (p *Provider) ensureSessionIDLocked() string {
	if p.sessionID != "" {
		return p.sessionID
	}
	p.sessionID = fmt.Sprintf("sess-%d", time.Now().UnixMilli())
	return p.sessionID
}

// Prewarm prepares the transport for the NEXT turn (harness plan F2,
// parity with cached-WS prewarm): ensures the authenticated connection
// and session governance are live so the next Stream pays no setup
// cost, and re-acquires the lease when it enters its renewal window.
// Failures are returned for the caller's audit log; prewarm never
// invalidates an existing healthy session.
func (p *Provider) Prewarm(ctx context.Context) error {
	p.mu.Lock()
	healthy := p.conn != nil
	p.mu.Unlock()
	if !healthy {
		return p.connect(ctx)
	}
	// Connection alive: check lease renewal window.
	if p.leaseClient != nil && p.leaseClient.NeedsRenewal() {
		// A renewal is a fresh SESSION_OPEN cycle on the live
		// connection; the relay re-runs setup and pushes a new lease.
		p.mu.Lock()
		conn := p.conn
		p.mu.Unlock()
		if err := p.openSession(conn); err != nil {
			return fmt.Errorf("dari: prewarm session refresh: %w", err)
		}
	}
	return nil
}

// SessionGrant returns the relay-issued, policy-signed DARI
// Authorization Grant bound to this session (nil when the relay did
// not send one, e.g. legacy deployments).
func (p *Provider) SessionGrant() *dariproto.GrantEnvelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sessionGrant
}

// StoredAdvisory is one relay-pushed broadcast/directive/advisory.
type StoredAdvisory struct {
	Kind    string
	Payload []byte
}

// storeAdvisory records a relay push (thread-safe; the reader
// goroutine is the only writer).
func (p *Provider) storeAdvisory(kind string, payload []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.advisories = append(p.advisories, StoredAdvisory{Kind: kind, Payload: append([]byte(nil), payload...)})
	if len(p.advisories) > 128 {
		p.advisories = p.advisories[len(p.advisories)-128:]
	}
}

// Advisories drains the stored relay pushes (E2/E3/E5 surfaces).
func (p *Provider) Advisories() []StoredAdvisory {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]StoredAdvisory, len(p.advisories))
	copy(out, p.advisories)
	return out
}

// SetDLPRuleSink installs the consumer of relay-pushed DLP rule packs
// (C1.3). The agent's DLP wrapper installs a sink that applies the
// org's class enables/disables to the live scanner.
func (p *Provider) SetDLPRuleSink(sink func(*dariproto.DLPRulePackWire)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dlpRuleSink = sink
}

// applyDLPRulePack routes a decoded pack to the installed sink.
func (p *Provider) applyDLPRulePack(pack *dariproto.DLPRulePackWire) {
	p.mu.Lock()
	sink := p.dlpRuleSink
	p.mu.Unlock()
	if sink == nil || pack == nil {
		return
	}
	sink(pack)
}

// SetGovernanceSink installs the consumer of relay-pushed
// governance-state snapshots (C3/C4/D1/D3-D6/E4). Boot installs a
// sink that builds a governed.State and installs it on the
// controller so the pushed gates fire on real tool calls.
func (p *Provider) SetGovernanceSink(sink func(*dariproto.GovernanceStateWire)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.governanceSink = sink
}

// applyGovernanceState routes a decoded snapshot to the installed sink.
func (p *Provider) applyGovernanceState(snap *dariproto.GovernanceStateWire) {
	p.mu.Lock()
	sink := p.governanceSink
	p.mu.Unlock()
	if sink == nil || snap == nil {
		return
	}
	sink(snap)
}
