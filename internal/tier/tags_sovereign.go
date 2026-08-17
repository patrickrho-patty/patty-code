//go:build profile_sovereign

package tier

// Default is Sovereign in sovereign builds (make build-sovereign);
// ADR 2026-08-16 decision 1.
const Default = Sovereign
