package tier

import (
	"fmt"
	"strings"
)

// LockInput is the profile-relevant config surface boot extracts right
// after config load. Names identify the offending [providers] entries in
// the error copy.
type LockInput struct {
	// ExcludedProviders names provider entries that the linked profile
	// rejects. Two categories flow into the same list so the operator can
	// see every fix-up in one boot attempt:
	//   - generic kinds the profile excludes (non-public builds)
	//   - entries with a Name but no Kind set, which would otherwise
	//     fail later at provider.New with the opaque "unknown kind"
	ExcludedProviders []string
	// BalanceProviders names provider entries carrying a balance_url
	// where the profile excludes balance fetching.
	BalanceProviders []string
}

// Lock fails closed when the linked profile excludes a capability the
// config tries to enable (ADR decision 2). boot.Build treats an error as
// a hard startup failure. Telemetry is intentionally not validated here:
// it is compiled out at the egress (post/Enabled), so no config value can
// enable it — erroring on a defaulted mode would brick sovereign boots.
//
// All reported problems are listed in a single error so the operator
// sees every fix-up they need to make in one pass, not in repeated
// edit/restart cycles.
func Lock(in LockInput) error {
	var b strings.Builder
	wrote := false
	if len(in.ExcludedProviders) > 0 && !Default.Allows(CapGenericProviders) {
		fmt.Fprintf(&b, " generic providers %v are excluded — remove them from [providers] or use the DARI relay;", in.ExcludedProviders)
		wrote = true
	}
	if len(in.BalanceProviders) > 0 && !Default.Allows(CapBalanceFetch) {
		fmt.Fprintf(&b, " provider balance_url is excluded — remove it from %v;", in.BalanceProviders)
		wrote = true
	}
	if !wrote {
		return nil
	}
	return fmt.Errorf("tier lock (%s profile):%s", Default, b.String())
}
