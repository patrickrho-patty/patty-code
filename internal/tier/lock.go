package tier

import (
	"fmt"
	"strings"
)

// LockInput is the profile-relevant config surface boot extracts right
// after config load. Names identify the offending [providers] entries in
// the error copy.
type LockInput struct {
	// ExcludedProviders names provider entries whose kind is excluded
	// from the linked profile (generic kinds outside public builds).
	ExcludedProviders []string
	// BalanceProviders names provider entries carrying a balance_url
	// where the profile excludes balance fetching.
	BalanceProviders []string
	// EmptyKindProviders names provider entries that have a Name but no
	// Kind set — they would fail later at provider.New with the opaque
	// "unknown kind" error, but flagging them at boot is more actionable.
	EmptyKindProviders []string
}

// Lock fails closed when the linked profile excludes a capability the
// config tries to enable (ADR decision 2). boot.Build treats an error as
// a hard startup failure. Telemetry is intentionally not validated here:
// it is compiled out at the egress (post/Enabled), so no config value can
// enable it — erroring on a defaulted mode would brick sovereign boots.
//
// All three classes of problem are reported in a single error so the
// operator sees every fix-up they need to make in one pass, not in
// repeated edit/restart cycles.
func Lock(in LockInput) error {
	if len(in.ExcludedProviders) == 0 &&
		len(in.BalanceProviders) == 0 &&
		len(in.EmptyKindProviders) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "tier lock (%s profile):", Default)
	if len(in.ExcludedProviders) > 0 && !Default.Allows(CapGenericProviders) {
		fmt.Fprintf(&b, " generic providers %v are excluded — remove them from [providers] or use the DARI relay;", in.ExcludedProviders)
	}
	if len(in.BalanceProviders) > 0 && !Default.Allows(CapBalanceFetch) {
		fmt.Fprintf(&b, " provider balance_url is excluded — remove it from %v;", in.BalanceProviders)
	}
	if len(in.EmptyKindProviders) > 0 {
		fmt.Fprintf(&b, " providers %v are missing a kind field — every [providers] entry needs kind = \"dari\" (or one of the generic kinds, where the profile allows them);", in.EmptyKindProviders)
	}
	return fmt.Errorf("%s", strings.TrimSuffix(b.String(), ";"))
}
