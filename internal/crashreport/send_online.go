//go:build !profile_sovereign

package crashreport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"patty/internal/netclient"
	"patty/internal/tier"
)

// reportEndpoint is the vendor crash collector (public/enterprise; consent
// or org policy gates the call). Sovereign builds exclude this file (ADR G2).
const reportEndpoint = "https://crash.patty.io/v1/report"

// Send uploads a single user-reviewed report. It does not remove local state;
// callers remove the report only after a successful response.
func Send(ctx context.Context, report Report, proxy netclient.ProxySpec) error {
	tier.AssertAllowed(tier.CapCrashUpload)
	client, err := netclient.NewHTTPClient(proxy, netclient.TransportOptions{
		DialTimeout:           3 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	client.Timeout = 10 * time.Second
	return sendWithClient(ctx, client, reportEndpoint, report)
}

func sendWithClient(ctx context.Context, client *http.Client, endpoint string, report Report) error {
	body, err := json.Marshal(sanitizeReport(report))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("crash endpoint returned %s", resp.Status)
	}
	return nil
}
