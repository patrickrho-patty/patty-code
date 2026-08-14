package paperproto

// PAPER protocol constants (PAPER §8-9).
// Vendored from PCCP for harness-side PAPER client support.

// PAPERPreface is the 8-byte connection preface for TLS/TCP binding (PAPER §8.2).
var PAPERPreface = []byte{0x50, 0x41, 0x50, 0x45, 0x52, 0x00, 0x01, 0x0A}

// ALPNProtocol is the ALPN identifier for PAPER.
const ALPNProtocol = "paper/1"

// VersionMajor is the PAPER protocol major version.
const VersionMajor byte = 1

// MessageType identifies PAPER messages.
type MessageType uint16

const (
	MsgHello         MessageType = 0x0001
	MsgHelloAck      MessageType = 0x0002
	MsgPing          MessageType = 0x0003
	MsgPong          MessageType = 0x0004
	MsgClose         MessageType = 0x0005

	// Sessions / capability leases (0x0200–0x02FF)
	MsgSessionOpen  MessageType = 0x0200
	MsgSessionGrant MessageType = 0x0201
	MsgSessionClose MessageType = 0x0202

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

	// Model catalog (0x0500–0x05FF) — paper.models/1 extension
	MsgCatalogRequest    MessageType = 0x0500
	MsgCatalogSnapshot   MessageType = 0x0501
	MsgCatalogDelta      MessageType = 0x0502
	MsgModelAnnounce     MessageType = 0x0503
	MsgModelWithdraw     MessageType = 0x0504
	MsgModelDefault      MessageType = 0x0505
	MsgModelAvailability MessageType = 0x0506
	MsgModelCapChanged   MessageType = 0x0507
	MsgModelUpgradeReq   MessageType = 0x0508
	MsgCatalogAck        MessageType = 0x0509
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
	default:
		return "UNKNOWN"
	}
}

// RecordKind classifies a PAPER record (PAPER §9).
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

// Flags is the 16-bit PAPER record flags bit field.
type Flags uint16

const (
	FlagCritical   Flags = 1 << 0
	FlagFinal      Flags = 1 << 1
	FlagEncrypted  Flags = 1 << 2
	FlagCompressed Flags = 1 << 3
)

// PeerProfile identifies the role of a PAPER peer.
type PeerProfile string

const (
	ProfileHarness  PeerProfile = "HARNESS"
	ProfileRelay    PeerProfile = "RELAY"
	ProfilePIA      PeerProfile = "INFERENCE"
)

// COSEAlgorithm identifies a COSE signing algorithm.
type COSEAlgorithm int

const (
	COSEAlgEdDSA COSEAlgorithm = -8
	COSEAlgES256 COSEAlgorithm = -7
)
