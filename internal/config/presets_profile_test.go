//go:build profile_public

package config

import "testing"

// DARI-only amendment (2026-08-19): public builds ship no curated BYOK
// presets — the catalog compiled out with the generic-provider surface.
// Public users reach models through Patty's pccp relay (DARI), not by
// bringing foreign endpoints into the harness.
func TestCuratedPresetsFollowBuildProfile(t *testing.T) {
	if got := CuratedProviderPresets(); len(got) != 0 {
		t.Fatalf("public builds must ship zero curated presets (DARI-only), got %d", len(got))
	}
	if _, ok := CuratedProviderPreset("deepseek"); ok {
		t.Fatal("deepseek preset must not resolve in a DARI-only public build")
	}
}
