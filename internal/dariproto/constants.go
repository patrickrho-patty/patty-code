package dariproto

// DARI protocol constants (DARI §8-9).
// Vendored from PCCP for harness-side DARI client support.

// The connection preface and the ALPN identifiers (canonical dari/1
// plus the legacy paper/1 fallback) live in legacy_paper1.go.

// VersionMajor is the DARI protocol major version.
const VersionMajor byte = 1

// MessageType identifies DARI messages.
type MessageType uint16

const (
	MsgHello    MessageType = 0x0001
	MsgHelloAck MessageType = 0x0002
	MsgPing     MessageType = 0x0003
	MsgPong     MessageType = 0x0004
	// Core allocation frozen by the deployed root registry (compat
	// map §6 rule 3 / §12): DRAIN is 0x0005 and CLOSE is 0x0006. The
	// connector previously carried CLOSE at 0x0005 — a renumbered
	// variant the root registry explicitly rejects — which made
	// relay-initiated CLOSE frames (0x0006) invisible to the reader.
	MsgDrain MessageType = 0x0005
	MsgClose MessageType = 0x0006

	// Sessions / capability leases (0x0200–0x02FF)
	MsgSessionOpen  MessageType = 0x0200
	MsgSessionGrant MessageType = 0x0201
	MsgSessionClose MessageType = 0x0202
	// Lease lifecycle (0x0210–0x0212) — mirrors the relay registry.
	MsgLeaseIssue  MessageType = 0x0210
	MsgLeaseRevoke MessageType = 0x0211
	MsgLeaseRenew  MessageType = 0x0212

	// Auth (0x0100–0x01FF)
	MsgAuthChallenge MessageType = 0x0100
	MsgAuthProof     MessageType = 0x0101
	MsgUserBind      MessageType = 0x0102
	MsgUserBindAck   MessageType = 0x0103
	MsgCapabilities  MessageType = 0x0104
	MsgAuthAck       MessageType = 0x0105

	// AI inference (0x0400–0x04FF)
	MsgAIOpen           MessageType = 0x0400
	MsgInferenceRequest MessageType = 0x0401
	MsgAITokenChunk     MessageType = 0x0402
	MsgAIComplete       MessageType = 0x0403

	// Model catalog (0x0D00–0x0DFF) — dari.model-supply/1 extension,
	// mirrors the relay's internal/paper/models.go registry.
	MsgCatalogRequest    MessageType = 0x0D00
	MsgCatalogSnapshot   MessageType = 0x0D01
	MsgCatalogDelta      MessageType = 0x0D02
	MsgModelAnnounce     MessageType = 0x0D03
	MsgModelWithdraw     MessageType = 0x0D04
	MsgModelDefault      MessageType = 0x0D05
	MsgModelAvailability MessageType = 0x0D06
	MsgModelCapChanged   MessageType = 0x0D07
	MsgModelUpgradeReq   MessageType = 0x0D08
	MsgCatalogAck        MessageType = 0x0D09
	// Policy-epoch push (0x0D10) — relay pushes the active epoch at
	// session setup and on policy change.
	MsgPolicyEpochPush MessageType = 0x0D10
	// DLP rule pack (0x0D11) — relay pushes the org's active DLP rules
	// so the connector's scanner enforces the same lexicon (C1.3).
	MsgDLPRulePack MessageType = 0x0D11
	// Governance state (0x0D12) — relay pushes the org's workflow
	// gates + tool registry + sandbox policies (C3/C4/D/E4).
	MsgGovernanceState MessageType = 0x0D12

	// Governance / evidence (0x0300–0x03FF) — mirrors the relay registry.
	MsgRelayVerdict       MessageType = 0x0304
	MsgEvidenceReceipt    MessageType = 0x0307
	MsgEvidenceReceiptAck MessageType = 0x0308

	// Provenance (0x0700–0x07FF) — mirrors the relay registry:
	// 0x0700 span, 0x0701 changeset, 0x0702 commit-bind, 0x0703 action.
	MsgProvenanceSpan       MessageType = 0x0700
	MsgProvenanceChangeSet  MessageType = 0x0701
	MsgProvenanceCommitBind MessageType = 0x0702
	MsgActionEnvelope       MessageType = 0x0703
	// ChangeSet NACK (0x0704) — relay refusal (e.g. active change freeze).
	MsgChangeSetNack MessageType = 0x0704

	// Admin / broadcast (0x0B00–0x0BFF) — mirrors the relay registry:
	// 0x0B00 broadcast, 0x0B01 admin directive, 0x0B02 result.
	MsgBroadcast          MessageType = 0x0B00
	MsgAdminCommand       MessageType = 0x0B01
	MsgAdminCommandResult MessageType = 0x0B02
	// Sovereign advisory (0x0B03) — relay-pushed offline advisory
	// (E3 air-gap mode). Rides its own type so broadcasts and
	// advisories stay distinguishable on the connector.
	MsgSovereignAdvisory MessageType = 0x0B03
	// Collab envelope (0x0B10) — relay-routed dari.collab/1 delivery
	// between org peers (member-encrypted; the relay cannot read it).
	MsgCollabEnvelope MessageType = 0x0B10
)

