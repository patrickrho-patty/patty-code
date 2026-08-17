//go:build !profile_sovereign

// Online telemetry consent UX (ADR G2): the first-run consent prompt,
// crash.patty.io disclosure, and reporter start-after-consent only exist in
// profiles that allow CapVendorTelemetry. Sovereign builds never prompt or
// upload, so these behavior tests are excluded there.

package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/config"
	"patty/internal/i18n"
	"patty/internal/telemetry"
)

func TestCLITelemetryConsentDefaultsYesAndPromptsOnlyOnce(t *testing.T) {
	isolateCLIConfigHome(t)
	clearCLITelemetryPolicyEnv(t)
	t.Cleanup(func() { i18n.DetectLanguage("en") })
	i18n.DetectLanguage("en")

	previousStart := startCLITelemetryReporter
	t.Cleanup(func() { startCLITelemetryReporter = previousStart })
	want := &telemetry.Reporter{}
	starts := 0
	startCLITelemetryReporter = func(opts telemetry.Options) *telemetry.Reporter {
		starts++
		saved, err := config.LoadForEditReadOnlyStrict(config.UserConfigPath())
		if err != nil || !saved.CLITelemetryConfigured() || saved.CLITelemetryMode() != "auto" {
			t.Fatalf("telemetry started before consent was saved: mode=%q configured=%v err=%v", saved.CLITelemetryMode(), saved.CLITelemetryConfigured(), err)
		}
		return want
	}

	cfg := config.Default()
	var out, errOut bytes.Buffer
	got := startCLITelemetryWithIO(cfg, telemetry.Options{
		Version: "v1.20.0", Interactive: true, CLIMode: "tui",
	}, strings.NewReader("\n"), &out, &errOut)
	if got != want || starts != 1 {
		t.Fatalf("first start = %p, calls=%d; want %p, 1", got, starts, want)
	}
	if !strings.Contains(out.String(), "crash.patty.io") || !strings.Contains(out.String(), "[Y/n]:") || !strings.Contains(out.String(), "patcode config telemetry off") {
		t.Fatalf("consent prompt is incomplete: %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected consent stderr: %q", errOut.String())
	}
	if !cfg.CLITelemetryConfigured() || cfg.CLITelemetryMode() != "auto" {
		t.Fatalf("runtime config was not synchronized: mode=%q configured=%v", cfg.CLITelemetryMode(), cfg.CLITelemetryConfigured())
	}

	var secondOut bytes.Buffer
	if got := startCLITelemetryWithIO(cfg, telemetry.Options{
		Version: "v1.20.0", Interactive: true, CLIMode: "tui",
	}, strings.NewReader("n\n"), &secondOut, &errOut); got != want {
		t.Fatalf("second start = %p, want %p", got, want)
	}
	if secondOut.Len() != 0 || starts != 2 {
		t.Fatalf("saved decision prompted again: output=%q calls=%d", secondOut.String(), starts)
	}
}

func TestCLITelemetryConsentNoDisablesAndCleansPending(t *testing.T) {
	isolateCLIConfigHome(t)
	clearCLITelemetryPolicyEnv(t)

	previousStart := startCLITelemetryReporter
	t.Cleanup(func() { startCLITelemetryReporter = previousStart })
	starts := 0
	startCLITelemetryReporter = func(telemetry.Options) *telemetry.Reporter {
		starts++
		return &telemetry.Reporter{}
	}
	home := config.PattyHomeDir()
	pending := filepath.Join(home, "cli-telemetry-pending")
	if err := os.MkdirAll(pending, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pending, "pending.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	var out, errOut bytes.Buffer
	if got := startCLITelemetryWithIO(cfg, telemetry.Options{
		Version: "v1.20.0", Interactive: true, CLIMode: "tui",
	}, strings.NewReader("n\n"), &out, &errOut); got != nil {
		t.Fatalf("declined telemetry returned reporter %p", got)
	}
	if starts != 0 {
		t.Fatalf("declined telemetry started upload %d times", starts)
	}
	if cfg.CLITelemetryMode() != "off" || !cfg.CLITelemetryConfigured() {
		t.Fatalf("decline was not saved in runtime config: mode=%q configured=%v", cfg.CLITelemetryMode(), cfg.CLITelemetryConfigured())
	}
	if _, err := os.Stat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("decline did not clear pending queue: %v", err)
	}
	saved, err := config.LoadForEditReadOnlyStrict(config.UserConfigPath())
	if err != nil || saved.CLITelemetryMode() != "off" || !saved.CLITelemetryConfigured() {
		t.Fatalf("saved decline = mode %q configured=%v err=%v", saved.CLITelemetryMode(), saved.CLITelemetryConfigured(), err)
	}
}

func TestCLITelemetryConsentSaveFailureDoesNotUpload(t *testing.T) {
	isolateCLIConfigHome(t)
	clearCLITelemetryPolicyEnv(t)

	previousSave := persistCLITelemetryConsent
	previousStart := startCLITelemetryReporter
	t.Cleanup(func() {
		persistCLITelemetryConsent = previousSave
		startCLITelemetryReporter = previousStart
	})
	persistCLITelemetryConsent = func(string) error { return errors.New("read-only config") }
	starts := 0
	startCLITelemetryReporter = func(telemetry.Options) *telemetry.Reporter {
		starts++
		return &telemetry.Reporter{}
	}

	cfg := config.Default()
	var out, errOut bytes.Buffer
	if got := startCLITelemetryWithIO(cfg, telemetry.Options{
		Version: "v1.20.0", Interactive: true, CLIMode: "tui",
	}, strings.NewReader("\n"), &out, &errOut); got != nil {
		t.Fatalf("save failure returned reporter %p", got)
	}
	if starts != 0 || cfg.CLITelemetryConfigured() {
		t.Fatalf("save failure started=%d configured=%v", starts, cfg.CLITelemetryConfigured())
	}
	if !strings.Contains(errOut.String(), "read-only config") {
		t.Fatalf("save failure was not explained: %q", errOut.String())
	}
}

func TestCLITelemetryConsentPromptIsLocalized(t *testing.T) {
	isolateCLIConfigHome(t)
	clearCLITelemetryPolicyEnv(t)
	previousSave := persistCLITelemetryConsent
	previousStart := startCLITelemetryReporter
	t.Cleanup(func() {
		persistCLITelemetryConsent = previousSave
		startCLITelemetryReporter = previousStart
		i18n.DetectLanguage("en")
	})
	persistCLITelemetryConsent = func(string) error { return nil }
	startCLITelemetryReporter = func(telemetry.Options) *telemetry.Reporter { return nil }

	for _, lang := range []string{"en", "ko-KR", "en-US"} {
		i18n.DetectLanguage(lang)
		var out bytes.Buffer
		startCLITelemetryWithIO(config.Default(), telemetry.Options{
			Version: "v1.20.0", Interactive: true, CLIMode: "tui",
		}, strings.NewReader("\n"), &out, io.Discard)
		for _, required := range []string{"crash.patty.io", "patcode config telemetry off", "[Y/n]:"} {
			if !strings.Contains(out.String(), required) {
				t.Fatalf("%s consent prompt missing %q: %q", lang, required, out.String())
			}
		}
	}
}
