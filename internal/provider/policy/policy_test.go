package policy

import (
	"testing"

	"patty/internal/tier"
)

// TestIsBlockedKindFollowsBuildProfile pins the ADR G4 gate: every generic
// kind is allowed exactly when the linked build profile allows it, with no
// env hatch — DARI and unknown kinds are never blocked (they fail later in
// the registry, not here).
func TestIsBlockedKindFollowsBuildProfile(t *testing.T) {
	generic := tier.Default.Allows(tier.CapGenericProviders)
	for _, kind := range []string{"openai", "anthropic", "responses", "dashscope-responses"} {
		if got := IsBlockedKind(kind); got == generic {
			t.Errorf("IsBlockedKind(%q) = %v under %s profile (ADR G4)", kind, got, tier.Default)
		}
	}
	for _, kind := range []string{"dari", "", "openai ", "OpenAI", "some-future-kind"} {
		if IsBlockedKind(kind) {
			t.Errorf("IsBlockedKind(%q) must be false — non-generic kinds are not policy's concern", kind)
		}
	}
}
