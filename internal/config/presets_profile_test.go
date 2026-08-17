//go:build profile_public

package config

import "testing"

func TestCuratedPresetsFollowBuildProfile(t *testing.T) {
	n := len(CuratedProviderPresets())
	if n == 0 {
		t.Fatal("public builds must ship the curated BYOK presets (ADR G4)")
	}
}
