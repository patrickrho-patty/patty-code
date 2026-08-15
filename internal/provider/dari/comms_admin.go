package dari

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"patty/internal/admin"
	"patty/internal/comms"
	"patty/internal/operational"
	"patty/internal/provenancewire"
	"patty/internal/sovereign"
)

// comms_admin.go drains relay-pushed advisories into the live
// connector surfaces (E2/E3/E5 production wiring). The provider owns
// the session's comms Inbox and admin Dispatcher; UI/CLI surfaces
// reach them through the accessors. Sovereign advisories stay in the
// advisory store (air-gap surfaces).

// relayBroadcastWire is the JSON body the relay's MsgBroadcast push
// carries (mirrors the relay's wireBroadcastMessage).
type relayBroadcastWire struct {
	MessageID  string `json:"message_id"`
	Type       string `json:"type"`
	SenderID   string `json:"sender_id"`
	Body       string `json:"body"`
	Severity   string `json:"severity"`
	IssuedAtMs int64  `json:"issued_at_ms"`
}

// CommsInbox returns the session's comms inbox, creating it on first
// use. Relay broadcasts land here for the chat/inbox surfaces.
func (p *Provider) CommsInbox() *comms.Inbox {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.commsInbox == nil {
		p.commsInbox = comms.NewInbox()
	}
	return p.commsInbox
}

// AdminDispatcher returns the session's admin dispatcher, creating it
// on first use. Directive verification is fail-closed until the
// AUTH_ACK policy issuer key arrives and is installed.
func (p *Provider) AdminDispatcher() *admin.Dispatcher {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.adminDisp == nil {
		// E5: the REAL executor — verified directives mutate the live
		// changeboard/session state (changeboard_admin.go).
		p.adminDisp = admin.NewDispatcher(directiveExecutor{p: p})
		if p.policyIssuerKey != nil {
			p.adminDisp.SetIssuerPubKey(p.policyIssuerKey)
		}
	}
	return p.adminDisp
}

// loggingAdminExecutor records executed directives. Harness-specific
// command handlers can replace it via SetAdminExecutor.
type loggingAdminExecutor struct{}

func (loggingAdminExecutor) Execute(cmd *admin.Command) error {
	// Commands that verify are governance-acknowledged; concrete
	// harness actions attach via SetAdminExecutor.
	return nil
}

// SetAdminExecutor replaces the directive executor (e.g. a harness
// wiring forced-version or session-pause commands).
func (p *Provider) SetAdminExecutor(exec admin.Executor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.adminExec = exec
	if p.adminDisp != nil && exec != nil {
		p.adminDisp.SetExecutor(exec)
	}
}

// handleBroadcastPayload decodes a relay broadcast into a comms
// message and delivers it to the inbox (best effort — malformed
// pushes stay in the advisory store).
func (p *Provider) handleBroadcastPayload(payload []byte) {
	var w relayBroadcastWire
	if err := json.Unmarshal(payload, &w); err != nil || w.MessageID == "" {
		return
	}
	msg := &comms.Message{
		MessageID: w.MessageID,
		Type:      comms.MsgBroadcast,
		SenderID:  w.SenderID,
		Body:      w.Body,
		IssuedAt:  w.IssuedAtMs,
	}
	_, _ = p.CommsInbox().Deliver(msg)
}

// handleAdminDirectivePayload decodes a signed directive and runs it
// through the dispatcher — signature verification, expiry, and
// execution recording all happen there (fail-closed without the
// issuer key).
func (p *Provider) handleAdminDirectivePayload(payload []byte) {
	var cmd admin.Command
	if err := json.Unmarshal(payload, &cmd); err != nil || cmd.CommandID == "" {
		return
	}
	_, _ = p.AdminDispatcher().Dispatch(&cmd, time.Now().UnixMilli())
}

// emitActionDraft records a governed action envelope on the session's
// provenance emitter (flushes with the next turn).
func (p *Provider) emitActionDraft(d *provenanceActionDraft) {
	env := &provenancewire.ActionEnvelope{
		ActionID:         d.ActionID,
		OrganizationID:   d.OrganizationID,
		SessionID:        d.SessionID,
		UserID:           d.UserID,
		HarnessID:        d.HarnessID,
		ModelPackageID:   d.ModelPackageID,
		ActionType:       d.ActionType,
		ActionPayload:    d.ActionPayload,
		OccurredAtUnixMs: p.nowMs(),
	}
	if err := p.ProvenanceEmitter().EmitAction(env); err != nil {
		slog.Warn("dari: changeboard submission envelope rejected", "err", err)
	}
}

// orgIDForSession derives the org for connector-emitted envelopes from
// the authenticated credential's organization (empty before auth).
func (p *Provider) orgIDForSession() string { return p.credOrgID }

// credOrgID is set at AUTH_PROOF from the verified credential.

// sovereignOps owns the E3 air-gap state and the E1 awareness client.
// Both become live: air-gap mode is config-driven
// (PATTY_AIRGAP=1 + PATTY_AIRGAP_ALLOWLIST=host1,host2), sovereign
// advisories apply through the verified update path, and awareness
// tracks the real operational signals (suspension, board queue,
// refresh cadence).

func (p *Provider) initSovereignOps() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.airgap != nil {
		return
	}
	ag := sovereign.NewAirGapMode()
	// Config first (patty.toml [sovereign] via SetSovereignConfig); the
	// env pair remains the deployment escape hatch.
	p.mu.Lock()
	cfgEnabled, cfgAllow := p.sovereignCfgEnabled, p.sovereignCfgAllowlist
	p.mu.Unlock()
	if cfgEnabled {
		ag.Enable()
		ag.SetOnlineAllowList(cfgAllow)
		slog.Warn("dari: sovereign air-gap mode ENABLED (config) — dials restricted to the allowlist")
	} else if os.Getenv("PATTY_AIRGAP") == "1" {
		ag.Enable()
		var allow []string
		for _, h := range strings.Split(os.Getenv("PATTY_AIRGAP_ALLOWLIST"), ",") {
			if h = strings.TrimSpace(h); h != "" {
				allow = append(allow, h)
			}
		}
		ag.SetOnlineAllowList(allow)
		slog.Warn("dari: sovereign air-gap mode ENABLED (env) — dials restricted to the allowlist")
	}
	p.airgap = ag
	p.awareness = operational.NewAwarenessClient(p.credOrgID, p.userID)
}

// AirGap exposes the air-gap mode (dial checks + advisory application).
func (p *Provider) AirGap() *sovereign.AirGapMode {
	p.initSovereignOps()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.airgap
}

// Awareness exposes the E1 operational awareness client (status bar).
func (p *Provider) Awareness() *operational.AwarenessClient {
	p.initSovereignOps()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.awareness
}

// SetSovereignConfig installs the [sovereign] posture from patty.toml.
func (p *Provider) SetSovereignConfig(enabled bool, allowlist []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sovereignCfgEnabled = enabled
	p.sovereignCfgAllowlist = allowlist
}
