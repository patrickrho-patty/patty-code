package tier

import "fmt"

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
}

// Lock fails closed when the linked profile excludes a capability the
// config tries to enable (ADR decision 2). boot.Build treats an error as
// a hard startup failure. Telemetry is intentionally not validated here:
// it is compiled out at the egress (post/Enabled), so no config value can
// enable it — erroring on a defaulted mode would brick sovereign boots.
func Lock(in LockInput) error {
	if len(in.ExcludedProviders) > 0 && !Default.Allows(CapGenericProviders) {
		return fmt.Errorf("tier lock: generic providers %v are excluded from the %s build profile — remove them from [providers] or use the DARI relay", in.ExcludedProviders, Default)
	}
	if len(in.BalanceProviders) > 0 && !Default.Allows(CapBalanceFetch) {
		return fmt.Errorf("tier lock: provider balance_url is excluded from the %s build profile — remove it from %v", Default, in.BalanceProviders)
	}
	return nil
}
