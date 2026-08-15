package acp

import (
	"os"
	"testing"
)

// TestMain disables the agent DLP wrapper for this package's tests:
// fixtures contain detector trigger phrases. See internal/guardian's
// TestMain note.
func TestMain(m *testing.M) {
	os.Setenv("PATTY_DLP_ENABLED", "0")
	os.Exit(m.Run())
}
