//go:build !profile_sovereign

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"patty/internal/tier"
)

// pingEndpoint is the anonymous launch-ping collector. Sovereign builds
// exclude this file entirely (ADR G2): the presence of the endpoint is
// itself a procurement finding.
var pingEndpoint = "https://crash.patty.io/v1/ping"

// postStartupPing ships the launch ping (public/enterprise only).
func postStartupPing(ctx context.Context, c *http.Client, endpoint string, p startupPing) error {
	tier.AssertAllowed(tier.CapVendorTelemetry)
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
