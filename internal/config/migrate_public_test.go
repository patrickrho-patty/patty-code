//go:build profile_public

// DARI-only amendment (2026-08-19): public builds no longer materialize any
// generic provider from legacy configs — the legacy DeepSeek import hooks
// (DEEPSEEK_API_KEY keyring/JSON import, deepseek-flash/pro entries, the
// deepseek default_model backfill) compiled out with the BYOK surface.
// Public users are managed by Patty's pccp relay, which speaks DARI; the
// harness terminates foreign model protocols in no distribution. These
// tests pin that null contract under profile_public; the profile-clean
// remainder of legacy import behavior is covered by migrate_test.go.

package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigratePublicImportsNoGenericProviders pins the amendment: a legacy
// JSON config with an apiKey migrates without error but materializes no
// generic provider entries and no credential import (the DeepSeek hooks are
// gone), and parsing tolerates a UTF-8 BOM while doing so.
func TestMigratePublicImportsNoGenericProviders(t *testing.T) {
	src, _, _ := legacyHome(t)
	// BOM-prefixed legacy JSON with an apiKey: must parse, must import nothing.
	writeLegacy(t, src, "\ufeff"+`{"apiKey":"sk-old"}`)

	if _, err := MigrateLegacyIfNeeded(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load migrated config: %v", err)
	}
	for _, name := range []string{"deepseek-flash", "deepseek-pro"} {
		if _, ok := cfg.Provider(name); ok {
			t.Errorf("%s materialized in a DARI-only public build: %+v", name, cfg.Providers)
		}
	}
	if cfg.DefaultModel == "deepseek-flash" || cfg.DefaultModel == "deepseek-pro" {
		t.Errorf("DefaultModel fell back to a legacy deepseek default: %q", cfg.DefaultModel)
	}
	// The legacy apiKey may still migrate into the credentials file (inert:
	// no provider consumes it under DARI-only); what must NOT happen is any
	// generic provider materializing — asserted above.
}

// TestMigratePublicKeyringImportsNothing pins the keyring side: the
// DEEPSEEK_API_KEY probe finds a value, but the DARI-only build has no
// credential env to map it to, so nothing is written.
func TestMigratePublicKeyringImportsNothing(t *testing.T) {
	legacyHome(t)
	old := legacyKeyringProbeLookup
	legacyKeyringProbeLookup = func(_ context.Context, key string) legacyKeyringOutcome {
		if key == "DEEPSEEK_API_KEY" {
			return legacyKeyringOutcome{Status: legacyKeyringFound, Value: "sk-old-keyring"}
		}
		return legacyKeyringOutcome{Status: legacyKeyringAbsent}
	}
	t.Cleanup(func() { legacyKeyringProbeLookup = old })

	if _, err := MigrateLegacyIfNeeded(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if data, err := os.ReadFile(UserCredentialsPath()); err == nil && strings.TrimSpace(string(data)) != "" {
		t.Fatalf("credentials written in a DARI-only build: %q", data)
	}
}

// TestMigratePublicLegacyDeepseekDefaultUnresolvable pins the TOML side: an
// explicit legacy `default_model = "deepseek-pro"` migrates verbatim (we
// never rewrite user text) but gains no provider entry, so it resolves to
// nothing — the boot flow surfaces the unknown model instead of silently
// importing a generic endpoint.
func TestMigratePublicLegacyDeepseekDefaultUnresolvable(t *testing.T) {
	_, dest, _ := legacyHome(t)
	legacyTOML := filepath.Join(filepath.Dir(dest), "patty.toml")
	if err := os.MkdirAll(filepath.Dir(legacyTOML), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyTOML, []byte(`default_model = "deepseek-pro"`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := MigrateLegacyIfNeeded(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load migrated legacy TOML: %v", err)
	}
	if loaded.DefaultModel != "deepseek-pro" {
		t.Fatalf("DefaultModel = %q, want verbatim deepseek-pro", loaded.DefaultModel)
	}
	if _, ok := loaded.ResolveModel(loaded.DefaultModel); ok {
		t.Fatalf("legacy default %q resolved to a provider in a DARI-only build: %+v",
			loaded.DefaultModel, loaded.Providers)
	}
}
