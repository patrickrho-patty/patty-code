package approvals

import (
	"strings"
	"testing"
)

// TestCheckToolApproved covers the C3 green path: an approved
// tool passes.
func TestCheckToolApproved(t *testing.T) {
	r := NewRegistry()
	r.SetTools([]*ToolRegistration{
		{ToolID: "tool-1", DisplayName: "Edit File", Status: StatusApproved, Version: "1.0.0"},
	}, 1)
	dec := r.CheckTool("tool-1", 1)
	if !dec.Allow {
		t.Errorf("approved tool must pass, got %s", dec.Reason)
	}
	if r.ToolAllowCountValue() != 1 {
		t.Errorf("allow count = %d, want 1", r.ToolAllowCountValue())
	}
}

// TestCheckToolBlocked covers the C3 fail-closed path: a blocked
// tool is refused.
func TestCheckToolBlocked(t *testing.T) {
	r := NewRegistry()
	r.SetTools([]*ToolRegistration{
		{ToolID: "tool-bad", Status: StatusBlocked, DescriptionKo: "차단됨"},
	}, 1)
	dec := r.CheckTool("tool-bad", 1)
	if dec.Allow {
		t.Fatal("blocked tool must fail")
	}
	if dec.BlockedBy != "blocked-tool" {
		t.Errorf("blocked by = %s, want blocked-tool", dec.BlockedBy)
	}
	if !strings.Contains(dec.ReasonKo, "차단") {
		t.Errorf("Korean reason missing block word: %q", dec.ReasonKo)
	}
	if r.ToolDenyCountValue() != 1 {
		t.Errorf("deny count = %d, want 1", r.ToolDenyCountValue())
	}
}

// TestCheckToolRequireReview covers the C3 review-required path:
// the harness must surface a review prompt rather than invoke the
// tool.
func TestCheckToolRequireReview(t *testing.T) {
	r := NewRegistry()
	r.SetTools([]*ToolRegistration{
		{ToolID: "tool-review", Status: StatusRequireReview},
	}, 1)
	dec := r.CheckTool("tool-review", 1)
	if dec.Allow {
		t.Fatal("review-required tool must fail at invocation")
	}
	if dec.BlockedBy != "require-review" {
		t.Errorf("blocked by = %s, want require-review", dec.BlockedBy)
	}
}

// TestCheckToolPending covers the C3 PENDING status: admin hasn't
// reviewed the tool yet, the harness refuses.
func TestCheckToolPending(t *testing.T) {
	r := NewRegistry()
	r.SetTools([]*ToolRegistration{
		{ToolID: "tool-pending", Status: StatusPending},
	}, 1)
	dec := r.CheckTool("tool-pending", 1)
	if dec.Allow {
		t.Fatal("pending tool must fail")
	}
	if dec.BlockedBy != "pending-review" {
		t.Errorf("blocked by = %s, want pending-review", dec.BlockedBy)
	}
}

// TestCheckToolUnregistered covers the boundary: an unknown tool
// fails closed.
func TestCheckToolUnregistered(t *testing.T) {
	r := NewRegistry()
	dec := r.CheckTool("ghost-tool", 1)
	if dec.Allow {
		t.Fatal("unknown tool must fail")
	}
	if dec.BlockedBy != "unregistered-tool" {
		t.Errorf("blocked by = %s, want unregistered-tool", dec.BlockedBy)
	}
}

// TestCheckToolExpiredRegistration covers the C3 expiry boundary:
// a registration past NotAfterUnixMs is treated as if the tool
// had no record.
func TestCheckToolExpiredRegistration(t *testing.T) {
	r := NewRegistry()
	r.SetTools([]*ToolRegistration{
		{
			ToolID:        "tool-expired",
			Status:       StatusApproved,
			NotAfterUnixMs: 1000,
		},
	}, 1)
	dec := r.CheckTool("tool-expired", 2000)
	if dec.Allow {
		t.Fatal("expired tool must fail")
	}
	if dec.BlockedBy != "expired-registration" {
		t.Errorf("blocked by = %s, want expired-registration", dec.BlockedBy)
	}
}

// TestCheckNetworkLoopback covers the C4 always-allowed path.
func TestCheckNetworkLoopback(t *testing.T) {
	r := NewRegistry()
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		dec := r.CheckNetwork(host)
		if !dec.Allow {
			t.Errorf("loopback %s must be allowed, got %s", host, dec.Reason)
		}
	}
}

