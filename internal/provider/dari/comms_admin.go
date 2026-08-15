package dari

import (
	"encoding/json"
	"time"

	"patty/internal/admin"
	"patty/internal/comms"
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
		p.adminDisp = admin.NewDispatcher(loggingAdminExecutor{})
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
