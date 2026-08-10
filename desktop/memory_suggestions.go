package main

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"patty/internal/agent"
	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/memory"
	"patty/internal/provider"
	"patty/internal/skill"
)

const (
	suggestionSessionLimit = 12
	memorySuggestionLimit  = 6
)

type MemorySuggestion struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Scope       string   `json:"scope"`
	Body        string   `json:"body"`
	Reason      string   `json:"reason"`
	Evidence    []string `json:"evidence"`
}

type SkillSuggestion struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scope       string   `json:"scope"`
	Body        string   `json:"body"`
	Reason      string   `json:"reason"`
	Evidence    []string `json:"evidence"`
}

type MemorySuggestionsView struct {
	Memories    []MemorySuggestion `json:"memories"`
	Skills      []SkillSuggestion  `json:"skills"`
	GeneratedAt string             `json:"generatedAt"`
	Available   bool               `json:"available"`
	Source      string             `json:"source"`
}

type suggestionSession struct {
	Path     string
	ID       string
	Preview  string
	LastSeen time.Time
	Messages []provider.Message
}

type workflowCategory struct {
	Name        string
	Description string
	Reason      string
	Keywords    []string
	Steps       []string
}

func (a *App) MemorySuggestions() MemorySuggestionsView {
	return a.MemorySuggestionsForTab("")
}

func (a *App) MemorySuggestionsForTab(tabID string) MemorySuggestionsView {
	view := emptyMemorySuggestionsView()

	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	var ctrl control.SessionAPI
	workspaceRoot := ""
	if tab != nil {
		ctrl = tab.Ctrl
		workspaceRoot = tab.WorkspaceRoot
	}
	a.mu.RUnlock()
	if ctrl == nil {
		return view
	}
	sessionDir := ""
	if path, ok := a.reconcileTabWithPinnedSessionMeta(tab); ok && strings.TrimSpace(path) != "" {
		sessionDir = filepath.Dir(path)
		workspaceRoot = tab.WorkspaceRoot
	} else {
		sessionDir = tabRuntimeSessionDir(tab)
	}
	set := ctrl.Memory()
	if set == nil {
		return view
	}
	view.Available = true
	view.Source = "local-history"

	sessions := loadSuggestionSessions(sessionDir, suggestionSessionLimit)
	view.Memories = suggestMemories(set, sessions)
	view.Skills = suggestSkills(workspaceRoot, ctrl.AllSkills(), sessions)
	return view
}

