//go:build profile_public

// Generic-provider desktop workflows (ADR G4): every test here drives a
// settings/model/effort rebuild path against generic OpenAI-kind provider
// fixtures. Generic providers exist only in public builds, so under the
// enterprise/sovereign tier lock these configs cannot boot — the assertions
// compile and run only in the public-profile leg (see Makefile test-profiles).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/agent"
	"patty/internal/boot"
	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/jobs"
	"patty/internal/provider"
	"patty/internal/store"
)

func TestSetProviderKeyRebuildSupersedesInFlightStartupBuild(t *testing.T) {
	isolateDesktopUserDirs(t)

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "old",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Model:     "old-model",
		APIKeyEnv: "OLD_MODEL_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "startup-build-in-flight.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write placeholder session: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	// Model the async startup build still being in flight: no controller yet,
	// a live build generation, and a cancellable build context.
	buildCtx, buildCancel := context.WithCancel(context.Background())
	const startupGeneration = 1
	tab := &WorkspaceTab{
		ID:              "tab_key_rebuild",
		Scope:           "global",
		SessionPath:     sessionPath,
		model:           "old/old-model",
		buildGeneration: startupGeneration,
		buildCancel:     buildCancel,
		disabledMCP:     map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(tab.releaseSessionLease)

	if _, err := app.SetProviderKey("OLD_MODEL_KEY", "sk-new"); err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}
	if tab.Ctrl == nil {
		t.Fatal("provider-key rebuild did not install a controller")
	}
	defer tab.Ctrl.Close()

	assertTabBuildSuperseded(t, app, tab, startupGeneration, buildCtx)
}

func TestDeferredRebuildRetryAppliesAfterLeaseRelease(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	prevInterval := deferredRebuildRetryInterval
	deferredRebuildRetryInterval = 20 * time.Millisecond
	t.Cleanup(func() { deferredRebuildRetryInterval = prevInterval })

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "old",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Model:     "old-model",
		APIKeyEnv: "OLD_MODEL_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "deferred-rebuild-retry.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write placeholder session: %v", err)
	}
	externalLease, err := agent.TryAcquireSessionLease(sessionPath)
	if err != nil {
		t.Fatalf("TryAcquireSessionLease: %v", err)
	}
	released := false
	defer func() {
		if !released {
			externalLease.Release()
		}
	}()

	oldExec := agent.New(nil, nil, agent.NewSession("old system prompt"), agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: sessionPath, Label: "old", Sink: event.Discard})
	defer oldCtrl.Close()

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.enableDeferredRebuildRetry()
	t.Cleanup(app.stopDeferredRebuildRetry)
	tab := &WorkspaceTab{
		ID:          "tab_deferred_retry",
		Scope:       "global",
		SessionPath: sessionPath,
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_deferred_retry", app: app},
		disabledMCP: map[string]ServerView{},
	}
	installNoopRuntimeEvents(app, tab.sink)
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if c := app.controllerForTab(tab); c != nil && c != oldCtrl {
			c.Close()
		}
		tab.releaseSessionLease()
	})

	if err := app.SetMaxSubagentDepth(1); err != nil {
		t.Fatalf("SetMaxSubagentDepth: %v", err)
	}
	if !app.deferredRebuildPending(tab.ID) {
		t.Fatal("deferred rebuild was not scheduled while the lease is held")
	}
	if app.controllerForTab(tab) != oldCtrl {
		t.Fatal("controller changed while the lease is still held")
	}

	externalLease.Release()
	released = true

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !app.deferredRebuildPending(tab.ID) && app.controllerForTab(tab) != oldCtrl {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if app.deferredRebuildPending(tab.ID) {
		t.Fatal("deferred rebuild is still pending after the lease was released")
	}
	if c := app.controllerForTab(tab); c == nil || c == oldCtrl {
		t.Fatalf("controller was not rebuilt after the lease release: got %p", c)
	}
}

func TestDeferredRebuildWaitsForTabToBecomeActive(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	prevInterval := deferredRebuildRetryInterval
	deferredRebuildRetryInterval = 20 * time.Millisecond
	t.Cleanup(func() { deferredRebuildRetryInterval = prevInterval })

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "old",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Model:     "old-model",
		APIKeyEnv: "OLD_MODEL_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "deferred-rebuild-inactive.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write placeholder session: %v", err)
	}
	externalLease, err := agent.TryAcquireSessionLease(sessionPath)
	if err != nil {
		t.Fatalf("TryAcquireSessionLease: %v", err)
	}
	released := false
	defer func() {
		if !released {
			externalLease.Release()
		}
	}()

	oldExec := agent.New(nil, nil, agent.NewSession("old system prompt"), agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: sessionPath, Label: "old", Sink: event.Discard})
	defer oldCtrl.Close()

	otherCtrl := control.New(control.Options{Label: "other"})
	defer otherCtrl.Close()

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.enableDeferredRebuildRetry()
	t.Cleanup(app.stopDeferredRebuildRetry)
	tab := &WorkspaceTab{
		ID:          "tab_pending",
		Scope:       "global",
		SessionPath: sessionPath,
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_pending", app: app},
		disabledMCP: map[string]ServerView{},
	}
	installNoopRuntimeEvents(app, tab.sink)
	other := &WorkspaceTab{
		ID:          "tab_other",
		Scope:       "global",
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        otherCtrl,
		sink:        &tabEventSink{tabID: "tab_other", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab, other.ID: other}
	app.tabOrder = []string{tab.ID, other.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if c := app.controllerForTab(tab); c != nil && c != oldCtrl {
			c.Close()
		}
		tab.releaseSessionLease()
	})

	if err := app.SetMaxSubagentDepth(1); err != nil {
		t.Fatalf("SetMaxSubagentDepth: %v", err)
	}
	if !app.deferredRebuildPending(tab.ID) {
		t.Fatal("deferred rebuild was not scheduled while the lease is held")
	}

	// Focus another tab, then release the lease: the retry must not rebuild
	// while the pending tab is inactive (rebuildSettingLocked acts on the
	// active tab), and must not touch the focused tab's runtime either.
	app.mu.Lock()
	app.activeTabID = other.ID
	app.mu.Unlock()
	externalLease.Release()
	released = true

	time.Sleep(150 * time.Millisecond)
	if !app.deferredRebuildPending(tab.ID) {
		t.Fatal("pending entry was consumed while its tab was inactive")
	}
	if app.controllerForTab(tab) != oldCtrl {
		t.Fatal("inactive pending tab was rebuilt")
	}
	if app.controllerForTab(other) != otherCtrl {
		t.Fatal("focused tab was rebuilt by another tab's deferred retry")
	}

	// Switch back: the retry should now refresh the pending tab.
	app.mu.Lock()
	app.activeTabID = tab.ID
	app.mu.Unlock()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !app.deferredRebuildPending(tab.ID) && app.controllerForTab(tab) != oldCtrl {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if app.deferredRebuildPending(tab.ID) {
		t.Fatal("deferred rebuild still pending after its tab became active again")
	}
	if c := app.controllerForTab(tab); c == nil || c == oldCtrl {
		t.Fatalf("controller was not rebuilt after tab reactivation: got %p", c)
	}
}

