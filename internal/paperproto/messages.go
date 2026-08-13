package paperproto

import (
	"encoding/json"

	"github.com/fxamacker/cbor/v2"
)

// HelloMessage is the HELLO message (PAPER §15.1).
type HelloMessage struct {
	CoreVersions          []uint8          `cbor:"1,keyasint"`
	PeerProfile           PeerProfile      `cbor:"2,keyasint"`
	TransportFeatures     []string         `cbor:"3,keyasint"`
	Extensions            map[string]uint8 `cbor:"4,keyasint"`
	EncodingProfiles      []string         `cbor:"5,keyasint"`
	CryptoProfiles        []string         `cbor:"6,keyasint"`
	ClientNonce           []byte           `cbor:"7,keyasint"`
	CredentialHint        []byte           `cbor:"8,keyasint,omitempty"`
	ImplementationName    string           `cbor:"9,keyasint,omitempty"`
	ImplementationVersion string           `cbor:"10,keyasint,omitempty"`
}

// HelloAckMessage is the HELLO_ACK (PAPER §15.2).
type HelloAckMessage struct {
	CoreVersion       uint8            `cbor:"1,keyasint"`
	ExtensionVersions map[string]uint8 `cbor:"2,keyasint"`
	CryptoProfile     string           `cbor:"3,keyasint"`
	ServerNonce       []byte           `cbor:"4,keyasint"`
	RelayCredential   []byte           `cbor:"5,keyasint"`
	AuthChallenge     []byte           `cbor:"6,keyasint"`
	MinHarnessVersion string           `cbor:"7,keyasint,omitempty"`
}

// AuthChallengeMessage is the AUTH_CHALLENGE (PAPER §18.1).
type AuthChallengeMessage struct {
	ServerNonce       []byte   `cbor:"1,keyasint"`
	ChallengeID       []byte   `cbor:"2,keyasint"`
	CredentialIssuers []string `cbor:"3,keyasint"`
	RevocationEpoch   uint64   `cbor:"4,keyasint"`
	AuthDeadlineMs    uint64   `cbor:"5,keyasint"`
}

// AuthProofMessage is the AUTH_PROOF (PAPER §18.2).
type AuthProofMessage struct {
	Credential   []byte        `cbor:"1,keyasint"`
	Signature    []byte        `cbor:"2,keyasint"`
	KeyAlgorithm COSEAlgorithm `cbor:"3,keyasint"`
	ChallengeID  []byte        `cbor:"4,keyasint"`
	RevocationEvidence []byte  `cbor:"5,keyasint,omitempty"`
}

// AIRequestPayload is the payload for AI_OPEN (PAPER §10B).
// Uses JSON for the request body so it can carry OpenAI-compatible messages.
type AIRequestPayload struct {
	Model       string        `json:"model"`
	Messages    []AIMessage   `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	Tools       []AIToolDef   `json:"tools,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIToolDef struct {
	Type     string         `json:"type"`
	Function AIToolFunction `json:"function"`
}

type AIToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// AITokenChunkPayload is the streaming token chunk (PAPER §10B.20).
type AITokenChunkPayload struct {
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// AICompletePayload is the completion message (PAPER §10B.18).
type AICompletePayload struct {
	Content       string `json:"content"`
	FinishReason  string `json:"finish_reason"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	TotalTokens   int    `json:"total_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

// MarshalCBOR wraps cbor.Marshal.
func MarshalCBOR(v interface{}) ([]byte, error) {
	return cbor.Marshal(v)
}

// UnmarshalCBOR wraps cbor.Unmarshal.
func UnmarshalCBOR(data []byte, v interface{}) error {
	return cbor.Unmarshal(data, v)
}