func emptyMemorySuggestionsView() MemorySuggestionsView {
	return MemorySuggestionsView{
		Memories:    []MemorySuggestion{},
		Skills:      []SkillSuggestion{},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func (a *App) AcceptMemorySuggestion(in MemorySuggestion) (string, error) {
	return a.AcceptMemorySuggestionForTab("", in)
}

func (a *App) AcceptMemorySuggestionForTab(tabID string, in MemorySuggestion) (string, error) {
	ctrl := a.ctrlByTabID(tabID)
	if ctrl == nil {
		return "", nil
	}
	desc := oneLine(in.Description)
	body := strings.TrimSpace(in.Body)
	if desc == "" || body == "" {
		return "", fmt.Errorf("memory suggestion requires description and body")
	}
	name := acceptedSuggestionName(in.Name, desc)
	return ctrl.SaveMemory(memory.Memory{
		Name:        name,
		Title:       oneLine(in.Title),
		Description: desc,
		Type:        memory.NormalizeType(in.Type),
		Scope:       memory.NormalizeFactScope(in.Scope),
		Body:        body,
	})
}

func (a *App) AcceptSkillSuggestion(in SkillSuggestion) (string, error) {
	return a.AcceptSkillSuggestionForTab("", in)
}

func (a *App) AcceptSkillSuggestionForTab(tabID string, in SkillSuggestion) (string, error) {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	workspaceRoot := ""
	if tab != nil {
		workspaceRoot = tab.WorkspaceRoot
	}
	a.mu.RUnlock()

	name := strings.TrimSpace(in.Name)
	desc := oneLine(in.Description)
	body := strings.TrimSpace(in.Body)
	if name == "" || desc == "" || body == "" {
		return "", fmt.Errorf("skill suggestion requires name, description, and body")
	}
	st := skillStoreForWorkspace(workspaceRoot)
	scope := skill.ScopeProject
	if strings.TrimSpace(in.Scope) == "global" || !st.HasProjectScope() {
		scope = skill.ScopeGlobal
	}
	content := skill.RenderSkillFile(skill.SkillFileOptions{Name: name, Description: desc, Body: body})
	return st.CreateWithContent(name, scope, content)
}

func loadSuggestionSessions(dir string, limit int) []suggestionSession {
	if strings.TrimSpace(dir) == "" || limit <= 0 {
		return nil
	}
	infos, err := agent.ListSessions(dir)
	if err != nil {
		return nil
	}
	var out []suggestionSession
	for _, info := range infos {
		if len(out) >= limit {
			break
		}
		loaded, err := agent.LoadSession(info.Path)
		if err != nil {
			continue
		}
		out = append(out, suggestionSession{
			Path:     info.Path,
			ID:       strings.TrimSuffix(filepath.Base(info.Path), filepath.Ext(info.Path)),
			Preview:  info.Preview,
			LastSeen: info.LastActivityAt,
			Messages: loaded.Snapshot(),
		})
	}
	return out
}

func suggestMemories(set *memory.Set, sessions []suggestionSession) []MemorySuggestion {
	if set == nil || len(sessions) == 0 {
		return []MemorySuggestion{}
	}
	existing := existingMemoryText(set)
	seen := map[string]bool{}
	var out []MemorySuggestion
	for _, sess := range sessions {
		for _, msg := range sess.Messages {
			if msg.Role != provider.RoleUser {
				continue
			}
			statement, reason := extractMemoryStatement(agent.UserMessageText(msg))
			if statement == "" {
				continue
			}
			key := normalizeSuggestionKey(statement)
			if key == "" || seen[key] || existingCovers(existing, key) {
				continue
			}
			seen[key] = true
			name := stableSuggestionName(statement, "memory-candidate")
			title := suggestionTitle(statement, "Memory candidate")
			typ := inferMemoryType(statement)
			out = append(out, MemorySuggestion{
				ID:          "memory-" + name,
				Name:        name,
				Title:       title,
				Description: oneLine(statement),
				Type:        string(typ),
				Scope:       string(memory.FactScopeProject),
				Body:        memoryCandidateBody(statement, reason, sess),
				Reason:      reason,
				Evidence:    []string{sessionEvidence(sess, statement)},
			})
			if len(out) >= memorySuggestionLimit {
				return out
			}
		}
	}
	return out
}

func suggestSkills(workspaceRoot string, existing []skill.Skill, sessions []suggestionSession) []SkillSuggestion {
	if len(sessions) == 0 {
		return []SkillSuggestion{}
	}
	existingNames := map[string]bool{}
	for _, sk := range existing {
		existingNames[config.SkillNameKey(sk.Name)] = true
	}
	scope := "project"
	if strings.TrimSpace(workspaceRoot) == "" {
		scope = "global"
	}

	var out []SkillSuggestion
	for _, cat := range workflowCategories() {
		if existingNames[config.SkillNameKey(cat.Name)] {
			continue
		}
		evidence := workflowEvidence(cat, sessions)
		if len(evidence) < 2 {
			continue
		}
		out = append(out, SkillSuggestion{
			ID:          "skill-" + cat.Name,
			Name:        cat.Name,
			Description: cat.Description,
			Scope:       scope,
			Body:        skillCandidateBody(cat, evidence),
			Reason:      cat.Reason,
			Evidence:    evidence,
		})
	}
	return out
}

func existingMemoryText(set *memory.Set) []string {
	var out []string
	for _, d := range set.Docs {
		out = append(out, normalizeSuggestionKey(d.Body))
	}
	for _, f := range set.Store.ListAll() {
		out = append(out, normalizeSuggestionKey(strings.Join([]string{f.Name, f.Title, f.Description, f.Body}, " ")))
	}
	return out
}

func existingCovers(existing []string, key string) bool {
	if key == "" {
		return true
	}
	for _, text := range existing {
		if text != "" && (strings.Contains(text, key) || strings.Contains(key, text)) {
			return true
		}
	}
	return false
}

func extractMemoryStatement(content string) (string, string) {
	text := oneLine(content)
	if len([]rune(text)) < 8 || len([]rune(text)) > 420 {
		return "", ""
	}
	lower := strings.ToLower(text)
	type marker struct {
		value  string
		reason string
	}
	markers := []marker{
		{"기억해요", "explicit remember request"},
		{"앞으로", "future-facing preference"},
		{"항상", "persistent working rule"},
		{"언제든", "persistent working rule"},
		{"매번", "repeated workflow preference"},
		{"기본", "default behavior preference"},
		{"하지마", "negative working preference"},
		{"선호사항", "user preference"},
		{"규칙", "durable rule"},
		{"약속", "project convention"},
		{"remember", "explicit remember request"},
		{"always", "persistent working rule"},
		{"never", "negative working preference"},
		{"prefer", "user preference"},
		{"preference", "user preference"},
		{"by default", "default behavior preference"},
	}
	for _, m := range markers {
		if strings.Contains(lower, m.value) {
			return trimMemoryLead(text, m.value), m.reason
		}
	}
	return "", ""
}

func trimMemoryLead(text, marker string) string {
	idx := strings.Index(strings.ToLower(text), marker)
	if idx < 0 {
		return text
	}
	trimmed := strings.TrimSpace(text[idx:])
	for _, sep := range []string{"：", ":", "-", "—"} {
		trimmed = strings.TrimPrefix(trimmed, marker+sep)
	}
	return strings.TrimSpace(trimmed)
}

func inferMemoryType(statement string) memory.Type {
	lower := strings.ToLower(statement)
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "github.com/") {
		return memory.TypeReference
	}
	if hasAny(lower, "피드백", "답변", "대답", "하지마", "always", "never", "항상", "언제든") {
		return memory.TypeFeedback
	}
	if hasAny(lower, "프로젝트", "분기", "pr", "pull request", "저장소", "repo", "약속") {
		return memory.TypeProject
	}
	return memory.TypeUser
}

func memoryCandidateBody(statement, reason string, sess suggestionSession) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(statement))
	b.WriteString("\n\n**Why:** Suggested from recent local history")
	if reason != "" {
		b.WriteString(" (" + reason + ")")
	}
	b.WriteString(".\n")
	b.WriteString("**How to apply:** Treat this as durable guidance only after the user confirms it still applies.\n")
	if sess.ID != "" {
		b.WriteString("\nEvidence: [" + sess.ID + "] " + truncateRunes(statement, 180))
	}
	return b.String()
}

