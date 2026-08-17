//go:build profile_sovereign

package main

import (
	"context"
	"net/http"
	"testing"
)

// ADR G2 in the desktop module: the vendor telemetry/metrics endpoints are
// compiled out of sovereign builds. These twins pin both the empty endpoint
// constants (binary-hygiene: no crash.patty.io string survives linking) and
// the no-op egress behavior.
func TestSovereignTelemetryTwinsAreNoOps(t *testing.T) {
	if pingEndpoint != "" {
		t.Fatalf("pingEndpoint = %q, want empty in sovereign builds", pingEndpoint)
	}
	if metricsEndpoint != "" {
		t.Fatalf("metricsEndpoint = %q, want empty in sovereign builds", metricsEndpoint)
	}
	if err := postStartupPing(context.Background(), http.DefaultClient, pingEndpoint, startupPing{}); err != nil {
		t.Fatalf("postStartupPing no-op returned %v", err)
	}
	app := NewApp()
	if !app.postMetrics(metricsPayload{}) {
		t.Fatal("postMetrics no-op must report success so pending counters are dropped, not retained")
	}
	// The crash queue is preserved (not wiped) when telemetry defaults off:
	// local list/show/delete survive per ADR G2 — pinned by
	// crash_post_sovereign_test.go's TestSovereignFlushPendingCrashKeepsFile.
}
