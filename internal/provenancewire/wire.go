package provenancewire

import (
	"errors"
	"fmt"
)

// EncodeChangeSetEnvelope serializes a change-set envelope for the
// relay. The relay's `provenance.CreateChangeSet` decodes the
// exact CBOR layout produced here.
func EncodeChangeSetEnvelope(env *ChangeSetEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil change set envelope")
	}
	return marshalEnvelope(env)
}

// DecodeChangeSetEnvelope parses a CBOR change-set envelope from
// the wire.
func DecodeChangeSetEnvelope(data []byte) (*ChangeSetEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty change set body")
	}
	env := &ChangeSetEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode change set: %w", err)
	}
	return env, nil
}

// EncodeSpanEnvelope serializes a span envelope.
func EncodeSpanEnvelope(env *ProvenanceSpanEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil span envelope")
	}
	return marshalEnvelope(env)
}

// DecodeSpanEnvelope parses a span envelope.
func DecodeSpanEnvelope(data []byte) (*ProvenanceSpanEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty span body")
	}
	env := &ProvenanceSpanEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode span: %w", err)
	}
	return env, nil
}

// EncodeActionEnvelope serializes an action envelope.
func EncodeActionEnvelope(env *ActionEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil action envelope")
	}
	return marshalEnvelope(env)
}

// DecodeActionEnvelope parses an action envelope.
func DecodeActionEnvelope(data []byte) (*ActionEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty action body")
	}
	env := &ActionEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode action: %w", err)
	}
	return env, nil
}

// EncodeCommitBindingEnvelope serializes a commit-binding envelope.
func EncodeCommitBindingEnvelope(env *CommitBindingEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil commit binding envelope")
	}
	return marshalEnvelope(env)
}

// DecodeCommitBindingEnvelope parses a commit-binding envelope.
func DecodeCommitBindingEnvelope(data []byte) (*CommitBindingEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty commit binding body")
	}
	env := &CommitBindingEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode commit binding: %w", err)
	}
	return env, nil
}

// EncodeEvidenceReceiptEnvelope serializes a relay-pushed
// evidence receipt. The relay uses this in the
// `MsgEvidenceReceipt` body (PRD §40.3).
func EncodeEvidenceReceiptEnvelope(env *EvidenceReceiptEnvelope) ([]byte, error) {
	if env == nil {
		return nil, errors.New("provenancewire: nil evidence receipt envelope")
	}
	return marshalEnvelope(env)
}

// DecodeEvidenceReceiptEnvelope parses a relay-pushed evidence
// receipt.
func DecodeEvidenceReceiptEnvelope(data []byte) (*EvidenceReceiptEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty evidence receipt body")
	}
	env := &EvidenceReceiptEnvelope{}
	if err := unmarshalEnvelope(data, env); err != nil {
		return nil, fmt.Errorf("provenancewire: decode evidence receipt: %w", err)
	}
	return env, nil
}

// EncodeReceiptAck serializes the connector's reply to a
// relay-pushed receipt.
func EncodeReceiptAck(ack *ReceiptAck) ([]byte, error) {
	if ack == nil {
		return nil, errors.New("provenancewire: nil receipt ack")
	}
	return marshalEnvelope(ack)
}

// DecodeReceiptAck parses the connector's reply.
func DecodeReceiptAck(data []byte) (*ReceiptAck, error) {
	if len(data) == 0 {
		return nil, errors.New("provenancewire: empty receipt ack body")
	}
	ack := &ReceiptAck{}
	if err := unmarshalEnvelope(data, ack); err != nil {
		return nil, fmt.Errorf("provenancewire: decode receipt ack: %w", err)
	}
	return ack, nil
}

// marshalEnvelope is a tiny CBOR encoder wrapper.
func marshalEnvelope(env interface{}) ([]byte, error) {
	return marshalCBOR(env)
}

// unmarshalEnvelope is a tiny CBOR decoder wrapper.
func unmarshalEnvelope(data []byte, env interface{}) error {
	return unmarshalCBOR(data, env)
}
