package cli

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/agent"
	"patty/internal/agent/testutil"
	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/i18n"
	"patty/internal/provider"
	"patty/internal/tool"
)

func TestRunStatuslineCmd(t *testing.T) {
	firstLineCmd := "printf 'row-one\\nrow-two\\n'"
	stdinCmd := "cat"
	failCmd := "exit 3"
	if runtime.GOOS == "windows" {
		firstLineCmd = "echo row-one & echo row-two"
		stdinCmd = "more"
		failCmd = "exit /b 3"
	}

	if got := runStatuslineCmd(firstLineCmd, "{}"); got != "row-one" {
		t.Errorf("multi-line output should collapse to the first row, got %q", got)
	}
	if got := runStatuslineCmd(stdinCmd, `{"model":"deepseek"}`); got != `{"model":"deepseek"}` {
		t.Errorf("stdin payload not forwarded, got %q", got)
	}
	if got := runStatuslineCmd(failCmd, "{}"); got != "" {
		t.Errorf("failed command should yield empty, got %q", got)
	}
}

func TestRunStatuslineCmdNormalizesQuotedNodeEval(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	script := "let input = ''; process.stdin.setEncoding('utf8'); process.stdin.on('data', chunk => input += chunk); process.stdin.on('end', () => { const payload = JSON.parse(input); console.log(payload.model) })"
	cmd := `node -e "\"` + script + `\""`
	timeout := statuslineCommandTimeout
	if runtime.GOOS == "windows" {
		// is not under test here — only the quoted-eval normalization is.
		timeout = 30 * time.Second
	}

	if got := runStatuslineCmdWithTimeout(cmd, `{"model":"deepseek"}`, timeout); got != "deepseek" {
		t.Fatalf("normalized statusline node -e output = %q, want deepseek", got)
	}
}

func TestRunStatuslineDisabled(t *testing.T) {
	m := chatTUI{} // no statuslineCmd, nil ctrl
	if cmd := m.runStatusline(); cmd != nil {
		t.Error("an unconfigured status line must return a nil tea.Cmd")
	}
}

func TestRunStatuslineIncludesSessionIdentityContext(t *testing.T) {
	path := "/tmp/patty-session-identity.jsonl"
	ctrl := control.New(control.Options{SessionPath: path})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 80)
	m.statuslineCmd = "cat"

	cmd := m.runStatusline()
	if cmd == nil {
		t.Fatal("configured statusline should return a command")
	}
	msg, ok := cmd().(statuslineMsg)
	if !ok {
		t.Fatalf("statusline command message = %T, want statuslineMsg", cmd())
	}
	if !strings.Contains(msg.out, `"sessionId":"`+agent.BranchID(path)+`"`) {
		t.Fatalf("statusline context missing session identity: %s", msg.out)
	}
}

func TestModelSwitchRefreshesCustomStatusline(t *testing.T) {
	oldCtrl := control.New(control.Options{Label: "old-model"})
	newCtrl := control.New(control.Options{Label: "new-model"})
	m := newChatTUI(oldCtrl, "", make(chan event.Event, 1), 80)
	m.statuslineCmd = "cat"
	m.statuslineOut = `{"model":"old-model"}`

	_, cmd := m.Update(modelSwitchMsg{
		ref:   "provider/new-model",
		ctrl:  newCtrl,
		label: "new-model",
	})
	if cmd == nil {
		t.Fatal("model switch should schedule commands")
	}
	if !statuslineCommandHasModel(cmd, "new-model") {
		t.Fatal("model switch did not refresh custom statusline with the new model")
	}
}

func statuslineCommandHasModel(cmd tea.Cmd, model string) bool {
	msg := cmd()
	switch msg := msg.(type) {
	case statuslineMsg:
		return strings.Contains(msg.out, `"model":"`+model+`"`)
	case tea.BatchMsg:
		for _, child := range msg {
			if child == nil {
				continue
			}
			if statuslineCommandHasModel(child, model) {
				return true
			}
		}
	}
	return false
}

func TestIdleStatuslineIsCompact(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.TrueColor
	i18n.DetectLanguage("en")

	content := renderStatuslineView(t, false)
	plain := bottomStatusPlain(content)
	if !strings.Contains(plain, "ready") {
		t.Fatalf("idle status line missing operational state:\n%s", plain)
	}
	if !strings.Contains(plain, "Shift+Tab ask/auto/plan · Ctrl+Y YOLO Mode") {
		t.Fatalf("idle status line missing plan-toggle hint:\n%s", plain)
	}
	for _, old := range []string{"Shift-Tab", "Ctrl-O", "Ctrl-D", "Enter sends", "Esc clears/exits state", "PgUp/PgDn"} {
		if strings.Contains(plain, old) {
			t.Fatalf("idle status line should not contain %q:\n%s", old, plain)
		}
	}
	for _, reject := range []string{"MODEL", "EFFORT", "deepseek-v4-flash", "[auto]", " Auto "} {
		if strings.Contains(plain, reject) {
			t.Fatalf("idle footer repeated masthead fact %q:\n%s", reject, plain)
		}
	}
	if raw := lastRenderedLine(content); strings.Contains(raw, "\x1b[48;") {
		t.Fatalf("operational status line should not use a mode pill background, got:\n%q", raw)
	}
}

