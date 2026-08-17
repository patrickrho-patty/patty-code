//go:build !profile_public

package config

// No legacy DeepSeek defaults exist outside the public profile (ADR G4):
// enterprise/sovereign builds embed no foreign LLM endpoints. The legacy
// import path in migrate.go treats these empty values as "nothing to import".
const (
	legacyDeepSeekDefaultModel  = ""
	legacyDeepSeekCredentialEnv = ""
)

// legacyDeepSeekProviderEntries is compiled out outside the public profile
// (ADR G4); callers in load.go/migrate.go treat nil as "no legacy entries".
func legacyDeepSeekProviderEntries() []ProviderEntry { return nil }
