package dari

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"patty/internal/changeboard"
)

// changeboard_admin_test.go pins the D2/E5 loop: a high-risk
// submission notifies the wire sink; a VERIFIED approve directive
// unblocks the write; an unknown directive fails closed.
func TestChangeboardDirectiveLoop(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	p := &Provider{}
	p.policyIssuerKey = pub

	// A pending submission exists on the durable board.
	board := changeboard.NewBoard()
	sub, err := board.Submit(&changeboard.Submission{
		SubmissionID: "sub-loop-1", RepositoryID: "repo-x",
		RiskClass: changeboard.RiskCrypto, Description: "kms change",
	})
	if err != nil {
		t.Fatal(err)
	}
	p.SetChangeBoard(board)

	// Signed approve directive arrives and executes.
	payload := mustJSON(t, map[string]string{"submission_id": sub.SubmissionID})
	signing := "admin|cmd-ok1|changeboard.approve|org-1|subj|admin|ok|1700000000000|0||" + string(payload)
	cmd := map[string]any{
		"CommandID": "cmd-ok1", "CommandType": "changeboard.approve",
		"OrganizationID": "org-1", "Target": "subj", "Reason": "ok",
		"IssuedBy": "admin", "IssuedAt": 1700000000000,
		"Payload":   payload,
		"Signature": ed25519.Sign(priv, []byte(signing)),
	}
	body, _ := json.Marshal(cmd)
	p.handleAdminDirectivePayload(body)

	if exec, ok := p.AdminDispatcher().GetExecution("cmd-ok1"); !ok || string(exec.Status) != "SUCCEEDED" {
		t.Fatalf("approve execution = %+v", exec)
	}
	got, ok := board.Get(sub.SubmissionID)
	if !ok || !got.IsApproved() {
		t.Fatalf("submission not approved: %+v", got)
	}
}

func TestUnknownDirectiveFailsClosed(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	p := &Provider{}
	p.policyIssuerKey = pub
	signing := "admin|cmd-x1|WIPE_EVERYTHING|org-1|subj|admin|x|1700000000000|0||"
	cmd := map[string]any{
		"CommandID": "cmd-x1", "CommandType": "WIPE_EVERYTHING",
		"OrganizationID": "org-1", "Target": "subj", "IssuedBy": "admin",
		"IssuedAt":  1700000000000,
		"Signature": ed25519.Sign(priv, []byte(signing)),
	}
	body, _ := json.Marshal(cmd)
	p.handleAdminDirectivePayload(body)
	if got := p.AdminDispatcher().RejectedCount(); got == 0 {
		t.Fatal("unknown directive must not silently succeed")
	}
}

func TestSubmissionSinkEmitsEnvelope(t *testing.T) {
	p := &Provider{userID: "u1"}
	sub := &changeboard.Submission{
		SubmissionID: "sub-wire-1", RiskClass: changeboard.RiskHigh,
		Description: "payment path change", Submitter: "patty-governed",
	}
	p.EmitChangeboardSubmission(sub)
	_, _, actions := p.ProvenanceEmitter().Pending()
	if len(actions) != 1 || actions[0].ActionType != "changeboard.submit" {
		t.Fatalf("actions = %+v", actions)
	}
	var payload map[string]any
	if json.Unmarshal([]byte(actions[0].ActionPayload), &payload) != nil || payload["submission_id"] != "sub-wire-1" {
		t.Fatalf("payload = %v", payload)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