func TestSetEffortForTabLeaseHeldKeepsOldControllerAlive(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{{
		Name:             "old",
		Kind:             "openai",
		BaseURL:          "https://example.invalid/v1",
		Model:            "old-model",
		APIKeyEnv:        "OLD_MODEL_KEY",
		SupportedEfforts: []string{"low", "max"},
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "externally-leased-effort-switch.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write placeholder session: %v", err)
	}
	externalLease, err := agent.TryAcquireSessionLease(sessionPath)
	if err != nil {
		t.Fatalf("TryAcquireSessionLease: %v", err)
	}
	released := false
	defer func() {
		if !released {
			externalLease.Release()
		}
	}()

	oldExec := agent.New(nil, nil, agent.NewSession("old system prompt"), agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: sessionPath, Label: "old", Sink: event.Discard})
	defer oldCtrl.Close()

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:          "tab_effort",
		Scope:       "global",
		SessionPath: sessionPath,
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_effort", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if c := app.controllerForTab(tab); c != nil && c != oldCtrl {
			c.Close()
		}
		tab.releaseSessionLease()
	})

	err = app.SetEffortForTab(tab.ID, "max")
	if !errors.Is(err, agent.ErrSessionLeaseHeld) {
		t.Fatalf("SetEffortForTab err = %v, want ErrSessionLeaseHeld", err)
	}
	if strings.Contains(err.Error(), sessionPath) || strings.Contains(err.Error(), "held by") {
		t.Fatalf("SetEffortForTab surfaced raw lease details: %v", err)
	}
	if tab.Ctrl != oldCtrl {
		t.Fatal("tab controller changed after failed effort switch")
	}

	// The failed switch must leave the old runtime alive: after the other
	// window releases the lease, retrying from the same tab has to succeed.
	// (The old code closed the old controller before acquiring the lease, so
	// this retry died on a snapshot of a closed session.)
	externalLease.Release()
	released = true
	if err := app.SetEffortForTab(tab.ID, "max"); err != nil {
		t.Fatalf("SetEffortForTab retry after lease release: %v", err)
	}
	if tab.Ctrl == oldCtrl {
		t.Fatal("retry did not rebuild the controller")
	}
}

func TestSetEffortForTabReanchorsDepthCapRecoveryBranch(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{{
		Name:             "old",
		Kind:             "openai",
		BaseURL:          "https://example.invalid/v1",
		Model:            "old-model",
		APIKeyEnv:        "OLD_MODEL_KEY",
		SupportedEfforts: []string{"low", "max"},
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	recoveryPath := filepath.Join(dir, "effort-switch-conflict-recovery-deadbeef.jsonl")
	disk := agent.NewSession("old system prompt")
	disk.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	disk.Add(provider.Message{Role: provider.RoleAssistant, Content: "one"})
	disk.Add(provider.Message{Role: provider.RoleUser, Content: "disk second"})
	if err := disk.Save(recoveryPath); err != nil {
		t.Fatalf("save recovery branch: %v", err)
	}
	meta, ok, err := agent.LoadBranchMeta(recoveryPath)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta ok=%v err=%v", ok, err)
	}
	meta.Recovered = true
	meta.ParentID = "effort-switch-conflict"
	meta.RecoveryReason = "snapshot conflict"
	meta.RecoveryDepth = agent.SessionRecoveryMaxDepth
	if err := agent.SaveBranchMeta(recoveryPath, meta); err != nil {
		t.Fatalf("SaveBranchMeta: %v", err)
	}

	stale := agent.NewSession("old system prompt")
	stale.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	stale.Add(provider.Message{Role: provider.RoleAssistant, Content: "one"})
	stale.Add(provider.Message{Role: provider.RoleUser, Content: "local second"})
	oldExec := agent.New(nil, nil, stale, agent.Options{}, event.Discard)

	app := NewApp()
	app.ctx = context.Background()
	app.runtimeEvents.emit = func(context.Context, string, ...any) {}
	tab := &WorkspaceTab{
		ID:          "tab_depth_cap_effort",
		Scope:       "global",
		SessionPath: recoveryPath,
		Ready:       true,
		model:       "old/old-model",
		disabledMCP: map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	oldCtrl := control.New(control.Options{
		Executor:            oldExec,
		SessionDir:          dir,
		SessionPath:         recoveryPath,
		Label:               "old",
		Sink:                tab.sink,
		SessionRecoveryMeta: app.tabSessionRecoveryMeta(tab),
		OnSessionRecovered:  app.handleTabSessionRecovered(tab),
	})
	tab.Ctrl = oldCtrl
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})
	stale.IncrementRewrite()

	if err := app.SetEffortForTab(tab.ID, "max"); err != nil {
		t.Fatalf("SetEffortForTab: %v", err)
	}
	if got := tab.Ctrl.SessionPath(); got != recoveryPath {
		t.Fatalf("session path after effort switch = %q, want current recovery branch %q", got, recoveryPath)
	}
	if got := tab.currentSessionPath(); got != recoveryPath {
		t.Fatalf("tab current session path = %q, want %q", got, recoveryPath)
	}
	if tab.sessionLease == nil || sessionRuntimeKey(tab.sessionLease.Path()) != sessionRuntimeKey(recoveryPath) {
		t.Fatalf("tab lease path = %q, want %q", tab.sessionLeaseRuntimeKey(), recoveryPath)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*-recovery-*.jsonl"))
	if err != nil {
		t.Fatalf("glob recovery branches: %v", err)
	}
	matches = primarySessionFiles(matches)
	if len(matches) != 1 || matches[0] != recoveryPath {
		t.Fatalf("recovery branches after effort switch = %v, want only %q", matches, recoveryPath)
	}

	lines := readConflictLogLines(t, store.SessionConflictLog(recoveryPath))
	if len(lines) != 1 {
		t.Fatalf("conflict log lines = %v, want one depth-cap diagnostic", lines)
	}
	if !strings.Contains(lines[0], `"outcome":"recovery_depth_cap_force_saved"`) {
		t.Fatalf("conflict diagnostic = %s, want depth-cap outcome", lines[0])
	}
	if strings.Contains(lines[0], dir) || strings.Contains(lines[0], recoveryPath) {
		t.Fatalf("conflict diagnostic leaked local path: %s", lines[0])
	}

	if err := tab.Ctrl.Snapshot(); err != nil {
		t.Fatalf("Snapshot after effort switch recovery: %v", err)
	}
	afterLines := readConflictLogLines(t, store.SessionConflictLog(recoveryPath))
	if len(afterLines) != len(lines) {
		t.Fatalf("follow-up snapshot appended conflict diagnostics: before=%v after=%v", lines, afterLines)
	}
	matches, err = filepath.Glob(filepath.Join(dir, "*-recovery-*.jsonl"))
	if err != nil {
		t.Fatalf("glob recovery branches after snapshot: %v", err)
	}
	matches = primarySessionFiles(matches)
	if len(matches) != 1 || matches[0] != recoveryPath {
		t.Fatalf("recovery branches after follow-up snapshot = %v, want only %q", matches, recoveryPath)
	}
}

func TestSetModelForTabRefreshesCarriedSystemPromptWithoutChangingDefaults(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "NEW_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
		{Name: "new", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "new-model", APIKeyEnv: "NEW_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := os.MkdirAll(config.MemoryUserDir(), 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	const freshRule = "Fresh global AGENTS rule for model switch"
	if err := os.WriteFile(filepath.Join(config.MemoryUserDir(), "AGENTS.md"), []byte(freshRule), 0o644); err != nil {
		t.Fatalf("write global AGENTS.md: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	oldSession := agent.NewSession("old system prompt without memory")
	oldSession.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	oldExec := agent.New(nil, nil, oldSession, agent.Options{}, event.Discard)
	oldPath := filepath.Join(dir, "old.jsonl")
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: oldPath, Label: "old", Sink: event.Discard})

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:          "tab_a",
		Scope:       "global",
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_a", app: app},
		disabledMCP: map[string]ServerView{},
	}
	sibling := &WorkspaceTab{
		ID:          "tab_b",
		Scope:       "global",
		Ready:       true,
		model:       "old/old-model",
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab, sibling.ID: sibling}
	app.tabOrder = []string{tab.ID, sibling.ID}
	app.activeTabID = tab.ID
	var switchTiming modelSwitchTiming
	app.modelSwitchTimingHook = func(timing modelSwitchTiming) { switchTiming = timing }
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	})

	if err := app.SetModelForTab(tab.ID, "new/new-model"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	history := tab.Ctrl.History()
	if len(history) < 2 {
		t.Fatalf("history length = %d, want system + user", len(history))
	}
	if history[0].Role != provider.RoleSystem {
		t.Fatalf("first message role = %s, want system", history[0].Role)
	}
	if !strings.Contains(history[0].Content, freshRule) {
		t.Fatalf("refreshed system prompt missing global AGENTS rule:\n%s", history[0].Content)
	}
	if history[1].Role != provider.RoleUser || history[1].Content != "hello" {
		t.Fatalf("carried user message changed: %+v", history[1])
	}
	if got := config.LoadForEdit(config.UserConfigPath()).DefaultModel; got != "old/old-model" {
		t.Fatalf("default model after session switch = %q, want old/old-model", got)
	}
	if sibling.model != "old/old-model" {
		t.Fatalf("sibling tab model after session switch = %q, want old/old-model", sibling.model)
	}
	if switchTiming.Outcome != "ok" || switchTiming.Total <= 0 {
		t.Fatalf("model switch timing = %+v, want successful non-zero observation", switchTiming)
	}
	if switchTiming.Build <= 0 || switchTiming.LeaseAndResume <= 0 || switchTiming.SwapAndPersist <= 0 {
		t.Fatalf("model switch stage timing incomplete: %+v", switchTiming)
	}
}

