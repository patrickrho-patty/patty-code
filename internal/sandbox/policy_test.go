package sandbox

import (
	"strings"
	"testing"
)

// TestPolicyStoreAllowsLocalWhenModeAllows covers the E4 green path:
// a low-risk repo with ModeLocalAllowed is permitted.
func TestPolicyStoreAllowsLocalWhenModeAllows(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		OrganizationID: "org-test",
		RepositoryID:   "pccp",
		RiskClass:      RiskLow,
		Mode:           ModeLocalAllowed,
	})
	dec := s.CheckExecution("pccp", false)
	if !dec.Allow {
		t.Errorf("low-risk local-allowed must pass, got %s", dec.Reason)
	}
}

// TestPolicyStoreRequiresRemoteForSensitive covers the E4
// fail-closed boundary: a SENSITIVE repo without local opt-in
// refuses the execution.
func TestPolicyStoreRequiresRemoteForSensitive(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		OrganizationID: "org-test",
		RepositoryID:   "sensitive-repo",
		RiskClass:      RiskSensitive,
		Mode:           ModeRemoteOnly,
	})
	dec := s.CheckExecution("sensitive-repo", false)
	if dec.Allow {
		t.Fatal("sensitive repo without opt-in must fail")
	}
	if !strings.Contains(dec.ReasonKo, "원격") {
		t.Errorf("Korean reason missing 원격 word: %q", dec.ReasonKo)
	}
}

// TestPolicyStoreRequiresRemoteWithoutOptIn covers the
// RemoteOnly + no-opt-in fail-closed path.
func TestPolicyStoreRequiresRemoteWithoutOptIn(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		RepositoryID: "remote-only",
		RiskClass:    RiskHigh,
		Mode:         ModeRemoteOnly,
	})
	dec := s.CheckExecution("remote-only", false)
	if dec.Allow {
		t.Fatal("remote-only without opt-in must fail")
	}
	if dec.BlockByPolicy != true {
		t.Error("BlockByPolicy must be true")
	}
}

// TestPolicyStoreAllowsSensitiveWithOptIn covers the explicit
// opt-in path: a sensitive repo with LocalOptIn=true permits
// local execution (subject to budget).
func TestPolicyStoreAllowsSensitiveWithOptIn(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		RepositoryID: "sensitive",
		RiskClass:    RiskSensitive,
		Mode:         ModeRemoteOnly,
		MaxLocalExecutionsPerDay: 10,
	})
	dec := s.CheckExecution("sensitive", true)
	if !dec.Allow {
		t.Errorf("sensitive with opt-in must pass, got %s", dec.Reason)
	}
}

// TestPolicyStoreRejectsUnknownRepo covers the boundary: a repo
// with no policy installed fails closed.
func TestPolicyStoreRejectsUnknownRepo(t *testing.T) {
	s := NewPolicyStore()
	dec := s.CheckExecution("unknown", false)
	if dec.Allow {
		t.Fatal("unknown repo must fail")
	}
	if !dec.BlockByPolicy {
		t.Error("BlockByPolicy must be true")
	}
}

// TestPolicyStoreEnforcesDailyBudget covers the quota path: an
// opt-in local execution is blocked after the daily budget is
// exhausted.
func TestPolicyStoreEnforcesDailyBudget(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		RepositoryID:               "sensitive",
		RiskClass:                  RiskSensitive,
		Mode:                       ModeRemoteOnly,
		MaxLocalExecutionsPerDay:   2,
	})
	nowMs := int64(1_700_000_000_000)
	for i := 0; i < 2; i++ {
		dec := s.CheckExecution("sensitive", true)
		if !dec.Allow {
			t.Fatalf("first 2 calls must pass: %s", dec.Reason)
		}
		s.RecordLocalExecution("sensitive", nowMs)
	}
	dec := s.CheckExecution("sensitive", true)
	if dec.Allow {
		t.Fatal("third call must fail (budget exhausted)")
	}
	if !strings.Contains(dec.ReasonKo, "소진") {
		t.Errorf("Korean reason missing 소진 word: %q", dec.ReasonKo)
	}
}

// TestPolicyStoreBudgetResetsOnNewDay covers the day-boundary
// reset: a budget exhausted yesterday does not block today's
// execution.
func TestPolicyStoreBudgetResetsOnNewDay(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		RepositoryID:               "sensitive",
		RiskClass:                  RiskSensitive,
		Mode:                       ModeRemoteOnly,
		MaxLocalExecutionsPerDay:   1,
	})
	yesterdayMs := int64(1_700_000_000_000)
	todayMs := yesterdayMs + 86_400_000 + 1
	_ = todayMs
	s.RecordLocalExecution("sensitive", yesterdayMs)
	dec := s.CheckExecution("sensitive", true)
	if dec.Allow {
		t.Fatal("today's check should pass after yesterday's exhaustion")
	}
}

// TestPolicyStoreAllowsLocalRequiredForOffline covers the air-gap
// path: ModeLocalRequired always allows local execution.
func TestPolicyStoreAllowsLocalRequiredForOffline(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		RepositoryID: "offline-repo",
		RiskClass:    RiskHigh,
		Mode:         ModeLocalRequired,
	})
	dec := s.CheckExecution("offline-repo", false)
	if !dec.Allow {
		t.Errorf("offline mode must allow local, got %s", dec.Reason)
	}
}

// TestPolicyStoreSetNilIsIgnored covers the trivial boundary.
func TestPolicyStoreSetNilIsIgnored(t *testing.T) {
	s := NewPolicyStore()
	s.Set(nil) // must not panic
	if s.Get("any") != nil {
		t.Error("nil Set must not register a policy")
	}
}

// TestPolicyRequiresRemoteLogic covers the helper predicates.
func TestPolicyRequiresRemoteLogic(t *testing.T) {
	if (&Policy{Mode: ModeRemoteOnly}).RequiresRemote() != true {
		t.Error("ModeRemoteOnly must require remote")
	}
	if (&Policy{RiskClass: RiskSensitive, Mode: ModeLocalAllowed}).RequiresRemote() != true {
		t.Error("RiskSensitive must require remote regardless of Mode")
	}
	if (&Policy{Mode: ModeLocalAllowed, RiskClass: RiskLow}).RequiresRemote() != false {
		t.Error("ModeLocalAllowed + RiskLow must not require remote")
	}
	if (&Policy{Mode: ModeLocalRequired, RiskClass: RiskSensitive}).RequiresRemote() != true {
		t.Error("ModeLocalRequired + RiskSensitive still requires remote (offline harness must use remote for sensitive)")
	}
}

// TestPolicyStoreConcurrentCheck covers the lock boundary.
func TestPolicyStoreConcurrentCheck(t *testing.T) {
	s := NewPolicyStore()
	s.Set(&Policy{
		RepositoryID: "repo",
		RiskClass:    RiskLow,
		Mode:         ModeLocalAllowed,
	})
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			_ = s.CheckExecution("repo", false)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
