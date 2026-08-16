package dariproto

import "time"

// §53 session resumption (mirrors the kernel types).

type SessionResumptionRequest struct {
	WorkingSessionID    string   `cbor:"1,keyasint"`
	ResumptionToken     []byte   `cbor:"2,keyasint"`
	LastAckLaneSeq      uint64   `cbor:"3,keyasint"`
	LastEvidenceReceipt []byte   `cbor:"4,keyasint,omitempty"`
	ActiveExchangeIDs   []string `cbor:"5,keyasint,omitempty"`
}

type SessionResumptionResponse struct {
	Granted             bool     `cbor:"1,keyasint"`
	Reason              string   `cbor:"2,keyasint,omitempty"`
	ResumedLaneIDs      []uint64 `cbor:"3,keyasint,omitempty"`
	ResumedFromSeq      uint64   `cbor:"4,keyasint,omitempty"`
	NewLeaseID          string   `cbor:"5,keyasint,omitempty"`
	RequiresFullRestart bool     `cbor:"6,keyasint,omitempty"`
}

// ResumptionCredential is the client-held resume token.
type ResumptionCredential struct {
	SessionID string    `json:"session_id"`
	Token     []byte    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	HarnessID string    `json:"harness_id"`
}
