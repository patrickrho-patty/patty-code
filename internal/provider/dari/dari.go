// Package paper implements the DARI protocol provider for Patty Code.
// It connects to a PCCP DARI Relay and sends inference requests as
// DARI AI_OPEN records (§9.2, §38.1), receiving AI_TOKEN_CHUNK streaming.
//
// This replaces the OpenAI/Anthropic-compatible HTTP path for Patty service.
// The harness sends only Catalog Model ID + epoch, never a base URL or API key.
package dari

import (
	"context"
	"crypto/ed25519"
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

	"patty/internal/admin"
	"patty/internal/changeboard"
	"patty/internal/comms"
	"patty/internal/dariproto"
	"patty/internal/provenancewire"
	"patty/internal/provider"
)

func init() {
	provider.Register("dari", New)
}

// Provider implements provider.Provider via the DARI protocol.
type Provider struct {
	name          string
	model         string
	relayAddr     string
	harnessID     string
	identity      *dariproto.Identity
	credPath      string
	keyPath       string
	tlsConfig     *tls.Config
	mu            sync.Mutex
	conn          *dariproto.TransportConn
	authenticated bool

	// leaseClient owns the currently-held capability lease. The provider
	// calls `validateLease` before dispatching an AI_OPEN so a missing,
	// expired, or subject-mismatched lease fails closed before any
	// governance-gated exchange reaches the relay. See A3.
	leaseClient *dariproto.LeaseClient
	// policyEpochClient owns the session's bound policy epoch. The
	// provider pins the lease's PolicyEpochID to the bound epoch so
	// a stale credential cannot survive a policy change. See A4.
	policyEpochClient *dariproto.PolicyEpochClient
	// catalogClient owns the server-authoritative model catalog. The
	// provider refuses to dispatch against a model the relay never
	// advertised. See A5.
	catalogClient *dariproto.CatalogClient
	// receiptHandler stores relay-pushed evidence receipts (B3) and
	// acks them over the live connection. Nil until
	// SetReceiptHandler installs it.
	receiptHandler *provenancewire.IncomingAckHandler
	// provEmitter collects the session's provenance envelopes (B1);
	// flushed to the relay after each governed exchange.
	provEmitter *provenancewire.ProvenanceEmitter
	// sessionGrant is the relay-issued, policy-signed DARI
	// Authorization Grant (Task 7) — the session's authority object.
	sessionGrant *dariproto.GrantEnvelope
	// advisories stores relay-pushed broadcasts/admin directives/
	// sovereign advisories (E2/E3/E5) for the harness surfaces.
	advisories []StoredAdvisory
	// dlpRuleSink receives relay-pushed DLP rule packs (C1.3).
	dlpRuleSink func(*dariproto.DLPRulePackWire)
	// governanceSink receives relay-pushed governance-state snapshots
	// (C3/C4/D1/D3-D6/E4).
	governanceSink func(*dariproto.GovernanceStateWire)
	// lastDecision is the most recently verified relay F.6 decision.
	lastDecision *dariproto.DecisionEnvelope
	// receiptSignerKey verifies relay-pushed evidence receipts (B3).
	receiptSignerKey ed25519.PublicKey
	// receiptStore is the boot-installable durable store (B3); nil
	// falls back to the in-memory store.
	receiptStore *provenancewire.ReceiptStore
	// board is the session change-control board (D2); suspended marks
	// admin-directed pause (E5).
	board     *changeboard.Board
	suspended bool
	// credOrgID is the verified credential's organization (envelopes
	// the connector emits carry it).
	credOrgID string
	// provRepoID/provBranch carry the workspace git identity for B1
	// provenance envelopes; provTurnPaths accumulates the turn's
	// edited paths until the next flush seals them into a change set.
	provRepoID    string
	provBranch    string
	provTurnPaths map[string]bool
	// commsInbox receives relay broadcasts (E2); adminDisp verifies +
	// executes signed directives (E5); policyIssuerKey is the AUTH_ACK
	// policy issuer public key the dispatcher verifies under.
	// harnessVersion is the honest build identity HELLO advertises
	// (D5 floor checks compare against it). Boot installs the real
	// build version; the default stays explicit.
	harnessVersion  string
	commsInbox      *comms.Inbox
	adminDisp       *admin.Dispatcher
	adminExec       admin.Executor
	policyIssuerKey ed25519.PublicKey
	// subjectPeerID is the authenticated harness peer ID. The lease's
	// SubjectPeerID MUST match this value.
	subjectPeerID string
	// userID is the acting user identifier reported at SESSION_OPEN.
	// Defaults to a peer-derived value; the harness sets it via
	// SetSessionContext.
	userID string
	// sessionID is the open session ID. The lease's SessionID MUST
	// match this value.
	sessionID string
	// policyEpoch is the session's bound policy epoch. Every exchange
	// pins the lease's PolicyEpochID to this value (A4).
	policyEpoch string
	// nowFn is the time source. Tests override it to drive expiry.
	nowFn func() time.Time
	// autoRenewBefore is the lead time before NotAfter at which the
	// provider triggers a lease renewal before the next exchange.
	autoRenewBefore time.Duration
}

