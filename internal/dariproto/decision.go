package dariproto

import (
	"crypto/ed25519"
	"errors"
	"fmt"
)

// decision.go is the connector-side decode + verification of the
// relay's signed F.6 Authorization Decision (RELAY_VERDICT, 0x0304).
// The body labels mirror the relay's internal/dari AuthorizationDecisionBody
// field-for-field; the cross-repo byte contract is pinned by the root
// conformance suite.

// DecisionAAD mirrors the kernel's F.6 external AAD.
const DecisionAAD = "DARI-AUTHORIZATION-DECISION-v1\x00"

// DecisionOutcome mirrors the kernel enum (uint CBOR).
type DecisionOutcome uint8

const (
	DecisionAllowDecision             DecisionOutcome = 1
	DecisionAllowWithObligations      DecisionOutcome = 2
	DecisionDenyDecision              DecisionOutcome = 3
	DecisionDenyWithEscalationOutcome DecisionOutcome = 4
)

// AuthorizationDecisionBody mirrors the F.6 body (labels 1-14).
type AuthorizationDecisionBody struct {
	Version                uint16          `cbor:"1,keyasint"`
	DecisionID             string          `cbor:"2,keyasint"`
	ExchangeID             string          `cbor:"3,keyasint"`
	GovernedExchangeDigest Digest          `cbor:"4,keyasint"`
	ActionDigest           Digest          `cbor:"5,keyasint"`
	LeafGrantDigest        Digest          `cbor:"6,keyasint"`
	PolicyCheckpointDigest Digest          `cbor:"7,keyasint"`
	EvaluatorPeerID        string          `cbor:"8,keyasint"`
	Outcome                DecisionOutcome `cbor:"9,keyasint"`
	Obligations            []interface{}   `cbor:"10,keyasint,omitempty"`
	ReasonCodes            []string        `cbor:"11,keyasint,omitempty"`
	IssuedAtMs             int64           `cbor:"12,keyasint"`
	ExpiresAtMs            int64           `cbor:"13,keyasint"`
	SupportingEvidence     []Digest        `cbor:"14,keyasint,omitempty"`
}

// DecisionEnvelope is the verified decision + its signed digest.
type DecisionEnvelope struct {
	Body      *AuthorizationDecisionBody
	COSEBytes []byte
}

// DecodeAuthorizationDecision verifies a RELAY_VERDICT payload under
// the AUTH_ACK policy issuer key: COSE decode, canonical re-encode
// equality (F.2), and signature verification.
func DecodeAuthorizationDecision(coseBytes []byte, signer ed25519.PublicKey) (*DecisionEnvelope, error) {
	if len(coseBytes) == 0 {
		return nil, errors.New("dari: empty decision payload")
	}
	var sign1 COSESign1
	if err := UnmarshalCBOR(coseBytes, &sign1); err != nil {
		return nil, fmt.Errorf("dari: decode decision COSE: %w", err)
	}
	var body AuthorizationDecisionBody
	if err := UnmarshalCBOR(sign1.Payload, &body); err != nil {
		return nil, fmt.Errorf("dari: decode decision body: %w", err)
	}
	reencoded, err := MarshalCBOR(&body)
	if err != nil {
		return nil, err
	}
	if string(reencoded) != string(sign1.Payload) {
		return nil, errors.New("dari: decision body is not canonical")
	}
	if err := VerifyCOSESign1WithAAD(&sign1, []byte(DecisionAAD), sign1.Payload, signer); err != nil {
		return nil, fmt.Errorf("dari: decision signature: %w", err)
	}
	return &DecisionEnvelope{Body: &body, COSEBytes: coseBytes}, nil
}

// Allows reports whether the decision authorizes consumption right
// now: an allow-family outcome inside its validity window.
func (e *DecisionEnvelope) Allows(nowMs int64) bool {
	if e == nil || e.Body == nil {
		return false
	}
	switch e.Body.Outcome {
	case DecisionAllowDecision, DecisionAllowWithObligations:
	default:
		return false
	}
	return nowMs >= e.Body.IssuedAtMs && nowMs < e.Body.ExpiresAtMs
}

// VerifyAtTime is the full connector-side acceptance check.
func (e *DecisionEnvelope) VerifyAtTime(exchangeID string, nowMs int64) error {
	if e == nil || e.Body == nil {
		return errors.New("dari: nil decision")
	}
	if exchangeID != "" && e.Body.ExchangeID != exchangeID {
		return fmt.Errorf("dari: decision bound to exchange %s, stream is %s", e.Body.ExchangeID, exchangeID)
	}
	if nowMs < e.Body.IssuedAtMs {
		return errors.New("dari: decision not yet valid")
	}
	if e.Body.ExpiresAtMs > 0 && nowMs >= e.Body.ExpiresAtMs {
		return errors.New("dari: decision expired")
	}
	switch e.Body.Outcome {
	case DecisionAllowDecision, DecisionAllowWithObligations:
		return nil
	case DecisionDenyDecision, DecisionDenyWithEscalationOutcome:
		return fmt.Errorf("dari: decision DENY (%v)", e.Body.ReasonCodes)
	default:
		return fmt.Errorf("dari: unknown decision outcome %d", e.Body.Outcome)
	}
}
