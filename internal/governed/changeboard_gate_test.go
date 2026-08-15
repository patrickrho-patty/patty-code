package governed

import (
	"testing"

	"patty/internal/changeboard"
)

// changeboard_gate_test.go pins the D2 production wiring: high-risk
// AI writes auto-submit to the change-control board and block until a
// reviewer approves; approved submissions pass; ordinary writes are
// untouched.

func boardState() *State {
	st := NewState()
	st.SetChangeBoard(changeboard.NewBoard())
	st.SetContext("repo-d2", "model-x")
	return st
}

func TestHighRiskWriteAutoSubmitsAndBlocks(t *testing.T) {
	st := boardState()
	args := []byte(`{"path":"internal/crypto/signer.go","content":"package crypto"}`)

	blocked, reason := st.CheckToolCall("write_file", args, false)
	if !blocked {
		t.Fatal("high-risk crypto write must block pending approval")
	}
	if reason == "" || !containsHangul(reason) {
		t.Fatalf("reason must surface Korean, got %q", reason)
	}
	// The block auto-submitted the change.
	pending := st.board.ListPending()
	if len(pending) != 1 || pending[0].RiskClass != changeboard.RiskCrypto {
		t.Fatalf("pending = %+v", pending)
	}
}

func TestApprovedSubmissionPasses(t *testing.T) {
	st := boardState()
	args := []byte(`{"path":"internal/crypto/signer.go","content":"package crypto"}`)
	if blocked, _ := st.CheckToolCall("write_file", args, false); !blocked {
		t.Fatal("precondition: first write blocks")
	}
	sub := st.board.ListPending()[0]
	if err := st.board.Approve(sub.SubmissionID, "reviewer-1", "ok", 0); err != nil {
		t.Fatal(err)
	}
	// Same content: fingerprint matches the approved submission.
	if blocked, _ := st.CheckToolCall("write_file", args, false); blocked {
		t.Fatal("approved submission must pass")
	}
}

func TestRejectedSubmissionStaysBlocked(t *testing.T) {
	st := boardState()
	args := []byte(`{"path":"billing/checkout.go","content":"payment"}`)
	if blocked, _ := st.CheckToolCall("write_file", args, false); !blocked {
		t.Fatal("payment write must block")
	}
	sub := st.board.ListPending()[0]
	_ = st.board.Reject(sub.SubmissionID, "reviewer-1", "no", 0)
	if blocked, _ := st.CheckToolCall("write_file", args, false); !blocked {
		t.Fatal("rejected submission must stay blocked")
	}
}

func TestOrdinaryWriteSkipsBoard(t *testing.T) {
	st := boardState()
	if blocked, _ := st.CheckToolCall("write_file", []byte(`{"path":"docs/readme.md","content":"hi"}`), false); blocked {
		t.Fatal("ordinary write is not board-scope")
	}
	if pending := st.board.ListPending(); len(pending) != 0 {
		t.Fatalf("no submission expected, got %d", len(pending))
	}
}

func TestDepManifestIsMediumRisk(t *testing.T) {
	if got := riskClassOf("go.mod", ""); got != changeboard.RiskMedium {
		t.Fatalf("go.mod risk = %q, want MEDIUM", got)
	}
	if got := riskClassOf("internal/crypto/kms.go", ""); got != changeboard.RiskCrypto {
		t.Fatalf("kms path risk = %q, want CRYPTO", got)
	}
	if got := riskClassOf("src/app/main.go", ""); got != "" {
		t.Fatalf("plain path risk = %q, want none", got)
	}
}

func containsHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

// TestAirGapDialPolicyIsAuthoritative pins E3: in air-gap mode an
// allowlisted host passes and everything else is refused even when no
// network grant exists.
func TestAirGapDialPolicyIsAuthoritative(t *testing.T) {
	st := NewState()
	allowed := map[string]bool{"mirror.internal": true}
	st.SetDialPolicy(func(host string) bool { return allowed[host] })

	if blocked, _ := st.CheckToolCall("bash", []byte(`{"command":"curl https://mirror.internal/pkg"}`), false); blocked {
		t.Fatal("allowlisted host must pass")
	}
	if blocked, reason := st.CheckToolCall("bash", []byte(`{"command":"curl https://evil.example.com/x"}`), false); !blocked {
		t.Fatal("non-allowlisted host must be refused in air-gap mode")
	} else if !containsHangul(reason) {
		t.Fatalf("reason must surface Korean, got %q", reason)
	}
}
