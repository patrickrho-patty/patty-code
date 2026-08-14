// Package paper implements the PAPER protocol provider for Patty Code.
// It connects to a PCCP PAPER Relay and sends inference requests as
// PAPER AI_OPEN records (§9.2, §38.1), receiving AI_TOKEN_CHUNK streaming.
//
// This replaces the OpenAI/Anthropic-compatible HTTP path for Patty service.
// The harness sends only Catalog Model ID + epoch, never a base URL or API key.
package paper

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"patty/internal/paperproto"
	"patty/internal/provider"
)

func init() {
	provider.Register("paper", New)
}

// Provider implements provider.Provider via the PAPER protocol.
type Provider struct {
	name          string
	model         string
	relayAddr     string
	harnessID     string
	identity      *paperproto.Identity
	credPath      string
	keyPath       string
	tlsConfig     *tls.Config
	mu            sync.Mutex
	conn          *paperproto.TransportConn
	authenticated bool
}

// New builds a PAPER provider from a resolved config.
func New(cfg provider.Config) (provider.Provider, error) {
	// relayAddr comes from BaseURL in config (e.g. "relay.example.com:8444")
	relayAddr := cfg.BaseURL
	if relayAddr == "" {
		return nil, fmt.Errorf("paper: relay address (base_url) is required for provider %q", cfg.Name)
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("paper: model (Catalog Model ID) is required for provider %q", cfg.Name)
	}

	credPath := envOr("PAPER_HARNESS_CREDENTIAL_FILE", "")
	keyPath := envOr("PAPER_HARNESS_KEY_FILE", "")
	harnessID := os.Getenv("PCCP_HARNESS_ID")

	// Eager-load the identity so configuration errors surface during
	// `patcode setup` instead of on the first inference request. The auth
	// path never falls back to placeholder credentials.
	var identity *paperproto.Identity
	if credPath != "" && keyPath != "" {
		loaded, err := paperproto.LoadIdentityFromDisk(credPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("paper: load harness identity: %w", err)
		}
		identity = loaded
	} else if credPath != "" || keyPath != "" {
		return nil, errors.New("paper: PAPER_HARNESS_CREDENTIAL_FILE and PAPER_HARNESS_KEY_FILE must both be set")
	}

	return &Provider{
		name:      cfg.Name,
		model:     cfg.Model,
		relayAddr: relayAddr,
		harnessID: harnessID,
		identity:  identity,
		credPath:  credPath,
		keyPath:   keyPath,
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true, // dev: PCCP uses self-signed certs
			MinVersion:         tls.VersionTLS13,
			NextProtos:         []string{paperproto.ALPNProtocol},
		},
	}, nil
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

// Name returns the provider instance name.
func (p *Provider) Name() string { return p.name }

