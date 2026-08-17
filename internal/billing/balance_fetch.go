//go:build profile_public

package billing

// Provider balance APIs (GET /user/balance) are a public-profile BYOK
// capability (ADR G4). Enterprise chargeback is relay-side; sovereign
// builds contain no foreign endpoints. The display half of this package
// (Balance/Info and its helpers, in balance.go) compiles everywhere; only
// the network path lives here.

import (
	"patty/internal/tier"

	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// deepseekResp mirrors the GET /user/balance response shape.
type deepseekResp struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

// httpClient bounds the balance query so a slow endpoint can't hang the status
// line; the per-call ctx still cancels it on shutdown.
var httpClient = &http.Client{Timeout: 12 * time.Second}

// Fetch queries url (a DeepSeek-style balance endpoint) with a Bearer apiKey and
// returns the normalized balance. An empty url yields (nil, nil) — "not
// configured", not an error — so callers can treat both the same and just omit
// the readout.
func Fetch(ctx context.Context, url, apiKey string) (*Balance, error) {
	tier.AssertAllowed(tier.CapBalanceFetch)
	return FetchWithClient(ctx, httpClient, url, apiKey)
}

// FetchWithClient queries the balance endpoint using the caller-provided client.
// A nil client falls back to the package default.
func FetchWithClient(ctx context.Context, client *http.Client, url, apiKey string) (*Balance, error) {
	if strings.TrimSpace(url) == "" {
		return nil, nil
	}
	if client == nil {
		client = httpClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("balance: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var dr deepseekResp
	if err := json.Unmarshal(body, &dr); err != nil {
		return nil, fmt.Errorf("balance: decode: %w", err)
	}
	b := &Balance{Available: dr.IsAvailable}
	for _, bi := range dr.BalanceInfos {
		b.Infos = append(b.Infos, Info{
			Currency:        bi.Currency,
			TotalBalance:    bi.TotalBalance,
			GrantedBalance:  bi.GrantedBalance,
			ToppedUpBalance: bi.ToppedUpBalance,
		})
	}
	return b, nil
}
