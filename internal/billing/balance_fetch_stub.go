//go:build !profile_public

package billing

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// ErrBalanceUnavailable is returned by the stub twins outside the public
// profile (ADR G4). In practice it is unreachable: non-public tier locks
// reject any configured balance_url at boot, so callers reach this only with
// an empty url, which they short-circuit to (nil, nil) themselves.
var ErrBalanceUnavailable = errors.New("provider balance fetching is not available in this build profile")

// Fetch mirrors the public profile's empty-url short-circuit: a blank URL
// is "not configured", not an error, so callers can use the same nil/nil
// branch they take on the public side. With a non-empty URL the stub
// returns ErrBalanceUnavailable because the network call cannot happen.
func Fetch(ctx context.Context, url, apiKey string) (*Balance, error) {
	if strings.TrimSpace(url) == "" {
		return nil, nil
	}
	return nil, ErrBalanceUnavailable
}

func FetchWithClient(ctx context.Context, client *http.Client, url, apiKey string) (*Balance, error) {
	if strings.TrimSpace(url) == "" {
		return nil, nil
	}
	return nil, ErrBalanceUnavailable
}
