package workflow

import (
	"strings"
	"testing"
	"time"
)

// TestGatesAllowWhenNoStateInstalled covers the green path: no
// freeze, no recalls, no pending acks means dispatch is allowed.
func TestGatesAllowWhenNoStateInstalled(t *testing.T) {
	g := NewGatesClient("org-test", "1.0.0", "stable")
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if !dec.Allow {
		t.Errorf("dispatch must be allowed, got %s", dec.Reason)
	}
}

// TestGatesBlockWhenVersionBelowMinimum covers D5: a harness
// running below the relay's minimum version is refused.
func TestGatesBlockWhenVersionBelowMinimum(t *testing.T) {
	g := NewGatesClient("org-test", "1.0.0", "stable")
	g.SetVersionRequirement(&VersionRequirement{
		OrganizationID: "org-test",
		MinVersion:     "2.0.0",
		BlockedRings:   nil,
		ReleaseRing:    "stable",
		CurrentVersion: "1.0.0",
		CurrentRing:    "stable",
	})
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if dec.Allow {
		t.Fatal("dispatch must be blocked for sub-minimum harness")
	}
	if dec.BlockedBy != "version-requirement" {
		t.Errorf("blocked by = %s, want version-requirement", dec.BlockedBy)
	}
	if !strings.Contains(dec.ReasonKo, "버전") {
		t.Errorf("Korean message missing version word: %q", dec.ReasonKo)
	}
	if g.BlockedCount() != 1 {
		t.Errorf("blocked count = %d, want 1", g.BlockedCount())
	}
}

// TestGatesAllowWhenVersionAtOrAboveMinimum covers D5: a
// harness at or above the minimum passes.
func TestGatesAllowWhenVersionAtOrAboveMinimum(t *testing.T) {
	g := NewGatesClient("org-test", "2.1.0", "stable")
	g.SetVersionRequirement(&VersionRequirement{
		OrganizationID: "org-test",
		MinVersion:     "2.0.0",
		CurrentVersion: "2.1.0",
		CurrentRing:    "stable",
	})
	if dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard"); !dec.Allow {
		t.Errorf("dispatch must be allowed at version 2.1.0: %s", dec.Reason)
	}
}

// TestGatesBlockOnBlockedRing covers D5: a harness on a blocked
// release ring is refused.
func TestGatesBlockOnBlockedRing(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "canary")
	g.SetVersionRequirement(&VersionRequirement{
		OrganizationID: "org-test",
		MinVersion:     "2.0.0",
		BlockedRings:   []string{"canary"},
		CurrentVersion: "2.0.0",
		CurrentRing:    "canary",
	})
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if dec.Allow {
		t.Fatal("dispatch must be blocked on blocked ring")
	}
	if dec.BlockedBy != "blocked-ring" {
		t.Errorf("blocked by = %s, want blocked-ring", dec.BlockedBy)
	}
}

// TestGatesBlockOnModelRecall covers D6: a recalled model is
// refused with a replacement suggestion.
func TestGatesBlockOnModelRecall(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetRecalls([]ModelRecall{
		{
			OrganizationID: "org-test",
			RecalledModel:  "patty-code-legacy",
			Replacement:    "patty-code-standard",
			Reason:         "PII leak",
		},
	})
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-legacy")
	if dec.Allow {
		t.Fatal("dispatch must be blocked for recalled model")
	}
	if dec.BlockedBy != "model-recall" {
		t.Errorf("blocked by = %s, want model-recall", dec.BlockedBy)
	}
	if !strings.Contains(dec.ReasonKo, "회수") {
		t.Errorf("Korean message missing recall word: %q", dec.ReasonKo)
	}
	if !strings.Contains(dec.ReasonKo, "patty-code-standard") {
		t.Errorf("Korean message missing replacement: %q", dec.ReasonKo)
	}
}

// TestGatesAllowOnUnrecalledModel covers D6: a non-recalled
// model passes.
func TestGatesAllowOnUnrecalledModel(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetRecalls([]ModelRecall{{RecalledModel: "patty-code-legacy"}})
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if !dec.Allow {
		t.Errorf("unrecalled model must pass, got %s", dec.Reason)
	}
}

