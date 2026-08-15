package dari

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
)

// comms_admin_test.go pins the E2/E5 drain: broadcasts land in the
// comms inbox, signed directives dispatch (verified under the AUTH_ACK
// issuer key), unsigned ones fail closed, sovereign advisories store.

func TestBroadcastPayloadDeliveredToInbox(t *testing.T) {
	p := &Provider{}
	body, _ := json.Marshal(relayBroadcastWire{
		MessageID:  "bc-1",
		SenderID:   "pccp-policy",
		Body:       "유지보수 공지",
		Severity:   "info",
		IssuedAtMs: 1700000000000,
	})
	p.handleBroadcastPayload(body)

	msg, ok := p.CommsInbox().Get("bc-1")
	if !ok {
		t.Fatal("broadcast must land in the inbox")
	}
	if msg.Body != "유지보수 공지" || msg.Type != "BROADCAST" {
		t.Fatalf("msg = %+v", msg)
	}
	// Dedup: a second delivery of the same ID is a no-op.
	p.handleBroadcastPayload(body)
	if got := p.CommsInbox().Count(); got != 1 {
		t.Fatalf("inbox count = %d, want 1 (dedup)", got)
	}
}

func TestAdminDirectiveVerifiedAndExecuted(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	p := &Provider{}
	p.policyIssuerKey = pub

	cmd := map[string]any{
		"CommandID": "cmd-1", "CommandType": "PAUSE_SESSION",
		"OrganizationID": "org-1", "Target": "harness-1",
		"Reason": "incident", "IssuedBy": "admin", "IssuedAt": 1700000000000,
	}
	// Signing bytes mirror admin.Command.SigningBytes.
	signing := "admin|cmd-1|PAUSE_SESSION|org-1|harness-1|admin|incident|1700000000000|0||"
	cmd["Signature"] = ed25519.Sign(priv, []byte(signing))
	body, _ := json.Marshal(cmd)
	p.handleAdminDirectivePayload(body)

	exec, ok := p.AdminDispatcher().GetExecution("cmd-1")
	if !ok {
		t.Fatal("directive must record an execution")
	}
	if exec.Status != "SUCCEEDED" {
		t.Fatalf("status = %q, want SUCCEEDED", exec.Status)
	}
}

func TestAdminDirectiveUnsignedFailsClosed(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	p := &Provider{}
	p.policyIssuerKey = pub
	body, _ := json.Marshal(map[string]any{
		"CommandID": "cmd-2", "CommandType": "PAUSE_SESSION",
		"OrganizationID": "org-1", "Target": "harness-1",
		"IssuedAt": 1700000000000,
	})
	p.handleAdminDirectivePayload(body)
	if got := p.AdminDispatcher().RejectedCount(); got != 1 {
		t.Fatalf("rejected = %d, want 1", got)
	}
}

func TestSovereignAdvisoryStored(t *testing.T) {
	p := &Provider{}
	p.storeAdvisory("sovereign", []byte("offline advisory"))
	kinds := map[string]bool{}
	for _, a := range p.Advisories() {
		kinds[a.Kind] = true
	}
	if !kinds["sovereign"] {
		t.Fatalf("advisory kinds = %v", kinds)
	}
}
