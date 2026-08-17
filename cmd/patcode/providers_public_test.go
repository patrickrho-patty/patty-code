//go:build profile_public

package main

import (
	"strings"
	"testing"

	"patty/internal/provider"
)

// TestPublicBinaryRegistersGenericProviders pins the ADR G4 wiring: the
// public patcode binary must actually link the generic provider packages,
// not just compile their presets. This test deliberately does NOT
// blank-import the packages itself — the assertion is that
// providers_public.go wired them in. A missing registration surfaces as
// provider.New's "unknown kind", not as a construction error (a keyless
// probe config may still fail construction for ordinary reasons).
func TestPublicBinaryRegistersGenericProviders(t *testing.T) {
	for _, kind := range []string{"openai", "anthropic", "responses"} {
		_, err := provider.New(kind, provider.Config{Name: "wiring-probe"})
		if err != nil && strings.Contains(err.Error(), "unknown kind") {
			t.Fatalf("provider.New(%q) = %v, want the kind registered in a public build", kind, err)
		}
	}
}
