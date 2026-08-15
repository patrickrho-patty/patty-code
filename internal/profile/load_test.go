package profile_test

import (
	"strings"
	"testing"

	"patty/internal/profile"
)

func TestLoadRealProductYaml(t *testing.T) {
	p, err := profile.Load("../../products/patty/product.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if p.HarnessID != "patty" {
		t.Errorf("HarnessID = %q, want patty", p.HarnessID)
	}
	if p.BannerArtwork.Path != "assets/banner.txt" {
		t.Errorf("BannerArtwork.Path = %q, want assets/banner.txt", p.BannerArtwork.Path)
	}
	if p.ColorPalette.Name != "hanji-workstation" {
		t.Errorf("palette = %q, want hanji-workstation", p.ColorPalette.Name)
	}
	if p.DefaultLocale != profile.Locale("ko") {
		t.Errorf("DefaultLocale = %q, want ko", p.DefaultLocale)
	}
	if p.SecurityBaseline.DegradedMode != profile.DegradedBlockAll {
		t.Errorf("DegradedMode = %d, want blockAll", p.SecurityBaseline.DegradedMode)
	}
	if len(p.SupportedLocales) != 2 {
		t.Errorf("SupportedLocales = %v, want [ko en]", p.SupportedLocales)
	}
	if p.DigestHex() == "" {
		t.Error("expected a non-empty canonical digest")
	}
}

func TestParseInvalidDegradedMode(t *testing.T) {
	raw := strings.ReplaceAll(`harnessID: patty
editionID: "1.0.0"
displayName: {ko: "Patty Code", en: "Patty Code"}
artifactName: patty-code
executableName: patty
userRoot: .patty
configFilename: patty.toml
envPrefix: PATTY_
storageNamespace: patty
defaultLocale: ko
supportedLocales: [ko, en]
bannerArtwork: {path: assets/banner.txt}
securityBaseline:
  degradedMode: bogus
`, "", "")
	_, err := profile.Parse([]byte(raw))
	if err == nil {
		t.Fatal("expected error for unknown degradedMode")
	}
	if !strings.Contains(err.Error(), "degradedMode") {
		t.Errorf("error should mention degradedMode, got %v", err)
	}
}
