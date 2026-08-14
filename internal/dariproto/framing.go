package dariproto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// PrefaceSize is the size of the 32-byte binary prelude.
const PrefaceSize = 32

// MaxPayloadLen is the maximum single payload length.
const MaxPayloadLen = 1024 * 1024

// Record is a DARI record after parsing the 32-byte prelude.
type Record struct {
	VersionMajor byte
	Kind         RecordKind
	Flags        Flags
	MessageType  uint16
	Header       []byte
	Payload      []byte
	LaneID       uint64
	LaneSequence uint64
}

// EncodeRecord writes a DARI record (32-byte prelude + header + payload) to w.
func EncodeRecord(w io.Writer, r *Record) error {
	if r.VersionMajor == 0 {
		r.VersionMajor = VersionMajor
	}
	if len(r.Payload) > MaxPayloadLen {
		return fmt.Errorf("dari: payload exceeds max length %d", MaxPayloadLen)
	}

	var prelude [PrefaceSize]byte
	prelude[0] = r.VersionMajor
	prelude[1] = byte(r.Kind)
	binary.BigEndian.PutUint16(prelude[2:4], uint16(r.Flags))
	binary.BigEndian.PutUint16(prelude[4:6], r.MessageType)
	binary.BigEndian.PutUint16(prelude[6:8], uint16(len(r.Header)))
	binary.BigEndian.PutUint32(prelude[8:12], uint32(len(r.Payload)))
	binary.BigEndian.PutUint64(prelude[12:20], r.LaneID)
	binary.BigEndian.PutUint64(prelude[20:28], r.LaneSequence)

	if _, err := w.Write(prelude[:]); err != nil {
		return fmt.Errorf("dari: write prelude: %w", err)
	}
	if len(r.Header) > 0 {
		if _, err := w.Write(r.Header); err != nil {
			return fmt.Errorf("dari: write header: %w", err)
		}
	}
	if len(r.Payload) > 0 {
		if _, err := w.Write(r.Payload); err != nil {
			return fmt.Errorf("dari: write payload: %w", err)
		}
	}
	return nil
}

// DecodeRecord reads a single DARI record from r.
func DecodeRecord(r io.Reader) (*Record, error) {
	var prelude [PrefaceSize]byte
	if _, err := io.ReadFull(r, prelude[:]); err != nil {
		return nil, fmt.Errorf("dari: read prelude: %w", err)
	}

	rec := &Record{
		VersionMajor: prelude[0],
		Kind:         RecordKind(prelude[1]),
		Flags:        Flags(binary.BigEndian.Uint16(prelude[2:4])),
		MessageType:  binary.BigEndian.Uint16(prelude[4:6]),
	}
	headerLen := binary.BigEndian.Uint16(prelude[6:8])
	payloadLen := binary.BigEndian.Uint32(prelude[8:12])
	rec.LaneID = binary.BigEndian.Uint64(prelude[12:20])
	rec.LaneSequence = binary.BigEndian.Uint64(prelude[20:28])

	if rec.VersionMajor != VersionMajor {
		return nil, fmt.Errorf("dari: unsupported version %d", rec.VersionMajor)
	}
	if payloadLen > MaxPayloadLen {
		return nil, fmt.Errorf("dari: payload length %d exceeds max", payloadLen)
	}

	if headerLen > 0 {
		rec.Header = make([]byte, headerLen)
		if _, err := io.ReadFull(r, rec.Header); err != nil {
			return nil, fmt.Errorf("dari: read header: %w", err)
		}
	}
	if payloadLen > 0 {
		rec.Payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(r, rec.Payload); err != nil {
			return nil, fmt.Errorf("dari: read payload: %w", err)
		}
	}
	return rec, nil
}

var _ = errors.New
