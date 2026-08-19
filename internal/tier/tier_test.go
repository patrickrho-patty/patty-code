package tier

import (
	"strings"
	"testing"
)

func TestAllowsMatchesADRProfileTable(t *testing.T) {
	cases := []struct {
		tier Tier
		caps map[Capability]bool
	}{
		{Public, map[Capability]bool{
			CapGenericProviders: false, CapPublicPresets: false, CapBalanceFetch: false,
			CapVendorTelemetry: true, CapCrashUpload: true, CapOnlineUpdate: true,
		}},
		{Enterprise, map[Capability]bool{
			CapGenericProviders: false, CapPublicPresets: false, CapBalanceFetch: false,
			CapVendorTelemetry: true, CapCrashUpload: true, CapOnlineUpdate: true,
		}},
		{Sovereign, map[Capability]bool{
			CapGenericProviders: false, CapPublicPresets: false, CapBalanceFetch: false,
			CapVendorTelemetry: false, CapCrashUpload: false, CapOnlineUpdate: false,
		}},
	}
	all := []Capability{CapGenericProviders, CapPublicPresets, CapBalanceFetch,
		CapVendorTelemetry, CapCrashUpload, CapOnlineUpdate}
	for _, tc := range cases {
		for _, cap := range all {
			if got, want := tc.tier.Allows(cap), tc.caps[cap]; got != want {
				t.Errorf("%s.Allows(%s) = %v, want %v", tc.tier, cap, got, want)
			}
		}
	}
}

func TestTierString(t *testing.T) {
	for tier, want := range map[Tier]string{Public: "public", Enterprise: "enterprise", Sovereign: "sovereign"} {
		if got := tier.String(); got != want {
			t.Errorf("Tier(%d).String() = %q, want %q", tier, got, want)
		}
	}
}

func TestLockFailsClosedOnExcludedCapabilities(t *testing.T) {
	// Lock is tier-driven, not Default-driven: test each tier's rules directly
	// by simulating its allow set via the exported rules in lock.go.
	// 2026-08-19 amendment: every Default (public included) fails closed.
	if err := Lock(LockInput{}); err != nil {
		t.Fatalf("empty input must lock clean under %s: %v", Default, err)
	}
	err := Lock(LockInput{ExcludedProviders: []string{"x"}, BalanceProviders: []string{"y"}})
	if err == nil {
		t.Fatalf("excluded providers/balances must fail under %s", Default)
	}
}

// TestLockReportsAllProblemsInOneError pins ADR decision 2's fail-closed
// contract: when several
// classes of problem are present at once, Lock returns a single error
// listing each one. Under sovereign every class is a hard failure; the
// operator must see every fix-up in one boot attempt.
func TestLockReportsAllProblemsInOneError(t *testing.T) {
	// 2026-08-19 amendment: no profile allows generic providers, so the
	// multi-class lock error is reachable from every Default.
	err := Lock(LockInput{
		ExcludedProviders: []string{"alpha", "gamma (empty kind)"},
		BalanceProviders:  []string{"beta"},
	})
	if err == nil {
		t.Fatal("expected combined error listing every excluded class")
	}
	msg := err.Error()
	for _, fragment := range []string{"alpha", "gamma (empty kind)", "beta"} {
		if !strings.Contains(msg, fragment) {
			t.Errorf("combined error %q missing %q", msg, fragment)
		}
	}
}

// TestLockFailsClosedOnEveryDefault pins the amendment's fail-closed
// surface directly: excluded providers and balance URLs are hard lock
// failures regardless of the linked Default.
func TestLockFailsClosedOnEveryDefault(t *testing.T) {
	if err := Lock(LockInput{ExcludedProviders: []string{"x"}}); err == nil {
		t.Fatalf("generic providers must fail the lock under %s", Default)
	}
	if err := Lock(LockInput{BalanceProviders: []string{"y"}}); err == nil {
		t.Fatalf("balance providers must fail the lock under %s", Default)
	}
}

// TestLockSkipsAllowedClasses was removed by the 2026-08-19 DARI-only
// amendment: no tier allows CapBalanceFetch anymore, so the "allowed
// class stays non-failing" corner case no longer exists.
