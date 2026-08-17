//go:build !profile_sovereign

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"patty/internal/tier"
)

// crashEndpoint is the vendor crash collector (public/enterprise; desktop
// telemetry opt-in or an explicit user click gates the call). Sovereign
// builds exclude this file (ADR G2).
var crashEndpoint = "https://crash.patty.io/v1/report"

func postCrashReport(ctx context.Context, c *http.Client, endpoint string, r crashReport) error {
	tier.AssertAllowed(tier.CapCrashUpload)
	body, err := json.Marshal(r)
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
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("crash endpoint returned %s", resp.Status)
	}
	return nil
}
