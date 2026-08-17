//go:build !profile_public

package config

import "testing"

func TestCuratedPresetsAbsentOutsidePublicProfile(t *testing.T) {
	if got := CuratedProviderPresets(); got != nil {
		t.Fatalf("non-public builds must not embed public-cloud presets, got %d", len(got))
	}
	if got := legacyDeepSeekProviderEntries(); got != nil {
		t.Fatalf("non-public builds must not embed legacy DeepSeek defaults, got %d", len(got))
	}
}