func TestYoloStatuslineUsesOperationalWarningWithoutModePill(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.TrueColor
	i18n.DetectLanguage("en")

	content := renderStatuslineView(t, true)
	plain := bottomStatusPlain(content)
	if !strings.Contains(plain, "YOLO") || !strings.Contains(plain, "Shift+Tab ask/auto/plan · Ctrl+Y YOLO Mode") {
		t.Fatalf("YOLO status line missing YOLO badge:\n%s", plain)
	}
	if raw := lastRenderedLine(content); strings.Contains(raw, "\x1b[48;") {
		t.Fatalf("YOLO operational warning should not use a mode pill background, got:\n%q", raw)
	}
}

func TestPlanStatuslineShowsCurrentModeLabel(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.TrueColor
	i18n.DetectLanguage("en")

	content := renderPlanStatuslineView(t)
	plain := bottomStatusPlain(content)
	// Plan mode surfaces its current-mode label in the operational footer so
	// Shift+Tab cycling is visible (ask -> auto -> plan).
	if !strings.Contains(plain, i18n.M.ChatModePlanLabel) || !strings.Contains(plain, "Shift+Tab ask/auto/plan · Ctrl+Y YOLO Mode") {
		t.Fatalf("plan status line missing current-mode label:\n%s", plain)
	}
	if raw := lastRenderedLine(content); strings.Contains(raw, "\x1b[48;") {
		t.Fatalf("plan operational status should not use a mode pill background, got:\n%q", raw)
	}
}

func TestStatuslineCycleHintFollowsLanguage(t *testing.T) {
	i18n.DetectLanguage("ko-KR")
	t.Cleanup(func() { i18n.DetectLanguage("en") })

	content := renderStatuslineView(t, false)
	plain := bottomStatusPlain(content)
	if !strings.Contains(plain, "확인") || !strings.Contains(plain, i18n.M.ChatStatusCycleHintCompact) {
		t.Fatalf("status line hint missing after ko-KR detection:\n%s", plain)
	}
}

func TestDesktopShortcutStatuslineUsesPlanToggleHint(t *testing.T) {
	i18n.DetectLanguage("en")

	content := renderStatuslineViewWithShortcutLayout(t, "desktop")
	plain := bottomStatusPlain(content)
	if !strings.Contains(plain, "Shift+Tab ask/auto/plan · Ctrl+Y YOLO Mode") {
		t.Fatalf("desktop shortcut status line missing unified plan-toggle hint:\n%s", plain)
	}
	if strings.Contains(plain, " Ask ") {
		t.Fatalf("desktop operational footer should not repeat the masthead mode:\n%s", plain)
	}
}

func TestStatuslineKeepsModelAndEffortOutOfPersistentFooter(t *testing.T) {
	i18n.DetectLanguage("en")

	content := renderStatuslineViewWithEffort(t, "auto")
	lines := strings.Split(ansi.Strip(content), "\n")
	statusLine := lines[len(lines)-1]
	for _, reject := range []string{"MODEL", "EFFORT", "deepseek-v4-flash"} {
		if strings.Contains(statusLine, reject) {
			t.Fatalf("persistent footer repeated masthead fact %q:\n%s", reject, statusLine)
		}
	}
}

