package billing

import (
	"testing"
)

// Display falls back to the first currency when no KRW entry is present, and maps
// USD to "$".
func TestDisplayUSDOnly(t *testing.T) {
	b := &Balance{Available: true, Infos: []Info{{Currency: "USD", TotalBalance: "9.99"}}}
	if got := b.Display(); got != "$9.99" {
		t.Errorf("Display = %q, want %q", got, "$9.99")
	}
	if got := b.DisplayForCurrency("KRW"); got != "USD $9.99" {
		t.Errorf("DisplayForCurrency(KRW) = %q, want explicit real fallback currency %q", got, "USD $9.99")
	}
}
