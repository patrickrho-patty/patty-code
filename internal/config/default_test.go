package config

import (
	"testing"

	"patty/internal/tier"
)

func TestDefaultRetiredAutoPlanCompatibilityOff(t *testing.T) {
	if got := Default().Agent.AutoPlan; got != "off" {
		t.Fatalf("default auto_plan = %q, want off", got)
	}
}

func TestDefaultReasoningLanguageAuto(t *testing.T) {
	if got := Default().ReasoningLanguage(); got != "auto" {
		t.Fatalf("default reasoning_language = %q, want auto", got)
	}
}

func TestDefaultDesktopAppearanceAutoGraphite(t *testing.T) {
	cfg := Default()
	if got := cfg.DesktopTheme(); got != "auto" {
		t.Fatalf("default desktop theme = %q, want auto", got)
	}
	if got := cfg.DesktopThemeStyle(); got != "" {
		t.Fatalf("default desktop theme style = %q, want empty so frontend resolves graphite", got)
	}
	if got := cfg.DesktopTerminalTheme(); got != "auto" {
		t.Fatalf("default desktop terminal theme = %q, want auto", got)
	}
}

func TestDefaultDesktopMetricsOn(t *testing.T) {
	cfg := Default()
	// The default follows the vendor-telemetry capability (ADR G2): on
	// where telemetry exists, off in sovereign builds.
	if got, want := cfg.DesktopMetrics(), tier.Default.Allows(tier.CapVendorTelemetry); got != want {
		t.Fatalf("default desktop metrics = %v, want %v (profile default)", got, want)
	}
	disabled := false
	cfg.Desktop.Metrics = &disabled
	if cfg.DesktopMetrics() {
		t.Fatal("desktop metrics explicit false = true, want false")
	}
}
