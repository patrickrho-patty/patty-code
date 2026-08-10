package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/agent"
	"patty/internal/control"
	"patty/internal/memory"
	"patty/internal/provider"
)

func TestMemorySuggestionsReturnsNonNilArraysBeforeStartup(t *testing.T) {
	isolateDesktopUserDirs(t)

	view := NewApp().MemorySuggestions()
	if view.Memories == nil || view.Skills == nil {
		t.Fatalf("MemorySuggestions() arrays must be non-nil before startup: %+v", view)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal MemorySuggestions(): %v", err)
	}
	for _, bad := range []string{`"memories":null`, `"skills":null`} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("MemorySuggestions() JSON contains %s; frontend expects []: %s", bad, raw)
		}
	}
}

func TestMemorySuggestionsAcceptMemoryCandidate(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	writeSuggestionSession(t, sessionDir, "pref.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "앞으로는 항상 기능정리검토로 답변해 주세요. 명시적으로 영어를 요청하지 않는 한요."},
		provider.Message{Role: provider.RoleAssistant, Content: "좋아요."},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: store, CWD: cwd, UserDir: userDir},
		SessionDir: sessionDir,
	}), "test-model")
	app.tabs["test"].WorkspaceRoot = cwd

	view := app.MemorySuggestions()
	if len(view.Memories) == 0 {
		t.Fatalf("MemorySuggestions() memories = %+v, want at least one candidate", view.Memories)
	}
	if view.Memories[0].Scope != string(memory.FactScopeProject) {
		t.Fatalf("candidate scope = %q, want project", view.Memories[0].Scope)
	}
	path, err := app.AcceptMemorySuggestion(view.Memories[0])
	if err != nil {
		t.Fatalf("AcceptMemorySuggestion: %v", err)
	}
	if path == "" {
		t.Fatal("AcceptMemorySuggestion returned empty path")
	}
	got := store.List()
	if len(got) != 1 || got[0].Scope != memory.FactScopeProject || !strings.Contains(got[0].Body, "기능정리검토로 답변해 주세요") {
		t.Fatalf("saved memories = %+v, want confirmed candidate body", got)
	}
}

func TestMemorySuggestionsForTabUsesSelectedTab(t *testing.T) {
	isolateDesktopUserDirs(t)
	activeUserDir := t.TempDir()
	selectedUserDir := t.TempDir()
	activeCwd := t.TempDir()
	selectedCwd := t.TempDir()
	activeSessionDir := t.TempDir()
	selectedSessionDir := t.TempDir()
	activeStore := memory.StoreFor(activeUserDir, activeCwd)
	selectedStore := memory.StoreFor(selectedUserDir, selectedCwd)
	writeSuggestionSession(t, selectedSessionDir, "selected.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "앞으로는 항상 기능정리검토로 답변해 주세요. 명시적으로 영어를 요청하지 않는 한요."},
		provider.Message{Role: provider.RoleAssistant, Content: "좋아요."},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: activeStore, CWD: activeCwd, UserDir: activeUserDir},
		SessionDir: activeSessionDir,
	}), "test-model")
	app.tabs["test"].WorkspaceRoot = activeCwd
	app.tabs["selected"] = &WorkspaceTab{
		ID:            "selected",
		Scope:         "project",
		WorkspaceRoot: selectedCwd,
		Ctrl: control.New(control.Options{
			Memory:     &memory.Set{Store: selectedStore, CWD: selectedCwd, UserDir: selectedUserDir},
			SessionDir: selectedSessionDir,
		}),
		Ready:       true,
		disabledMCP: map[string]ServerView{},
	}

	if view := app.MemorySuggestions(); len(view.Memories) != 0 {
		t.Fatalf("active tab suggestions = %+v, want none", view.Memories)
	}
	view := app.MemorySuggestionsForTab("selected")
	if len(view.Memories) == 0 {
		t.Fatalf("MemorySuggestionsForTab(selected) memories = %+v, want at least one candidate", view.Memories)
	}
	path, err := app.AcceptMemorySuggestionForTab("selected", view.Memories[0])
	if err != nil {
		t.Fatalf("AcceptMemorySuggestionForTab: %v", err)
	}
	if !strings.HasPrefix(path, selectedStore.Dir) && !strings.HasPrefix(path, selectedStore.GlobalDir) {
		t.Fatalf("memory path = %q, want selected store under %q or %q", path, selectedStore.Dir, selectedStore.GlobalDir)
	}
	if got := activeStore.List(); len(got) != 0 {
		t.Fatalf("active store should remain untouched, got %+v", got)
	}
	got := selectedStore.List()
	if len(got) != 1 || !strings.Contains(got[0].Body, "기능정리검토로 답변해 주세요") {
		t.Fatalf("selected store = %+v, want confirmed candidate body", got)
	}

	skillPath, err := app.AcceptSkillSuggestionForTab("selected", SkillSuggestion{
		ID:          "selected-skill",
		Name:        "selected-workflow",
		Description: "Selected workspace workflow",
		Scope:       "project",
		Body:        "Use the selected workspace context before changing files.",
	})
	if err != nil {
		t.Fatalf("AcceptSkillSuggestionForTab: %v", err)
	}
	wantSkillPath := filepath.Join(selectedCwd, ".patty", "skills", "selected-workflow", "SKILL.md")
	if skillPath != wantSkillPath {
		t.Fatalf("skill path = %q, want %q", skillPath, wantSkillPath)
	}
	if _, err := os.Stat(filepath.Join(activeCwd, ".patty", "skills", "selected-workflow", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("active workspace should not receive selected skill, stat err = %v", err)
	}
	body, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read selected skill: %v", err)
	}
	if !strings.Contains(string(body), "selected workspace context") {
		t.Fatalf("selected skill body missing candidate content:\n%s", body)
	}
}

