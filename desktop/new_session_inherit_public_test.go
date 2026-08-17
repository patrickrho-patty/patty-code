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
	"sync/atomic"
	"testing"
	"time"

	"patty/internal/agent"
	"patty/internal/config"
)

func TestEnsureBlankTabRetargetsReusedSameNameModelToDefaultProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	model, oldRef, defaultRef := configureSameNameModelProviders(t)

	globalRoot := globalWorkspaceRoot()
	path, err := createEmptySessionFile(desktopSessionDir(globalRoot), model)
	if err != nil {
		t.Fatalf("create empty session: %v", err)
	}
	if err := agent.SetBranchModelPreserveUpdated(path, oldRef); err != nil {
		t.Fatalf("seed old session model: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	tab := testTab("reused", globalRoot)
	tab.Scope = "global"
	tab.WorkspaceRoot = globalRoot
	tab.SessionPath = path
	tab.model = oldRef
	tab.Ctrl.SetSessionPath(path)
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	meta, err := app.EnsureBlankTab("global", "")
	if err != nil {
		t.Fatalf("EnsureBlankTab: %v", err)
	}
	if meta.ID != tab.ID {
		t.Fatalf("reused tab = %q, want %q", meta.ID, tab.ID)
	}
	if tab.model != defaultRef || tab.Ctrl.ModelRef() != defaultRef {
		t.Fatalf("reused runtime model = tab:%q controller:%q, want %q", tab.model, tab.Ctrl.ModelRef(), defaultRef)
	}
	if stored, ok := agent.LoadSessionModel(tab.currentSessionPath()); !ok || stored != defaultRef {
		t.Fatalf("stored session model = %q, %v, want %q", stored, ok, defaultRef)
	}

	current := ""
	for _, info := range app.ModelsForTab(tab.ID) {
		if info.Current {
			if current != "" {
				t.Fatalf("multiple current models: %q and %q", current, info.Ref)
			}
			current = info.Ref
		}
	}
	if current != defaultRef {
		t.Fatalf("model switcher current ref = %q, want %q", current, defaultRef)
	}
}

func TestEnsureBlankTabRepairsStaleStoredProviderWhenRuntimeAlreadyDefault(t *testing.T) {
	isolateDesktopUserDirs(t)
	model, oldRef, defaultRef := configureSameNameModelProviders(t)

	globalRoot := globalWorkspaceRoot()
	path, err := createEmptySessionFile(desktopSessionDir(globalRoot), model)
	if err != nil {
		t.Fatalf("create empty session: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	tab := testTab("stale-meta", globalRoot)
	tab.Scope = "global"
	tab.WorkspaceRoot = globalRoot
	tab.SessionPath = path
	tab.model = oldRef
	tab.Ctrl.SetSessionPath(path)
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	if err := app.SetModelForTab(tab.ID, defaultRef); err != nil {
		t.Fatalf("seed default runtime: %v", err)
	}
	if err := agent.SetBranchModelPreserveUpdated(path, oldRef); err != nil {
		t.Fatalf("seed stale stored model: %v", err)
	}

	if _, err := app.EnsureBlankTab("global", ""); err != nil {
		t.Fatalf("EnsureBlankTab: %v", err)
	}
	if tab.model != defaultRef || tab.Ctrl.ModelRef() != defaultRef {
		t.Fatalf("runtime model = tab:%q controller:%q, want %q", tab.model, tab.Ctrl.ModelRef(), defaultRef)
	}
	if stored, ok := agent.LoadSessionModel(tab.currentSessionPath()); !ok || stored != defaultRef {
		t.Fatalf("stored session model = %q, %v, want repaired %q", stored, ok, defaultRef)
	}
}

func TestEnsureBlankTabConcurrentModelSwitchKeepsLastSelection(t *testing.T) {
	isolateDesktopUserDirs(t)
	model, oldRef, defaultRef := configureSameNameModelProviders(t)

	globalRoot := globalWorkspaceRoot()
	path, err := createEmptySessionFile(desktopSessionDir(globalRoot), model)
	if err != nil {
		t.Fatalf("create empty session: %v", err)
	}
	if err := agent.SetBranchModelPreserveUpdated(path, oldRef); err != nil {
		t.Fatalf("seed old session model: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	tab := testTab("last-click", globalRoot)
	tab.Scope = "global"
	tab.WorkspaceRoot = globalRoot
	tab.SessionPath = path
	tab.model = oldRef
	tab.Ctrl.SetSessionPath(path)
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	firstSwitchReturned := make(chan struct{})
	releaseFirstSwitch := make(chan struct{})
	var switchCount atomic.Int32
	app.modelSwitchTimingHook = func(modelSwitchTiming) {
		if switchCount.Add(1) == 1 {
			close(firstSwitchReturned)
			<-releaseFirstSwitch
		}
	}

	ensureDone := make(chan error, 1)
	go func() {
		_, err := app.EnsureBlankTab("global", "")
		ensureDone <- err
	}()

	select {
	case <-firstSwitchReturned:
	case <-time.After(5 * time.Second):
		close(releaseFirstSwitch)
		t.Fatal("timed out waiting for default model switch")
	}

	// Model selection remains available while EnsureBlankTab is completing.
	// The explicit second switch is the user's last click and must own both the
	// live runtime and the persisted provider identity.
	lastSwitchErr := app.SetModelForTab(tab.ID, oldRef)
	close(releaseFirstSwitch)
	if lastSwitchErr != nil {
		t.Fatalf("last model switch: %v", lastSwitchErr)
	}
	select {
	case err := <-ensureDone:
		if err != nil {
			t.Fatalf("EnsureBlankTab: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for EnsureBlankTab")
	}

	if tab.model != oldRef || tab.Ctrl.ModelRef() != oldRef {
		t.Fatalf("last selected runtime = tab:%q controller:%q, want %q (default was %q)", tab.model, tab.Ctrl.ModelRef(), oldRef, defaultRef)
	}
	if stored, ok := agent.LoadSessionModel(tab.currentSessionPath()); !ok || stored != oldRef {
		t.Fatalf("stored session model = %q, %v, want last selection %q", stored, ok, oldRef)
	}
}

func TestEnsureBlankTabRestartsStartingBlankWithDefaultProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	model, oldRef, defaultRef := configureSameNameModelProviders(t)

	globalRoot := globalWorkspaceRoot()
	path, err := createEmptySessionFile(desktopSessionDir(globalRoot), model)
	if err != nil {
		t.Fatalf("create empty session: %v", err)
	}
	if err := agent.SetBranchModelPreserveUpdated(path, oldRef); err != nil {
		t.Fatalf("seed old session model: %v", err)
	}

	cancelled := false
	app := NewApp()
	tab := &WorkspaceTab{
		ID:            "starting",
		Scope:         "global",
		WorkspaceRoot: globalRoot,
		SessionPath:   path,
		model:         oldRef,
		buildCancel:   func() { cancelled = true },
		disabledMCP:   map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	meta, err := app.EnsureBlankTab("global", "")
	if err != nil {
		t.Fatalf("EnsureBlankTab: %v", err)
	}
	if meta.ID != tab.ID || !cancelled {
		t.Fatalf("starting blank reuse = id:%q cancelled:%v, want %q and cancelled old build", meta.ID, cancelled, tab.ID)
	}
	if !tab.Ready || tab.Ctrl == nil || tab.model != defaultRef || tab.Ctrl.ModelRef() != defaultRef {
		t.Fatalf("restarted blank runtime = ready:%v controller:%v tab:%q, want %q", tab.Ready, tab.Ctrl != nil, tab.model, defaultRef)
	}
	if stored, ok := agent.LoadSessionModel(tab.currentSessionPath()); !ok || stored != defaultRef {
		t.Fatalf("stored session model = %q, %v, want %q", stored, ok, defaultRef)
	}
}

func TestDesktopNewSessionDefaultsKeepsConfiguredDefaultModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	seedUserCredentials(t, "AGENTS_PATTY_API_KEY=test-key\n")

	cfg := config.LoadForEdit(config.UserConfigPath())
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	model, _ := desktopNewSessionDefaults("global", "")
	// Runtime-default surface: the stock default model is the DARI relay's
	// patty-code-standard (PRD v2 §0.2), preserved verbatim.
	if model != "patty/patty-code-standard" {
		t.Fatalf("new session model = %q, want configured default verbatim", model)
	}
}

func TestDesktopNewSessionDefaultsKeepsKeylessDefaultWhenNothingConfigured(t *testing.T) {
	isolateDesktopUserDirs(t)
	// No credentials seeded: every provider resolves as unconfigured.

	cfg := config.LoadForEdit(config.UserConfigPath())
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	// With no configured provider at all, the raw default must survive so the
	// boot-time missing-key notice still tells the user what to fix.
	model, _ := desktopNewSessionDefaults("global", "")
	if model != "patty/patty-code-standard" {
		t.Fatalf("new session model = %q, want raw keyless default preserved", model)
	}
}
