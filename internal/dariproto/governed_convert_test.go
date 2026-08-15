package dariproto

import (
	"testing"

	"patty/internal/governed"
)

// governed_convert_test.go pins the production wiring path: a decoded
// governance snapshot builds governed clients whose checks fire on
// tool calls (C3/C4/D1/D3-D6/E4).

func convertTestSnapshot() *GovernanceStateWire {
	return &GovernanceStateWire{
		Version: 1,
		OrgID:   "org-1",
		RepoID:  "repo-1",
		ModelID: "model-1",
		Freeze: &GovernanceFreezeWire{
			Reason:        "quarter close",
			AffectedRepos: []string{"repo-1"},
			NotAfterMs:    4102444800000, // 2100-01-01
		},
		Tools: []GovernanceToolWire{
			{ToolID: "bash", Status: "APPROVED"},
			{ToolID: "web_search", Status: "BLOCKED"},
		},
	}
}

func TestGovernedStateBlocksFrozenWrite(t *testing.T) {
	snap := convertTestSnapshot()
	id := HarnessIdentity{Version: "1.0.0", Ring: "stable"}

	st := governed.NewState()
	st.SetGates(snap.BuildGatesClient(id))
	st.SetRegistry(snap.BuildApprovalsRegistry())
	st.SetSandboxPolicy(snap.BuildSandboxStore())
	st.SetContext(snap.RepoID, snap.ModelID)

	// D3: a change-frozen repo blocks AI writes.
	if blocked, reason := st.CheckToolCall("write_file", []byte(`{"path":"a.txt","content":"x"}`), false); !blocked {
		t.Fatal("frozen repo must block file writes")
	} else if reason == "" {
		t.Fatal("block must carry a reason")
	}

	// C3: a BLOCKED tool is denied even without a freeze.
	if blocked, _ := st.CheckToolCall("web_search", nil, true); !blocked {
		t.Fatal("blocked tool must be denied")
	}

	// Reads still pass under freeze (D3 semantics).
	if blocked, _ := st.CheckToolCall("bash", []byte(`{"command":"go test ./..."}`), true); blocked {
		t.Fatal("read-only shell must pass under freeze")
	}
}

func TestGovernedStateVersionRefusesOldHarness(t *testing.T) {
	snap := convertTestSnapshot()
	snap.VersionReq = &GovernanceVersionWire{MinVersion: "2.0.0"}
	gates := snap.BuildGatesClient(HarnessIdentity{Version: "1.0.0", Ring: "stable"})
	// Any dispatch on a sub-minimum build must deny (D5).
	if dec := gates.CheckDispatch("tool_use", "repo-1", "model-1"); dec.Allow {
		t.Fatalf("sub-minimum harness must be denied, got %+v", dec)
	}
}

func TestGovernanceStateWireRoundTrip(t *testing.T) {
	// Encode via the same CBOR path the relay uses, decode, convert.
	snap := convertTestSnapshot()
	data, err := MarshalCBOR(snap)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeGovernanceState(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.OrgID != "org-1" || decoded.Freeze == nil || len(decoded.Tools) != 2 {
		t.Fatalf("decoded = %+v", decoded)
	}
	if _, err := DecodeGovernanceState(nil); err == nil {
		t.Fatal("empty body must fail")
	}
}
