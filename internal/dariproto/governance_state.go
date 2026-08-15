package dariproto

import (
	"errors"
	"fmt"
)

// governance_state.go is the connector-side decode of the relay's
// governance-state snapshot (harness plans C3/C4/D1/D3-D6/E4). The
// wire structs mirror the relay's internal/relay/governance_state.go
// field-for-field; the cross-repo conformance suite pins the bytes.
//
// The decoded snapshot converts into the connector's governed.State
// clients (workflow.GatesClient, approvals.Registry,
// sandbox.PolicyStore) so the executor gates fire on real tool calls.

// GovernanceStateWire is the snapshot body (mirrors the relay).
type GovernanceStateWire struct {
	Version    uint16                   `cbor:"1,keyasint"`
	OrgID      string                   `cbor:"2,keyasint"`
	RepoID     string                   `cbor:"3,keyasint,omitempty"`
	ModelID    string                   `cbor:"4,keyasint,omitempty"`
	Freeze     *GovernanceFreezeWire    `cbor:"5,keyasint,omitempty"`
	Recalls    []GovernanceRecallWire   `cbor:"6,keyasint,omitempty"`
	Acks       []GovernanceAckWire      `cbor:"7,keyasint,omitempty"`
	VersionReq *GovernanceVersionWire   `cbor:"8,keyasint,omitempty"`
	Standards  []GovernanceStandardWire `cbor:"9,keyasint,omitempty"`
	Tools      []GovernanceToolWire     `cbor:"10,keyasint,omitempty"`
	Sandboxes  []GovernanceSandboxWire  `cbor:"11,keyasint,omitempty"`
}

// GovernanceFreezeWire mirrors the relay's freeze struct.
type GovernanceFreezeWire struct {
	Reason         string   `cbor:"1,keyasint"`
	ReasonKo       string   `cbor:"2,keyasint"`
	AffectedRepos  []string `cbor:"3,keyasint"`
	AllowedActions []string `cbor:"4,keyasint"`
	NotAfterMs     int64    `cbor:"5,keyasint"`
}

// GovernanceRecallWire mirrors the relay's recall struct.
type GovernanceRecallWire struct {
	Model       string `cbor:"1,keyasint"`
	Reason      string `cbor:"2,keyasint"`
	Replacement string `cbor:"3,keyasint"`
}

// GovernanceAckWire mirrors the relay's ack struct.
type GovernanceAckWire struct {
	PolicyEpochID string `cbor:"1,keyasint"`
	SummaryKo     string `cbor:"2,keyasint"`
	Blocking      bool   `cbor:"3,keyasint"`
}

// GovernanceVersionWire mirrors the relay's version struct.
type GovernanceVersionWire struct {
	MinVersion string `cbor:"1,keyasint"`
	Ring       string `cbor:"2,keyasint"`
}

// GovernanceStandardWire mirrors the relay's standard struct.
type GovernanceStandardWire struct {
	RuleID        string `cbor:"1,keyasint"`
	BlockPattern  string `cbor:"2,keyasint"`
	Description   string `cbor:"3,keyasint"`
	DescriptionKo string `cbor:"4,keyasint"`
}

// GovernanceToolWire mirrors the relay's tool status struct.
type GovernanceToolWire struct {
	ToolID string `cbor:"1,keyasint"`
	Status string `cbor:"2,keyasint"`
}

// GovernanceSandboxWire mirrors the relay's sandbox struct.
type GovernanceSandboxWire struct {
	RepositoryID string `cbor:"1,keyasint"`
	Mode         string `cbor:"2,keyasint"`
	RiskClass    string `cbor:"3,keyasint"`
}

// DecodeGovernanceState parses a GOVERNANCE_STATE body.
func DecodeGovernanceState(data []byte) (*GovernanceStateWire, error) {
	if len(data) == 0 {
		return nil, errors.New("dari: empty governance-state body")
	}
	var snap GovernanceStateWire
	if err := UnmarshalCBOR(data, &snap); err != nil {
		return nil, fmt.Errorf("dari: decode governance state: %w", err)
	}
	if snap.OrgID == "" {
		return nil, errors.New("dari: governance state missing org binding")
	}
	return &snap, nil
}
