package control

import (
	"os"
	"testing"

	"patty/internal/testenv"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Fixtures contain DLP trigger phrases; the wrapper is tested in internal/dlp.
	os.Setenv("PATTY_DLP_ENABLED", "0")
	cleanupUserState, err := testenv.IsolateUserState()
	if err != nil {
		panic(err)
	}
	if os.Getenv("PATTY_CREDENTIALS_STORE") == "" {
		_ = os.Setenv("PATTY_CREDENTIALS_STORE", "file")
	}
	goleak.VerifyTestMain(m, goleak.Cleanup(func(exitCode int) {
		cleanupUserState()
		os.Exit(exitCode)
	}))
}