func workflowCategories() []workflowCategory {
	return []workflowCategory{
		{
			Name:        "patty-pr-followup",
			Description: "Review or update a patty GitHub PR, address feedback, verify, and publish safely.",
			Reason:      "recent history repeatedly touched PR review, bot feedback, commits, or GitHub publication",
			Keywords:    []string{"pr", "pull request", "github", "review", "봇", "검토", "PR에 커밋", "PR 업데이트", "code rabbit", "coderabbit"},
			Steps: []string{
				"Fetch the live PR state and confirm branch, base, head SHA, and review status.",
				"Inspect the real diff and related implementation before changing code.",
				"Fix only actionable feedback, run focused verification, and keep cache-sensitive surfaces stable.",
				"Stage intended files, commit with an English behavior-focused message, push to the verified PR head, and update the PR.",
			},
		},
		{
			Name:        "patty-memory-ui",
			Description: "Iterate on the patty desktop Memory page with source-backed UI decisions and browser verification.",
			Reason:      "recent history repeatedly discussed Memory page layout, labels, filters, and interaction details",
			Keywords:    []string{"memory", "메모리", "설정-메모리", "memory panel", "명령 파일", "아카이브", "글로벌", "프로젝트", "추가메모리"},
			Steps: []string{
				"Identify the active Memory settings component and current browser-rendered state before editing.",
				"Keep active memories, archived memories, instruction files, and suggestions visually distinct.",
				"Use neutral secondary actions and confirmation for persistent writes or archive operations.",
				"Run frontend checks and verify the affected Memory page in the in-app browser.",
			},
		},
		{
			Name:        "desktop-ui-iteration",
			Description: "Apply focused desktop UI layout feedback, preserve existing design tokens, and verify in browser.",
			Reason:      "recent history repeatedly involved screenshot-driven desktop UI layout and interaction feedback",
			Keywords:    []string{"ui", "레이아웃", "디자인", "상호작용", "빨간 상자", "페이지", "버튼", "브라우저", "frontend", "desktop"},
			Steps: []string{
				"Map the screenshot target to the exact component, selector, and state in source.",
				"Patch the smallest component and CSS surface using existing settings/page recipes.",
				"Check responsive behavior and text overflow for the changed controls.",
				"Verify with the running local UI instead of relying only on code inspection.",
			},
		},
	}
}

