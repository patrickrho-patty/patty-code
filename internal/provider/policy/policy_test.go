package policy

import (
	"testing"

	"patty/internal/tier"
)

// TestIsBlockedKindFollowsBuildProfile pins the ADR G4 gate: generic kinds
// are allowed exactly when the linked build profile allows them, with no env
// hatch — and DARI is never blocked.
func TestIsBlockedKindFollowsBuildProfile(t *testing.T) {
	blocked := IsBlockedKind("openai")
	if tier.Default.Allows(tier.CapGenericProviders) {
		if blocked {
			t.Fatal("public profile must allow generic providers by default (BYOK, ADR G4)")
		}
	} else if !blocked {
		t.Fatal("enterprise/sovereign profiles must block generic kinds with no env hatch")
	}
	if IsBlockedKind("dari") {
		t.Fatal("DARI kind is never blocked")
	}
}