// TestSetModelForTabRestoresSessionAuthorizations pins the fix for a model
// switch dropping same-session "Allow for this session" tool grants and
// Plan-mode read-only command trust, forcing the user to re-approve
// something already granted this session after every model/effort/token-mode
// switch.
func TestSetModelForTabRestoresSessionAuthorizations(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "NEW_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
		{Name: "new", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "new-model", APIKeyEnv: "NEW_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	oldExec := agent.New(nil, nil, agent.NewSession("old system prompt"), agent.Options{}, event.Discard)
	oldPath := filepath.Join(dir, "old.jsonl")
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: oldPath, Label: "old", Sink: event.Discard})
	oldCtrl.RestoreSessionAuthorizations(control.SessionAuthorizations{
		Grants:                   []string{"bash|go test ./..."},
		PlanModeReadOnlyCommands: []string{"go test ./..."},
	})

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:          "tab_a",
		Scope:       "global",
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_a", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	})

	if err := app.SetModelForTab(tab.ID, "new/new-model"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}

	newCtrl, ok := tab.Ctrl.(*control.Controller)
	if !ok {
		t.Fatalf("tab.Ctrl = %T, want *control.Controller", tab.Ctrl)
	}
	got := newCtrl.SessionAuthorizations()
	if len(got.Grants) != 1 || got.Grants[0] != "bash|go test ./..." {
		t.Fatalf("restored grants = %+v, want [\"bash|go test ./...\"]", got.Grants)
	}
	if len(got.PlanModeReadOnlyCommands) != 1 || got.PlanModeReadOnlyCommands[0] != "go test ./..." {
		t.Fatalf("restored plan-mode read-only commands = %+v, want [\"go test ./...\"]", got.PlanModeReadOnlyCommands)
	}
}

// TestRebuildSettingLockedRestoresSessionAuthorizations covers the same
// dropped-session-authorization bug for the settings-change rebuild path
// (also used by the deferred-rebuild retry loop), independent from
// SetModelForTab's own rebuild.
func TestRebuildSettingLockedRestoresSessionAuthorizations(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	oldExec := agent.New(nil, nil, agent.NewSession("old system prompt"), agent.Options{}, event.Discard)
	oldPath := filepath.Join(dir, "old.jsonl")
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: oldPath, Label: "old", Sink: event.Discard})
	oldCtrl.RestoreSessionAuthorizations(control.SessionAuthorizations{
		Grants:                   []string{"bash|go test ./..."},
		PlanModeReadOnlyCommands: []string{"go test ./..."},
	})

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:          "tab_a",
		Scope:       "global",
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_a", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	app.readyHook = func() {}
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	})

	if err := app.rebuildSetting("settings"); err != nil {
		t.Fatalf("rebuildSetting: %v", err)
	}

	newCtrl, ok := tab.Ctrl.(*control.Controller)
	if !ok {
		t.Fatalf("tab.Ctrl = %T, want *control.Controller", tab.Ctrl)
	}
	got := newCtrl.SessionAuthorizations()
	if len(got.Grants) != 1 || got.Grants[0] != "bash|go test ./..." {
		t.Fatalf("restored grants = %+v, want [\"bash|go test ./...\"]", got.Grants)
	}
	if len(got.PlanModeReadOnlyCommands) != 1 || got.PlanModeReadOnlyCommands[0] != "go test ./..." {
		t.Fatalf("restored plan-mode read-only commands = %+v, want [\"go test ./...\"]", got.PlanModeReadOnlyCommands)
	}
}