// connect establishes a PAPER connection to the Relay and performs handshake/auth.
func (p *Provider) connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		// Test connection with ping
		if err := p.conn.SendControl(paperproto.MsgPing, nil, []byte("ping")); err != nil {
			p.conn.Close()
			p.conn = nil
		} else {
			return nil
		}
	}

	slog.Info("paper: connecting to relay", "addr", p.relayAddr, "model", p.model)

	// Dial PAPER Relay
	conn, err := paperproto.DialTCP(p.relayAddr, p.tlsConfig, paperproto.DefaultTransportConfig())
	if err != nil {
		return fmt.Errorf("paper: dial relay %s: %w", p.relayAddr, err)
	}

	// PAPER HELLO handshake
	hello := &paperproto.HelloMessage{
		CoreVersions:          []uint8{1},
		PeerProfile:           paperproto.ProfileHarness,
		TransportFeatures:     []string{"tcp-tls"},
		Extensions:            map[string]uint8{"paper.ai/1": 1, "paper.models/1": 1},
		EncodingProfiles:      []string{"cbor", "json"},
		CryptoProfiles:        []string{"PAPER-BASE-1"},
		ClientNonce:           make([]byte, 32),
		ImplementationName:    "patty-code",
		ImplementationVersion: "v2-paper",
	}

	helloBytes, err := paperproto.MarshalCBOR(hello)
	if err != nil {
		conn.Close()
		return fmt.Errorf("paper: marshal HELLO: %w", err)
	}

	if err := conn.SendControl(paperproto.MsgHello, nil, helloBytes); err != nil {
		conn.Close()
		return fmt.Errorf("paper: send HELLO: %w", err)
	}

	// Receive HELLO_ACK
	rec, err := conn.RecvRecord()
	if err != nil {
		conn.Close()
		return fmt.Errorf("paper: recv HELLO_ACK: %w", err)
	}

	if rec.Kind != paperproto.KindControl || paperproto.MessageType(rec.MessageType) != paperproto.MsgHelloAck {
		conn.Close()
		return fmt.Errorf("paper: expected HELLO_ACK, got kind=%d msg=%d", rec.Kind, rec.MessageType)
	}

	var ack paperproto.HelloAckMessage
	if err := paperproto.UnmarshalCBOR(rec.Payload, &ack); err != nil {
		conn.Close()
		return fmt.Errorf("paper: decode HELLO_ACK: %w", err)
	}

	slog.Info("paper: HELLO_ACK received",
		"core_version", ack.CoreVersion,
		"extensions", ack.ExtensionVersions,
		"min_harness_version", ack.MinHarnessVersion)

	// Receive AUTH_CHALLENGE
	rec, err = conn.RecvRecord()
	if err != nil {
		conn.Close()
		return fmt.Errorf("paper: recv AUTH_CHALLENGE: %w", err)
	}

	var challenge paperproto.AuthChallengeMessage
	if err := paperproto.UnmarshalCBOR(rec.Payload, &challenge); err != nil {
		conn.Close()
		return fmt.Errorf("paper: decode AUTH_CHALLENGE: %w", err)
	}

	// Send AUTH_PROOF with the enrolled COSE-Sign1 peer credential and a
	// transcript-bound Ed25519 signature. The relay independently computes
	// the same auth context and verifies the proof under the issuer-defined
	// subject public key embedded in the credential body.
	if p.identity == nil {
		conn.Close()
		return fmt.Errorf("paper: harness identity is not enrolled; set PAPER_HARNESS_CREDENTIAL_FILE and PAPER_HARNESS_KEY_FILE, then run `patcode setup`")
	}
	proof, err := paperproto.BuildAuthProof(paperproto.AuthProofInput{
		PrivateKey:      p.identity.PrivateKey,
		Credential:      p.identity.Credential,
		Hello:           hello,
		HelloAck:        &ack,
		ChallengeID:     challenge.ChallengeID,
		RevocationEpoch: challenge.RevocationEpoch,
		ChannelBinding:  []byte("tcp-exporter"),
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("paper: build AUTH_PROOF: %w", err)
	}

	proofBytes, err := paperproto.MarshalCBOR(proof)
	if err != nil {
		conn.Close()
		return fmt.Errorf("paper: marshal AUTH_PROOF: %w", err)
	}

	if err := conn.SendControl(paperproto.MsgAuthProof, nil, proofBytes); err != nil {
		conn.Close()
		return fmt.Errorf("paper: send AUTH_PROOF: %w", err)
	}

	// Receive AUTH_ACK
	rec, err = conn.RecvRecord()
	if err != nil {
		conn.Close()
		return fmt.Errorf("paper: recv AUTH_ACK: %w", err)
	}

	if paperproto.MessageType(rec.MessageType) == paperproto.MsgClose {
		conn.Close()
		return fmt.Errorf("paper: authentication rejected")
	}

	p.conn = conn
	p.authenticated = true
	slog.Info("paper: authenticated with relay", "addr", p.relayAddr)

	return nil
}

