//go:build profile_public

// Public-surface desktop tests (ADR G4): every test here needs generic
// OpenAI/Anthropic-kind provider fixtures (BYOK flows, official-provider
// surfaces, model/effort/token-mode rebuilds against generic endpoints) or
// another public-only capability (e.g. balance fetch). The enterprise and
// sovereign tier locks refuse those configs at boot by design, so these
// assertions compile and run only in the public-profile leg (see Makefile
// test-profiles).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"patty/internal/agent"
	"patty/internal/config"
)

func TestBuildTabControllerIgnoresStaleSessionModelWhenTabModelResolves(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("PATTY_TEST_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "default-provider/default-model"

[[providers]]
name = "default-provider"
kind = "openai"
base_url = "https://default.invalid/v1"
model = "default-model"
api_key_env = "PATTY_TEST_KEY"

[[providers]]
name = "tab-provider"
kind = "openai"
base_url = "https://tab.invalid/v1"
model = "tab-model"
api_key_env = "PATTY_TEST_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	pinned := writeLegacySession(t, dir, "stale-model.jsonl", "resume with tab model", time.Now())
	meta, err := agent.EnsureBranchMeta(pinned)
	if err != nil {
		t.Fatal(err)
	}
	meta.Model = "missing-provider/missing-model"
	if err := agent.SaveBranchMetaPreserveUpdated(pinned, meta); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), "", "tab_stale_model")
	tab.SessionPath = pinned
	tab.model = "tab-provider/tab-model"
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built: %s", tab.StartupErr)
	}
	defer tab.Ctrl.Close()
	if tab.model != "tab-provider/tab-model" {
		t.Fatalf("tab model = %q, want valid tab model", tab.model)
	}
}

func TestBuildTabControllerSurfacesPinnedSessionLoadError(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("PATTY_TEST_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "test-provider/test-model"

[[providers]]
name = "test-provider"
kind = "openai"
base_url = "https://test.invalid/v1"
model = "test-model"
api_key_env = "PATTY_TEST_KEY"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dir := desktopSessionDir(globalWorkspaceRoot())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := writeLegacySession(t, dir, "unsafe-startup.jsonl", "checkpoint", time.Now())
	events := `{"schema_version":1,"type":"replace","messages":[{"role":"user","content":"newer"}]}` + "\n"
	logPath := agent.SessionEventLogPath(path)
	if err := os.WriteFile(logPath, []byte(events), 0o600); err != nil {
		t.Fatalf("write native event log: %v", err)
	}
	const oversizedSparseLog = int64(1 << 30)
	if err := os.Truncate(logPath, oversizedSparseLog); err != nil {
		t.Fatalf("make sparse oversized event log: %v", err)
	}

	app := NewApp()
	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), "", "tab_unsafe_startup")
	tab.SessionPath = path
	tab.model = "test-provider/test-model"
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.Ctrl != nil || tab.Ready {
		t.Fatalf("unsafe session runtime = hasCtrl:%v ready:%v, want failed startup", tab.Ctrl != nil, tab.Ready)
	}
	if !strings.Contains(tab.StartupErr, agent.ErrSessionReplayLimitExceeded.Error()) || strings.Contains(tab.StartupErr, path) {
		t.Fatalf("startup error = %q, want path-free replay-budget error", tab.StartupErr)
	}
	if filepath.Clean(tab.SessionPath) != filepath.Clean(path) {
		t.Fatalf("session path = %q, want original %q", tab.SessionPath, path)
	}
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat event log after startup refusal: %v", err)
	}
	if info.Size() != oversizedSparseLog {
		t.Fatalf("event log size after startup refusal = %d, want %d", info.Size(), oversizedSparseLog)
	}
	app.sharedHostsMu.Lock()
	sharedHosts := len(app.sharedHosts)
	app.sharedHostsMu.Unlock()
	if sharedHosts != 0 {
		t.Fatalf("shared hosts after failed startup = %d, want 0", sharedHosts)
	}
}