func TestSetModelForTabContinuesRecoveryPathAfterSnapshotConflict(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "NEW_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
		{Name: "new", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "new-model", APIKeyEnv: "NEW_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	originalPath := filepath.Join(dir, "model-switch-conflict.jsonl")
	current := agent.NewSession("old system prompt")
	current.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	current.Add(provider.Message{Role: provider.RoleAssistant, Content: "one"})
	current.Add(provider.Message{Role: provider.RoleUser, Content: "disk second"})
	if err := current.Save(originalPath); err != nil {
		t.Fatalf("save current session: %v", err)
	}

	stale := agent.NewSession("old system prompt")
	stale.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	stale.Add(provider.Message{Role: provider.RoleAssistant, Content: "one"})
	stale.Add(provider.Message{Role: provider.RoleUser, Content: "local second"})
	oldExec := agent.New(nil, nil, stale, agent.Options{}, event.Discard)

	app := NewApp()
	app.ctx = context.Background()
	app.runtimeEvents.emit = func(context.Context, string, ...any) {}
	tab := &WorkspaceTab{
		ID:          "tab_recovery_model",
		Scope:       "global",
		SessionPath: originalPath,
		Ready:       true,
		model:       "old/old-model",
		disabledMCP: map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	oldCtrl := control.New(control.Options{
		Executor:            oldExec,
		SessionDir:          dir,
		SessionPath:         originalPath,
		Label:               "old",
		Sink:                tab.sink,
		SessionRecoveryMeta: app.tabSessionRecoveryMeta(tab),
		OnSessionRecovered:  app.handleTabSessionRecovered(tab),
	})
	tab.Ctrl = oldCtrl
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	if err := app.SetModelForTab(tab.ID, "new/new-model"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	recoveryPath := tab.Ctrl.SessionPath()
	if recoveryPath == "" || recoveryPath == originalPath || !strings.Contains(filepath.Base(recoveryPath), "-recovery-") {
		t.Fatalf("model switch session path = %q, want recovery path distinct from %q", recoveryPath, originalPath)
	}
	if got := tab.currentSessionPath(); got != recoveryPath {
		t.Fatalf("tab current session path = %q, want recovery path %q", got, recoveryPath)
	}
	if tab.sessionLease == nil || sessionRuntimeKey(tab.sessionLease.Path()) != sessionRuntimeKey(recoveryPath) {
		t.Fatalf("tab lease path = %q, want recovery path %q", tab.sessionLeaseRuntimeKey(), recoveryPath)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*-recovery-*.jsonl"))
	if err != nil {
		t.Fatalf("glob recovery branches: %v", err)
	}
	matches = primarySessionFiles(matches)
	if len(matches) != 1 || matches[0] != recoveryPath {
		t.Fatalf("recovery branches after model switch = %v, want only %q", matches, recoveryPath)
	}
	if err := tab.Ctrl.Snapshot(); err != nil {
		t.Fatalf("Snapshot after model switch recovery: %v", err)
	}
	matches, err = filepath.Glob(filepath.Join(dir, "*-recovery-*.jsonl"))
	if err != nil {
		t.Fatalf("glob recovery branches after snapshot: %v", err)
	}
	matches = primarySessionFiles(matches)
	if len(matches) != 1 || matches[0] != recoveryPath {
		t.Fatalf("recovery branches after follow-up snapshot = %v, want only %q", matches, recoveryPath)
	}
}

func TestSetModelForTabReusesCurrentSessionLease(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "NEW_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
		{Name: "new", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "new-model", APIKeyEnv: "NEW_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	oldSession := agent.NewSession("old system prompt")
	oldSession.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	oldExec := agent.New(nil, nil, oldSession, agent.Options{}, event.Discard)
	oldPath := filepath.Join(dir, "leased-model-switch.jsonl")
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: oldPath, Label: "old", Sink: event.Discard})

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:          "tab_a",
		Scope:       "global",
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_a", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	if err := tab.ensureSessionLease(oldPath); err != nil {
		t.Fatalf("ensureSessionLease: %v", err)
	}
	if err := app.SetModelForTab(tab.ID, "new/new-model"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	if tab.Ctrl == nil || tab.Ctrl == oldCtrl {
		t.Fatalf("tab controller was not rebuilt")
	}
	if got := tab.model; got != "new/new-model" {
		t.Fatalf("tab model = %q, want new/new-model", got)
	}
	if tab.sessionLease == nil || sessionRuntimeKey(tab.sessionLease.Path()) != sessionRuntimeKey(oldPath) {
		t.Fatalf("session lease path = %q, want %q", tab.currentSessionPath(), oldPath)
	}
	history := tab.Ctrl.History()
	if len(history) < 2 || history[1].Role != provider.RoleUser || history[1].Content != "hello" {
		t.Fatalf("carried history = %+v, want original user message", history)
	}
}

func TestSetModelForTabWaitsForConcurrentBlankSessionLease(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "NEW_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
		{Name: "new", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "new-model", APIKeyEnv: "NEW_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := desktopSessionDir(globalTabWorkspaceRoot())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := filepath.Join(dir, "blank-model-switch-race.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write blank session: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:            "tab_blank_race",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		SessionPath:   path,
		Ready:         true,
		model:         "old/old-model",
		sink:          &tabEventSink{tabID: "tab_blank_race", app: app},
		disabledMCP:   map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	acquired := make(chan struct{})
	releaseHook := make(chan struct{})
	var once sync.Once
	// releaseOnce guarantees the hook is released exactly once: the happy
	// path closes it inline, and a Fatalf before that point still unblocks
	// the parked background goroutine during cleanup instead of hanging the
	// test binary (the goroutine must finish before releaseSessionLease in
	// the tab cleanup can return).
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseHook) }) }
	sessionLeaseAcquireHookForTest = func() {
		once.Do(func() {
			close(acquired)
			<-releaseHook
		})
	}
	t.Cleanup(func() { sessionLeaseAcquireHookForTest = nil })
	t.Cleanup(release)

	buildErr := make(chan error, 1)
	go func() {
		buildErr <- tab.ensureSessionLease(path)
	}()

	select {
	case <-acquired:
	case err := <-buildErr:
		t.Fatalf("background lease acquire returned before hook: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("background lease acquire did not start")
	}

	switchErr := make(chan error, 1)
	go func() {
		switchErr <- app.SetModelForTab(tab.ID, "new/new-model")
	}()

	select {
	case err := <-switchErr:
		t.Fatalf("SetModelForTab returned before concurrent lease was bound: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	release()
	if err := <-buildErr; err != nil {
		t.Fatalf("background ensureSessionLease: %v", err)
	}
	if err := <-switchErr; err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	if tab.Ctrl == nil {
		t.Fatal("model switch did not build a controller")
	}
	if got := tab.model; got != "new/new-model" {
		t.Fatalf("tab model = %q, want new/new-model", got)
	}
	if tab.sessionLease == nil || sessionRuntimeKey(tab.sessionLease.Path()) != sessionRuntimeKey(path) {
		t.Fatalf("session lease path = %q, want %q", tab.currentSessionPath(), path)
	}
}

func TestSetModelForTabReattachesDetachedRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "NEW_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
		{Name: "new", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "new-model", APIKeyEnv: "NEW_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := desktopSessionDir(globalTabWorkspaceRoot())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "detached-model-switch.jsonl")
	oldSession := agent.NewSession("old system prompt")
	oldSession.Add(provider.Message{Role: provider.RoleUser, Content: "hello from detached"})
	oldExec := agent.New(nil, nil, oldSession, agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: path, Label: "old", Sink: event.Discard})
	lease, err := agent.TryAcquireSessionLease(path)
	if err != nil {
		t.Fatalf("TryAcquireSessionLease: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	key := sessionRuntimeKey(path)
	detached := &WorkspaceTab{
		ID:             detachedRuntimeTabID(key),
		Scope:          "global",
		SessionPath:    path,
		Ctrl:           oldCtrl,
		Ready:          true,
		model:          "old/old-model",
		disabledMCP:    map[string]ServerView{},
		SharedHostKey:  "detached-host",
		ActivityStatus: "",
	}
	detached.adoptSessionLease(lease)
	tab := &WorkspaceTab{
		ID:          "tab_a",
		Scope:       "global",
		SessionPath: path,
		Ready:       true,
		model:       "old/old-model",
		sink:        &tabEventSink{tabID: "tab_a", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.detachedSessions = map[string]*WorkspaceTab{key: detached}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
		if detached.sessionLease != nil {
			detached.releaseSessionLease()
		}
	})

	if err := app.SetModelForTab(tab.ID, "new/new-model"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	if _, ok := app.detachedSessions[key]; ok {
		t.Fatal("detached runtime was not consumed")
	}
	if tab.Ctrl == nil || tab.Ctrl == oldCtrl {
		t.Fatalf("tab controller was not rebuilt from detached runtime")
	}
	if got := tab.model; got != "new/new-model" {
		t.Fatalf("tab model = %q, want new/new-model", got)
	}
	if tab.sessionLease == nil || sessionRuntimeKey(tab.sessionLease.Path()) != key {
		t.Fatalf("session lease path = %q, want %q", tab.currentSessionPath(), path)
	}
	history := tab.Ctrl.History()
	if len(history) < 2 || history[1].Content != "hello from detached" {
		t.Fatalf("carried history = %+v, want detached user message", history)
	}
}

func TestEnsureTabControllerWorkspaceRebuildsStaleWorkspace(t *testing.T) {
	f := newStaleWorkspaceBindingFixture(t, "rebuild_workspace")

	if err := f.app.ensureTabControllerWorkspace(f.tab); err != nil {
		t.Fatalf("ensureTabControllerWorkspace: %v", err)
	}
	assertTabRebuiltToPinnedWorkspace(t, f)
}

func TestEnsureTabControllerWorkspaceWarnsWhenPinnedSessionSwitchesWorkspace(t *testing.T) {
	f := newStaleWorkspaceBindingFixture(t, "warn_workspace_switch")
	events := make(chan event.Event, 8)
	f.tab.sink.SetBotSink(event.FuncSink(func(e event.Event) {
		events <- e
	}))

	if err := f.app.ensureTabControllerWorkspace(f.tab); err != nil {
		t.Fatalf("ensureTabControllerWorkspace: %v", err)
	}
	assertTabRebuiltToPinnedWorkspace(t, f)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Kind == event.Notice &&
				e.Level == event.LevelWarn &&
				strings.Contains(strings.ToLower(e.Text), strings.ToLower(f.projectA)) &&
				strings.Contains(e.Text, "switched tab") {
				return
			}
		case <-deadline:
			t.Fatal("did not receive workspace switch warning notice")
		}
	}
}

func TestSteerForTabReconcilesStaleWorkspaceBeforeRejectingIdleGuidance(t *testing.T) {
	f := newStaleWorkspaceBindingFixture(t, "steer_idle_fallback")

	err := f.app.SteerForTab(f.tab.ID, "steer guidance")
	if err == nil || !strings.Contains(err.Error(), "remain queued") {
		t.Fatalf("SteerForTab error = %v, want explicit rejected-guidance result", err)
	}
	assertTabRebuiltToPinnedWorkspace(t, f)
}

func TestCompactReconcilesStaleWorkspaceBeforeCompaction(t *testing.T) {
	f := newStaleWorkspaceBindingFixture(t, "compact")

	if err := f.app.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	assertTabRebuiltToPinnedWorkspace(t, f)
}

func TestEffortCommandUsesPinnedSessionOwnerBeforeStaleWorkspaceRoot(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OWNER_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "STALE_MODEL_KEY", "sk-test")

	projectA := t.TempDir()
	projectB := t.TempDir()
	if err := addProject(projectA, "Project A"); err != nil {
		t.Fatalf("add project A: %v", err)
	}
	if err := addProject(projectB, "Project B"); err != nil {
		t.Fatalf("add project B: %v", err)
	}
	ownerConfig := `default_model = "owner/owner-model"
[[providers]]
name = "owner"
kind = "openai"
base_url = "https://owner.example.invalid/v1"
model = "owner-model"
api_key_env = "OWNER_MODEL_KEY"
supported_efforts = ["max"]
default_effort = "max"
`
	if err := os.WriteFile(filepath.Join(projectA, "patty.toml"), []byte(ownerConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	staleConfig := `default_model = "stale/stale-model"
[[providers]]
name = "stale"
kind = "openai"
base_url = "https://stale.example.invalid/v1"
model = "stale-model"
api_key_env = "STALE_MODEL_KEY"
reasoning_protocol = "none"
`
	if err := os.WriteFile(filepath.Join(projectB, "patty.toml"), []byte(staleConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	topicID := "topic_effort_owner"
	topicTitle := "Effort owner"
	sessionDirA := desktopSessionDir(projectA)
	sessionDirB := desktopSessionDir(projectB)
	if err := os.MkdirAll(sessionDirA, 0o755); err != nil {
		t.Fatalf("mkdir project A sessions: %v", err)
	}
	if err := os.MkdirAll(sessionDirB, 0o755); err != nil {
		t.Fatalf("mkdir project B sessions: %v", err)
	}
	sessionPathA := writeTopicSessionWithPrompt(t, sessionDirA, "project-a.jsonl", topicID, topicTitle, projectA, "project A prompt", time.Now())
	oldCtrl := control.New(control.Options{
		SessionDir:    sessionDirB,
		SessionPath:   filepath.Join(sessionDirB, "wrong.jsonl"),
		WorkspaceRoot: projectB,
		Sink:          event.Discard,
	})

	app := NewApp()
	app.readyHook = func() {}
	tab := &WorkspaceTab{
		ID:            "tab_stale_effort",
		Scope:         "project",
		WorkspaceRoot: projectB,
		TopicID:       topicID,
		TopicTitle:    topicTitle,
		SessionPath:   sessionPathA,
		Ready:         true,
		Ctrl:          oldCtrl,
		sink:          &tabEventSink{tabID: "tab_stale_effort", app: app},
		disabledMCP:   map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	})

	if err := app.SubmitToTab(tab.ID, "/effort max"); err != nil {
		t.Fatalf("SubmitToTab(/effort max): %v", err)
	}
	waitNotRunning(t, tab.Ctrl)
	if tab.effort == nil || *tab.effort != "max" {
		t.Fatalf("tab effort = %#v, want max from pinned project A provider", tab.effort)
	}
	if got := normalizeProjectRoot(tab.WorkspaceRoot); got != normalizeProjectRoot(projectA) {
		t.Fatalf("tab workspace root = %q, want project A %q", got, normalizeProjectRoot(projectA))
	}
	if got := normalizeProjectRoot(tab.Ctrl.WorkspaceRoot()); got != normalizeProjectRoot(projectA) {
		t.Fatalf("controller workspace root = %q, want project A %q", got, normalizeProjectRoot(projectA))
	}
}

func TestClassicLayoutQuickClicksSerializeWorkspaceRebuild(t *testing.T) {
	runQuickClickWorkspaceReconcileTest(t, "classic")
}

func TestWorkbenchLayoutQuickClicksSerializeWorkspaceRebuild(t *testing.T) {
	runQuickClickWorkspaceReconcileTest(t, "workbench")
}

func TestCreationLayoutQuickClicksSerializeWorkspaceRebuild(t *testing.T) {
	runQuickClickWorkspaceReconcileTest(t, "creation")
}

func TestClearActiveSessionRuntimeSupersedesInFlightStartupBuild(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "old",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Model:     "old-model",
		APIKeyEnv: "OLD_MODEL_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "clear-runtime-in-flight.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write placeholder session: %v", err)
	}

	oldSession := agent.NewSession("old system prompt")
	oldExec := agent.New(nil, nil, oldSession, agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: sessionPath, Label: "old", Sink: event.Discard})

	app := NewApp()
	// A runtime is attached while an older async build is still in flight
	// (e.g. attached via topic activation); destroying the session must
	// invalidate that build so it cannot resurrect the destroyed session.
	buildCtx, buildCancel := context.WithCancel(context.Background())
	tab := &WorkspaceTab{
		ID:              "tab_clear",
		Scope:           "global",
		SessionPath:     sessionPath,
		model:           "old/old-model",
		Ready:           true,
		Ctrl:            oldCtrl,
		buildGeneration: 1,
		buildCancel:     buildCancel,
		disabledMCP:     map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(tab.releaseSessionLease)

	if err := app.clearActiveSessionRuntime(tab, oldCtrl); err != nil {
		t.Fatalf("clearActiveSessionRuntime: %v", err)
	}
	if tab.Ctrl == nil || tab.Ctrl == oldCtrl {
		t.Fatalf("clear did not install a fresh controller (ctrl=%v)", tab.Ctrl)
	}
	defer tab.Ctrl.Close()
	assertTabBuildSuperseded(t, app, tab, 1, buildCtx)
}

func TestClearActiveSessionRuntimeReleasesResourcesWhenTabReplaced(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "old",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Model:     "old-model",
		APIKeyEnv: "OLD_MODEL_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "clear-runtime-replaced-tab.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write placeholder session: %v", err)
	}

	oldSession := agent.NewSession("old system prompt")
	oldExec := agent.New(nil, nil, oldSession, agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: sessionPath, Label: "old", Sink: event.Discard})

	app := NewApp()
	tab := &WorkspaceTab{
		ID:          "tab_replaced",
		Scope:       "global",
		SessionPath: sessionPath,
		model:       "old/old-model",
		Ready:       true,
		Ctrl:        oldCtrl,
		disabledMCP: map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	// The tab entry now points at a replacement struct (the tab was closed and
	// reopened while the clear ran off-lock), so the swap must not apply.
	replacement := &WorkspaceTab{ID: tab.ID, Scope: "global"}
	app.tabs = map[string]*WorkspaceTab{tab.ID: replacement}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(tab.releaseSessionLease)

	err := app.clearActiveSessionRuntime(tab, oldCtrl)
	if err == nil || !strings.Contains(err.Error(), "changed while clearing") {
		t.Fatalf("clearActiveSessionRuntime error = %v, want tab-changed error", err)
	}
	if replacement.Ctrl != nil {
		t.Fatalf("replacement tab controller = %v, want untouched nil", replacement.Ctrl)
	}
	if tab.Ctrl != oldCtrl {
		t.Fatalf("replaced tab controller = %v, want left on the destroyed runtime", tab.Ctrl)
	}
	if key := tab.sessionLeaseRuntimeKey(); key != "" {
		t.Fatalf("replaced tab still holds a session lease for %q; the fresh lease leaked", key)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("old session artifacts were not destroyed (stat err=%v)", err)
	}
}

func TestConnectKeyRejectsUnusableCatalogBeforeSavingKey(t *testing.T) {
	tests := []struct {
		name      string
		fetch     func(context.Context, config.ProviderEntry, string) ([]string, error)
		wantError string
	}{
		{
			name: "model fetch failure",
			fetch: func(context.Context, config.ProviderEntry, string) ([]string, error) {
				return nil, errors.New("catalog unavailable")
			},
			wantError: "validate: catalog unavailable",
		},
		{
			name: "stock model missing",
			fetch: func(context.Context, config.ProviderEntry, string) ([]string, error) {
				return []string{"small", "large"}, nil
			},
			// wantError is derived from the live stock entry below.
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			entry := onboardingTestProvider(t)
			if test.name == "stock model missing" {
				test.wantError = fmt.Sprintf("validate: stock model %q is unavailable", entry.Model)
			}
			t.Setenv(entry.APIKeyEnv, "")
			if err := os.Unsetenv(entry.APIKeyEnv); err != nil {
				t.Fatalf("unset onboarding key: %v", err)
			}

			oldFetch := connectKeyModelFetch
			connectKeyModelFetch = test.fetch
			t.Cleanup(func() { connectKeyModelFetch = oldFetch })

			app := NewApp()
			app.ctx = context.Background()
			_, err := app.ConnectKey("sk-test")
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("ConnectKey error = %v, want %q", err, test.wantError)
			}
			data, readErr := os.ReadFile(config.UserCredentialsPath())
			if readErr == nil && strings.Contains(string(data), entry.APIKeyEnv) {
				t.Fatalf("unusable onboarding key should not be saved:\n%s", data)
			}
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatalf("read credential store: %v", readErr)
			}
		})
	}
}