func workflowEvidence(cat workflowCategory, sessions []suggestionSession) []string {
	seenSession := map[string]bool{}
	var evidence []string
	for _, sess := range sessions {
		for _, msg := range sess.Messages {
			if msg.Role != provider.RoleUser {
				continue
			}
			text := oneLine(agent.UserMessageText(msg))
			if text == "" || !hasAny(strings.ToLower(text), cat.Keywords...) {
				continue
			}
			if seenSession[sess.ID] {
				continue
			}
			seenSession[sess.ID] = true
			evidence = append(evidence, sessionEvidence(sess, text))
			break
		}
	}
	if len(evidence) > 4 {
		return evidence[:4]
	}
	return evidence
}

func skillCandidateBody(cat workflowCategory, evidence []string) string {
	var b strings.Builder
	title := strings.TrimPrefix(strings.ReplaceAll(cat.Name, "-", " "), "patty ")
	b.WriteString("# " + cases.Title(language.Und).String(title) + "\n\n")
	b.WriteString("Use this skill when the user asks for this repeated Patty Code workflow.\n\n")
	b.WriteString("## Evidence\n\n")
	for _, ev := range evidence {
		b.WriteString("- " + ev + "\n")
	}
	b.WriteString("\n## Workflow\n\n")
	for i, step := range cat.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}
	b.WriteString("\n## Stop Condition\n\n")
	b.WriteString("Finish only after the requested change is implemented, verified, and any requested PR or UI update is delivered.\n")
	return b.String()
}

func skillStoreForWorkspace(workspaceRoot string) *skill.Store {
	cfg, err := config.LoadForRoot(workspaceRoot)
	var custom, excluded []string
	var pluginPaths map[string][]string
	var pluginAgentPaths map[string][]string
	maxDepth := 3
	if err == nil && cfg != nil {
		custom = cfg.SkillCustomPaths()
		excluded = cfg.SkillExcludedPaths()
		pluginPaths = cfg.PluginPackageSkillOwners()
		pluginAgentPaths = cfg.PluginPackageAgentOwners()
		maxDepth = cfg.SkillMaxDepth()
	}
	return skill.New(skill.Options{
		ProjectRoot:      strings.TrimSpace(workspaceRoot),
		CustomPaths:      custom,
		PluginPaths:      pluginPaths,
		PluginAgentPaths: pluginAgentPaths,
		ExcludedPaths:    excluded,
		MaxDepth:         maxDepth,
	})
}

func suggestionName(given, source, fallback string) string {
	if name := asciiSlug(given); name != "" {
		return name
	}
	if name := asciiSlug(source); name != "" {
		return name
	}
	if name := asciiSlug(fallback); name != "" {
		return name
	}
	return "candidate"
}

func acceptedSuggestionName(given, desc string) string {
	if isWellFormedSlug(given) {
		return given
	}
	return suggestionName("", desc, "memory-candidate")
}

func isWellFormedSlug(s string) bool {
	if s == "" || len(s) > 128 || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			prevDash = false
		case r == '-':
			if prevDash {
				return false
			}
			prevDash = true
		default:
			return false
		}
	}
	return true
}

//
func stableSuggestionName(source, fallback string) string {
	slug := asciiSlug(source)
	if slug != "" && len(slug) < 56 {
		return slug
	}
	base := suggestionName("", source, fallback)
	h := fnv.New32a()
	_, _ = h.Write([]byte(source))
	return fmt.Sprintf("%s-%08x", base, h.Sum32())
}

func asciiSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		case unicode.IsSpace(r):
			if b.Len() > 0 && !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
		if b.Len() >= 56 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func suggestionTitle(s, fallback string) string {
	title := truncateRunes(oneLine(s), 64)
	if title == "" {
		return fallback
	}
	return title
}

func sessionEvidence(sess suggestionSession, text string) string {
	label := sess.ID
	if label == "" {
		label = filepath.Base(sess.Path)
	}
	return label + ": " + truncateRunes(oneLine(text), 160)
}

func normalizeSuggestionKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "..."
}

func hasAny(hay string, needles ...string) bool {
	hay = strings.ToLower(hay)
	for _, needle := range needles {
		if strings.Contains(hay, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
