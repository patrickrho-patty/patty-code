package dariproto

import (
	"errors"
	"fmt"
	"sort"

	"crypto/ed25519"
)

// decision.go is the connector-side decode + verification of the
// relay's signed F.6 Authorization Decision (RELAY_VERDICT, 0x0304).
// The struct mirrors the relay's kernel AuthorizationDecisionBody
// BYTE-FOR-BYTE: same labels, same omitempty set (labels 10/11 carry
// NO omitempty — nil encodes as CBOR null), same enum values, and the
// same canonical sort (ReasonCodes + Obligations ordered by ID).
// The cross-repo conformance suite pins these bytes.

// DecisionAAD mirrors the kernel's F.6 external AAD.
const DecisionAAD = "DARI-AUTHORIZATION-DECISION-v1\x00"

// DecisionOutcome mirrors the kernel enum EXACTLY.
type DecisionOutcome uint8

const (
	DecisionAllow                DecisionOutcome = 1
	DecisionDeny                 DecisionOutcome = 2
	DecisionAllowWithObligations DecisionOutcome = 3
)

// ObligationPhase mirrors the kernel (PRE_ACTION / POST_ACTION).
type ObligationPhase uint8

const (
	PhasePreAction  ObligationPhase = 1
	PhasePostAction ObligationPhase = 2
)

// ObligationState mirrors the kernel (PENDING / SATISFIED / FAILED).
type ObligationState uint8

const (
	ObligationPending   ObligationState = 1
	ObligationSatisfied ObligationState = 2
	ObligationFailed    ObligationState = 3
)

// Obligation mirrors the kernel F.6 obligation object (labels 1-8).
type Obligation struct {
	ObligationID    string          `cbor:"1,keyasint"`
	Kind            string          `cbor:"2,keyasint"`
	ParameterDigest Digest          `cbor:"3,keyasint"`
	Phase           ObligationPhase `cbor:"4,keyasint"`
	State           ObligationState `cbor:"5,keyasint"`
	ResponsiblePeer string          `cbor:"6,keyasint"`
	DeadlineMs      int64           `cbor:"7,keyasint,omitempty"`
	EvidenceDigest  Digest          `cbor:"8,keyasint,omitempty"`
}

// AuthorizationDecisionBody mirrors the F.6 body (labels 1-14) with
// the kernel's exact omitempty set.
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
	Obligations            []Obligation    `cbor:"10,keyasint"`
	ReasonCodes            []string        `cbor:"11,keyasint"`
	IssuedAtMs             int64           `cbor:"12,keyasint"`
	ExpiresAtMs            int64           `cbor:"13,keyasint"`
	SupportingEvidence     []Digest        `cbor:"14,keyasint,omitempty"`
}

// encodeCanonical mirrors the kernel's EncodeDecisionBody: sort
// ReasonCodes and Obligations (by ObligationID), then marshal.
func encodeCanonical(b *AuthorizationDecisionBody) ([]byte, error) {
	codes := sortedStringsCopy(b.ReasonCodes)
	obs := append([]Obligation(nil), b.Obligations...)
	sort.SliceStable(obs, func(i, j int) bool {
		return obs[i].ObligationID < obs[j].ObligationID
	})
	sorted := *b
	sorted.ReasonCodes = codes
	sorted.Obligations = obs
	return MarshalCBOR(&sorted)
}

// DecisionEnvelope is the verified decision + its raw bytes.
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
	reencoded, err := encodeCanonical(&body)
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

// VerifyAtTime is the full connector-side acceptance check: the
// decision must be bound to this exchange, inside its validity window,
// with an allow-family outcome.
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
	case DecisionAllow, DecisionAllowWithObligations:
		return nil
	case DecisionDeny:
		return fmt.Errorf("dari: decision DENY (%v)", e.Body.ReasonCodes)
	default:
		return fmt.Errorf("dari: unknown decision outcome %d", e.Body.Outcome)
	}
}

// Allows reports whether the decision authorizes consumption right now.
func (e *DecisionEnvelope) Allows(nowMs int64) bool {
	return e.VerifyAtTime("", nowMs) == nil
}

// sortedStringsCopy returns a sorted copy (decision-local helper; the
// package's other sortedCopy is declared in authorization.go).
func sortedStringsCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