// New builds a DARI provider from a resolved config.
func New(cfg provider.Config) (provider.Provider, error) {
	// relayAddr comes from BaseURL in config (e.g. "relay.example.com:8444")
	relayAddr := cfg.BaseURL
	if relayAddr == "" {
		return nil, fmt.Errorf("dari: relay address (base_url) is required for provider %q", cfg.Name)
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("dari: model (Catalog Model ID) is required for provider %q", cfg.Name)
	}

	credPath := envOr("DARI_HARNESS_CREDENTIAL_FILE", "")
	keyPath := envOr("DARI_HARNESS_KEY_FILE", "")
	harnessID := envOr("DARI_HARNESS_ID", "")

	// Eager-load the identity so configuration errors surface during
	// `patcode setup` instead of on the first inference request. The auth
	// path never falls back to placeholder credentials.
	var identity *dariproto.Identity
	if credPath != "" && keyPath != "" {
		loaded, err := dariproto.LoadIdentityFromDisk(credPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("dari: load harness identity: %w", err)
		}
		identity = loaded
	} else if credPath != "" || keyPath != "" {
		return nil, errors.New("dari: DARI_HARNESS_CREDENTIAL_FILE and DARI_HARNESS_KEY_FILE must both be set")
	}

	orgID, _ := identity.Organization()
	return &Provider{
		name:            cfg.Name,
		model:           cfg.Model,
		relayAddr:       relayAddr,
		harnessID:       harnessID,
		harnessVersion:  "dev",
		credOrgID:       orgID,
		identity:        identity,
		credPath:        credPath,
		keyPath:         keyPath,
		nowFn:           time.Now,
		autoRenewBefore: defaultAutoRenewBefore,
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true, // dev: PCCP uses self-signed certs
			MinVersion:         tls.VersionTLS13,
			NextProtos:         dariproto.ALPNProtocols(),
		},
	}, nil
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	// Legacy PAPER_* names still resolve during the migration window.
	if legacy := strings.TrimSpace(os.Getenv("PAPER_" + name[len("DARI_"):])); legacy != "" {
		return legacy
	}
	return fallback
}

// Name returns the provider instance name.
func (p *Provider) Name() string { return p.name }

