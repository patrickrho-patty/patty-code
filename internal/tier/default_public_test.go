//go:build profile_public

package tier

import "testing"

func TestDefaultTierIsPublic(t *testing.T) {
	if Default != Public {
		t.Fatalf("Default = %v, want Public", Default)
	}
}

// TestAssertAllowedPanicsUnderDARIOnly pins the audit-trail guarantee
// after the 2026-08-19 DARI-only amendment: profile_public no longer
// allows CapGenericProviders, so AssertAllowed must panic if some
// build-tag-excluded generic file is ever consulted from a public
// build. AssertDisallowed is the quiet path now — it must not panic.
func TestAssertAllowedPanicsUnderDARIOnly(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("AssertAllowed(CapGenericProviders) did not panic under profile_public — audit-trail regression")
		}
	}()
	AssertAllowed(CapGenericProviders)
}

func TestAssertDisallowedQuietUnderDARIOnly(t *testing.T) {
	// Must not panic: public disallows generics, so the offline stub's
	// assertion holds.
	AssertDisallowed(CapGenericProviders)
}
