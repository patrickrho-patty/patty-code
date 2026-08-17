//go:build profile_sovereign

package tier

import "testing"

func TestDefaultTierIsSovereign(t *testing.T) {
	if Default != Sovereign {
		t.Fatalf("Default = %v, want Sovereign", Default)
	}
}

// TestAssertAllowedPanicsOutsideItsBuildTag pins the audit-trail guarantee
// on the online-twin side: a public/enterprise twin consulted from
// profile_sovereign must panic because the profile disallows the
// capability. The twin file is excluded by build tag here, but invoking
// AssertAllowed directly simulates the "wrong tag set" programmer error
// — the panic is the loud failure mode that surfaces it.
func TestAssertAllowedPanicsOutsideItsBuildTag(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("AssertAllowed(CapGenericProviders) did not panic under profile_sovereign — audit-trail regression")
		}
	}()
	AssertAllowed(CapGenericProviders)
}
