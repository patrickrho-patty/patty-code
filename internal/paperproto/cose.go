package paperproto

import (
	"crypto/ed25519"
	"errors"
	"fmt"
)

// CreateCOSESign1 produces a COSE-Sign1 envelope over the supplied payload.
// The connector uses this for the harness-side AUTH_PROOF and lease
// presentations; the relay verifies the same envelope shape via its own
// `internal/paper/cose.go::VerifyCOSESign1`. Both sides MUST agree on the
// protected header (alg = EdDSA, kid = issuer-defined label) and the
// Sig_structure (RFC 8152 §4.4).
func CreateCOSESign1(payload []byte, priv ed25519.PrivateKey, kid []byte) ([]byte, error) {
	if len(priv) == 0 {
		return nil, errors.New("paper: empty private key")
	}
	protected := map[int]interface{}{
		1: int(-8), // COSEAlgEdDSA
		4: kid,
	}
	protectedBytes, err := MarshalCBOR(protected)
	if err != nil {
		return nil, fmt.Errorf("paper: marshal protected header: %w", err)
	}
	sigInput := []interface{}{
		"Signature1",
		protectedBytes,
		[]byte{},
		payload,
	}
	sigStruct, err := MarshalCBOR(sigInput)
	if err != nil {
		return nil, fmt.Errorf("paper: marshal sig structure: %w", err)
	}
	signature := ed25519.Sign(priv, sigStruct)
	sign1 := &coseSign1{
		Protected:   protectedBytes,
		Unprotected: map[int]interface{}{},
		Payload:     payload,
		Signature:   signature,
	}
	return MarshalCBOR(sign1)
}

// coseSign1 is the connector-side mirror of the canonical COSE-Sign1
// tag layout. The relay decodes this exact shape via fxamacker/cbor when
// verifying the credential inside an AUTH_PROOF. Field 0 is the
// protected header bytes, field 1 is the unprotected header map, field
// 2 is the payload, field 3 is the signature.
type coseSign1 struct {
	Protected   []byte              `cbor:"0,keyasint"`
	Unprotected map[int]interface{} `cbor:"1,keyasint"`
	Payload     []byte              `cbor:"2,keyasint"`
	Signature   []byte              `cbor:"3,keyasint"`
}

// DecodeCOSESign1 decodes a CBOR-encoded COSE-Sign1 envelope. The
// returned struct exposes the protected header bytes, the payload, and
// the signature so the verifier can recompute the Sig_structure and
// verify the Ed25519 signature.
func DecodeCOSESign1(data []byte) (*coseSign1, error) {
	var sign1 coseSign1
	if err := UnmarshalCBOR(data, &sign1); err != nil {
		return nil, fmt.Errorf("paper: decode COSE-Sign1: %w", err)
	}
	return &sign1, nil
}

// VerifyCOSESign1 verifies the Ed25519 signature embedded in a COSE-Sign1
// envelope against the supplied public key. The relay and the connector
// MUST use the same Sig_structure layout (context "Signature1", the
// bytes of the protected header, an empty external AAD, and the payload).
func VerifyCOSESign1(sign1 *coseSign1, pub ed25519.PublicKey) error {
	if sign1 == nil {
		return errors.New("paper: nil COSE-Sign1")
	}
	if len(pub) == 0 {
		return errors.New("paper: empty public key")
	}
	sigInput := []interface{}{
		"Signature1",
		sign1.Protected,
		[]byte{},
		sign1.Payload,
	}
	sigBytes, err := MarshalCBOR(sigInput)
	if err != nil {
		return fmt.Errorf("paper: marshal sig structure for verify: %w", err)
	}
	if !ed25519.Verify(pub, sigBytes, sign1.Signature) {
		return errors.New("paper: COSE-Sign1 signature verification failed")
	}
	return nil
}
