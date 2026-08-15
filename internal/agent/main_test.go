package agent

import (
	"context"
	"os"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Fixtures contain DLP trigger phrases; the wrapper is tested in internal/dlp.
	os.Setenv("PATTY_DLP_ENABLED", "0")
	// Stream body retries use multi-second backoff in production; collapse it
	// in package tests so recovery suites stay deterministic and fast.
	streamRetrySleep = func(ctx context.Context, _ int) bool {
		return ctx.Err() == nil
	}
	goleak.VerifyTestMain(m)
}
