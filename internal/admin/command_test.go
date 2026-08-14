package admin

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/dariproto"
)

// fakeExecutor records every command it executes.
type fakeExecutor struct {
	mu    sync.Mutex
	calls []*Command
	err   error
}

func (e *fakeExecutor) Execute(cmd *Command) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, cmd)
	return e.err
}

// fakeTransport is the dispatcher-side seam for the harness's
// PAPER transport layer. The harness builds a fakeTransport to
// test the admin command dispatch path without a live relay.
func fakeTransport() *dariproto.TransportConn {
	return &dariproto.TransportConn{}
}

func sampleCommand() *Command {
	return &Command{
		CommandID:      "cmd-1",
		CommandType:    CmdLockHarness,
		OrganizationID: "org-test",
		Target:         "harness-1",
		Reason:         "security-incident",
		IssuedBy:       "admin-alice",
		IssuedAt:       time.Now().UnixMilli(),
		NotAfter:       0,
		Payload:        nil,
	}
}

// signCommand signs the supplied command's SigningBytes with the
// supplied issuer key and stores the signature on the command.
func signCommand(t *testing.T, cmd *Command, issuerPriv ed25519.PrivateKey) {
	t.Helper()
	cmd.Signature = ed25519.Sign(issuerPriv, cmd.SigningBytes())
}

func newIssuer(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("issuer key: %v", err)
	}
	return pub, priv
}

// TestDispatcherVerifiesSignature covers the E5 trust boundary:
// an unsigned command is rejected before any side effect.
func TestDispatcherVerifiesSignature(t *testing.T) {
	executor := &fakeExecutor{}
	issuerPub, _ := newIssuer(t)
	d := NewDispatcher(executor)
	d.SetIssuerPubKey(issuerPub)

	cmd := sampleCommand()
	// No signature.
	exec, err := d.Dispatch(cmd, time.Now().UnixMilli())
	if err == nil {
		t.Fatal("unsigned command must fail")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected signature error, got %v", err)
	}
	if exec.Status != StatusRejected {
		t.Errorf("status = %s, want REJECTED", exec.Status)
	}
	if d.RejectedCount() != 1 {
		t.Errorf("rejected count = %d, want 1", d.RejectedCount())
	}
	if len(executor.calls) != 0 {
		t.Errorf("executor must not run on rejected command")
	}
}

// TestDispatcherRejectsTamperedSignature covers the trust-boundary
// drift: a command with a signature for a different issuer is
// rejected.
func TestDispatcherRejectsTamperedSignature(t *testing.T) {
	executor := &fakeExecutor{}
	issuerPub, _ := newIssuer(t)
	d := NewDispatcher(executor)
	d.SetIssuerPubKey(issuerPub)

	cmd := sampleCommand()
	_, otherPriv := newIssuer(t)
	signCommand(t, cmd, otherPriv) // signed by wrong issuer

	_, err := d.Dispatch(cmd, time.Now().UnixMilli())
	if err == nil {
		t.Fatal("rogue-issuer command must fail")
	}
}

