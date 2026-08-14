package anthropic

import (
	"os"
	"testing"
)

// TestMain opts the anthropic test binary into the developer escape hatch
// (PATTY_ALLOW_GENERIC=1) so the suite can exercise the generic
// OpenAI/Anthropic-compatible provider registry directly. The official
// Harness enforces the PAPER-only policy at runtime; tests bypass the gate
// intentionally to validate the wired code paths.
func TestMain(m *testing.M) {
	os.Setenv("PATTY_ALLOW_GENERIC", "1")
	os.Exit(m.Run())
}
