package dariproto

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/fxamacker/cbor/v2"
)

// HelloMessage is the HELLO message (DARI §15.1).
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

// HelloAckMessage is the HELLO_ACK (DARI §15.2). Field 8 (ResourceLimits) MUST
// be present in the CBOR transcript even when empty so the relay's canonical
// ack encoding matches the connector's decoded view: the AUTH_PROOF verifies
// against SHA256("DARI-AUTH-v1\0" || canonical(HELLO) || canonical(HELLO_ACK)
// || …), and a missing field 8 silently changes the canonical bytes.
type HelloAckMessage struct {
	CoreVersion       uint8             `cbor:"1,keyasint"`
	ExtensionVersions map[string]uint8  `cbor:"2,keyasint"`
	CryptoProfile     string            `cbor:"3,keyasint"`
	ServerNonce       []byte            `cbor:"4,keyasint"`
	RelayCredential   []byte            `cbor:"5,keyasint"`
	AuthChallenge     []byte            `cbor:"6,keyasint"`
	MinHarnessVersion string            `cbor:"7,keyasint,omitempty"`
	ResourceLimits    map[string]uint64 `cbor:"8,keyasint,omitempty"`
}

// AuthChallengeMessage is the AUTH_CHALLENGE (DARI §18.1).
type AuthChallengeMessage struct {
	ServerNonce       []byte   `cbor:"1,keyasint"`
	ChallengeID       []byte   `cbor:"2,keyasint"`
	CredentialIssuers []string `cbor:"3,keyasint"`
	RevocationEpoch   uint64   `cbor:"4,keyasint"`
	AuthDeadlineMs    uint64   `cbor:"5,keyasint"`
}

// AuthProofMessage is the AUTH_PROOF (DARI §18.2).
type AuthProofMessage struct {
	Credential         []byte        `cbor:"1,keyasint"`
	Signature          []byte        `cbor:"2,keyasint"`
	KeyAlgorithm       COSEAlgorithm `cbor:"3,keyasint"`
	ChallengeID        []byte        `cbor:"4,keyasint"`
	RevocationEvidence []byte        `cbor:"5,keyasint,omitempty"`
}

