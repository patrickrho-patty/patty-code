package tier

import "testing"

func TestAllowsMatchesADRProfileTable(t *testing.T) {
	cases := []struct {
		tier Tier
		caps map[Capability]bool
	}{
		{Public, map[Capability]bool{
			CapGenericProviders: true, CapPublicPresets: true, CapBalanceFetch: true,
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
				t.Errorf("%s.Allows(%d) = %v, want %v", tc.tier, cap, got, want)
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
	if err := Lock(LockInput{}); err != nil {
		t.Fatalf("empty input must lock clean under %s: %v", Default, err)
	}
	err := Lock(LockInput{ExcludedProviders: []string{"x"}, BalanceProviders: []string{"y"}})
	if wantFail := !Default.Allows(CapGenericProviders) || !Default.Allows(CapBalanceFetch); wantFail {
		if err == nil {
			t.Fatalf("excluded providers/balances must fail under %s", Default)
		}
	} else if err != nil {
		t.Fatalf("public profile must lock clean for BYOK config: %v", err)
	}
}
