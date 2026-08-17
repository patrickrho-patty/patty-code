package boot

import (
	"os"
	"testing"

	"patty/internal/testenv"
)

func TestMain(m *testing.M) {
	// Fixtures contain DLP trigger phrases; the wrapper is tested in internal/dlp.
	os.Setenv("PATTY_DLP_ENABLED", "0")
	// PATTY_ALLOW_GENERIC=1 is a TEST-ONLY bridge kept until Task 6 stubs the
	// legacy presets: it suppresses boot's tierLockInput lock for the
	// legacy-migration fixtures (whose legacy DeepSeek entries carry
	// balance_url under the enterprise default). provider.IsBlockedKind
	// ignores it — the gate itself is compile-time since ADR G4.
	os.Setenv("PATTY_ALLOW_GENERIC", "1")
	testenv.RunWithIsolatedUserState(m)
}
