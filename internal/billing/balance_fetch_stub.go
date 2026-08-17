//go:build !profile_public

package billing

import (
	"context"
	"errors"
	"net/http"
)

// ErrBalanceUnavailable is returned by the stub twins outside the public
// profile (ADR G4). In practice it is unreachable: non-public tier locks
// reject any configured balance_url at boot, so callers reach this only with
// an empty url, which they short-circuit to (nil, nil) themselves.
var ErrBalanceUnavailable = errors.New("provider balance fetching is not available in this build profile")

func Fetch(ctx context.Context, url, apiKey string) (*Balance, error) {
	return nil, ErrBalanceUnavailable
}

func FetchWithClient(ctx context.Context, client *http.Client, url, apiKey string) (*Balance, error) {
	return nil, ErrBalanceUnavailable
}