func TestStatuslineShowsCacheRatesInPersistentFooter(t *testing.T) {
	i18n.DetectLanguage("en")

	content := renderStatuslineViewWithCache(t)
	lines := bottomStatusPlainLines(content)
	if len(lines) != 3 {
		t.Fatalf("status block lines = %d, want 3:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if strings.Contains(lines[0], "MODEL") || strings.Contains(lines[0], "deepseek-v4-flash") {
		t.Fatalf("operational row should not repeat the masthead model:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[2], "CACHE turn hit 90.00% · avg 90.00%") {
		t.Fatalf("telemetry row should show cache rates:\n%s", strings.Join(lines, "\n"))
	}
}

func TestStatuslineShowsGitWithoutRepeatingSessionFacts(t *testing.T) {
	i18n.DetectLanguage("en")

	content := renderStatuslineViewWithGitAndEffort(t)
	lines := bottomStatusPlainLines(content)
	if len(lines) != 3 {
		t.Fatalf("status block lines = %d, want 3:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if strings.Contains(lines[0], "MODEL") || strings.Contains(lines[0], "EFFORT") || strings.Contains(lines[0], "deepseek-v4-flash") {
		t.Fatalf("operational row should not repeat masthead session facts:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[2], "Patty Code@codex/demo  +3 -1 ?2") {
		t.Fatalf("telemetry row should start with git identity:\n%s", strings.Join(lines, "\n"))
	}
}

func TestStatuslineShowsWorkModeAndBalanceInPersistentFooter(t *testing.T) {
	i18n.DetectLanguage("en")

	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 120)
	m.label = "deepseek-v4-flash"
	m.runtimeProfile = "delivery"
	m.balance = "¥12.34"
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	lines := bottomStatusPlainLines(next.(chatTUI).View().Content)
	if len(lines) != 3 {
		t.Fatalf("status block lines = %d, want 3:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if strings.Contains(lines[0], "MODEL") || strings.Contains(lines[0], "deepseek-v4-flash") {
		t.Fatalf("operational row should not repeat the masthead model:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Contains(lines[0], "WORK") {
		t.Fatalf("mode row should not surface the WORK badge:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[2], "BAL ¥12.34") {
		t.Fatalf("telemetry row should show balance:\n%s", strings.Join(lines, "\n"))
	}
}

func TestEffortTagExplicitValueUsesThemeInfo(t *testing.T) {
	i18n.DetectLanguage("en")
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.ANSI256

	for _, tt := range []struct {
		mode, infoSGR string
	}{
		{mode: "dark", infoSGR: "\033[1;38;5;80m"},
		{mode: "light", infoSGR: "\033[1;38;5;24m"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			configureCLITheme(tt.mode)
			m := newTestChatTUI()
			m.effortLevel = "max"
			content := m.effortTag()
			if !strings.Contains(ansi.Strip(content), "EFFORT max") {
				t.Fatalf("status data line should show explicit effort:\n%s", ansi.Strip(content))
			}
			if !strings.Contains(content, tt.infoSGR+"max") {
				t.Fatalf("%s explicit effort should use theme info colour, got:\n%q", tt.mode, content)
			}
		})
	}
}

func TestRefreshEffortStatusUsesCurrentModel(t *testing.T) {
	isolateUserConfig(t)
	writeDeepSeekTestUserConfig(t)

	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 80)
	m.modelRef = "deepseek-flash/deepseek-v4-flash"
	m.refreshEffortStatus()
	if m.effortLevel != "auto" {
		t.Fatalf("effortLevel = %q, want auto", m.effortLevel)
	}
}

func renderStatuslineView(t *testing.T, yolo bool) string {
	t.Helper()

	ctrl := control.New(control.Options{})
	ctrl.SetAutoApproveTools(yolo)
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 80)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(chatTUI).View().Content
}

func renderStatuslineViewWithShortcutLayout(t *testing.T, layout string) string {
	t.Helper()

	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 80)
	m.cfg = config.Default()
	if err := m.cfg.SetUIShortcutLayout(layout); err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(chatTUI).View().Content
}

func renderStatuslineViewWithEffort(t *testing.T, effort string) string {
	t.Helper()

	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 120)
	m.label = "deepseek-v4-flash"
	m.effortLevel = effort
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	return next.(chatTUI).View().Content
}

func renderStatuslineViewWithGitAndEffort(t *testing.T) string {
	t.Helper()

	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 120)
	m.label = "deepseek-v4-flash"
	m.effortLevel = "auto"
	m.gitStatus = gitStatus{
		Repo:      "Patty Code",
		Branch:    "codex/demo",
		Added:     3,
		Removed:   1,
		Untracked: 2,
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	return next.(chatTUI).View().Content
}

func renderStatuslineViewWithCache(t *testing.T) string {
	t.Helper()

	prov := testutil.NewMock("deepseek-v4-flash", testutil.Turn{
		Text: "ok",
		Usage: &provider.Usage{
			CacheHitTokens:   900,
			CacheMissTokens:  100,
			CompletionTokens: 50,
			PromptTokens:     1000,
			TotalTokens:      1050,
		},
	})
	exec := agent.New(prov, tool.NewRegistry(), agent.NewSession(""), agent.Options{MaxSteps: 1, ContextWindow: 200_000}, event.Discard)
	if err := exec.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("seed agent usage: %v", err)
	}
	ctrl := control.New(control.Options{Executor: exec})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 160)
	m.label = "deepseek-v4-flash"
	m.effortLevel = "auto"
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 24})
	return next.(chatTUI).View().Content
}

func renderPlanStatuslineView(t *testing.T) string {
	t.Helper()

	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 80)
	m.planMode = true
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(chatTUI).View().Content
}

func bottomStatusPlain(content string) string {
	return ansi.Strip(lastRenderedLine(content))
}

func lastRenderedLine(content string) string {
	lines := nonBlankRenderedLines(content)
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func bottomStatusPlainLines(content string) []string {
	lines := nonBlankRenderedLines(ansi.Strip(content))
	if len(lines) < 3 {
		return lines
	}
	return lines[len(lines)-3:]
}

func nonBlankRenderedLines(content string) []string {
	lines := strings.Split(content, "\n")
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
