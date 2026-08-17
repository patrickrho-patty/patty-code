package boot

import (
	"os"
	"testing"

	"patty/internal/testenv"
)

func TestMain(m *testing.M) {
	// Fixtures contain DLP trigger phrases; the wrapper is tested in internal/dlp.
	os.Setenv("PATTY_DLP_ENABLED", "0")
	testenv.RunWithIsolatedUserState(m)
}
