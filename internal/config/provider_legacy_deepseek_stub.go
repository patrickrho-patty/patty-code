//go:build !profile_public

package config

import "patty/internal/tier"

// No legacy DeepSeek defaults exist outside the public profile (ADR G4):
// enterprise/sovereign builds embed no foreign LLM endpoints. The legacy
// import path in migrate.go treats these empty values as "nothing to import".
const (
	legacyDeepSeekDefaultModel  = ""
	legacyDeepSeekCredentialEnv = ""
)

// legacyDeepSeekProviderEntries is compiled out outside the public profile
// (ADR G4); callers in load.go/migrate.go treat nil as "no legacy entries".
func legacyDeepSeekProviderEntries() []ProviderEntry {
	tier.AssertDisallowed(tier.CapPublicPresets)
	return nil
}

// backfillLegacyDeepSeekBalanceURL is a no-op twin of the public-profile hook
// (Task 7 addendum): the wallet-endpoint literal must not compile into
// enterprise/sovereign builds, and injecting it would arm the boot-side tier
// lock against a URL the user never wrote (ADR G4). The vendor context-window
// backfill in load.go remains unconditional.
func backfillLegacyDeepSeekBalanceURL(*ProviderEntry) {}

// isStandardDeepSeekProviderTemplate is always false outside the public
// profile: no stock DeepSeek template is embedded, so no config — not even a
// hand-pasted copy — is treated as the stock one.
func isStandardDeepSeekProviderTemplate(*ProviderEntry) bool { return false }

// deepSeekOfficialTemplateEntry has no stock template outside the public
// profile (ADR G4): provider_access=["deepseek"] does not materialize a
// generic provider in enterprise/sovereign builds.
func deepSeekOfficialTemplateEntry() (ProviderEntry, bool) {
	tier.AssertDisallowed(tier.CapPublicPresets)
	return ProviderEntry{}, false
}