func (m MessageType) String() string {
	switch m {
	case MsgHello:
		return "HELLO"
	case MsgHelloAck:
		return "HELLO_ACK"
	case MsgAuthChallenge:
		return "AUTH_CHALLENGE"
	case MsgAuthProof:
		return "AUTH_PROOF"
	case MsgUserBind:
		return "USER_BIND"
	case MsgUserBindAck:
		return "USER_BIND_ACK"
	case MsgCapabilities:
		return "CAPABILITIES"
	case MsgAuthAck:
		return "AUTH_ACK"
	case MsgPing:
		return "PING"
	case MsgPong:
		return "PONG"
	case MsgDrain:
		return "DRAIN"
	case MsgClose:
		return "CLOSE"
	case MsgAIOpen:
		return "AI_OPEN"
	case MsgInferenceRequest:
		return "INFERENCE_REQUEST"
	case MsgAITokenChunk:
		return "AI_TOKEN_CHUNK"
	case MsgAIComplete:
		return "AI_COMPLETE"
	case MsgCatalogRequest:
		return "MODEL_CATALOG_REQUEST"
	case MsgCatalogSnapshot:
		return "MODEL_CATALOG_SNAPSHOT"
	case MsgCatalogDelta:
		return "MODEL_CATALOG_DELTA"
	case MsgModelAnnounce:
		return "MODEL_ANNOUNCE"
	case MsgModelWithdraw:
		return "MODEL_WITHDRAW"
	case MsgModelDefault:
		return "MODEL_DEFAULT_CHANGED"
	case MsgModelAvailability:
		return "MODEL_AVAILABILITY"
	case MsgModelCapChanged:
		return "MODEL_CAPABILITY_CHANGED"
	case MsgModelUpgradeReq:
		return "MODEL_UPGRADE_REQUIRED"
	case MsgCatalogAck:
		return "CATALOG_ACK"
	case MsgProvenanceChangeSet:
		return "PROVENANCE_CHANGESET"
	case MsgProvenanceSpan:
		return "PROVENANCE_SPAN"
	case MsgProvenanceCommitBind:
		return "PROVENANCE_COMMIT_BIND"
	case MsgEvidenceReceipt:
		return "EVIDENCE_RECEIPT"
	case MsgRelayVerdict:
		return "RELAY_VERDICT"
	case MsgEvidenceReceiptAck:
		return "EVIDENCE_RECEIPT_ACK"
	case MsgActionEnvelope:
		return "ACTION_ENVELOPE"
	case MsgBroadcast:
		return "BROADCAST"
	case MsgAdminCommand:
		return "ADMIN_DIRECTIVE"
	case MsgAdminCommandResult:
		return "ADMIN_COMMAND_RESULT"
	case MsgSovereignAdvisory:
		return "SOVEREIGN_ADVISORY"
	case MsgCollabEnvelope:
		return "COLLAB_ENVELOPE"
	case MsgChangeSetNack:
		return "CHANGESET_NACK"
	case MsgLeaseIssue:
		return "LEASE_ISSUE"
	case MsgLeaseRevoke:
		return "LEASE_REVOKE"
	case MsgLeaseRenew:
		return "LEASE_RENEW"
	case MsgPolicyEpochPush:
		return "POLICY_EPOCH"
	case MsgDLPRulePack:
		return "DLP_RULE_PACK"
	case MsgGovernanceState:
		return "GOVERNANCE_STATE"
	default:
		return "UNKNOWN"
	}
}

// RecordKind classifies a DARI record (DARI §9).
type RecordKind byte

const (
	KindControl RecordKind = 0
	KindMessage RecordKind = 1
	KindData    RecordKind = 2
	KindAck     RecordKind = 3
	KindReset   RecordKind = 4
	KindReceipt RecordKind = 5
	KindError   RecordKind = 6
	KindPing    RecordKind = 7
)

// Flags is the 16-bit DARI record flags bit field.
type Flags uint16

const (
	FlagCritical   Flags = 1 << 0
	FlagFinal      Flags = 1 << 1
	FlagEncrypted  Flags = 1 << 2
	FlagCompressed Flags = 1 << 3
)

// PeerProfile identifies the role of a DARI peer.
type PeerProfile string

const (
	ProfileHarness PeerProfile = "HARNESS"
	ProfileRelay   PeerProfile = "RELAY"
	ProfilePIA     PeerProfile = "INFERENCE"
)

// COSEAlgorithm identifies a COSE signing algorithm.
type COSEAlgorithm int

const (
	COSEAlgEdDSA COSEAlgorithm = -8
	COSEAlgES256 COSEAlgorithm = -7
)