// TestCheckNetworkAllowedByGlob covers the C4 glob-match path.
func TestCheckNetworkAllowedByGlob(t *testing.T) {
	r := NewRegistry()
	r.SetNetworkGrants([]*NetworkGrant{
		{HostPattern: "*.anthropic.com", TokenBudget: 1000},
		{HostPattern: "api.openai.com"},
	})
	for _, host := range []string{"api.anthropic.com", "claude.anthropic.com"} {
		dec := r.CheckNetwork(host)
		if !dec.Allow {
			t.Errorf("%s must be allowed, got %s", host, dec.Reason)
		}
	}
	if dec := r.CheckNetwork("evil.com"); dec.Allow {
		t.Errorf("evil.com must be blocked")
	}
	if dec := r.CheckNetwork("api.openai.com"); !dec.Allow {
		t.Errorf("literal pattern must allow")
	}
}

// TestCheckNetworkNoGrant covers the C4 fail-closed path: an
// unregistered host is refused.
func TestCheckNetworkNoGrant(t *testing.T) {
	r := NewRegistry()
	dec := r.CheckNetwork("example.com")
	if dec.Allow {
		t.Fatal("no-grant host must be blocked")
	}
	if dec.BlockedBy != "no-grant" {
		t.Errorf("blocked by = %s, want no-grant", dec.BlockedBy)
	}
}

// TestCheckNetworkExhaustedGrant covers the C4 quota-exhaustion
// path.
func TestCheckNetworkExhaustedGrant(t *testing.T) {
	r := NewRegistry()
	r.SetNetworkGrants([]*NetworkGrant{
		{HostPattern: "*.example.com", TokenBudget: 100, TokensConsumed: 100},
	})
	dec := r.CheckNetwork("api.example.com")
	if dec.Allow {
		t.Fatal("exhausted grant must be blocked")
	}
	if dec.BlockedBy != "exhausted-grant" {
		t.Errorf("blocked by = %s, want exhausted-grant", dec.BlockedBy)
	}
}

// TestCheckSecretAllowed covers the C4 secret-broker green path.
func TestCheckSecretAllowed(t *testing.T) {
	r := NewRegistry()
	r.SetSecretGrants([]*SecretGrant{
		{SecretID: "github-token", MaxReads: 5, ReadsIssued: 1},
	})
	dec := r.CheckSecret("github-token")
	if !dec.Allow {
		t.Errorf("secret with quota remaining must pass, got %s", dec.Reason)
	}
}

// TestCheckSecretExhausted covers the C4 quota-exhaustion path.
func TestCheckSecretExhausted(t *testing.T) {
	r := NewRegistry()
	r.SetSecretGrants([]*SecretGrant{
		{SecretID: "github-token", MaxReads: 5, ReadsIssued: 5},
	})
	dec := r.CheckSecret("github-token")
	if dec.Allow {
		t.Fatal("exhausted secret must be blocked")
	}
	if dec.BlockedBy != "exhausted-grant" {
		t.Errorf("blocked by = %s, want exhausted-grant", dec.BlockedBy)
	}
}

// TestCheckSecretNoGrant covers the C4 fail-closed path.
func TestCheckSecretNoGrant(t *testing.T) {
	r := NewRegistry()
	dec := r.CheckSecret("missing-secret")
	if dec.Allow {
		t.Fatal("no-grant secret must be blocked")
	}
	if dec.BlockedBy != "no-grant" {
		t.Errorf("blocked by = %s, want no-grant", dec.BlockedBy)
	}
}

// TestMatchGlobGlobAndLiteral covers the glob matcher used by
// NetworkGrant.AllowsHost.
func TestMatchGlobGlobAndLiteral(t *testing.T) {
	cases := []struct {
		pattern, host string
		want          bool
	}{
		{"*", "example.com", true},
		{"*.example.com", "api.example.com", true},
		{"*.example.com", "example.com", false}, // host has no leading subdomain
		{"api.*.com", "api.example.com", true},
		{"api.*.com", "api.example.org", false},
		{"api.openai.com", "api.openai.com", true},
		{"api.openai.com", "evil.openai.com", false},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.host); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.host, got, c.want)
		}
	}
}

// TestRegistryConcurrentCheck covers the lock boundary.
func TestRegistryConcurrentCheck(t *testing.T) {
	r := NewRegistry()
	r.SetTools([]*ToolRegistration{{ToolID: "tool", Status: StatusApproved}}, 1)
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			_ = r.CheckTool("tool", 1)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
