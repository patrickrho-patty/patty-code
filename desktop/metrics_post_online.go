//go:build !profile_sovereign

package main

import (
	"bytes"
	"encoding/json"
	"net/http"

	"patty/internal/tier"
)

// metricsEndpoint is the anonymous usage-metrics collector. Sovereign
// builds exclude this file entirely (ADR G2).
const metricsEndpoint = "https://crash.patty.io/v1/metrics"

// postMetrics ships the usage counters (public/enterprise only).
func (a *App) postMetrics(p metricsPayload) bool {
	tier.AssertAllowed(tier.CapVendorTelemetry)
	body, err := json.Marshal(p)
	if err != nil {
		return false
	}
	c, err := httpClient()
	if err != nil {
		return false
	}
	c.Timeout = metricsPostTimeout
	req, err := http.NewRequestWithContext(a.bootContext(), http.MethodPost, metricsEndpoint, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
