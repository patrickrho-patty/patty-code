package guardian

import (
	"os"
	"testing"
)

// TestMain disables the agent DLP wrapper for this package's tests:
// the guardian fixtures deliberately contain the detector's trigger
// phrases (jailbreak/DAN/injection patterns) to test the guardian's
// own untrusted-input handling. The DLP suite exercises the wrapper
// directly in internal/dlp.
func TestMain(m *testing.M) {
	os.Setenv("PATTY_DLP_ENABLED", "0")
	os.Exit(m.Run())
}
