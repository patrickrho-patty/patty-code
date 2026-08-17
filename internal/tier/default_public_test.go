//go:build profile_public

package tier

import "testing"

func TestDefaultTierIsPublic(t *testing.T) {
	if Default != Public {
		t.Fatalf("Default = %v, want Public", Default)
	}
}

// TestAssertDisallowedPanicsOutsideItsBuildTag pins the audit-trail
// guarantee on the offline-stub side: a sovereign stub consulted from
// profile_public must panic because the profile already allows the
// capability. The twin file is excluded by build tag here, but invoking
// AssertDisallowed directly simulates the "wrong tag set" programmer
// error — the panic is the loud failure mode that surfaces it.
func TestAssertDisallowedPanicsOutsideItsBuildTag(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("AssertDisallowed(CapGenericProviders) did not panic under profile_public — audit-trail regression")
		}
	}()
	AssertDisallowed(CapGenericProviders)
}
