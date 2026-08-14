package boot

import (
	"os"
	"testing"

	"patty/internal/testenv"
)

func TestMain(m *testing.M) {
	// Boot tests instantiate providers from `kind = "openai"` test configs
	// to validate the lifecycle flow without dialing a real PAPER relay. The
	// PRD-mandated PAPER-only policy blocks those generic kinds unless
	// PATTY_ALLOW_GENERIC=1 is set, so the test binary always opts in.
	os.Setenv("PATTY_ALLOW_GENERIC", "1")
	testenv.RunWithIsolatedUserState(m)
}
