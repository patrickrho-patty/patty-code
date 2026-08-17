//go:build profile_sovereign

package telemetry

import (
	"context"

	"patty/internal/tier"
)

// post is the sovereign no-op egress twin (ADR G2): sovereign builds
// contain no vendor telemetry endpoint and never dial out. Enabled() also
// returns false, so this is defense in depth — no payload is ever built.
func (c *Client) post(ctx context.Context, path string, payload any) error {
	tier.AssertDisallowed(tier.CapVendorTelemetry)
	_ = ctx
	_ = path
	_ = payload
	return nil
}