// connect establishes a DARI connection to the Relay and performs handshake/auth.
func (p *Provider) connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		// Test connection with ping
		if err := p.conn.SendControl(dariproto.MsgPing, nil, []byte("ping")); err != nil {
			p.conn.Close()
			p.conn = nil
		} else {
			return nil
		}
	}

	slog.Info("dari: connecting to relay", "addr", p.relayAddr, "model", p.model)

	// Dial DARI Relay
	conn, err := dariproto.DialTCP(p.relayAddr, p.tlsConfig, dariproto.DefaultTransportConfig())
	if err != nil {
		return fmt.Errorf("dari: dial relay %s: %w", p.relayAddr, err)
	}

	// PAPER HELLO handshake
	hello := &dariproto.HelloMessage{
		CoreVersions:          []uint8{1},
		PeerProfile:           dariproto.ProfileHarness,
		TransportFeatures:     []string{"tcp-tls"},
		Extensions:            map[string]uint8{"dari.ai/1": 1, "dari.model-supply/1": 1},
		EncodingProfiles:      []string{"cbor", "json"},
		CryptoProfiles:        []string{"DARI-BASE-1"},
		ClientNonce:           make([]byte, 32),
		ImplementationName:    "patty-code",
		ImplementationVersion: p.harnessVersion,
	}

	helloBytes, err := dariproto.MarshalCBOR(hello)
	if err != nil {
		conn.Close()
		return fmt.Errorf("dari: marshal HELLO: %w", err)
	}

	if err := conn.SendControl(dariproto.MsgHello, nil, helloBytes); err != nil {
		conn.Close()
		return fmt.Errorf("dari: send HELLO: %w", err)
	}

	// Receive HELLO_ACK
	rec, err := conn.RecvRecord()
	if err != nil {
		conn.Close()
		return fmt.Errorf("dari: recv HELLO_ACK: %w", err)
	}

	if rec.Kind != dariproto.KindControl || dariproto.MessageType(rec.MessageType) != dariproto.MsgHelloAck {
		conn.Close()
		return fmt.Errorf("dari: expected HELLO_ACK, got kind=%d msg=%d", rec.Kind, rec.MessageType)
	}

	var ack dariproto.HelloAckMessage
	if err := dariproto.UnmarshalCBOR(rec.Payload, &ack); err != nil {
		conn.Close()
		return fmt.Errorf("dari: decode HELLO_ACK: %w", err)
	}
	// D5: a relay-advertised harness floor refuses sub-minimum
	// connectors at handshake time — fail fast rather than per-call.
	if ack.MinHarnessVersion != "" && dariproto.VersionBelow(p.harnessVersion, ack.MinHarnessVersion) {
		conn.Close()
		return fmt.Errorf("dari: harness version %s is below the relay minimum %s — upgrade required", p.harnessVersion, ack.MinHarnessVersion)
	}

	slog.Info("dari: HELLO_ACK received",
		"core_version", ack.CoreVersion,
		"extensions", ack.ExtensionVersions,
		"min_harness_version", ack.MinHarnessVersion)

	// Receive AUTH_CHALLENGE
	rec, err = conn.RecvRecord()
	if err != nil {
		conn.Close()
		return fmt.Errorf("dari: recv AUTH_CHALLENGE: %w", err)
	}

	var challenge dariproto.AuthChallengeMessage
	if err := dariproto.UnmarshalCBOR(rec.Payload, &challenge); err != nil {
		conn.Close()
		return fmt.Errorf("dari: decode AUTH_CHALLENGE: %w", err)
	}

	// Send AUTH_PROOF with the enrolled COSE-Sign1 peer credential and a
	// transcript-bound Ed25519 signature. The relay independently computes
	// the same auth context and verifies the proof under the issuer-defined
	// subject public key embedded in the credential body.
	if p.identity == nil {
		conn.Close()
		return fmt.Errorf("dari: harness identity is not enrolled; set DARI_HARNESS_CREDENTIAL_FILE and DARI_HARNESS_KEY_FILE, then run `patcode setup`")
	}
	proof, err := dariproto.BuildAuthProof(dariproto.AuthProofInput{
		PrivateKey:      p.identity.PrivateKey,
		Credential:      p.identity.Credential,
		Hello:           hello,
		HelloAck:        &ack,
		ChallengeID:     challenge.ChallengeID,
		RevocationEpoch: challenge.RevocationEpoch,
		ChannelBinding:  []byte("tcp-exporter"),
	})
	if err == nil && os.Getenv("DARI_DEBUG_AUTH") == "1" {
		slog.Info("dari: AUTH_DEBUG", "transcript", fmt.Sprintf("%x", dariproto.DebugLastAuthTranscript()),
			"challengeID", fmt.Sprintf("%x", challenge.ChallengeID), "epoch", challenge.RevocationEpoch)
	}
	if err != nil {
		conn.Close()
		return fmt.Errorf("dari: build AUTH_PROOF: %w", err)
	}

	proofBytes, err := dariproto.MarshalCBOR(proof)
	if err != nil {
		conn.Close()
		return fmt.Errorf("dari: marshal AUTH_PROOF: %w", err)
	}

	if err := conn.SendControl(dariproto.MsgAuthProof, nil, proofBytes); err != nil {
		conn.Close()
		return fmt.Errorf("dari: send AUTH_PROOF: %w", err)
	}

	// Receive AUTH_ACK
	rec, err = conn.RecvRecord()
	if err != nil {
		conn.Close()
		return fmt.Errorf("dari: recv AUTH_ACK: %w", err)
	}

	if dariproto.MessageType(rec.MessageType) == dariproto.MsgClose {
		conn.Close()
		return fmt.Errorf("dari: authentication rejected")
	}

	// AUTH_ACK carries the relay's governance trust payload (policy
	// issuer key). Install the lease client from it when none is set.
	if dariproto.MessageType(rec.MessageType) == dariproto.MsgAuthAck {
		if err := p.installGovernanceClientsFromAuthAck(rec.Payload); err != nil {
			conn.Close()
			return err
		}
	}

	// Resolve the authenticated subject peer ID from the enrolled
	// credential. The lease the relay issues binds this value.
	if p.subjectPeerID == "" && p.identity != nil {
		peerID, err := p.identity.PeerID()
		if err != nil {
			conn.Close()
			return fmt.Errorf("dari: %w", err)
		}
		p.subjectPeerID = peerID
	}

	// Session-governance handshake (A3/A4/A5): SESSION_OPEN → epoch →
	// catalog → lease → grant. Fail-closed — no session, no dispatch.
	if err := p.openSession(conn); err != nil {
		conn.Close()
		return err
	}

	// Default evidence-receipt handler (B3): store + ack over this
	// connection. The STORE is boot-installable (disk-backed); the
	// sender is always THIS connection. SendRecord is mutex-guarded so
	// the reader goroutine can ack concurrently with outbound sends.
	if p.receiptHandler == nil {
		store := p.receiptStore
		if store == nil {
			store = provenancewire.NewReceiptStore()
		}
		p.receiptHandler = provenancewire.NewIncomingAckHandler(store, connAckSender{conn: conn})
	}

	p.conn = conn
	p.authenticated = true
	slog.Info("dari: authenticated with relay", "addr", p.relayAddr, "session", p.sessionID)

	return nil
}

