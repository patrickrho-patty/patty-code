//go:build !profile_public

package billing

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// TestBalanceFetchFailsClosedOutsidePublicProfile pins the ADR G4 stub twins:
// outside the public profile the network path is compiled out entirely, so
// both entry points fail closed with ErrBalanceUnavailable — no dial, no
// dependency on DNS behavior for the invalid URL.
func TestBalanceFetchFailsClosedOutsidePublicProfile(t *testing.T) {
	if _, err := Fetch(context.Background(), "https://example.invalid", "k"); !errors.Is(err, ErrBalanceUnavailable) {
		t.Fatalf("Fetch must fail closed with ErrBalanceUnavailable outside the public profile, got %v", err)
	}
	if _, err := FetchWithClient(context.Background(), http.DefaultClient, "https://example.invalid", "k"); !errors.Is(err, ErrBalanceUnavailable) {
		t.Fatalf("FetchWithClient must return ErrBalanceUnavailable, got %v", err)
	}
}
