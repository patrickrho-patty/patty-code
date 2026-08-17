// Package tier encodes the deployment-tier build profile (ADR
// 2026-08-16-harness-build-profiles). Exactly one tags_*.go file compiles
// into a binary, setting Default. tier is a leaf package: it must not
// import other patty/internal packages so any package can seam against it.
package tier

// Tier is a deployment tier. The zero value is invalid; Default is the
// compile-time truth set by build tags.
type Tier uint8

const (
	Public Tier = iota + 1
	Enterprise
	Sovereign
)

func (t Tier) String() string {
	switch t {
	case Public:
		return "public"
	case Enterprise:
		return "enterprise"
	case Sovereign:
		return "sovereign"
	default:
		return "unknown"
	}
}

// Capability is a feature class from the ADR gate inventory. Every gate
// added to the codebase must map to one row here.
type Capability uint8

const (
	CapGenericProviders Capability = iota // G4: generic OpenAI/Anthropic/Responses chat providers
	CapPublicPresets                      // G4: public-cloud BYOK presets + legacy DeepSeek defaults
	CapBalanceFetch                       // G4: provider balance API (GET /user/balance)
	CapVendorTelemetry                    // G2: crash.patty.io daily ping/counters
	CapCrashUpload                        // G2: crash report upload
	CapOnlineUpdate                       // G3: vendor/GitHub release fetch + self-replace
)

// allowed mirrors the ADR profile-semantics table. Sovereign excludes all
// six; enterprise excludes the BYOK surface; public excludes nothing.
var allowed = map[Tier]map[Capability]bool{
	Public: {
		CapGenericProviders: true, CapPublicPresets: true, CapBalanceFetch: true,
		CapVendorTelemetry: true, CapCrashUpload: true, CapOnlineUpdate: true,
	},
	Enterprise: {
		CapVendorTelemetry: true, CapCrashUpload: true, CapOnlineUpdate: true,
	},
	Sovereign: {},
}

// Allows reports whether the linked tier permits the capability at build
// time. Unknown tiers allow nothing (fail closed).
func (t Tier) Allows(c Capability) bool {
	m, ok := allowed[t]
	return ok && m[c]
}
