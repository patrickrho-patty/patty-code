package billing

import (
	"testing"
)

// symbol

func TestSymbolKRW(t *testing.T) {
	if got := symbol("KRW"); got != "₩" {
		t.Errorf("symbol(KRW) = %q", got)
	}
}

func TestSymbolWON(t *testing.T) {
	if got := symbol("WON"); got != "₩" {
		t.Errorf("symbol(WON) = %q", got)
	}
}

func TestNormalizeCurrencyKRW(t *testing.T) {
	for _, in := range []string{"KRW", "WON", "₩"} {
		if got := normalizeCurrency(in); got != "KRW" {
			t.Errorf("normalizeCurrency(%q) = %q, want KRW", in, got)
		}
	}
}

func TestSymbolUSD(t *testing.T) {
	if got := symbol("USD"); got != "$" {
		t.Errorf("symbol(USD) = %q", got)
	}
}

func TestSymbolUnknown(t *testing.T) {
	if got := symbol("EUR"); got != "EUR " {
		t.Errorf("symbol(EUR) = %q, want \"EUR \"", got)
	}
}

func TestSymbolEmpty(t *testing.T) {
	if got := symbol(""); got != "" {
		t.Errorf("symbol(\"\") = %q, want empty", got)
	}
}

func TestSymbolLowercase(t *testing.T) {
	// symbol should be case-insensitive.
	if got := symbol("usd"); got != "$" {
		t.Errorf("symbol(usd) = %q", got)
	}
}

// Display

func TestDisplayNil(t *testing.T) {
	var b *Balance
	if got := b.Display(); got != "" {
		t.Errorf("nil Display = %q", got)
	}
}

func TestDisplayEmptyInfos(t *testing.T) {
	b := &Balance{Available: true}
	if got := b.Display(); got != "" {
		t.Errorf("empty infos Display = %q", got)
	}
}

func TestDisplayPrefersKRW(t *testing.T) {
	b := &Balance{Infos: []Info{
		{Currency: "USD", TotalBalance: "10.00"},
		{Currency: "KRW", TotalBalance: "50.00"},
	}}
	if got := b.Display(); got != "₩50" {
		t.Errorf("Display = %q, want ₩50", got)
	}
}

func TestDisplayKRWStripsDecimals(t *testing.T) {
	b := &Balance{Infos: []Info{
		{Currency: "KRW", TotalBalance: "3200.00"},
	}}
	if got := b.Display(); got != "₩3200" {
		t.Errorf("Display = %q, want ₩3200", got)
	}
}

func TestDisplayFallsBackToFirst(t *testing.T) {
	b := &Balance{Infos: []Info{
		{Currency: "EUR", TotalBalance: "25.00"},
	}}
	if got := b.Display(); got != "EUR 25.00" {
		t.Errorf("Display = %q, want \"EUR 25.00\"", got)
	}
}

// Fetch edge cases live in balance_fetch_test.go (profile_public): the stub
// twins outside the public profile are covered by
// balance_fetch_stub_test.go.