func TestConnectKeyRejectsProviderConflictBeforeSavingKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	entry := onboardingTestProvider(t)
	t.Setenv(entry.APIKeyEnv, "")
	_ = os.Unsetenv(entry.APIKeyEnv)
	cfg := config.Default()
	cfg.Providers = []config.ProviderEntry{{
		Name: "patty", Kind: "openai", BaseURL: "https://custom.example.test/v1",
		Model: "custom-medium", APIKeyEnv: entry.APIKeyEnv,
	}}
	cfg.DefaultModel = "patty/custom-medium"
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save conflicting provider: %v", err)
	}

	oldFetch := connectKeyModelFetch
	connectKeyModelFetch = func(context.Context, config.ProviderEntry, string) ([]string, error) {
		return []string{dariPattyStockEntry().Model}, nil
	}
	t.Cleanup(func() { connectKeyModelFetch = oldFetch })

	app := NewApp()
	app.ctx = context.Background()
	_, err := app.ConnectKey("sk-test")
	if err == nil || !strings.Contains(err.Error(), `provider "patty" conflicts`) {
		t.Fatalf("ConnectKey conflict error = %v", err)
	}
	if data, readErr := os.ReadFile(config.UserCredentialsPath()); readErr == nil && strings.Contains(string(data), entry.APIKeyEnv) {
		t.Fatalf("conflicting onboarding key should not be saved:\n%s", data)
	}
}

func TestConnectKeyRestoresPattyProviderAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	cfg := config.Default()
	cfg.DefaultModel = "custom/custom-model"
	cfg.Desktop.ProviderAccess = []string{"custom"}
	cfg.Providers = []config.ProviderEntry{
		{
			Name: "custom", Kind: "openai", BaseURL: "https://models.example.invalid/v1",
			Model: "custom-model", APIKeyEnv: "AGENTS_PATTY_API_KEY",
		},
		{
			Name: "patty", Kind: "anthropic", BaseURL: "https://omni.agents.patty.io/v1",
			Model: "wrong-model", APIKeyEnv: "CUSTOM_PATTY_KEY",
			Headers: map[string]string{"X-Patty-Route": "custom"},
		},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save custom provider config: %v", err)
	}

	oldFetch := connectKeyModelFetch
	stockProbe := dariPattyStockEntry()
	connectKeyModelFetch = func(_ context.Context, entry config.ProviderEntry, apiKey string) ([]string, error) {
		if entry.BaseURL != stockProbe.BaseURL || apiKey != "sk-test" {
			t.Fatalf("model probe = %q/%q", entry.BaseURL, apiKey)
		}
		return []string{stockProbe.Model}, nil
	}
	t.Cleanup(func() { connectKeyModelFetch = oldFetch })

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.setTestCtrl(control.New(control.Options{Label: "custom"}), "custom/custom-model")
	defer func() {
		if ctrl := app.activeCtrl(); ctrl != nil {
			ctrl.Close()
		}
	}()
	if _, err := app.ConnectKey("sk-test"); err != nil {
		t.Fatalf("ConnectKey: %v", err)
	}

	got := config.LoadForEditWithoutCredentials(config.UserConfigPath())
	if !providerAccessSet(got.Desktop.ProviderAccess)["patty"] {
		t.Fatalf("provider_access = %v, want Patty restored", got.Desktop.ProviderAccess)
	}
	patty, ok := got.Provider("patty")
	if !ok {
		t.Fatal("Patty provider template should be restored")
	}
	stock := dariPattyStockEntry()
	if patty.Kind != stock.Kind || patty.APIKeyEnv != stock.APIKeyEnv || patty.Model != stock.Model || patty.ContextWindow != stock.ContextWindow || patty.BaseURL != stock.BaseURL {
		t.Fatalf("restored Patty provider = %+v, want stock wire contract %+v", patty, stock)
	}
	if patty.Headers["X-Patty-Route"] != "custom" {
		t.Fatalf("restored Patty provider lost safe custom headers: %+v", patty.Headers)
	}
	if app.NeedsOnboarding() {
		t.Fatal("restored Patty access and saved key should satisfy onboarding")
	}
}

