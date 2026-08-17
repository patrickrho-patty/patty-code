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

// AssertAllowed panics if Default disallows cap. It exists for the audit
// trail: build-tagged twins (e.g. the online upgrade file) carry a runtime
// assertion at function entry, so an auditor grepping for the Capability
// constant finds the consultation here in addition to the compile-time
// build-tag exclusion. The assertion is unreachable when the call site is
// correctly tagged: under the excluded profile, the file simply isn't
// compiled, so this panic can only fire if a caller invokes the file
// outside its declared tag set — a programmer error worth surfacing loudly.
func AssertAllowed(c Capability) {
	if !Default.Allows(c) {
		panic("tier: capability " + c.String() + " consulted from a file tagged outside its build profile")
	}
}

// AssertDisallowed is the mirror of AssertAllowed for the offline twin:
// it panics if Default *allows* cap. The stub file is reachable only
// under the excluded profile, so any other outcome means the build-tag
// exclusion and the capability table are out of sync — fail loud rather
// than silently re-enable a forbidden network path.
func AssertDisallowed(c Capability) {
	if Default.Allows(c) {
		panic("tier: capability " + c.String() + " stub consulted from a profile that allows it")
	}
}

// String returns a stable human-readable label for the capability so
// assertAllowed's panic message is grep-friendly.
func (c Capability) String() string {
	switch c {
	case CapGenericProviders:
		return "CapGenericProviders"
	case CapPublicPresets:
		return "CapPublicPresets"
	case CapBalanceFetch:
		return "CapBalanceFetch"
	case CapVendorTelemetry:
		return "CapVendorTelemetry"
	case CapCrashUpload:
		return "CapCrashUpload"
	case CapOnlineUpdate:
		return "CapOnlineUpdate"
	default:
		return "unknown"
	}
}
