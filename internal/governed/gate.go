package governed

import (
	"context"
	"encoding/json"
	"fmt"
)

// Gate wraps an inner gate (the permission policy's gate or a
// headless variant) with the governance layer. It satisfies the
// agent's Gate interface structurally and passes the optional
// ExplicitDenyGate probe through to the inner gate so MCP deny
// rules keep working unchanged.
type Gate struct {
	inner Checker
	state *State
}

// Wrap returns a gate that runs the governance checks before the
// inner gate. A nil state (no PAPER session) or a nil-safe state
// with no installed clients makes the wrapper a pass-through, so
// local development and tests behave exactly as before.
func Wrap(inner Checker, state *State) Checker {
	if state == nil || state.empty() {
		return inner
	}
	return &Gate{inner: inner, state: state}
}

// empty reports whether no governance client is installed.
func (s *State) empty() bool {
	if s == nil {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registry == nil && s.gates == nil && s.sandbox == nil
}

// Check runs the governance layer first, then the inner gate. A
// governance denial is terminal: the base permission policy never
// sees the call. The denial reason is fed back to the model (no
// error), matching how permission denials already surface.
func (g *Gate) Check(ctx context.Context, toolName string, args json.RawMessage, readOnly bool) (bool, string, error) {
	if blocked, reason := g.state.CheckToolCall(toolName, args, readOnly); blocked {
		return false, denyForReason(boundedReason(reason)), nil
	}
	return g.inner.Check(ctx, toolName, args, readOnly)
}

// ExplicitlyDenies passes the MCP explicit-deny probe through to
// the inner gate. A governance block is not an "explicit deny
// rule" in the permission-policy sense (it is org policy, not a
// user rule), so it does not participate here.
func (g *Gate) ExplicitlyDenies(toolName string, args json.RawMessage) bool {
	if d, ok := g.inner.(interface {
		ExplicitlyDenies(string, json.RawMessage) bool
	}); ok {
		return d.ExplicitlyDenies(toolName, args)
	}
	return false
}

// denyForReason renders a governance block as the model-facing
// denial reason.
func denyForReason(reason string) string {
	return fmt.Sprintf("blocked by PCCP governance policy: %s — this restriction comes from your organization's control plane and cannot be overridden locally. Choose another approach or contact your administrator.", reason)
}
