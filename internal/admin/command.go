// Package admin is the connector-side admin-command channel
// (harness feature plan E5). The relay pushes governed admin
// commands to the live harness over PAPER; the harness executes
// them and emits evidence so an admin's lock/freeze-branch/
// force-update/recall reaches the live machine, not just the DB.
//
// Every command carries a signature from the relay's policy
// issuer. The harness verifies the signature under the trust
// bundle pushed at AUTH_PROOF time; an unsigned or
// signature-mismatched command is rejected before any side effect.
package admin

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CommandType is the typed enumeration of admin commands the
// relay pushes to enrolled harnesses.
type CommandType string

const (
	CmdLockHarness    CommandType = "LOCK_HARNESS"
	CmdFreezeBranch   CommandType = "FREEZE_BRANCH"
	CmdForceUpdate    CommandType = "FORCE_UPDATE"
	CmdModelRecall    CommandType = "MODEL_RECALL"
	CmdReleaseHarness CommandType = "RELEASE_HARNESS"
	CmdUnfreezeBranch CommandType = "UNFREEZE_BRANCH"
	CmdApproveTool    CommandType = "APPROVE_TOOL"
	CmdBlockTool      CommandType = "BLOCK_TOOL"
)

// Command is a single relay-pushed admin command. The harness
// executes it locally and emits evidence for the audit log.
type Command struct {
	CommandID      string
	CommandType    CommandType
	OrganizationID string
	Target         string // harness ID, repo:branch, model ID, etc.
	Reason         string
	IssuedBy       string // admin user/role
	IssuedAt       int64  // unix-ms
	NotAfter       int64  // unix-ms; 0 = no expiry
	// Payload carries command-specific data (e.g. forced version).
	Payload []byte
	// Signature is the ed25519 signature over the canonical command
	// bytes produced by SigningBytes.
	Signature []byte
}

// SigningBytes produces the canonical bytes the relay signs and the
// harness verifies. The format MUST match the relay's admin
// command signer; the cross-repo conformance test pins the
// contract.
func (c *Command) SigningBytes() []byte {
	notAfterStr := ""
	if c.NotAfter > 0 {
		notAfterStr = fmt.Sprintf("%d", c.NotAfter)
	}
	data := fmt.Sprintf("admin|%s|%s|%s|%s|%s|%s|%d|%d|%s|%s",
		c.CommandID, c.CommandType, c.OrganizationID, c.Target,
		c.IssuedBy, c.Reason, c.IssuedAt, c.NotAfter, notAfterStr,
		string(c.Payload))
	return []byte(data)
}

// VerifySignature checks the command's signature under the
// supplied issuer public key. The harness's trust bundle carries
// the issuer key; an untrusted command is rejected.
func (c *Command) VerifySignature(issuerPub ed25519.PublicKey) error {
	if len(issuerPub) == 0 {
		return errors.New("admin: issuer public key not configured")
	}
	if len(c.Signature) == 0 {
		return errors.New("admin: command has no signature")
	}
	if !ed25519.Verify(issuerPub, c.SigningBytes(), c.Signature) {
		return errors.New("admin: command signature verification failed")
	}
	return nil
}

// IsExpired reports whether the command is past its NotAfter
// deadline.
func (c *Command) IsExpired(nowMs int64) bool {
	return c.NotAfter > 0 && nowMs >= c.NotAfter
}

// ExecutionStatus is the lifecycle state of a command on the
// harness side.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "PENDING"
	StatusExecuting ExecutionStatus = "EXECUTING"
	StatusSucceeded ExecutionStatus = "SUCCEEDED"
	StatusFailed    ExecutionStatus = "FAILED"
	StatusRejected  ExecutionStatus = "REJECTED"
)

// Execution is the harness's record of a single command execution.
// The relay stores the audit trail.
type Execution struct {
	CommandID      string
	Status         ExecutionStatus
	StartedAt      int64
	CompletedAt    int64
	ErrorMessage   string
	EvidenceDigest [32]byte
}

// Executor is the harness-side admin command executor. The
// harness supplies an implementation that wires the command to
// the corresponding client-side behavior (e.g. CmdFreezeBranch
// calls into workflow.GatesClient.SetFreeze).
type Executor interface {
	Execute(cmd *Command) error
}

