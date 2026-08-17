package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"patty/internal/agent"
	"patty/internal/boot"
	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/evidence"
	"patty/internal/store"
)

func newGoalDeliveryYoloTestApp(t *testing.T, goalStatus string) (*App, *WorkspaceTab, control.SessionAPI, string) {
	t.Helper()
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "GOAL_DELIVERY_KEY", "sk-test")
	setDesktopTestCredential(t, "GOAL_DELIVERY_ALT_KEY", "sk-test")
	cfg := config.Default()
	cfg.DefaultModel = "test/model"
	cfg.Desktop.ProviderAccess = []string{"test", "alt"}
	cfg.Providers = []config.ProviderEntry{
		{
			Name: "test", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "model", APIKeyEnv: "GOAL_DELIVERY_KEY",
			SupportedEfforts: []string{"low", "high"},
		},
		{Name: "alt", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "alt-model", APIKeyEnv: "GOAL_DELIVERY_ALT_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "goal-delivery-yolo.jsonl")
	writeHistoryTestSession(t, path, "continue the delivery")
	checkpoint := evidence.DeliveryCheckpoint{
		ScopeID:             "goal-test-scope",
		CriteriaEstablished: true,
		WorkObserved:        true,
		MutationObserved:    true,
		PendingMutation:     true,
	}
	state := map[string]any{
		"goal":               "ship the combined mode",
		"status":             goalStatus,
		"researchMode":       control.GoalResearchOn,
		"autoResearchTaskID": "research-task-1",
		"scopeID":            checkpoint.ScopeID,
		"deliveryCheckpoint": checkpoint,
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SessionGoalState(path), data, 0o600); err != nil {
		t.Fatalf("write Goal sidecar: %v", err)
	}

	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	exec := agent.New(nil, nil, loaded, agent.Options{}, event.Discard)
	oldCtrl := control.New(control.Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "test/model", Sink: event.Discard})
	oldCtrl.Resume(loaded, path)
	oldCtrl.SetToolApprovalMode(control.ToolApprovalYolo)

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	tab := &WorkspaceTab{
		ID: "tab_goal_delivery_yolo", Scope: "global", Ready: true,
		SessionPath: path, model: "test/model", tokenMode: boot.TokenModeFull,
		mode: "yolo", toolApprovalMode: control.ToolApprovalYolo,
		goal: "stale tab goal", Ctrl: oldCtrl,
		disabledMCP: map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	if err := tab.ensureSessionLease(path); err != nil {
		t.Fatalf("ensure session lease: %v", err)
	}
	app.mu.Lock()
	app.newSessionRuntimeLocked(tab, sessionRuntimeKey(path))
	app.advanceSessionRuntimeEpochLocked(tab)
	app.mu.Unlock()
	t.Cleanup(func() {
		if ctrl := app.controllerForTab(tab); ctrl != nil {
			ctrl.Close()
		}
		tab.releaseSessionLease()
	})
	return app, tab, oldCtrl, path
}

func TestGoalAndCollaborationResyncBeforeSendPreserveRunningDeliveryScope(t *testing.T) {
	app, tab, ctrl, path := newGoalDeliveryYoloTestApp(t, control.GoalStatusRunning)
	tab.goal = ctrl.Goal()
	app.SetCollaborationModeForTab(tab.ID, "goal")
	app.SetGoalForTab(tab.ID, ctrl.Goal())

	var persisted struct {
		ScopeID string `json:"scopeID"`
	}
	data, err := os.ReadFile(store.SessionGoalState(path))
	if err != nil {
		t.Fatalf("read Goal sidecar: %v", err)
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode Goal sidecar: %v", err)
	}
	if persisted.ScopeID != "goal-test-scope" {
		t.Fatalf("Goal resync replaced delivery scope: got %q", persisted.ScopeID)
	}
}