func TestMemorySuggestionsAcceptSkillCandidate(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	writeSuggestionSession(t, sessionDir, "pr-a.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "이 PR을 로컬에 병합하고 주요 변경 사항을 설명해 주세요."},
		provider.Message{Role: provider.RoleAssistant, Content: "확인했습니다."},
	)
	writeSuggestionSession(t, sessionDir, "pr-b.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "이 pr에서 봇이 제기한 문제를 해결하고, 합리적인 문제는 수정해 주세요."},
		provider.Message{Role: provider.RoleAssistant, Content: "처리했습니다."},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: store, CWD: cwd, UserDir: userDir},
		SessionDir: sessionDir,
	}), "test-model")
	app.tabs["test"].WorkspaceRoot = cwd

	view := app.MemorySuggestions()
	var candidate SkillSuggestion
	for _, item := range view.Skills {
		if item.Name == "patty-pr-followup" {
			candidate = item
			break
		}
	}
	if candidate.Name == "" {
		t.Fatalf("MemorySuggestions() skills = %+v, want patty-pr-followup", view.Skills)
	}
	path, err := app.AcceptSkillSuggestion(candidate)
	if err != nil {
		t.Fatalf("AcceptSkillSuggestion: %v", err)
	}
	wantSuffix := filepath.Join(".patty", "skills", "patty-pr-followup", "SKILL.md")
	if !strings.HasSuffix(path, wantSuffix) {
		t.Fatalf("skill path = %q, want suffix %q", path, wantSuffix)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(body), "Review or update a patty GitHub PR") {
		t.Fatalf("skill body missing description: %s", body)
	}
}

func writeSuggestionSession(t *testing.T, dir, name string, messages ...provider.Message) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sess := agent.NewSession("")
	for _, msg := range messages {
		sess.Add(msg)
	}
	if err := sess.Save(filepath.Join(dir, name)); err != nil {
		t.Fatalf("save session %s: %v", name, err)
	}
}

func TestHistoryEnglishCandidateNameBackwardCompat(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	writeSuggestionSession(t, sessionDir, "en.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "Always prefer English for code comments."},
		provider.Message{Role: provider.RoleAssistant, Content: "Got it."},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: store, CWD: cwd, UserDir: userDir},
		SessionDir: sessionDir,
	}), "test-model")
	app.tabs["test"].WorkspaceRoot = cwd

	view := app.MemorySuggestions()
	if len(view.Memories) == 0 {
		t.Fatalf("no candidates")
	}
	// Old code: suggestionName("", statement, "memory-candidate-1") = asciiSlug(statement)
	oldName := asciiSlug("Always prefer English for code comments.")
	if view.Memories[0].Name != oldName {
		t.Fatalf("Name = %q, want old-compatible %q (no hash suffix for short ASCII slugs)", view.Memories[0].Name, oldName)
	}
}

