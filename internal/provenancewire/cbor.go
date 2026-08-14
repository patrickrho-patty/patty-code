package provenancewire

import "github.com/fxamacker/cbor/v2"

// marshalCBOR encodes the envelope into deterministic CBOR bytes.
// The relay and the connector MUST agree on the encoding.
func marshalCBOR(v interface{}) ([]byte, error) {
	return cbor.Marshal(v)
}

// unmarshalCBOR decodes an envelope from CBOR bytes.
func unmarshalCBOR(data []byte, v interface{}) error {
	return cbor.Unmarshal(data, v)
}
