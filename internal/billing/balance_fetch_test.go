//go:build profile_public

package billing

// Network-path tests for Fetch/FetchWithClient (ADR G4): these exercise the
// real client compiled only into public builds; the fail-closed stub twin is
// covered by balance_fetch_stub_test.go.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A DeepSeek-shaped response parses, exposes Available, and Display prefers KRW
// with the right symbol; the request carries the bearer key.
func TestFetchDeepSeekShape(t *testing.T) {
	const body = `{
		"is_available": true,
		"balance_infos": [
			{"currency": "USD", "total_balance": "15.30", "granted_balance": "0.00", "topped_up_balance": "15.30"},
			{"currency": "KRW", "total_balance": "110.00", "granted_balance": "10.00", "topped_up_balance": "100.00"}
		]
	}`
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	b, err := Fetch(context.Background(), srv.URL, "secret-key")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if b == nil || !b.Available {
		t.Fatalf("want available balance, got %+v", b)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-key")
	}
	if len(b.Infos) != 2 {
		t.Fatalf("want 2 infos, got %d", len(b.Infos))
	}
	// Display prefers KRW → "₩110", not the first (USD) entry. The Won has no
	// fractional subdivision, so the trailing ".00" is stripped.
	if got := b.Display(); got != "₩110" {
		t.Errorf("Display = %q, want %q", got, "₩110")
	}
	if got := b.DisplayForCurrency("USD"); got != "$15.30" {
		t.Errorf("DisplayForCurrency(USD) = %q, want %q", got, "$15.30")
	}
	if got := b.DisplayForCurrency("₩"); got != "₩110" {
		t.Errorf("DisplayForCurrency(₩) = %q, want %q", got, "₩110")
	}
}

// An empty url is "not configured", not an error: (nil, nil), and Display on a nil
// balance is "".
func TestFetchEmptyURL(t *testing.T) {
	b, err := Fetch(context.Background(), "", "key")
	if err != nil || b != nil {
		t.Fatalf("Fetch(\"\") = (%v, %v), want (nil, nil)", b, err)
	}
	if got := b.Display(); got != "" {
		t.Errorf("nil Display = %q, want empty", got)
	}
}

// A non-200 surfaces an error rather than a bogus zero balance.
func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()
	if _, err := Fetch(context.Background(), srv.URL, "bad"); err == nil {
		t.Fatal("want error on 401, got nil")
	}
}

func TestFetchContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := Fetch(ctx, srv.URL, "key")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()
	_, err := Fetch(context.Background(), srv.URL, "key")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error should mention decode: %v", err)
	}
}

func TestFetchNoAPIKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"is_available":true,"balance_infos":[]}`))
	}))
	defer srv.Close()
	_, err := Fetch(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization should be empty when no key, got %q", gotAuth)
	}
}

func TestFetchWhitespaceURL(t *testing.T) {
	b, err := Fetch(context.Background(), "   ", "key")
	if err != nil || b != nil {
		t.Fatalf("whitespace URL should return (nil,nil), got (%v, %v)", b, err)
	}
}

func TestFetchServerUnavailable(t *testing.T) {
	// Use a URL that won't connect.
	_, err := Fetch(context.Background(), "http://127.0.0.1:1", "key")
	if err == nil {
		t.Fatal("expected error for unavailable server")
	}
}