func TestBalanceForTabUsesDesktopPricingCurrency(t *testing.T) {
	isolateDesktopUserDirs(t)
	cfg := config.Default()
	cfg.Desktop.Currency = "USD"
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save USD desktop currency: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"is_available":true,"balance_infos":[{"currency":"KRW","total_balance":"70.16"},{"currency":"USD","total_balance":"9.82"}]}`)
	}))
	defer srv.Close()

	app := NewApp()
	app.ctx = context.Background()
	ctrl := control.New(control.Options{BalanceURL: srv.URL, BalanceClient: srv.Client()})
	t.Cleanup(ctrl.Close)
	app.setTestCtrl(ctrl, "deepseek/deepseek-v4-flash")

	got := app.BalanceForTab("test")
	if !got.Available || got.Display != "$9.82" || got.Err != "" {
		t.Fatalf("USD desktop balance = %+v, want available $9.82", got)
	}
}

func TestConnectKeyRebuildLeaseHeldKeepsCurrentController(t *testing.T) {
	isolateDesktopUserDirs(t)
	keyEnv := onboardingTestProvider(t).APIKeyEnv
	t.Setenv(keyEnv, "")
	os.Unsetenv(keyEnv)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	oldFetch := connectKeyModelFetch
	connectKeyModelFetch = func(context.Context, config.ProviderEntry, string) ([]string, error) {
		return []string{dariPattyStockEntry().Model}, nil
	}
	t.Cleanup(func() { connectKeyModelFetch = oldFetch })

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "externally-leased-connect-key.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write placeholder session: %v", err)
	}
	externalLease, err := agent.TryAcquireSessionLease(sessionPath)
	if err != nil {
		t.Fatalf("TryAcquireSessionLease: %v", err)
	}
	defer externalLease.Release()

	oldSession := agent.NewSession("old system prompt")
	oldSession.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	oldExec := agent.New(nil, nil, oldSession, agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: sessionPath, Label: "old", Sink: event.Discard})
	defer oldCtrl.Close()

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:          "tab_connect",
		Scope:       "global",
		SessionPath: sessionPath,
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_connect", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	warning, err := app.ConnectKey("sk-test")
	if err != nil {
		t.Fatalf("ConnectKey: %v", err)
	}
	if !strings.Contains(warning, "another patty window") {
		t.Fatalf("ConnectKey warning = %q, want user-facing lease warning", warning)
	}
	if tab.Ctrl != oldCtrl {
		t.Fatalf("tab controller changed after failed connect-key rebuild")
	}
	if tab.StartupErr != "" {
		t.Fatalf("tab startup error = %q, want unchanged current session", tab.StartupErr)
	}
	if !config.CredentialStored(keyEnv) {
		t.Fatal("onboarding key should be persisted even when hot rebuild is deferred")
	}
}

func TestSetEffortRebuildsController(t *testing.T) {
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

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "deepseek/deepseek-v4-flash")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetEffort("max"); err != nil {
		t.Fatalf("SetEffort(max): %v", err)
	}
	if c := app.activeCtrl(); c == nil {
		t.Fatal("SetEffort should leave a rebuilt controller")
	}
	if c := app.activeCtrl(); c == old {
		t.Fatal("SetEffort should rebuild the active controller so the provider sees the new effort")
	}
	if got := app.Effort().Current; got != "max" {
		t.Fatalf("Effort current = %q, want max", got)
	}
}

func TestSetEffortMigratesStaleOfficialDeepSeekTabModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "deepseek/deepseek-v4-flash"
	cfg.Desktop.ProviderAccess = []string{"deepseek"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "deepseek",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		Model:     "glm-5",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "patty/patty-code-standard")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetEffort("max"); err != nil {
		t.Fatalf("SetEffort(max): %v", err)
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if tab.model != "deepseek/deepseek-v4-flash" {
		t.Fatalf("tab model = %q, want migrated official ref", tab.model)
	}
}

// seedOfficialPattyFixture writes an explicit official-patty (stock DARI)
// config plus credential so rebuild paths can resolve the stock model ref
// without depending on the runtime default. A legacy omni fixture would be
// migrated to exactly this shape at load (normalizeLegacyOmniOfficialProvider).
func seedOfficialPattyFixture(t *testing.T) {
	t.Helper()
	stock := dariPattyStockEntry()
	setDesktopTestCredential(t, stock.APIKeyEnv, "sk-test")
	cfg := config.Default()
	cfg.Providers = []config.ProviderEntry{stock}
	cfg.DefaultModel = "patty/" + stock.Model
	cfg.Desktop.ProviderAccess = []string{"patty"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func TestSetTokenModeRebuildsController(t *testing.T) {
	isolateDesktopUserDirs(t)
	seedOfficialPattyFixture(t)

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "patty/patty-code-standard")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetTokenMode("economy"); err != nil {
		t.Fatalf("SetTokenMode(economy): %v", err)
	}
	if c := app.activeCtrl(); c == nil {
		t.Fatal("SetTokenMode should leave a rebuilt controller")
	}
	if c := app.activeCtrl(); c == old {
		t.Fatal("SetTokenMode should rebuild the active controller so the provider sees the new tool profile")
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if got := currentTabTokenMode(tab); got != "economy" {
		t.Fatalf("token mode = %q, want economy", got)
	}
	if got := app.Meta().TokenMode; got != "economy" {
		t.Fatalf("Meta token mode = %q, want economy", got)
	}
	saved := loadTabsFile()
	if len(saved.Tabs) != 1 || saved.Tabs[0].TokenMode != "economy" {
		t.Fatalf("saved tabs = %+v, want economy token mode", saved.Tabs)
	}
}

func TestSetTokenModeDeliveryRebuildsAndPersistsProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	seedOfficialPattyFixture(t)

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "patty/patty-code-standard")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetTokenMode(boot.TokenModeDelivery); err != nil {
		t.Fatalf("SetTokenMode(delivery): %v", err)
	}
	if c := app.activeCtrl(); c == nil || c == old {
		t.Fatal("delivery profile should rebuild the active controller")
	}
	tab := app.activeTab()
	if got := currentTabTokenMode(tab); got != boot.TokenModeDelivery {
		t.Fatalf("token mode = %q, want delivery", got)
	}
	if got := app.Meta().TokenMode; got != boot.TokenModeDelivery {
		t.Fatalf("Meta token mode = %q, want delivery", got)
	}
	saved := loadTabsFile()
	if len(saved.Tabs) != 1 || saved.Tabs[0].TokenMode != boot.TokenModeDelivery {
		t.Fatalf("saved tabs = %+v, want delivery profile", saved.Tabs)
	}

	// Leaving delivery must clear the persisted tokenMode so a restart does not
	// re-arm final-readiness gates (#6582).
	if err := app.SetTokenMode(boot.TokenModeFull); err != nil {
		t.Fatalf("SetTokenMode(full): %v", err)
	}
	if got := currentTabTokenMode(app.activeTab()); got != boot.TokenModeFull {
		t.Fatalf("token mode after full = %q, want full", got)
	}
	if got := app.Meta().TokenMode; got != boot.TokenModeFull {
		t.Fatalf("Meta token mode after full = %q, want full", got)
	}
	saved = loadTabsFile()
	if len(saved.Tabs) != 1 {
		t.Fatalf("saved tabs = %+v", saved.Tabs)
	}
	if saved.Tabs[0].TokenMode != "" {
		t.Fatalf("saved tokenMode = %q, want omitted/empty for full", saved.Tabs[0].TokenMode)
	}
}

func TestSetTokenModeReusesCurrentSessionLease(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	session := agent.NewSession("old system prompt")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	exec := agent.New(nil, nil, session, agent.Options{}, event.Discard)
	path := filepath.Join(dir, "leased-token-mode-switch.jsonl")
	oldCtrl := control.New(control.Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "old", Sink: event.Discard})

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:          "tab_a",
		Scope:       "global",
		Ready:       true,
		model:       "old/old-model",
		Ctrl:        oldCtrl,
		sink:        &tabEventSink{tabID: "tab_a", app: app},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		tab.releaseSessionLease()
	})

	if err := tab.ensureSessionLease(path); err != nil {
		t.Fatalf("ensureSessionLease: %v", err)
	}
	if err := app.SetTokenModeForTab(tab.ID, "economy"); err != nil {
		t.Fatalf("SetTokenModeForTab: %v", err)
	}
	if tab.Ctrl == nil || tab.Ctrl == oldCtrl {
		t.Fatalf("tab controller was not rebuilt")
	}
	if got := currentTabTokenMode(tab); got != "economy" {
		t.Fatalf("token mode = %q, want economy", got)
	}
	if tab.sessionLease == nil || sessionRuntimeKey(tab.sessionLease.Path()) != sessionRuntimeKey(path) {
		t.Fatalf("session lease path = %q, want %q", tab.currentSessionPath(), path)
	}
	history := tab.Ctrl.History()
	if len(history) < 2 || history[1].Role != provider.RoleUser || history[1].Content != "hello" {
		t.Fatalf("carried history = %+v, want original user message", history)
	}
}

func TestSetTokenModeMigratesStaleOfficialDeepSeekTabModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "deepseek/deepseek-v4-flash"
	cfg.Desktop.ProviderAccess = []string{"deepseek"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "deepseek",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		Model:     "glm-5",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "deepseek-flash/deepseek-v4-flash")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetTokenMode("economy"); err != nil {
		t.Fatalf("SetTokenMode(economy): %v", err)
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if tab.model != "deepseek/deepseek-v4-flash" {
		t.Fatalf("tab model = %q, want migrated official ref", tab.model)
	}
	if got := currentTabTokenMode(tab); got != "economy" {
		t.Fatalf("token mode = %q, want economy", got)
	}
}

func TestMetaForTabReportsImageInputCapability(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "CUSTOM_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "custom/text-only"
	cfg.Desktop.ProviderAccess = []string{"custom"}
	cfg.Providers = []config.ProviderEntry{{
		Name:         "custom",
		Kind:         "openai",
		BaseURL:      "https://example.invalid/v1",
		APIKeyEnv:    "CUSTOM_KEY",
		Models:       []string{"text-only", "vision-pro"},
		VisionModels: []string{"vision-pro"},
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.setTestCtrl(control.New(control.Options{Label: "custom/text-only"}), "custom/text-only")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if got := app.Meta().ImageInputEnabled; got {
		t.Fatal("text-only meta should disable image input")
	}
	if err := app.SetModel("custom/vision-pro"); err != nil {
		t.Fatalf("SetModel(custom/vision-pro): %v", err)
	}
	if got := app.Meta().ImageInputEnabled; !got {
		t.Fatal("vision model meta should enable image input")
	}
}

func TestSetTokenModeRejectsBackgroundJobs(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "jobs.jsonl")
	jm := jobs.NewManager(event.Discard)
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	app := NewApp()
	app.ctx = context.Background()
	app.setTestCtrl(ctrl, "old/old-model")
	t.Cleanup(func() {
		if current := app.activeCtrl(); current != nil {
			current.Close()
		}
	})

	release := make(chan struct{})
	job := jm.StartForSession(agent.BranchID(path), "bash", "long job", func(ctx context.Context, _ io.Writer) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-release:
			return "", nil
		}
	})
	t.Cleanup(func() { close(release) })

	err := app.SetTokenMode("economy")
	if err == nil || !strings.Contains(err.Error(), "background_jobs=1") {
		t.Fatalf("SetTokenMode with background job error = %v, want exact background-job guard", err)
	}
	cancelled, err := app.CancelJobForTab("", job.ID)
	if err != nil || !cancelled {
		t.Fatalf("CancelJobForTab = %v, %v, want true, nil", cancelled, err)
	}
	if result := jm.WaitForSession(context.Background(), agent.BranchID(path), []string{job.ID}, 5); len(result) != 1 || result[0].Status != jobs.Killed {
		t.Fatalf("stopped background job = %+v, want one killed result", result)
	}
	if err := app.SetTokenMode("economy"); err != nil {
		t.Fatalf("SetTokenMode after stopping background job: %v", err)
	}
}

func TestClearSessionCancelsRunningRuntimeAndKeepsTopic(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "clear-running.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	oldCtrl := control.New(control.Options{Runner: runner, SessionDir: dir, SessionPath: path, Label: "test"})
	app := NewApp()
	app.projectTreeChangedHook = func() {}
	seedOfficialPattyFixture(t)
	app.setTestCtrl(oldCtrl, "patty/patty-code-standard")
	app.tabs["test"].TopicID = "topic_clear"
	app.tabs["test"].TopicTitle = "Clear topic"
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	oldCtrl.Submit("work")
	<-runner.started
	if err := app.ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	waitNotRunning(t, oldCtrl)
	tab := app.activeTab()
	if tab == nil || tab.Ctrl == nil {
		t.Fatalf("active tab/controller missing after clear")
	}
	if tab.Ctrl == oldCtrl {
		t.Fatalf("clear should replace the active controller after cancelling old work")
	}
	if tab.TopicID != "topic_clear" || tab.TopicTitle != "Clear topic" {
		t.Fatalf("clear changed topic identity: %+v", tab)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("old cleared session artifacts should be removed, stat err = %v", err)
	}
	if got := tab.currentSessionPath(); got == "" || got == path {
		t.Fatalf("new session path = %q, want fresh path", got)
	}
}

func TestClearSessionRemovesRunningJobArtifacts(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "clear-running-job.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	jm := jobs.NewManager(event.Discard)
	oldCtrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	app := NewApp()
	app.projectTreeChangedHook = func() {}
	seedOfficialPattyFixture(t)
	app.setTestCtrl(oldCtrl, "patty/patty-code-standard")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	started := make(chan struct{})
	jm.StartForSession(agent.BranchID(path), "bash", "clear artifact", func(ctx context.Context, _ io.Writer) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	<-started
	jobsDir := jobs.ArtifactDir(path)
	if _, err := os.Stat(jobsDir); err != nil {
		t.Fatalf("job sidecar should exist before clear: %v", err)
	}

	if err := app.ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	if _, err := os.Stat(jobsDir); !os.IsNotExist(err) {
		t.Fatalf("old job sidecar should be removed after clear, stat err = %v", err)
	}
}

func TestBeginTabTurnWorkspaceRepairDoesNotRecursivelyLockAdmission(t *testing.T) {
	fixture := newStaleWorkspaceBindingFixture(t, "admission_writer")
	fixture.tab.reconcileMu.Lock()

	turnDone := make(chan error, 1)
	go func() {
		admission, _, err := fixture.app.beginTabTurn(fixture.tab.ID, false)
		if admission != nil {
			admission.abort()
		}
		turnDone <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for fixture.app.runtimeAdmissionMu.TryLock() {
		fixture.app.runtimeAdmissionMu.Unlock()
		if time.Now().After(deadline) {
			fixture.tab.reconcileMu.Unlock()
			t.Fatal("beginTabTurn never acquired the admission read lock")
		}
		time.Sleep(time.Millisecond)
	}

	writerRebuildLocked := make(chan struct{})
	writerAdmissionLocked := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		fixture.app.runtimeRebuildMu.Lock()
		close(writerRebuildLocked)
		fixture.app.runtimeAdmissionMu.Lock()
		close(writerAdmissionLocked)
		fixture.app.runtimeAdmissionMu.Unlock()
		fixture.app.runtimeRebuildMu.Unlock()
		close(writerDone)
	}()
	<-writerRebuildLocked
	for fixture.app.runtimeAdmissionMu.TryRLock() {
		fixture.app.runtimeAdmissionMu.RUnlock()
		time.Sleep(time.Millisecond)
	}
	fixture.tab.reconcileMu.Unlock()

	select {
	case err := <-turnDone:
		if err != nil {
			t.Fatalf("beginTabTurn after workspace repair: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("workspace repair recursively waited on runtimeAdmissionMu with a writer pending")
	}
	select {
	case <-writerAdmissionLocked:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle writer never acquired runtimeAdmissionMu after repaired turn admission")
	}
	select {
	case <-writerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle writer did not complete after repaired turn admission")
	}
}