// AIRequestPayload is the payload for AI_OPEN (DARI §10B).
// Uses JSON for the request body so it can carry OpenAI-compatible messages.
type AIRequestPayload struct {
	Model       string      `json:"model"`
	Messages    []AIMessage `json:"messages"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Temperature *float64    `json:"temperature,omitempty"`
	Tools       []AIToolDef `json:"tools,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
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

// AITokenChunkPayload is the streaming token chunk (DARI §10B.20).
type AITokenChunkPayload struct {
	Text         string `json:"text"`
	Done         bool   `json:"done"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// AICompletePayload is the completion message (DARI §10B.18).
type AICompletePayload struct {
	Content          string `json:"content"`
	FinishReason     string `json:"finish_reason"`
	InputTokens      int    `json:"input_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int    `json:"cache_write_tokens,omitempty"`
}

// MarshalCBOR wraps cbor.Marshal.
func MarshalCBOR(v interface{}) ([]byte, error) {
	return defaultEncMode.Marshal(v)
}

// defaultEncMode is the CANONICAL encoder (RFC 8949 core
// deterministic) — byte-identical to the root kernel's encoder, which
// is the whole point of the mirror: multi-key maps must not encode in
// Go map-iteration order.
var defaultEncMode = func() cbor.EncMode {
	mode, err := cbor.EncOptions{
		Sort:        cbor.SortCoreDeterministic,
		IndefLength: cbor.IndefLengthForbidden,
		Time:        cbor.TimeUnix,
	}.EncMode()
	if err != nil {
		panic("dariproto: init cbor encoder mode: " + err.Error())
	}
	return mode
}()

// UnmarshalCBOR wraps cbor.Unmarshal.
func UnmarshalCBOR(data []byte, v interface{}) error {
	return cbor.Unmarshal(data, v)
}

// canonicalKV / canonicalHello / canonicalAck are the deterministic
// (map-order-free) forms both peers hash into the AUTH transcript.
// They mirror the relay's paper.CanonicalHelloCBOR / CanonicalAckCBOR.
type canonicalKV struct {
	Key   string `cbor:"1,keyasint"`
	Value uint8  `cbor:"2,keyasint"`
}

type canonicalHello struct {
	CoreVersions          []uint8       `cbor:"1,keyasint"`
	PeerProfile           PeerProfile   `cbor:"2,keyasint"`
	TransportFeatures     []string      `cbor:"3,keyasint"`
	Extensions            []canonicalKV `cbor:"4,keyasint"`
	EncodingProfiles      []string      `cbor:"5,keyasint"`
	CryptoProfiles        []string      `cbor:"6,keyasint"`
	ClientNonce           []byte        `cbor:"7,keyasint"`
	CredentialHint        []byte        `cbor:"8,keyasint,omitempty"`
	ImplementationName    string        `cbor:"9,keyasint,omitempty"`
	ImplementationVersion string        `cbor:"10,keyasint,omitempty"`
}

type canonicalLimitKV struct {
	Key   string `cbor:"1,keyasint"`
	Value uint64 `cbor:"2,keyasint"`
}

type canonicalAckMsg struct {
	CoreVersion       uint8              `cbor:"1,keyasint"`
	ExtensionVersions []canonicalKV      `cbor:"2,keyasint"`
	CryptoProfile     string             `cbor:"3,keyasint"`
	ServerNonce       []byte             `cbor:"4,keyasint"`
	RelayCredential   []byte             `cbor:"5,keyasint"`
	AuthChallenge     []byte             `cbor:"6,keyasint"`
	MinHarnessVersion string             `cbor:"7,keyasint,omitempty"`
	ResourceLimits    []canonicalLimitKV `cbor:"8,keyasint,omitempty"`
}

func sortedKVPairs(m map[string]uint8) []canonicalKV {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]canonicalKV, 0, len(m))
	for _, k := range keys {
		out = append(out, canonicalKV{Key: k, Value: m[k]})
	}
	return out
}

func sortedLimitPairs(m map[string]uint64) []canonicalLimitKV {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]canonicalLimitKV, 0, len(m))
	for _, k := range keys {
		out = append(out, canonicalLimitKV{Key: k, Value: m[k]})
	}
	return out
}

// CanonicalHelloCBOR renders a HELLO into deterministic CBOR (maps as
// key-sorted arrays) for the AUTH transcript hash. Mirrors the relay's
// paper.CanonicalHelloCBOR byte-for-byte.
func CanonicalHelloCBOR(h *HelloMessage) ([]byte, error) {
	if h == nil {
		return nil, errors.New("dari: nil hello")
	}
	return MarshalCBOR(canonicalHello{
		CoreVersions:          h.CoreVersions,
		PeerProfile:           h.PeerProfile,
		TransportFeatures:     h.TransportFeatures,
		Extensions:            sortedKVPairs(h.Extensions),
		EncodingProfiles:      h.EncodingProfiles,
		CryptoProfiles:        h.CryptoProfiles,
		ClientNonce:           h.ClientNonce,
		CredentialHint:        h.CredentialHint,
		ImplementationName:    h.ImplementationName,
		ImplementationVersion: h.ImplementationVersion,
	})
}

// CanonicalAckCBOR renders a HELLO_ACK into deterministic CBOR.
// Mirrors the relay's paper.CanonicalAckCBOR byte-for-byte.
func CanonicalAckCBOR(a *HelloAckMessage) ([]byte, error) {
	if a == nil {
		return nil, errors.New("dari: nil ack")
	}
	return MarshalCBOR(canonicalAckMsg{
		CoreVersion:       a.CoreVersion,
		ExtensionVersions: sortedKVPairs(a.ExtensionVersions),
		CryptoProfile:     a.CryptoProfile,
		ServerNonce:       a.ServerNonce,
		RelayCredential:   a.RelayCredential,
		AuthChallenge:     a.AuthChallenge,
		MinHarnessVersion: a.MinHarnessVersion,
		ResourceLimits:    sortedLimitPairs(a.ResourceLimits),
	})
}
