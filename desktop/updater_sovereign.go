//go:build profile_sovereign

package main

import "patty/internal/tier"

// manifestEndpoints is empty in sovereign builds: there is no online update
// channel at all (ADR G3).
func manifestEndpoints(string) []string {
	tier.AssertDisallowed(tier.CapOnlineUpdate)
	return nil
}

// desktopAssetBases is empty in sovereign builds (ADR G3).
func desktopAssetBases(selected, version string, allowLegacyPreview bool) []string {
	tier.AssertDisallowed(tier.CapOnlineUpdate)
	return nil
}