// Stream starts a PAPER streaming completion.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	// Ensure connected
	if err := p.connect(ctx); err != nil {
		return nil, err
	}

	// Build AI request payload
	messages := make([]paperproto.AIMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = paperproto.AIMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	payload := paperproto.AIRequestPayload{
		Model:     p.model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	if req.Temperature != nil {
		payload.Temperature = req.Temperature
	}

	// Convert tools
	for _, t := range req.Tools {
		payload.Tools = append(payload.Tools, paperproto.AIToolDef{
			Type: "function",
			Function: paperproto.AIToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("paper: marshal request: %w", err)
	}

	// Send AI_OPEN
	if err := p.conn.SendMessage(paperproto.MsgAIOpen, nil, payloadBytes, 1, 1); err != nil {
		p.conn.Close()
		p.conn = nil
		return nil, fmt.Errorf("paper: send AI_OPEN: %w", err)
	}

	// Create output channel
	out := make(chan provider.Chunk, 32)

	// Receive streaming response
	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				out <- provider.Chunk{Type: provider.ChunkError, Err: ctx.Err()}
				return
			default:
			}

			rec, err := p.conn.RecvRecord()
			if err != nil {
				// Connection broken — report as stream interrupt
				out <- provider.Chunk{
					Type: provider.ChunkError,
					Err:  provider.StreamInterrupt(fmt.Errorf("paper: connection lost: %w", err), provider.StreamInterruptConnectionReset),
				}
				p.mu.Lock()
				if p.conn != nil {
					p.conn.Close()
					p.conn = nil
				}
				p.mu.Unlock()
				return
			}

			msgType := paperproto.MessageType(rec.MessageType)

			switch msgType {
			case paperproto.MsgAITokenChunk:
				var chunk paperproto.AITokenChunkPayload
				if err := json.Unmarshal(rec.Payload, &chunk); err != nil {
					continue
				}
				if chunk.Text != "" {
					select {
					case out <- provider.Chunk{Type: provider.ChunkText, Text: chunk.Text}:
					case <-ctx.Done():
						return
					}
				}

			case paperproto.MsgAIComplete:
				var result paperproto.AICompletePayload
				if err := json.Unmarshal(rec.Payload, &result); err != nil {
					out <- provider.Chunk{Type: provider.ChunkError, Err: fmt.Errorf("paper: decode AI_COMPLETE: %w", err)}
					return
				}

				// If content was sent directly (non-streaming), emit as text
				if result.Content != "" {
					select {
					case out <- provider.Chunk{Type: provider.ChunkText, Text: result.Content}:
					case <-ctx.Done():
						return
					}
				}

				// Send usage chunk
				out <- provider.Chunk{
					Type: provider.ChunkUsage,
					Usage: &provider.Usage{
						PromptTokens:     result.InputTokens,
						CompletionTokens: result.OutputTokens,
						TotalTokens:      result.TotalTokens,
						CacheHitTokens:   result.CacheReadTokens,
						CacheWriteTokens: result.CacheWriteTokens,
						FinishReason:     normalizeFinishReason(result.FinishReason),
					},
				}

				out <- provider.Chunk{Type: provider.ChunkDone}
				return

			case paperproto.MsgPing:
				p.conn.SendControl(paperproto.MsgPong, nil, []byte("pong"))

			case paperproto.MsgClose:
				var errMsg map[string]string
				json.Unmarshal(rec.Payload, &errMsg)
				out <- provider.Chunk{
					Type: provider.ChunkError,
					Err:  fmt.Errorf("paper: relay closed: %s", errMsg["error"]),
				}
				p.mu.Lock()
				if p.conn != nil {
					p.conn.Close()
					p.conn = nil
				}
				p.mu.Unlock()
				return
			}
		}
	}()

	return out, nil
}

// normalizeFinishReason maps PAPER finish reasons to provider.Chunk conventions.
func normalizeFinishReason(reason string) string {
	switch reason {
	case "stop", "":
		return "stop"
	case "tool_calls", "tools":
		return "tool_calls"
	case "length":
		return "length"
	case "content_filter":
		return "content_filter"
	default:
		return reason
	}
}

// Ensure net import used (for DialTCP in paperproto).
var _ = net.Dial
var _ time.Duration
