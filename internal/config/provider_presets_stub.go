//go:build !profile_public

package config

// CuratedProviderPresets is a public-profile capability (ADR G4): BYOK
// public-cloud presets are compiled out of enterprise/sovereign builds —
// enterprise catalogs come from the org relay at runtime.
func CuratedProviderPresets() []ProviderPreset { return nil }

// CuratedProviderPreset is the public-profile single-preset lookup; outside
// the public profile no curated preset exists, so every id misses.
func CuratedProviderPreset(id string) (ProviderPreset, bool) {
	return ProviderPreset{}, false
}
