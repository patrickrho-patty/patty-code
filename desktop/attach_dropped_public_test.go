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
	"time"

	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/event"
)

func TestAttachDroppedOutsideWorkspaceDirRegistersAfterPinnedOwnerRebuild(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "TEST_MODEL_KEY", "sk-test")
	cfg := config.Default()
	cfg.DefaultModel = "test/test-model"
	cfg.Desktop.ProviderAccess = []string{"test"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "test", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "test-model", APIKeyEnv: "TEST_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	projectA := t.TempDir()
	projectB := t.TempDir()
	outside := filepath.Join(t.TempDir(), "External")
	if err := os.MkdirAll(filepath.Join(outside, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "sub", "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := addProject(projectA, "Project A"); err != nil {
		t.Fatalf("add project A: %v", err)
	}
	if err := addProject(projectB, "Project B"); err != nil {
		t.Fatalf("add project B: %v", err)
	}
	sessionDirA := desktopSessionDir(projectA)
	sessionDirB := desktopSessionDir(projectB)
	if err := os.MkdirAll(sessionDirA, 0o755); err != nil {
		t.Fatalf("mkdir project A sessions: %v", err)
	}
	if err := os.MkdirAll(sessionDirB, 0o755); err != nil {
		t.Fatalf("mkdir project B sessions: %v", err)
	}
	sessionPathA := writeTopicSessionWithPrompt(t, sessionDirA, "project-a.jsonl", "topic_external_ref", "External ref", projectA, "project A prompt", time.Now())
	sessionPathB := filepath.Join(sessionDirB, "wrong.jsonl")
	oldCtrl := control.New(control.Options{
		SessionDir:    sessionDirB,
		SessionPath:   sessionPathB,
		WorkspaceRoot: projectB,
		Sink:          event.Discard,
	})
	app := NewApp()
	app.readyHook = func() {}
	tab := &WorkspaceTab{
		ID:            "project",
		Scope:         "project",
		WorkspaceRoot: projectB,
		TopicID:       "topic_external_ref",
		TopicTitle:    "External ref",
		SessionPath:   sessionPathA,
		Ready:         true,
		model:         "test/test-model",
		Ctrl:          oldCtrl,
		sink:          &tabEventSink{tabID: "project", app: app},
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

	got, err := app.AttachDropped(outside)
	if err != nil {
		t.Fatalf("AttachDropped: %v", err)
	}
	if tab.Ctrl == oldCtrl {
		t.Fatal("stale controller was reused for external folder ref")
	}
	if gotRoot := normalizeProjectRoot(tab.Ctrl.WorkspaceRoot()); gotRoot != normalizeProjectRoot(projectA) {
		t.Fatalf("controller workspace root = %q, want project A %q", gotRoot, normalizeProjectRoot(projectA))
	}
	resolver, ok := tab.Ctrl.(interface {
		ResolveScopedRefs(context.Context, string) (string, []string)
	})
	if !ok {
		t.Fatalf("rebuilt controller does not resolve scoped refs: %T", tab.Ctrl)
	}
	block, errs := resolver.ResolveScopedRefs(context.Background(), "inspect @"+got.Path+"/")
	if len(errs) != 0 {
		t.Fatalf("ResolveScopedRefs errors = %v", errs)
	}
	if !strings.Contains(block, "sub/notes.txt") {
		t.Fatalf("external dropped folder should resolve on rebuilt controller:\n%s", block)
	}
}
