//go:build profile_public

// Legacy v0.x config-import tests (ADR G4): the import contract materializes
// the legacy DeepSeek defaults — DEEPSEEK_API_KEY key migration, deepseek-flash
// and deepseek-pro entries — which are compiled only into public builds.
// Enterprise/sovereign legs run the profile-clean remainder in migrate_test.go.

package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCustomBaseURLWarns(t *testing.T) {
	src, _, _ := legacyHome(t)
	writeLegacy(t, src, `{"apiKey":"sk-x","baseUrl":"https://my-proxy.example/v1"}`)
	res, err := MigrateLegacyIfNeeded()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Error("a non-DeepSeek base_url should produce a warning")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load migrated config: %v", err)
	}
	for _, name := range []string{"deepseek-flash", "deepseek-pro"} {
		p, ok := cfg.Provider(name)
		if !ok || p.BaseURL != "https://my-proxy.example/v1" {
			t.Fatalf("%s base_url was not migrated: %+v", name, p)
		}
	}
}

func TestMigrateToleratesUTF8BOM(t *testing.T) {
	src, _, _ := legacyHome(t)
	writeLegacy(t, src, "\ufeff"+`{"apiKey":"sk-bom"}`)
	res, err := MigrateLegacyIfNeeded()
	if err != nil {
		t.Fatalf("a BOM-prefixed legacy config must still parse: %v", err)
	}
	if res == nil || !res.KeyToEnv {
		t.Fatalf("BOM-prefixed config did not migrate: %+v", res)
	}
	data, _ := os.ReadFile(UserCredentialsPath())
	if !strings.Contains(string(data), "DEEPSEEK_API_KEY=sk-bom") {
		t.Errorf("key not migrated from BOM-prefixed config: %q", data)
	}
}

func TestMigrateImportsLegacyKeyringCredentials(t *testing.T) {
	legacyHome(t)
	old := legacyKeyringProbeLookup
	legacyKeyringProbeLookup = func(_ context.Context, key string) legacyKeyringOutcome {
		if key == "DEEPSEEK_API_KEY" {
			return legacyKeyringOutcome{Status: legacyKeyringFound, Value: "sk-old-keyring"}
		}
		return legacyKeyringOutcome{Status: legacyKeyringAbsent}
	}
	t.Cleanup(func() { legacyKeyringProbeLookup = old })

	res, err := MigrateLegacyIfNeeded()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res != nil {
		t.Fatalf("no config migration should be needed, got %+v", res)
	}
	data, err := os.ReadFile(UserCredentialsPath())
	if err != nil {
		t.Fatalf("read migrated credentials: %v", err)
	}
	if string(data) != "DEEPSEEK_API_KEY=sk-old-keyring\n" {
		t.Fatalf("migrated credentials = %q", data)
	}
}

func TestMigrateLegacyV1TOMLPreservesExplicitProDefault(t *testing.T) {
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
		t.Fatalf("DefaultModel = %q, want explicit deepseek-pro", loaded.DefaultModel)
	}
	if _, ok := loaded.ResolveModel(loaded.DefaultModel); !ok {
		t.Fatalf("explicit legacy default %q is not resolvable in %+v", loaded.DefaultModel, loaded.Providers)
	}
}