// distinct NameID. Without the hash suffix they would both fall back to
// "memory-candidate-<ordinal>" — but ordinals depend on iteration order and
// wouldnt survive refresh, and Store.Save overwrites by name.
func TestHistoryMemoryCandidateNamesUniqueForCJK(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	// Two pure-CJK "always" statements that pass extractMemoryStatement but
	writeSuggestionSession(t, sessionDir, "ko-a.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "앞으로 항상 A안으로 병합 충돌을 처리하세요."},
		provider.Message{Role: provider.RoleAssistant, Content: "좋아요."},
	)
	writeSuggestionSession(t, sessionDir, "ko-b.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "앞으로 항상 B안으로 배포 롤백을 처리하세요."},
		provider.Message{Role: provider.RoleAssistant, Content: "좋아요."},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: store, CWD: cwd, UserDir: userDir},
		SessionDir: sessionDir,
	}), "test-model")
	app.tabs["test"].WorkspaceRoot = cwd

	view := app.MemorySuggestions()
	if len(view.Memories) < 2 {
		t.Fatalf("memories = %+v, want at least 2 CJK candidates", view.Memories)
	}
	names := map[string]bool{}
	ids := map[string]bool{}
	for _, m := range view.Memories {
		if names[m.Name] {
			t.Fatalf("duplicate Name %q among history candidates", m.Name)
		}
		if ids[m.ID] {
			t.Fatalf("duplicate ID %q among history candidates", m.ID)
		}
		names[m.Name] = true
		ids[m.ID] = true
	}

	// Accept both  two distinct persisted memories.
	for _, c := range view.Memories {
		if _, err := app.AcceptMemorySuggestion(c); err != nil {
			t.Fatalf("AcceptMemorySuggestion(%s): %v", c.Name, err)
		}
	}
	saved := store.List()
	if len(saved) != len(view.Memories) {
		t.Fatalf("saved %d memories, want %d (Name collision caused overwrite)", len(saved), len(view.Memories))
	}
}

// a refresh keeps the same ID and the frontends accepted-state map stays valid.
func TestHistoryMemoryCandidateNamesStableAcrossRefreshes(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	writeSuggestionSession(t, sessionDir, "pref.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "앞으로는 항상 기능정리검토로 답변해 주세요. 명시적으로 영어를 요청하지 않는 한요."},
		provider.Message{Role: provider.RoleAssistant, Content: "좋아요."},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: store, CWD: cwd, UserDir: userDir},
		SessionDir: sessionDir,
	}), "test-model")
	app.tabs["test"].WorkspaceRoot = cwd

	first := app.MemorySuggestions()
	second := app.MemorySuggestions()
	if len(first.Memories) == 0 || len(first.Memories) != len(second.Memories) {
		t.Fatalf("memories counts differ across refreshes: %d vs %d", len(first.Memories), len(second.Memories))
	}
	for i := range first.Memories {
		if first.Memories[i].ID != second.Memories[i].ID || first.Memories[i].Name != second.Memories[i].Name {
			t.Fatalf("refresh changed candidate #%d: %q/%q → %q/%q",
				i, first.Memories[i].Name, first.Memories[i].ID,
				second.Memories[i].Name, second.Memories[i].ID)
		}
	}
}

func TestMemorySuggestionsDeduplicateAllScopedFactsAndInstructionBodies(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	if _, err := (memory.Store{Dir: store.GlobalDir}).Save(memory.Memory{
		Name: "response-language", Description: "Global response language", Scope: memory.FactScopeGlobal, Type: memory.TypeUser,
		Body: "Always answer in Korean unless the user explicitly asks for English.",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := (memory.Store{Dir: store.Dir}).Save(memory.Memory{
		Name: "response-language-project", Description: "Project response language", Scope: memory.FactScopeProject, Type: memory.TypeProject,
		Body: "Always answer in Korean unless the user explicitly asks for English.",
	}); err != nil {
		t.Fatal(err)
	}
	set := &memory.Set{Store: store, CWD: cwd, UserDir: userDir, Docs: []memory.Source{{Body: "Always use tabs for indentation."}}}
	writeSuggestionSession(t, sessionDir, "dedupe.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "Always answer in Korean unless the user explicitly asks for English."},
		provider.Message{Role: provider.RoleUser, Content: "Always use tabs for indentation."},
	)
	got := suggestMemories(set, loadSuggestionSessions(sessionDir, suggestionSessionLimit))
	if len(got) != 0 {
		t.Fatalf("suggestions = %+v, want all candidates covered by scoped facts/docs to be omitted", got)
	}
}
