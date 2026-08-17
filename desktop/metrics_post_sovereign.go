//go:build profile_sovereign

package main

import "patty/internal/tier"

// metricsEndpoint has no value in sovereign builds (ADR G2).
const metricsEndpoint = ""

// postMetrics is the sovereign no-op twin (ADR G2): nothing is uploaded.
// It reports success so flushMetrics drops the pending file instead of
// retaining local counters forever for an upload that can never happen.
func (a *App) postMetrics(p metricsPayload) bool {
	tier.AssertDisallowed(tier.CapVendorTelemetry)
	_ = p
	return true
}
