//go:build !profile_sovereign

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"patty/internal/tier"
)

// endpoint is the vendor telemetry collector. Sovereign builds exclude this
// file entirely (ADR G2): the presence of the endpoint is itself a
// procurement finding.
const endpoint = "https://crash.patty.io/v1"

// post ships a payload to the vendor endpoint (public/enterprise only).
func (c *Client) post(ctx context.Context, path string, payload any) error {
	tier.AssertAllowed(tier.CapVendorTelemetry)
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telemetry: HTTP %d", resp.StatusCode)
	}
	return nil
}
