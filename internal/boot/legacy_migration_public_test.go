//go:build profile_public

// Legacy-config migration tests whose migrated configs resolve to generic
// (openai-kind) presets — DeepSeek and MiMo presets are public-build
// capabilities (ADR G4), so constructing the migrated provider compiles only
// here. Task 6 replaces the legacy presets with DARI-stub equivalents; until
// then these assertions run on the public leg only.
package boot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/config"
	"patty/internal/event"
)

// TestBuildMigratesLegacyConfigEndToEnd drives the real boot path: a v0.x
// ~/.patty/config.json with no v1+ config present must be imported during
// Build — config written, key pinned into the env, and the user told via a notice.
func TestBuildMigratesLegacyConfigEndToEnd(t *testing.T) {
	home := robustTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)                               // os.UserHomeDir on Windows
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // os.UserConfigDir on Linux
	t.Setenv("AppData", filepath.Join(home, "AppData"))         // os.UserConfigDir on Windows
	t.Setenv("PATTY_CREDENTIALS_STORE", "file")
	t.Setenv("DEEPSEEK_API_KEY", "") // track for cleanup; migration os.Setenv's it live

	proj := robustTempDir(t)
	t.Chdir(proj)
	// Project config merges over the migrated user config without dropping the
	// migrated plugins.
	writeFile(t, proj, "patty.toml", "")
	writeFile(t, filepath.Join(home, ".patty"), "config.json",
		`{"apiKey":"sk-e2e","lang":"ko-KR","mcpServers":{"fs":{"command":"npx","args":["-y","server-fs"]}}}`)
	writeFile(t, filepath.Join(home, ".patty", "sessions"), "chat-1.events.jsonl",
		`{"type":"user.message","id":1,"ts":"t","turn":0,"text":"hello from v0.x"}`+"\n"+
			`{"type":"model.final","id":2,"ts":"t","turn":0,"content":"hi","toolCalls":[],"usage":{},"costUsd":0}`+"\n")

	var notices []string
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e.Text)
		}
	})

	ctrl, err := Build(context.Background(), Options{Sink: sink})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	migrated := false
	for _, n := range notices {
		if strings.Contains(n, "migrated your previous configuration") {
			migrated = true
		}
	}
	if !migrated {
		t.Fatalf("no migration notice emitted; got %v", notices)
	}

	dest := config.UserConfigPath()
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("v2 config not written to %s: %v", dest, err)
	}
	if !strings.Contains(string(data), `name    = "fs"`) || !strings.Contains(string(data), `language      = "ko-KR"`) {
		t.Errorf("migrated config missing plugin/lang:\n%s", data)
	}

	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "sk-e2e" {
		t.Errorf("DEEPSEEK_API_KEY not pinned into env after migration: %q", got)
	}

	if data, err := os.ReadFile(config.UserCredentialsPath()); err != nil || !strings.Contains(string(data), "DEEPSEEK_API_KEY=sk-e2e") {
		t.Errorf("credentials store missing migrated key: %q (err %v)", data, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".env")); !os.IsNotExist(err) {
		t.Errorf("migration must not write the user's ~/.env, stat err=%v", err)
	}

	sessionImported := false
	for _, n := range notices {
		if strings.Contains(n, "imported") && strings.Contains(n, "past session") {
			sessionImported = true
		}
	}
	if !sessionImported {
		t.Errorf("no session-import notice emitted; got %v", notices)
	}
	migratedSession := filepath.Join(config.SessionDir(), "chat-1.jsonl")
	if _, err := os.Stat(migratedSession); err != nil {
		t.Errorf("legacy session not imported to %s: %v", migratedSession, err)
	}
}

func TestBuildMigratesLegacyBareMimoModelOverride(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "patty.toml", `
default_model = "deepseek-flash"

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://example.invalid"
model = "deepseek-v4-flash"
api_key_env = "PATTY_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, Model: "mimo-v2.5-pro"})
	if err != nil {
		t.Fatalf("Build should migrate legacy bare MiMo model override: %v", err)
	}
	defer ctrl.Close()
	if ctrl.Label() != "mimo-v2.5-pro" {
		t.Fatalf("controller label = %q, want mimo-v2.5-pro", ctrl.Label())
	}
}
