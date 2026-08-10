// Package profile — tests for validation, resolution, capabilities, and modules.
package profile_test

import (
	"testing"

	"patty/internal/profile"
)

func TestProfileValidate_Valid(t *testing.T) {
	p := &profile.Profile{
		HarnessID:        "patty",
		EditionID:        "1.0.0",
		DisplayName:      profile.LocalizedString{"ko": "Patty Code", "en": "Patty Code"},
		ArtifactName:     "patty-code",
		ExecutableName:   "patty",
		UserRoot:         ".patty",
		ConfigFilename:   "patty.toml",
		EnvPrefix:        "PATTY_",
		StorageNamespace: "patty",
		DefaultLocale:    "ko",
		SupportedLocales: []string{"ko", "en"},
		BannerArtwork:    profile.AssetCoordinate{Path: "assets/banner.txt"},
		SecurityBaseline: profile.BaselineConfig{
			DegradedMode: profile.DegradedBlockAll,
		},
	}
	errs := p.Validate()
	if len(errs) > 0 {
		t.Fatalf("expected valid profile, got errors: %v", errs)
	}
}

func TestProfileValidate_MissingRequired(t *testing.T) {
	p := &profile.Profile{}
	errs := p.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation errors for empty profile")
	}
}

func TestProfileLocalizedString_KoreanWins(t *testing.T) {
	l := profile.LocalizedString{"ko": "한국어", "en": "English"}
	if got := l.String(); got != "한국어" {
		t.Errorf("Expected Korean, got %q", got)
	}

	l2 := profile.LocalizedString{"en": "English"}
	if got := l2.String(); got != "English" {
		t.Errorf("Expected fallback to English, got %q", got)
	}

	l3 := profile.LocalizedString{}
	if got := l3.String(); got != "" {
		t.Errorf("Expected empty string, got %q", got)
	}
}

func TestResolveDerived_SimpleOverride(t *testing.T) {
	base := &profile.Profile{
		HarnessID:        "patty",
		EditionID:        "1.0.0",
		DisplayName:      profile.LocalizedString{"ko": "Patty Code", "en": "Patty Code"},
		ArtifactName:     "patty-code",
		ExecutableName:   "patty",
		UserRoot:         ".patty",
		ConfigFilename:   "patty.toml",
		EnvPrefix:        "PATTY_",
		StorageNamespace: "patty",
		DefaultLocale:    "ko",
		SupportedLocales: []string{"ko", "en"},
		BannerArtwork:    profile.AssetCoordinate{Path: "banner.txt"},
		SecurityBaseline: profile.BaselineConfig{DegradedMode: profile.DegradedAllowReadonly},
	}
	derived := &profile.Profile{
		HarnessID:        "gongcode",
		EditionID:        "1.0.0-d1",
		DisplayName:      profile.LocalizedString{"ko": "공코드", "en": "GongCode"},
		ArtifactName:     "gongcode-server",
		ExecutableName:   "gongcode",
		UserRoot:         ".gongcode",
		ConfigFilename:   "gongcode.toml",
		EnvPrefix:        "GONGCODE_",
		StorageNamespace: "gongcode",
		// Security baseline must not be weaker than base
		SecurityBaseline: profile.BaselineConfig{DegradedMode: profile.DegradedBlockAll},
		BannerArtwork:    profile.AssetCoordinate{Path: "banner.txt"},
	}

	result, err := profile.ResolveDerived(base, derived)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Errors != nil && len(result.Errors) > 0 {
		t.Fatalf("merge validation failed: %v", result.Errors)
	}

	r := result.Resolved
	if r.HarnessID != "gongcode" {
		t.Errorf("HarnessID = %q, want %q", r.HarnessID, "gongcode")
	}
	if r.ExecutableName != "gongcode" {
		t.Errorf("ExecutableName = %q, want %q", r.ExecutableName, "gongcode")
	}
	if r.UserRoot != ".gongcode" {
		t.Errorf("UserRoot = %q, want %q", r.UserRoot, ".gongcode")
	}
	if r.EnvPrefix != "GONGCODE_" {
		t.Errorf("EnvPrefix = %q, want %q", r.EnvPrefix, "GONGCODE_")
	}
	if r.DefaultLocale != "ko" {
		t.Errorf("DefaultLocale inherited, got %q", r.DefaultLocale)
	}
	if r.Extends() != "patty" {
		t.Errorf("Extends = %q, want %q", r.Extends(), "patty")
	}
}

func TestResolveDerived_DerivedWithoutValidBase(t *testing.T) {
	badBase := &profile.Profile{} // missing required fields
	goodDerived := &profile.Profile{
		HarnessID: "test",
	}

	_, err := profile.ResolveDerived(badBase, goodDerived)
	if err == nil {
		t.Fatal("expected error when base is invalid")
	}
}

func TestRegistry_EnforcedModuleCannotDisable(t *testing.T) {
	reg := profile.NewRegistry()
	mod := &profile.Module{
		ID:      "core.tui",
		Version: "1.0.0",
		Enabled: true,
	}
	if err := reg.Register(mod, true); err != nil {
		t.Fatal(err)
	}

	err := reg.Disable("core.tui")
	if err == nil {
		t.Fatal("expected error disabling enforced module")
	}
}

func TestRegistry_OptionalModuleCanBeRemoved(t *testing.T) {
	reg := profile.NewRegistry()
	mod := &profile.Module{
		ID:        "feature.test",
		Version:   "1.0.0",
		Enabled:   true,
		DependsOn: nil,
	}
	if err := reg.Register(mod, false); err != nil {
		t.Fatal(err)
	}

	err := reg.Remove("feature.test")
	if err != nil {
		t.Fatalf("expected no error removing optional module: %v", err)
	}

	if _, ok := reg.Get("feature.test"); ok {
		t.Fatal("module should not exist after removal")
	}
}

func TestRegistry_ModuleWithDependencies(t *testing.T) {
	reg := profile.NewRegistry()
	dep := &profile.Module{ID: "dep.mod", Version: "1.0.0", Enabled: true}
	mainMod := &profile.Module{
		ID:      "main.mod",
		Version: "1.0.0",
		Enabled: true,
		DependsOn: []string{"dep.mod"},
	}

	if err := reg.Register(dep, false); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(mainMod, false); err != nil {
		t.Fatal(err)
	}

	// Disable dependency - should fail because main depends on it
	err := reg.Disable("dep.mod")
	if err == nil {
		t.Fatal("expected error when disabling enabled dependency")
	}
}

func TestCapabilityRegistry_RegisterAndUnregister(t *testing.T) {
	reg := profile.NewCapReg()
	cap := &profile.Capability{
		ID:          "test.cap",
		Type:        profile.CapabilityProvider,
		Priority:    100,
		Description: profile.LocalizedString{"ko": "테스트", "en": "Test"},
	}

	if err := reg.Register(cap, "test.mod"); err != nil {
		t.Fatal(err)
	}

	found, _ := reg.Get("test.cap")
	if found == nil {
		t.Fatal("capability should exist after register")
	}

	if err := reg.Unregister("test.cap", "test.mod"); err != nil {
		t.Fatal(err)
	}

	if _, ok := reg.Get("test.cap"); ok {
		t.Fatal("capability should not exist after unregister")
	}
}

func TestIntegrityResult_NoEnforcedModules(t *testing.T) {
	reg := profile.NewRegistry()
	result := reg.CheckIntegrity(nil)
	if !result.AllEnforcedPresent {
		t.Error("expected all enforced present when none registered")
	}
}