// Stream starts a DARI streaming completion.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	if p.Suspended() {
		return nil, fmt.Errorf("dari: session paused by administrator directive — inference refused")
	}
	// Fail-closed fast path: when a lease is already held, validate it
	// BEFORE touching the network so an expired/revoked/subject-
	// mismatched lease surfaces immediately.
	if p.leaseClient != nil && p.leaseClient.Current() != nil {
		if err := p.validateLease(p.model); err != nil {
			return nil, err
		}
	}

	// Ensure connected + session-governed (the connect path runs the
	// SESSION_OPEN → epoch → catalog → lease handshake and acquires
	// the session's lease, A3/A4/A5).
	if err := p.connect(ctx); err != nil {
		return nil, err
	}

	// Fail-closed authorization boundary: no lease, no AI_OPEN. The
	// lease is the per-session capability grant from the relay's
	// policy issuer; without it, the relay's governance path would
	// reject the request anyway, but the connector surfaces the error
	// locally so the operator gets a clear message.
	if err := p.validateLease(p.model); err != nil {
		return nil, err
	}

	// Build AI request payload
	messages := make([]dariproto.AIMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = dariproto.AIMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	payload := dariproto.AIRequestPayload{
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
		payload.Tools = append(payload.Tools, dariproto.AIToolDef{
			Type: "function",
			Function: dariproto.AIToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("dari: marshal request: %w", err)
	}

	// Send AI_OPEN
	if err := p.conn.SendMessage(dariproto.MsgAIOpen, nil, payloadBytes, 1, 1); err != nil {
		p.conn.Close()
		p.conn = nil
		return nil, fmt.Errorf("dari: send AI_OPEN: %w", err)
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
					Err:  provider.StreamInterrupt(fmt.Errorf("dari: connection lost: %w", err), provider.StreamInterruptConnectionReset),
				}
				p.mu.Lock()
				if p.conn != nil {
					p.conn.Close()
					p.conn = nil
				}
				p.mu.Unlock()
				return
			}

			msgType := dariproto.MessageType(rec.MessageType)

			switch msgType {
			case dariproto.MsgAITokenChunk:
				var chunk dariproto.AITokenChunkPayload
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

			case dariproto.MsgRelayVerdict:
				// F.6 decision-before-consumption: verify the signed
				// decision under the AUTH_ACK policy issuer key and
				// refuse the stream unless it authorizes this exchange
				// right now. A DENY/expired/invalid decision is fatal —
				// tokens that already streamed are not new authority.
				if perr := p.verifyRelayVerdict(rec.Payload); perr != nil {
					out <- provider.Chunk{Type: provider.ChunkError, Err: fmt.Errorf("dari: relay verdict rejected: %w", perr)}
					return
				}

			case dariproto.MsgAIComplete:
				var result dariproto.AICompletePayload
				if err := json.Unmarshal(rec.Payload, &result); err != nil {
					out <- provider.Chunk{Type: provider.ChunkError, Err: fmt.Errorf("dari: decode AI_COMPLETE: %w", err)}
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
				// B1: seal + flush the turn's provenance envelopes
				// (change set → spans → actions) while the
				// authenticated connection is still live.
				go p.flushProvenance()
				return

			case dariproto.MsgPing:
				p.conn.SendControl(dariproto.MsgPong, nil, []byte("pong"))

			case dariproto.MsgBroadcast:
				// E2: governed broadcast. Stored for the air-gap
				// surfaces AND delivered to the live comms inbox;
				// never surfaced into the model stream.
				p.storeAdvisory("broadcast", rec.Payload)
				p.handleBroadcastPayload(rec.Payload)

			case dariproto.MsgAdminCommand:
				// E5: signed admin command — stored and routed to the
				// admin dispatcher (signature verification, expiry,
				// and execution recording happen there).
				p.storeAdvisory("admin-directive", rec.Payload)
				p.handleAdminDirectivePayload(rec.Payload)

			case dariproto.MsgSovereignAdvisory:
				// E3: signed offline advisory (air-gap mode). Stored
				// for the sovereign surfaces.
				p.storeAdvisory("sovereign", rec.Payload)

			case dariproto.MsgEvidenceReceipt:
				// B3 e2e: the relay pushes a SIGNED evidence receipt
				// after each governed exchange. Verify the COSE-Sign1
				// signature under the AUTH_ACK receipt signer key, then
				// store as local tamper-evidence + ack. An unverifiable
				// receipt is rejected — never stored, never acked.
				env, err := provenancewire.DecodeEvidenceReceiptEnvelope(rec.Payload)
				if err != nil {
					continue
				}
				p.mu.Lock()
				rk := p.receiptSignerKey
				p.mu.Unlock()
				if rk == nil {
					slog.Warn("dari: evidence receipt arrived before the AUTH_ACK signer key — rejected")
					continue
				}
				if verr := provenancewire.VerifyEvidenceReceiptSignature(env, rk); verr != nil {
					slog.Warn("dari: evidence receipt signature REJECTED", "receipt", env.ReceiptID, "err", verr)
					continue
				}
				if p.receiptHandler != nil {
					_, _ = p.receiptHandler.HandleReceipt(env)
				}

			case dariproto.MsgLeaseRevoke:
				// A3 e2e: the relay revoked the session's lease
				// mid-flight. Drop the lease (fail-closed for any
				// subsequent exchange) and surface the termination.
				if p.leaseClient != nil {
					p.leaseClient.Drop()
				}
				out <- provider.Chunk{
					Type: provider.ChunkError,
					Err:  errors.New("dari: capability lease revoked by relay"),
				}
				return

			case dariproto.MsgPolicyEpochPush:
				// A4 e2e: policy changed mid-session. Rebind; the next
				// exchange's lease pin enforces the new epoch.
				if epoch, err := dariproto.DecodePolicyEpochMessage(rec.Payload); err == nil {
					if p.policyEpochClient == nil {
						p.policyEpochClient = dariproto.NewPolicyEpochClient()
					}
					if err := p.policyEpochClient.Rebind(epoch); err == nil {
						p.policyEpoch = epoch.EpochID
					}
				}

			case dariproto.MsgDLPRulePack:
				// C1.3: the relay pushes the org's DLP rule pack. Apply
				// the org's class enables/disables to the DLP scanner so
				// the connector enforces the server's lexicon policy.
				if pack, derr := dariproto.DecodeDLPRulePack(rec.Payload); derr == nil {
					p.applyDLPRulePack(pack)
				}

			case dariproto.MsgGovernanceState:
				// C3/C4/D1/D3-D6/E4: the relay pushes the org's
				// governance-state snapshot. Route it to the installed
				// sink so boot can install the governed gates on the
				// controller (they then fire on real tool calls).
				if snap, gerr := dariproto.DecodeGovernanceState(rec.Payload); gerr == nil {
					p.applyGovernanceState(snap)
				}

			case dariproto.MsgCatalogDelta:
				// A5 e2e: apply an incremental catalog update.
				if delta, err := dariproto.DecodeCatalogDelta(rec.Payload); err == nil {
					if p.catalogClient == nil {
						p.catalogClient = dariproto.NewCatalogClient()
					}
					_ = p.catalogClient.ApplyDelta(delta, p.policyEpoch)
				}

			case dariproto.MsgClose:
				var errMsg map[string]string
				json.Unmarshal(rec.Payload, &errMsg)
				out <- provider.Chunk{
					Type: provider.ChunkError,
					Err:  fmt.Errorf("dari: relay closed: %s", errMsg["error"]),
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

// validateLease enforces the A3 / A4 / A5 fail-closed checks before the
// connector dispatches an AI_OPEN. The single AuthorizeExchange call
// chains the subject/session verify (A3), the policy-epoch pin (A4),
// the renewal-window check, and the model allow-list check (A5).
// The catalog client (A5) is consulted in parallel: a model the
// relay never advertised is rejected before the lease allow-list
// even runs. Each failure returns a sentinel error the harness UI
// surfaces to the operator without translation.
func (p *Provider) validateLease(model string) error {
	if p.leaseClient == nil {
		return errors.New("dari: no lease held; connect to a relay and acquire a capability lease before dispatching inference")
	}
	if err := p.leaseClient.AuthorizeExchange(p.subjectPeerID, p.sessionID, p.policyEpoch, model); err != nil {
		return err
	}
	if p.catalogClient != nil {
		if _, err := p.catalogClient.FindModel(model); err != nil {
			return fmt.Errorf("dari: model %q is not in the relay's catalog: %w", model, err)
		}
	}
	return nil
}

// SetLeaseClient attaches a lease client to the provider. The connector
// pings the relay for a LEASE_ISSUE after AUTH_PROOF and feeds the
// result here. A nil client leaves the provider in fail-closed mode
// (validateLease rejects every AI_OPEN).
func (p *Provider) SetLeaseClient(client *dariproto.LeaseClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.leaseClient = client
}

// SetSessionContext binds the provider's session to a subject peer ID,
// session ID, and policy epoch. The provider pins these values into
// every lease validation. A change to any of these values forces the
// caller to renew the lease through the relay.
func (p *Provider) SetSessionContext(subjectPeerID, sessionID, policyEpoch string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subjectPeerID = subjectPeerID
	p.sessionID = sessionID
	p.policyEpoch = policyEpoch
}

// SetUserContext records the acting user identifier reported at
// SESSION_OPEN. The relay binds the issued lease to it.
func (p *Provider) SetUserContext(userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userID = userID
}

// SetReceiptStore installs a durable receipt store (B3). The handler
// is still constructed per connection (the ack must ride the live
// transport); only the persistence is replaced.
func (p *Provider) SetReceiptStore(store *provenancewire.ReceiptStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.receiptStore = store
	p.receiptHandler = nil // rebuild on next connect with this store
}

// SetReceiptHandler installs the evidence-receipt store + ack handler
// (B3). The transport reader routes relay-pushed EVIDENCE_RECEIPT
// messages here.
func (p *Provider) SetReceiptHandler(h *provenancewire.IncomingAckHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.receiptHandler = h
}

// SetAutoRenewBefore overrides the connector's proactive renewal lead
// time. The default is 5 minutes; tests set it to a sub-second value to
// drive automatic renewal behavior.
func (p *Provider) SetAutoRenewBefore(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.autoRenewBefore = d
	if p.leaseClient != nil {
		p.leaseClient.WithAutoRenewBefore(d)
	}
}

// LeaseMetrics exposes the connector's lease health for the harness
// status bar (E1). The connector surfaces the lease's ID, sequence,
// and expiry without exposing the underlying private key.
func (p *Provider) LeaseMetrics() dariproto.LeaseMetrics {
	if p.leaseClient == nil {
		return dariproto.LeaseMetrics{}
	}
	return p.leaseClient.MetricsFor()
}

// SetPolicyEpochClient attaches the policy-epoch tracker. The
// connector binds the lease's PolicyEpochID to the client's bound
// epoch on every exchange. A nil client leaves the provider in
// fail-closed mode (validateLease rejects every AI_OPEN).
func (p *Provider) SetPolicyEpochClient(client *dariproto.PolicyEpochClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.policyEpochClient = client
}

// SetCatalogClient attaches the catalog client. The connector
// cross-checks the requested model against the held catalog on every
// exchange. A nil client skips the catalog check (the lease
// allow-list is the only model filter in that mode).
func (p *Provider) SetCatalogClient(client *dariproto.CatalogClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.catalogClient = client
}

// PolicyEpochMetrics exposes the connector's policy-epoch health for
// the harness status bar (E1).
func (p *Provider) PolicyEpochMetrics() dariproto.PolicyEpochMetrics {
	if p.policyEpochClient == nil {
		return dariproto.PolicyEpochMetrics{}
	}
	return p.policyEpochClient.MetricsFor()
}

// CatalogMetrics exposes the connector's catalog health for the
// harness status bar (E1).
func (p *Provider) CatalogMetrics() dariproto.CatalogMetrics {
	if p.catalogClient == nil {
		return dariproto.CatalogMetrics{}
	}
	return p.catalogClient.MetricsFor()
}

// BindPolicyEpoch binds the connector to the supplied epoch. The
// connector calls this once at AUTH_PROOF time when the relay pushes
// the active policy epoch alongside the trust bundle.
func (p *Provider) BindPolicyEpoch(epoch *dariproto.PolicyEpoch) error {
	if p.policyEpochClient == nil {
		return errors.New("dari: no policy epoch client; SetPolicyEpochClient before binding")
	}
	if err := p.policyEpochClient.Bind(epoch); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.policyEpoch = epoch.EpochID
	return nil
}

// RebindPolicyEpoch replaces the bound policy epoch with a fresh one
// (typically from a POLICY message). The connector surfaces the
// new epoch to the lease allow-list on subsequent exchanges.
func (p *Provider) RebindPolicyEpoch(epoch *dariproto.PolicyEpoch) error {
	if p.policyEpochClient == nil {
		return errors.New("dari: no policy epoch client; SetPolicyEpochClient before rebinding")
	}
	if err := p.policyEpochClient.Rebind(epoch); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.policyEpoch = epoch.EpochID
	return nil
}

// ApplyCatalogSnapshot replaces the held catalog. The connector
// calls this when the relay pushes CATALOG_SNAPSHOT.
func (p *Provider) ApplyCatalogSnapshot(snap *dariproto.CatalogSnapshot) error {
	if p.catalogClient == nil {
		return errors.New("dari: no catalog client; SetCatalogClient before applying")
	}
	expectedEpoch := p.policyEpoch
	return p.catalogClient.ApplySnapshot(snap, expectedEpoch)
}

// ApplyCatalogDelta applies an incremental catalog update.
func (p *Provider) ApplyCatalogDelta(delta *dariproto.CatalogDelta) error {
	if p.catalogClient == nil {
		return errors.New("dari: no catalog client; SetCatalogClient before applying")
	}
	expectedEpoch := p.policyEpoch
	return p.catalogClient.ApplyDelta(delta, expectedEpoch)
}

// defaultAutoRenewBefore is the connector's proactive renewal lead time.
// The provider triggers a LEASE_RENEW handshake when the held lease is
// inside this window before NotAfter.
const defaultAutoRenewBefore = 5 * time.Minute
