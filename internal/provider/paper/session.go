package paper

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"patty/internal/paperproto"
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
func (p *Provider) openSession(conn *paperproto.TransportConn) error {
	sessionID := p.ensureSessionID()
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
		return fmt.Errorf("paper: marshal SESSION_OPEN: %w", err)
	}
	if err := conn.SendMessage(paperproto.MsgSessionOpen, nil, body, 0, 0); err != nil {
		return fmt.Errorf("paper: send SESSION_OPEN: %w", err)
	}

	// Consume the relay's setup pushes until SESSION_GRANT. The relay
	// sends POLICY_EPOCH, CATALOG_SNAPSHOT, LEASE_ISSUE, SESSION_GRANT
	// in that order; tolerate re-pushes (epoch change mid-setup).
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return errors.New("paper: session setup timed out waiting for SESSION_GRANT")
		}
		rec, err := conn.RecvRecord()
		if err != nil {
			return fmt.Errorf("paper: session setup recv: %w", err)
		}
		msgType := paperproto.MessageType(rec.MessageType)
		switch msgType {
		case paperproto.MsgPolicyEpochPush:
			epoch, err := paperproto.DecodePolicyEpochMessage(rec.Payload)
			if err != nil {
				return fmt.Errorf("paper: decode POLICY_EPOCH: %w", err)
			}
			if p.policyEpochClient == nil {
				p.policyEpochClient = paperproto.NewPolicyEpochClient()
			}
			if err := p.policyEpochClient.Bind(epoch); err != nil {
				return fmt.Errorf("paper: bind POLICY_EPOCH: %w", err)
			}
			p.policyEpoch = epoch.EpochID

		case paperproto.MsgCatalogSnapshot:
			snap, err := paperproto.DecodeCatalogSnapshot(rec.Payload)
			if err != nil {
				return fmt.Errorf("paper: decode CATALOG_SNAPSHOT: %w", err)
			}
			if p.catalogClient == nil {
				p.catalogClient = paperproto.NewCatalogClient()
			}
			if err := p.catalogClient.ApplySnapshot(snap, p.policyEpoch); err != nil {
				// A snapshot for a different epoch than we just bound is a
				// relay-side incoherence — fail closed rather than adopt it.
				return fmt.Errorf("paper: apply CATALOG_SNAPSHOT: %w", err)
			}

		case paperproto.MsgLeaseIssue:
			lease, err := paperproto.DecodeLeaseResponse(rec.Payload)
			if err != nil {
				return fmt.Errorf("paper: decode LEASE_ISSUE: %w", err)
			}
			if p.leaseClient == nil {
				return errors.New("paper: LEASE_ISSUE received but no lease client is configured (missing AUTH_ACK issuer key)")
			}
			if err := p.leaseClient.Acquire(p.subjectPeerID, sessionID, lease); err != nil {
				return fmt.Errorf("paper: acquire lease: %w", err)
			}

		case paperproto.MsgSessionGrant:
			return nil

		case paperproto.MsgClose:
			var errMsg map[string]string
			json.Unmarshal(rec.Payload, &errMsg)
			return fmt.Errorf("paper: relay rejected session: %s", errMsg["error"])

		case paperproto.MsgPing:
			if err := conn.SendControl(paperproto.MsgPong, nil, []byte("pong")); err != nil {
				return fmt.Errorf("paper: pong during setup: %w", err)
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
	if p.sessionID != "" {
		return p.sessionID
	}
	p.sessionID = fmt.Sprintf("sess-%d", time.Now().UnixMilli())
	return p.sessionID
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
		return fmt.Errorf("paper: invalid policy issuer public key in AUTH_ACK")
	}
	if p.leaseClient == nil {
		p.leaseClient = paperproto.NewLeaseClient(ed25519.PublicKey(pubBytes), info.PolicyIssuer)
		p.leaseClient.WithAutoRenewBefore(p.autoRenewBefore)
	}
	return nil
}

// connAckSender adapts the live PAPER transport to the provenancewire
// AckSender seam so evidence-receipt acks ride the authenticated
// connection.
type connAckSender struct {
	conn *paperproto.TransportConn
}

func (s connAckSender) SendRecord(rec *paperproto.Record) error {
	if s.conn == nil {
		return errors.New("paper: no live connection for ack")
	}
	return s.conn.SendRecord(rec)
}
