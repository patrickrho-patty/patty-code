//go:build !profile_public

package openaiapi

// host_stub.go is the non-public twin of host.go. Under enterprise and
// sovereign profiles the BYOK vendor hostnames must not appear in the
// binary — every literal in host.go is a procurement finding. The
// functions defined here keep the package's symbol surface identical so
// the existing imports (internal/cli/cli.go, internal/config/*.go) keep
// compiling, and every predicate returns false: there is no vendor
// endpoint to recognize under these profiles.

type vendorHostFamily struct {
	apex      string
	canonical []string
}

func matchesVendorHostFamilies(string, ...vendorHostFamily) bool { return false }

func matchesVendorHost(string, string, ...string) bool { return false }

// IsDeepSeek always returns false outside the public profile.
func IsDeepSeek(string) bool { return false }

func IsOpenAI(string) bool { return false }

func DeepSeekPrefixChatURL(chatURL string) string {
	return chatURL
}

func IsGeminiAPI(string) bool { return false }

func UsesGeminiThoughtSignatures(string, string) bool { return false }

func NormalizeModelID(_, model string) string { return model }

func IsMiniMax(string) bool { return false }

func IsMiMo(string) bool { return false }

func IsZhipu(string) bool { return false }

func IsTokenRhythm(string) bool { return false }

func IsLongCat(string) bool { return false }

func IsKimiAPI(string) bool { return false }

func IsOllamaCloud(string) bool { return false }