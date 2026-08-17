//go:build profile_public

package tier

// Default is set per build profile; public builds ship the full BYOK
// surface (make build-public).
const Default = Public