// TestGatesBlockOnChangeFreeze covers D3: a write against a
// frozen repo is refused, but read/review/test pass.
func TestGatesBlockOnChangeFreeze(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetFreeze(&ChangeFreeze{
		OrganizationID:  "org-test",
		Reason:         "Q4 release freeze",
		ReasonKo:       "4분기 릴리스 동결",
		AffectedRepos:  []string{"pccp"},
		AllowedActions: nil, // default: only read/review/test
		InitiatedBy:    "release-team",
		StartedAt:      time.Now(),
	})
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if dec.Allow {
		t.Fatal("dispatch must be blocked during freeze")
	}
	if dec.BlockedBy != "change-freeze" {
		t.Errorf("blocked by = %s, want change-freeze", dec.BlockedBy)
	}
	if !strings.Contains(dec.ReasonKo, "동결") {
		t.Errorf("Korean message missing freeze word: %q", dec.ReasonKo)
	}

	// Read/review/test should pass under the freeze.
	for _, action := range []string{"read", "review", "test"} {
		if dec := g.CheckDispatch(action, "pccp", "patty-code-standard"); !dec.Allow {
			t.Errorf("%s must pass during freeze, got %s", action, dec.Reason)
		}
	}
}

// TestGatesAllowOnUnfrozenRepo covers D3: a write against an
// unfrozen repo passes the freeze check.
func TestGatesAllowOnUnfrozenRepo(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetFreeze(&ChangeFreeze{
		OrganizationID: "org-test",
		AffectedRepos: []string{"pccp"},
	})
	dec := g.CheckDispatch("file_write", "other-repo", "patty-code-standard")
	if !dec.Allow {
		t.Errorf("unfrozen repo must pass, got %s", dec.Reason)
	}
}

// TestGatesBlockOnMissingAcknowledgement covers D1: a blocking
// policy epoch is refused until acked.
func TestGatesBlockOnMissingAcknowledgement(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetAcknowledgements([]AcknowledgementRequirement{
		{
			OrganizationID: "org-test",
			PolicyEpochID:  "epoch-2026-01",
			SummaryKo:      "신규 데이터 처리 정책에 동의해주세요",
			Blocking:       true,
		},
	})
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if dec.Allow {
		t.Fatal("dispatch must be blocked until acknowledgement")
	}
	if dec.BlockedBy != "policy-acknowledgement" {
		t.Errorf("blocked by = %s, want policy-acknowledgement", dec.BlockedBy)
	}

	// Acknowledge the policy, dispatch should pass.
	if err := g.AcknowledgePolicy("epoch-2026-01", 1_700_000_000_000); err != nil {
		t.Fatalf("ack: %v", err)
	}
	dec = g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if !dec.Allow {
		t.Errorf("acked dispatch must pass, got %s", dec.Reason)
	}
}

// TestGatesAllowOnNonBlockingAcknowledgement covers D1: a
// non-blocking ack is informational and does not gate dispatch.
func TestGatesAllowOnNonBlockingAcknowledgement(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetAcknowledgements([]AcknowledgementRequirement{
		{
			OrganizationID: "org-test",
			PolicyEpochID:  "epoch-2026-01",
			SummaryKo:      "informational only",
			Blocking:       false,
		},
	})
	dec := g.CheckDispatch("file_write", "pccp", "patty-code-standard")
	if !dec.Allow {
		t.Errorf("non-blocking ack must not gate dispatch, got %s", dec.Reason)
	}
}

// TestGatesAcknowledgePolicyUnknownFails covers the trivial
// boundary.
func TestGatesAcknowledgePolicyUnknownFails(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	if err := g.AcknowledgePolicy("epoch-unknown", 1); err == nil {
		t.Fatal("unknown ack must fail")
	}
}

