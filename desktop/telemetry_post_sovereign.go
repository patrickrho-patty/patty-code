//go:build profile_sovereign

package main

import (
	"context"
	"net/http"

	"patty/internal/tier"
)

// pingEndpoint has no value in sovereign builds (ADR G2): the vendor
// telemetry collector is compiled out, so the constant carries no host.
var pingEndpoint = ""

// postStartupPing is the sovereign no-op twin (ADR G2): nothing is
// uploaded, no dial is made. Callers treat the error as "ping failed",
// which is inert — the pending-ping machinery does not exist here either.
func postStartupPing(ctx context.Context, c *http.Client, endpoint string, p startupPing) error {
	tier.AssertDisallowed(tier.CapVendorTelemetry)
	_ = ctx
	_ = c
	_ = endpoint
	_ = p
	return nil
}