func TestMigrateImportsLegacyV1TOMLBeforeJSON(t *testing.T) {
	srcJSON, dest, _ := legacyHome(t)
	legacyTOML := filepath.Join(filepath.Dir(dest), "patty.toml")
	if err := os.MkdirAll(filepath.Dir(legacyTOML), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyTOML, []byte(`
default_model = "deepseek-flash"
language = "en"

[ui]
theme = "light"
theme_style = "glacier"
close_behavior = "quit"

[[plugins]]
name = "legacy-v1"
command = "legacy-bin"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeLegacy(t, srcJSON, `{"apiKey":"sk-json-should-not-win"}`)

	res, err := MigrateLegacyIfNeeded()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res == nil || res.From != legacyTOML {
		t.Fatalf("expected v1 TOML migration, got %+v", res)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	text := string(got)
	for _, want := range []string{`config_version = 5`, `[desktop]`, `close_behavior = "quit"`, `name    = "legacy-v1"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("migrated TOML missing %q:\n%s", want, text)
		}
	}
	if _, err := os.Stat(UserCredentialsPath()); !os.IsNotExist(err) {
		t.Fatalf("v1 TOML migration should not import lower-priority JSON key, credentials stat err=%v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("load migrated legacy TOML: %v", err)
	}
	if _, ok := loaded.ResolveModel(loaded.DefaultModel); !ok {
		t.Fatalf("migrated legacy default %q is not resolvable in %+v", loaded.DefaultModel, loaded.Providers)
	}
}

func TestMigrateImportsKeyPluginsAndLang(t *testing.T) {
	src, dest, home := legacyHome(t)
	writeLegacy(t, src, `{
		"apiKey": "sk-legacy-123",
		"model": "deepseek-v4-pro",
		"lang": "ko-KR",
		"mcpServers": {
			"fs": {"command": "npx", "args": ["-y", "server-fs"], "type": "stdio"},
			"stripe": {"type": "http", "url": "https://mcp.stripe.com", "disabled": true}
		},
		"mcpEnv": {"fs": {"ROOT": "/tmp"}}
	}`)

	res, err := MigrateLegacyIfNeeded()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res == nil {
		t.Fatal("expected a migration result")
	} else if !res.KeyToEnv || res.Plugins != 2 {
		t.Errorf("result = %+v, want KeyToEnv=true Plugins=2", res)
	}

	envData, err := os.ReadFile(UserCredentialsPath())
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if !strings.Contains(string(envData), "DEEPSEEK_API_KEY=sk-legacy-123") {
		t.Errorf("credentials missing key: %q", envData)
	}
	if _, err := os.Stat(filepath.Join(home, ".env")); !os.IsNotExist(err) {
		t.Errorf("migration must not write the user's ~/.env, stat err=%v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest config: %v", err)
	}
	toml := string(got)
	for _, want := range []string{`language      = "ko-KR"`, `[desktop]`, `language = "ko-KR"`, `name    = "fs"`, `name    = "stripe"`, `type    = "http"`, `auto_start = false`} {
		if !strings.Contains(toml, want) {
			t.Errorf("dest config missing %q:\n%s", want, toml)
		}
	}
	if !strings.Contains(toml, `default_model = "deepseek-pro/deepseek-v4-pro"`) {
		t.Errorf("dest config missing imported model:\n%s", toml)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.DefaultModel != "deepseek-pro/deepseek-v4-pro" {
		t.Errorf("DefaultModel = %q, want deepseek-pro/deepseek-v4-pro", loaded.DefaultModel)
	}

	if _, err := os.Stat(src); err != nil {
		t.Errorf("legacy file must be left untouched: %v", err)
	}
}

func TestMigrateLegacyWithoutModelKeepsResolvableLegacyDefault(t *testing.T) {
	src, _, _ := legacyHome(t)
	writeLegacy(t, src, `{}`)

	if _, err := MigrateLegacyIfNeeded(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load migrated config: %v", err)
	}
	if cfg.DefaultModel != "deepseek-flash" {
		t.Fatalf("DefaultModel = %q, want legacy deepseek-flash", cfg.DefaultModel)
	}
	if _, ok := cfg.ResolveModel(cfg.DefaultModel); !ok {
		t.Fatalf("migrated default %q is not resolvable in %+v", cfg.DefaultModel, cfg.Providers)
	}
}

// TestMigrateImportsLegacyMCPStringList covers the pre-mcpServers `mcp` format
// (#3949): `--mcp`-style strings, with mcpEnv/mcpDisabled keyed by name and
// mcpServers winning a name collision.