// TestDispatcherExecutesSignedCommand covers the E5 green path:
// a properly-signed command runs.
func TestDispatcherExecutesSignedCommand(t *testing.T) {
	executor := &fakeExecutor{}
	issuerPub, issuerPriv := newIssuer(t)
	d := NewDispatcher(executor)
	d.SetIssuerPubKey(issuerPub)

	cmd := sampleCommand()
	signCommand(t, cmd, issuerPriv)

	exec, err := d.Dispatch(cmd, time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if exec.Status != StatusSucceeded {
		t.Errorf("status = %s, want SUCCEEDED", exec.Status)
	}
	if len(executor.calls) != 1 {
		t.Errorf("executor must run exactly once, got %d", len(executor.calls))
	}
	if d.ExecutedCount() != 1 {
		t.Errorf("executed count = %d, want 1", d.ExecutedCount())
	}
}

// TestDispatcherRejectsExpiredCommand covers the E5 expiry
// boundary: a past-NotAfter command is rejected.
func TestDispatcherRejectsExpiredCommand(t *testing.T) {
	executor := &fakeExecutor{}
	issuerPub, issuerPriv := newIssuer(t)
	d := NewDispatcher(executor)
	d.SetIssuerPubKey(issuerPub)

	cmd := sampleCommand()
	cmd.NotAfter = time.Now().Add(-time.Hour).UnixMilli() // already expired
	signCommand(t, cmd, issuerPriv)

	_, err := d.Dispatch(cmd, time.Now().UnixMilli())
	if err == nil {
		t.Fatal("expired command must fail")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got %v", err)
	}
}

// TestDispatcherCapturesExecutorError covers the E5 audit path:
// an executor failure is recorded with the failure status.
func TestDispatcherCapturesExecutorError(t *testing.T) {
	executor := &fakeExecutor{err: errors.New("harness refused")}
	issuerPub, issuerPriv := newIssuer(t)
	d := NewDispatcher(executor)
	d.SetIssuerPubKey(issuerPub)

	cmd := sampleCommand()
	signCommand(t, cmd, issuerPriv)
	exec, err := d.Dispatch(cmd, time.Now().UnixMilli())
	if err == nil {
		t.Fatal("executor failure must propagate")
	}
	if exec.Status != StatusFailed {
		t.Errorf("status = %s, want FAILED", exec.Status)
	}
	if exec.ErrorMessage == "" {
		t.Error("error message must be captured")
	}
}

// TestDispatcherInvokesCommandSink covers the audit-log upload:
// the sink callback receives every execution for durable storage.
func TestDispatcherInvokesCommandSink(t *testing.T) {
	executor := &fakeExecutor{}
	issuerPub, issuerPriv := newIssuer(t)
	d := NewDispatcher(executor)
	d.SetIssuerPubKey(issuerPub)

	var sinked []*Execution
	d.SetCommandSink(func(exec *Execution) {
		sinked = append(sinked, exec)
	})

	cmd := sampleCommand()
	signCommand(t, cmd, issuerPriv)
	if _, err := d.Dispatch(cmd, time.Now().UnixMilli()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(sinked) != 1 {
		t.Errorf("sink must receive one execution, got %d", len(sinked))
	}
}

// TestDispatcherRequiresIssuerKey covers the trivial boundary:
// the harness can't dispatch without an issuer public key.
func TestDispatcherRequiresIssuerKey(t *testing.T) {
	d := NewDispatcher(&fakeExecutor{})
	_, err := d.Dispatch(sampleCommand(), 0)
	if err == nil {
		t.Fatal("dispatch without issuer key must fail")
	}
}

// TestDispatcherRequiresExecutor covers the trivial boundary.
func TestDispatcherRequiresExecutor(t *testing.T) {
	issuerPub, issuerPriv := newIssuer(t)
	d := NewDispatcher(nil)
	d.SetIssuerPubKey(issuerPub)
	cmd := sampleCommand()
	signCommand(t, cmd, issuerPriv)
	_, err := d.Dispatch(cmd, time.Now().UnixMilli())
	if err == nil {
		t.Fatal("dispatch without executor must fail")
	}
}

// TestCommandSigningBytesBoundAllFields pins the binding
// invariant: every field change must change the signing bytes.
func TestCommandSigningBytesBoundAllFields(t *testing.T) {
	now := time.Now().UnixMilli()
	primary := (&Command{
		CommandID:      "cmd-1",
		CommandType:    CmdLockHarness,
		OrganizationID: "org-test",
		Target:         "harness-1",
		IssuedBy:       "admin",
		IssuedAt:       now,
	}).SigningBytes()
	mutations := []struct {
		name string
		fn   func(*Command)
	}{
		{"id", func(c *Command) { c.CommandID = "cmd-2" }},
		{"type", func(c *Command) { c.CommandType = CmdFreezeBranch }},
		{"org", func(c *Command) { c.OrganizationID = "org-other" }},
		{"target", func(c *Command) { c.Target = "harness-2" }},
		{"issued_by", func(c *Command) { c.IssuedBy = "bob" }},
		{"issued_at", func(c *Command) { c.IssuedAt = now + 1 }},
		{"not_after", func(c *Command) { c.NotAfter = now + 100 }},
		{"payload", func(c *Command) { c.Payload = []byte("p") }},
	}
	for _, m := range mutations {
		clone := (&Command{
			CommandID:      "cmd-1",
			CommandType:    CmdLockHarness,
			OrganizationID: "org-test",
			Target:         "harness-1",
			IssuedBy:       "admin",
			IssuedAt:       now,
		})
		m.fn(clone)
		if string(clone.SigningBytes()) == string(primary) {
			t.Errorf("signing bytes unchanged after %s mutation", m.name)
		}
	}
}

// TestCommandConcurrentDispatchAndRecord covers the lock boundary.
func TestCommandConcurrentDispatchAndRecord(t *testing.T) {
	executor := &fakeExecutor{}
	issuerPub, issuerPriv := newIssuer(t)
	d := NewDispatcher(executor)
	d.SetIssuerPubKey(issuerPub)

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(i int) {
			cmd := sampleCommand()
			cmd.CommandID = cmd.CommandID + "_" + string(rune('a'+i%26))
			signCommand(t, cmd, issuerPriv)
			_, _ = d.Dispatch(cmd, time.Now().UnixMilli())
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
