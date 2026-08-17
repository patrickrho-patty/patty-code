// Package billing renders a provider's wallet balance for the status line.
// Balance is strictly optional: a provider with no balance_url is never
// queried — callers pass "" and get (nil, nil) back, and surfaces simply omit
// the readout. The display half (Balance/Info and the helpers below) compiles
// into every profile; the network fetch is a public-profile capability and
// lives in balance_fetch.go / balance_fetch_stub.go (ADR G4).
package billing

import (
	"strings"
)

// Balance is a wallet balance normalized for display.
type Balance struct {
	Available bool   // the provider reports the account can still serve API calls
	Infos     []Info // one entry per currency the provider returns
}

// Info is one currency's balance (DeepSeek returns one per currency).
type Info struct {
	Currency        string // "KRW" | "USD"
	TotalBalance    string // total available (granted + topped-up)
	GrantedBalance  string // unexpired promotional credit
	ToppedUpBalance string // paid-in credit
}

// symbol maps an ISO currency code to a compact symbol; an unknown code passes
// through with a trailing space ("XYZ 12.00").
func symbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "KRW", "WON":
		return "₩"
	case "USD":
		return "$"
	default:
		if currency == "" {
			return ""
		}
		return currency + " "
	}
}

// Display renders the primary balance compactly, e.g. "₩110". It preserves
// the legacy KRW-first behavior for callers that have no display-currency
// preference.
func (b *Balance) Display() string {
	return b.DisplayForCurrency("")
}

// DisplayForCurrency renders the balance matching the requested pricing
// currency. When the provider does not return that currency, it falls back to
// Display's legacy KRW-first selection and prefixes the provider's real ISO
// currency (for example "KRW ₩70.16"); it never performs an implicit
// exchange-rate conversion.
func (b *Balance) DisplayForCurrency(currency string) string {
	if b == nil || len(b.Infos) == 0 {
		return ""
	}
	pick := b.Infos[0]
	preferred := normalizeCurrency(currency)
	if preferred != "" {
		for _, i := range b.Infos {
			if normalizeCurrency(i.Currency) == preferred {
				return symbol(i.Currency) + formatBalanceAmount(i.Currency, i.TotalBalance)
			}
		}
	}
	for _, i := range b.Infos {
		if normalizeCurrency(i.Currency) == "KRW" {
			pick = i
			break
		}
	}
	display := symbol(pick.Currency) + formatBalanceAmount(pick.Currency, pick.TotalBalance)
	actual := normalizeCurrency(pick.Currency)
	if preferred != "" && actual != "" && actual != preferred {
		return actual + " " + display
	}
	return display
}

func normalizeCurrency(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "KRW", "WON", "₩":
		return "KRW"
	case "USD", "$", "US$":
		return "USD"
	default:
		return ""
	}
}

// formatBalanceAmount renders a balance amount for display. The Korean Won
// has no fractional subdivision, so a trailing ".00" (or ".0") from a
// provider that reports decimal strings is stripped for KRW specifically.
func formatBalanceAmount(currency, amount string) string {
	amount = strings.TrimSpace(amount)
	if normalizeCurrency(currency) == "KRW" {
		amount = strings.TrimSuffix(strings.TrimSuffix(amount, ".00"), ".0")
	}
	return amount
}