// Dispatcher receives commands from the PAPER transport, verifies
// their signatures, and invokes the supplied Executor.
type Dispatcher struct {
	mu          sync.RWMutex
	executor    Executor
	issuerPub   ed25519.PublicKey
	executions  map[string]*Execution
	commandSink func(*Execution) // optional: relay upload
	rejected    int64
	executed    int64
}

// NewDispatcher constructs a dispatcher. The harness calls
// SetIssuerPubKey when the trust bundle is updated.
func NewDispatcher(executor Executor) *Dispatcher {
	return &Dispatcher{
		executor:   executor,
		executions: make(map[string]*Execution),
	}
}

// SetIssuerPubKey updates the issuer public key used for command
// signature verification.
func (d *Dispatcher) SetIssuerPubKey(pub ed25519.PublicKey) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.issuerPub = pub
}

// SetExecutor replaces the command executor (harness-specific
// directive handlers install themselves after construction).
func (d *Dispatcher) SetExecutor(exec Executor) {
	if exec == nil {
		return
	}
	d.mu.Lock()
	d.executor = exec
	d.mu.Unlock()
}

// SetCommandSink wires an optional audit-log upload. The harness
// passes a callback that ships the Execution to the relay for
// durable storage.
func (d *Dispatcher) SetCommandSink(sink func(*Execution)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.commandSink = sink
}

// Dispatch is the single entry point the harness's transport
// layer calls when a `MsgAdminCommand` record arrives. The
// dispatcher verifies the signature, executes the command, and
// stores the Execution for the audit log.
func (d *Dispatcher) Dispatch(cmd *Command, nowMs int64) (*Execution, error) {
	if cmd == nil {
		return nil, errors.New("admin: nil command")
	}
	d.mu.Lock()
	issuerPub := d.issuerPub
	executor := d.executor
	sink := d.commandSink
	d.mu.Unlock()
	if issuerPub == nil {
		return nil, errors.New("admin: issuer public key not configured")
	}
	if err := cmd.VerifySignature(issuerPub); err != nil {
		d.mu.Lock()
		d.rejected++
		d.mu.Unlock()
		exec := &Execution{
			CommandID: cmd.CommandID,
			Status:    StatusRejected,
			StartedAt: nowMs,
		}
		d.recordExecution(exec)
		return exec, fmt.Errorf("admin: command signature: %w", err)
	}
	if cmd.IsExpired(nowMs) {
		d.mu.Lock()
		d.rejected++
		d.mu.Unlock()
		exec := &Execution{
			CommandID: cmd.CommandID,
			Status:    StatusRejected,
			StartedAt: nowMs,
		}
		d.recordExecution(exec)
		return exec, errors.New("admin: command expired")
	}
	if executor == nil {
		return nil, errors.New("admin: executor not configured")
	}
	d.mu.Lock()
	d.executed++
	d.mu.Unlock()
	exec := &Execution{
		CommandID: cmd.CommandID,
		Status:    StatusExecuting,
		StartedAt: nowMs,
	}
	err := executor.Execute(cmd)
	exec.CompletedAt = time.Now().UnixMilli()
	if err != nil {
		exec.Status = StatusFailed
		exec.ErrorMessage = err.Error()
	} else {
		exec.Status = StatusSucceeded
	}
	d.recordExecution(exec)
	if sink != nil {
		sink(exec)
	}
	return exec, err
}

// recordExecution stores the Execution in the dispatcher's map.
func (d *Dispatcher) recordExecution(exec *Execution) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.executions[exec.CommandID] = exec
}

// GetExecution returns the Execution for a given command ID, if
// recorded.
func (d *Dispatcher) GetExecution(commandID string) (*Execution, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	exec, ok := d.executions[commandID]
	return exec, ok
}

// ExecutedCount returns the E1 status-bar counter for executed
// commands.
func (d *Dispatcher) ExecutedCount() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.executed
}

// RejectedCount returns the E1 status-bar counter for rejected
// commands (signature failures, expirations, schema errors).
func (d *Dispatcher) RejectedCount() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.rejected
}
