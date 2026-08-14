package paperproto

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
)

// coseSign1STAForTest constructs a COSE-Sign1 structure with the same header
// shape the relay issues. The relay's
// `internal/paper/cose.go::CreateCOSESign1` is the canonical producer; this
// mirror exists only because the nested connector does not depend on the root
// repo's `internal/paper` package and we still need a deterministic CBOR
// COSE-Sign1 in tests.
func coseSign1STAForTest(payload []byte, priv ed25519.PrivateKey, kid []byte) coseSign1 {
	protected := map[int]interface{}{
		1: int(COSEAlgEdDSA),
		4: kid,
	}
	protectedBytes, err := MarshalCBOR(protected)
	if err != nil {
		panic(fmt.Sprintf("cbor protected: %v", err))
	}
	sigStructure := []interface{}{
		"Signature1",
		protectedBytes,
		[]byte{},
		payload,
	}
	sigBytes, err := MarshalCBOR(sigStructure)
	if err != nil {
		panic(fmt.Sprintf("cbor sig_structure: %v", err))
	}
	signature := ed25519.Sign(priv, sigBytes)
	return coseSign1{
		Protected:   protectedBytes,
		Unprotected: map[int]interface{}{},
		Payload:     payload,
		Signature:   signature,
	}
}

// coseSign1 is the local mirror of the canonical COSE-Sign1 tag layout. The
// relay decodes this exact shape via fxamacker/cbor when verifying the
// credential inside an AUTH_PROOF. Field 0 is the protected header bytes,
// field 1 is the unprotected header map, field 2 is the payload, field 3 is
// the signature.
type coseSign1 struct {
	Protected   []byte             `cbor:"0,keyasint"`
	Unprotected map[int]interface{} `cbor:"1,keyasint"`
	Payload     []byte             `cbor:"2,keyasint"`
	Signature   []byte             `cbor:"3,keyasint"`
}

// Ensure binary is referenced; builds may otherwise complain about unused
// imports when the helper evolves.
var _ = binary.BigEndian
