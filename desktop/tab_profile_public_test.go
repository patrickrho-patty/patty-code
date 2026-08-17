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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/config"
)

func TestSetEffortForTabIsTabLocal(t *testing.T) {
	isolateDesktopUserDirs(t)
	entries, _, err := officialProviderTemplate("deepseek", "en")
	if err != nil {
		t.Fatalf("officialProviderTemplate: %v", err)
	}
	cfg := config.Default()
	cfg.DefaultModel = "deepseek/deepseek-v4-flash"
	cfg.Providers = entries
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save DeepSeek effort fixture: %v", err)
	}

	rootA := t.TempDir()
	rootB := t.TempDir()
	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	tabA := testTab("a", rootA)
	tabB := testTab("b", rootB)
	tabA.model = "deepseek/deepseek-v4-flash"
	tabB.model = "deepseek/deepseek-v4-flash"
	tabA.sink = &tabEventSink{tabID: tabA.ID, app: app}
	tabB.sink = &tabEventSink{tabID: tabB.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tabA.ID: tabA, tabB.ID: tabB}
	app.tabOrder = []string{tabA.ID, tabB.ID}
	app.activeTabID = tabA.ID
	defer func() {
		for _, tab := range app.tabs {
			if tab.Ctrl != nil {
				tab.Ctrl.Close()
			}
		}
	}()

	if err := app.SetEffortForTab(tabA.ID, "max"); err != nil {
		t.Fatalf("SetEffortForTab: %v", err)
	}
	if got := app.EffortForTab(tabA.ID).Current; got != "max" {
		t.Fatalf("tab A effort = %q, want max", got)
	}
	if got := app.EffortForTab(tabB.ID).Current; got != "auto" {
		t.Fatalf("tab B effort = %q, want auto", got)
	}
	if tabB.effort != nil {
		t.Fatalf("tab B stored effort = %q, want nil", *tabB.effort)
	}
	body, err := os.ReadFile(userConfigPathForTest())
	if err == nil && strings.Contains(string(body), `effort`) {
		t.Fatalf("tab-local effort should not write provider config:\n%s", body)
	}
}

func TestBuildTabControllerMigratesPersistedBareMimoModel(t *testing.T) {
	isolateDesktopUserDirs(t)

	root := t.TempDir()
	configBody := `default_model = "deepseek-flash"
[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "deepseek-v4-flash"
api_key_env = "PATTY_TEST_KEY_UNSET"
`
	if err := os.WriteFile(filepath.Join(root, "patty.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.readyHook = func() {}
	tab := testTab("mimo", root)
	tab.model = "mimo-v2.5-pro"
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.StartupErr != "" {
		t.Fatalf("tab startup error = %q", tab.StartupErr)
	}
	defer tab.Ctrl.Close()
	if tab.model != "mimo-pro/mimo-v2.5-pro" {
		t.Fatalf("tab model = %q, want migrated MiMo provider ref", tab.model)
	}
}