// TestChangeFreezeAllowsReadReviewTest covers D3 read-side
// semantics: even during a freeze, the developer can still
// review and test.
func TestChangeFreezeAllowsReadReviewTest(t *testing.T) {
	f := &ChangeFreeze{}
	for _, action := range []string{"read", "review", "test"} {
		if !f.AllowsAction(action) {
			t.Errorf("freeze must allow %s", action)
		}
	}
	for _, action := range []string{"file_write", "tool_use", "mcp_call", "network_dial"} {
		if f.AllowsAction(action) {
			t.Errorf("freeze must block %s", action)
		}
	}
}

// TestChangeFreezeAllowedActions overrides the default read-only
// behavior: a freeze that lists `tool_use` in AllowedActions lets
// tool_use through.
func TestChangeFreezeAllowedActions(t *testing.T) {
	f := &ChangeFreeze{AllowedActions: []string{"tool_use"}}
	if !f.AllowsAction("tool_use") {
		t.Error("allowed action must pass")
	}
	if f.AllowsAction("file_write") {
		t.Error("non-allowed action must fail")
	}
}

// TestChangeFreezeAllowsActionWildcard covers the all-allow
// escape hatch.
func TestChangeFreezeAllowsActionWildcard(t *testing.T) {
	f := &ChangeFreeze{AllowedActions: []string{"*"}}
	for _, action := range []string{"file_write", "tool_use", "mcp_call"} {
		if !f.AllowsAction(action) {
			t.Errorf("wildcard allow must permit %s", action)
		}
	}
}

// TestChangeFreezeAffectsRepo covers D3 repo coverage.
func TestChangeFreezeAffectsRepo(t *testing.T) {
	f := &ChangeFreeze{AffectedRepos: []string{"pccp", "patty"}}
	if !f.Affects("pccp") {
		t.Error("pccp must be affected")
	}
	if !f.Affects("patty") {
		t.Error("patty must be affected")
	}
	if f.Affects("other") {
		t.Error("other must not be affected")
	}
}

// TestVersionComparison exercises D5 version semantics.
func TestVersionComparison(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0", "1.0.0", -1}, // missing segment treated as 0
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestCheckCodingStandard covers D4: the harness consults the
// cached standards before accepting a write.
func TestCheckCodingStandard(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetStandards([]CodingStandard{
		{
			RuleID:        "no-fmt-println",
			Severity:      SeverityBlock,
			Description:  "Use a structured logger instead of fmt.Println",
			DescriptionKo: "fmt.Println 대신 구조화된 로거를 사용하세요",
			BlockPattern:  "fmt.Println",
		},
	})
	std := g.CheckCodingStandard("main.go", "func main() { fmt.Println(\"hi\") }")
	if std == nil {
		t.Fatal("standard must match")
	}
	if std.RuleID != "no-fmt-println" {
		t.Errorf("rule id = %s, want no-fmt-println", std.RuleID)
	}
	if !strings.Contains(std.DescriptionKo, "fmt.Println") {
		t.Errorf("Korean description missing pattern: %q", std.DescriptionKo)
	}

	// File without the pattern matches no standard.
	if std := g.CheckCodingStandard("clean.go", "package main"); std != nil {
		t.Errorf("clean file must match no standard, got %v", std)
	}
}

// TestCheckCodingStandardEmptyPattern guards the trivial
// boundary: an empty BlockPattern matches nothing (a rule without
// a pattern is a configuration error; admin UI should reject).
func TestCheckCodingStandardEmptyPattern(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetStandards([]CodingStandard{
		{RuleID: "all", BlockPattern: ""}, // empty pattern
	})
	if std := g.CheckCodingStandard("clean.go", "package main"); std != nil {
		t.Errorf("empty pattern must not match, got %v", std)
	}
}

// TestGatesClientConcurrentCheckDispatch guards the lock boundary.
func TestGatesClientConcurrentCheckDispatch(t *testing.T) {
	g := NewGatesClient("org-test", "2.0.0", "stable")
	g.SetFreeze(&ChangeFreeze{AffectedRepos: []string{"pccp"}})
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			_ = g.CheckDispatch("file_write", "pccp", "m")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
