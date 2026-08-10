package cli

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/i18n"
	pattyassets "patty/products/patty/assets"
)

func TestLaunchArtworkMatchesApprovedCenteredMarks(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.NoTTY
	configureCLITheme("seoul-night")

	want := pattyassets.Banner()
	if got := ansi.Strip(renderLaunchArtwork(100)); got != want {
		t.Fatalf("full launch artwork does not match the approved centered marks:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestLaunchArtworkAdaptsWithoutSquashingOrDroppingMonochromeMarks(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.NoTTY
	configureCLITheme("seoul-night")

	for _, width := range []int{40, 20} {
		got := renderLaunchArtwork(width)
		if strings.Contains(got, "\x1b[") {
			t.Fatalf("NO_COLOR artwork contains ANSI escapes at width %d: %q", width, got)
		}
		for row, line := range strings.Split(got, "\n") {
			if gotWidth := visibleWidth(line); gotWidth > width {
				t.Fatalf("width %d artwork row %d has %d cells: %q", width, row, gotWidth, line)
			}
		}
		if width == 40 && (strings.Count(got, "+-----------------------------+") != 4 || !strings.Contains(got, "@@@@@@@")) {
			t.Fatalf("narrow artwork should stack both complete marks:\n%s", got)
		}
	}
}

func TestLaunchArtworkUsesNationalAndThemeSemanticColors(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.ANSI256
	configureCLITheme("seoul-night")

	got := renderLaunchArtwork(100)
	for _, want := range []string{
		"\033[38;5;196m@",
		"\033[38;5;21m@",
		"\033[38;5;167m/=",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("colored artwork missing semantic sequence %q:\n%s", want, got)
		}
	}
}

func TestSessionFactsAreKoreanFirstAndCoLocated(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("ko")
	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4"
	m.effortLevel = "medium"

	plain := ansi.Strip(renderSessionFacts(m, 100))
	for _, want := range []string{"작업", "자동", "모델", "deepseek-v4", "추론", "보통", "여유"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Korean fact strip missing %q:\n%s", want, plain)
		}
	}
	for _, reject := range []string{"MODE", "MODEL", "EFFORT", "ENGINE"} {
		if strings.Contains(plain, reject) {
			t.Fatalf("Korean fact strip leaked %q:\n%s", reject, plain)
		}
	}
}

func TestSessionFactsUseEnglishModelLabelAndWrapBySemanticGroup(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("en")
	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4"
	m.effortLevel = "medium"

	plain := ansi.Strip(renderSessionFacts(m, 30))
	for _, want := range []string{"MODE", "AUTO", "MODEL", "deepseek-v4", "EFFORT", "MEDIUM", "HEADROOM"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("English fact strip missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "ENGINE") {
		t.Fatalf("English fact strip must use MODEL, not ENGINE:\n%s", plain)
	}
	for row, line := range strings.Split(plain, "\n") {
		if width := visibleWidth(line); width > 30 {
			t.Fatalf("fact row %d has width %d, want <= 30: %q", row, width, line)
		}
	}
}

func TestSessionHeaderRemainsPinnedWhenTranscriptFollowsTail(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	t.Cleanup(func() { i18n.DetectLanguage(previousLanguage) })
	i18n.DetectLanguage("ko")

	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 80)
	m.label = "deepseek-v4"
	m.effortLevel = "medium"
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	m = next.(chatTUI)
	for range 30 {
		next, _ = m.Update(agentEventMsg(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: "tail activity"}))
		m = next.(chatTUI)
	}

	want := ansi.Strip(renderSessionHeader(m, 80)) + "\n"
	got := ansi.Strip(m.View().Content)
	if !strings.HasPrefix(got, want) {
		t.Fatalf("session header must remain above a tail-following transcript:\n--- got ---\n%s\n--- want prefix ---\n%s", got, want)
	}
}

func TestNativeScrollbackViewKeepsPersistentSessionHeader(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	t.Cleanup(func() { i18n.DetectLanguage(previousLanguage) })
	i18n.DetectLanguage("ko")

	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 80)
	m.nativeScrollback = true
	m.label = "deepseek-v4"
	m.effortLevel = "medium"
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	m = next.(chatTUI)

	want := ansi.Strip(renderSessionHeader(m, 80)) + "\n"
	got := ansi.Strip(m.View().Content)
	if !strings.HasPrefix(got, want) {
		t.Fatalf("native-scrollback view must keep the session header above its bottom rail:\n--- got ---\n%s\n--- want prefix ---\n%s", got, want)
	}
}

func TestLaunchMastheadDoesNotDuplicatePersistentSessionFacts(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	t.Cleanup(func() { i18n.DetectLanguage(previousLanguage) })
	i18n.DetectLanguage("ko")

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4"
	m.effortLevel = "medium"
	plain := ansi.Strip(renderLaunchMasthead(m, "", 100))
	for _, duplicate := range []string{i18n.M.ChatMastheadTitle, "deepseek-v4", i18n.M.ChatStatusHeadroomLabel} {
		if strings.Contains(plain, duplicate) {
			t.Fatalf("launch scrollback duplicated persistent header value %q:\n%s", duplicate, plain)
		}
	}
}

func TestContextHeadroomReportsRemainingWindow(t *testing.T) {
	for _, tt := range []struct {
		used, window int
		want         string
	}{
		{18, 100, "82%"},
		{100, 100, "0%"},
		{120, 100, "0%"},
		{0, 0, "—"},
	} {
		t.Run(fmt.Sprintf("%d-of-%d", tt.used, tt.window), func(t *testing.T) {
			if got := contextHeadroom(tt.used, tt.window); got != tt.want {
				t.Fatalf("contextHeadroom(%d, %d) = %q, want %q", tt.used, tt.window, got, tt.want)
			}
		})
	}
}

func TestLocalizedEffortFactPreservesValidProviderSpecificLevels(t *testing.T) {
	for _, tt := range []struct {
		in, want string
	}{
		{in: "", want: i18n.M.ChatEffortAuto},
		{in: "auto", want: i18n.M.ChatEffortAuto},
		{in: "medium", want: i18n.M.ChatEffortMedium},
		{in: "disabled", want: "disabled"},
		{in: "none", want: "none"},
		{in: "provider-custom", want: "provider-custom"},
	} {
		if got := localizedEffortFact(tt.in); got != tt.want {
			t.Errorf("localizedEffortFact(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestComposerChromeIsKoreanFirstRoundedAndComplete(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("ko")
	m := newTestChatTUI()
	m.input.Reset()
	m.refreshInputPlaceholder()

	plain := ansi.Strip(renderComposerChrome(m, 80))
	for _, want := range []string{
		"╭─ 메시지 입력",
		"╮",
		"명령 또는 질문을 입력해보세요",
		"╰",
		"╯",
		"도움말",
		"/ 명령어",
		"@ 파일",
		"! 셸",
		"? 단축키",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Korean composer chrome missing %q:\n%s", want, plain)
		}
	}
	for _, reject := range []string{"❯", "┌", "┐", "└", "┘", "▌"} {
		if strings.Contains(plain, reject) {
			t.Fatalf("rounded composer retained rejected glyph %q:\n%s", reject, plain)
		}
	}
	lines := strings.Split(plain, "\n")
	if !strings.HasPrefix(lines[0], "╭─ 메시지 입력") {
		t.Fatalf("composer heading should be embedded in the top border:\n%s", plain)
	}
	if !strings.HasPrefix(lines[1], "│ ") || !strings.HasSuffix(lines[1], " │") {
		t.Fatalf("composer input row should have vertical rounded-box sides:\n%s", plain)
	}
	if !strings.HasPrefix(lines[2], "╰") || !strings.HasSuffix(lines[2], "╯") {
		t.Fatalf("composer should close the rounded input rectangle before hints:\n%s", plain)
	}
	if got := len(lines); got != 5 {
		t.Fatalf("one-row composer chrome has %d rows, want top border + input + bottom border + spacer + hints:\n%s", got, plain)
	}
}

func TestComposerChromeUsesEnglishCatalogAndFitsWidth(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("en")
	m := newTestChatTUI()
	m.input.Reset()
	m.refreshInputPlaceholder()

	plain := ansi.Strip(renderComposerChrome(m, 48))
	flattened := strings.Join(strings.Fields(plain), " ")
	for _, want := range []string{
		"╭─ MESSAGE INPUT",
		"MESSAGE INPUT",
		"Type a command or ask a question",
		"╰",
		"HELP",
		"/ commands",
		"@ files",
		"! shell",
		"? shortcuts",
	} {
		if !strings.Contains(flattened, want) {
			t.Fatalf("English composer chrome missing %q:\n%s", want, plain)
		}
	}
	for row, line := range strings.Split(plain, "\n") {
		if got := visibleWidth(line); got > 48 {
			t.Fatalf("composer row %d has width %d, want <= 48: %q", row, got, line)
		}
	}
}

func TestComposerChromeCompactsHintsBeforeCrowdingOutEditor(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("ko")
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 16)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 16, Height: 14})
	m = next.(chatTUI)
	m.commitLine("active transcript")

	plain := ansi.Strip(renderComposerChrome(m, 16))
	for _, symbol := range []string{"/", "@", "!", "?"} {
		if !strings.Contains(plain, symbol) {
			t.Fatalf("compact composer hints missing %q:\n%s", symbol, plain)
		}
	}
	if m.input.MaxHeight < 3 {
		t.Fatalf("compact hints left only %d editor rows, want at least 3", m.input.MaxHeight)
	}
}

func TestChromeUsesSingleCanvasWithoutAnsiBackgroundBands(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.ANSI256
	configureCLITheme("seoul-night")

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4"
	m.effortLevel = "medium"
	m.gitStatus = gitStatus{Repo: "patty-code", Branch: "main"}

	for name, got := range map[string]string{
		"session header": renderSessionHeader(m, 80),
		"composer":       renderComposerChrome(m, 80),
		"status":         m.renderStatusBlock("", 80),
	} {
		if strings.Contains(got, "\033[48;") {
			t.Fatalf("%s painted an ANSI background band into a nested text surface:\n%q", name, got)
		}
	}
	v := m.View()
	if v.BackgroundColor == nil {
		t.Fatal("view did not establish the active theme canvas")
	}
}

func TestGitOnlyStatusIsOneQuietRowWithoutDivider(t *testing.T) {
	m := newTestChatTUI()
	m.gitStatus = gitStatus{Repo: "patty-code", Branch: "main", Added: 2}

	plain := ansi.Strip(m.renderStatusBlock("", 80))
	lines := strings.Split(plain, "\n")
	if len(lines) != 1 {
		t.Fatalf("git-only status rows = %d, want one quiet row:\n%s", len(lines), plain)
	}
	if strings.ContainsRune(plain, '─') {
		t.Fatalf("git-only status retained a decorative divider:\n%s", plain)
	}
}

func TestShortTerminalKeepsSessionHeaderAndFitsExactHeight(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	t.Cleanup(func() { i18n.DetectLanguage(previousLanguage) })
	i18n.DetectLanguage("ko")

	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 80)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 7})
	m = next.(chatTUI)

	if got := m.sessionHeaderRowCount(); got == 0 {
		t.Fatal("short terminal dropped the persistent session header")
	}
	plain := ansi.Strip(m.View().Content)
	if !strings.HasPrefix(plain, ansi.Strip(renderSessionHeader(m, 80))+"\n") {
		t.Fatalf("short terminal did not keep the session header at the top:\n%s", plain)
	}
	if got := len(strings.Split(plain, "\n")); got > m.height {
		t.Fatalf("short terminal rendered %d rows, want <= %d:\n%s", got, m.height, plain)
	}
}

func TestSixRowTerminalKeepsCompactSessionHeader(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	t.Cleanup(func() { i18n.DetectLanguage(previousLanguage) })
	i18n.DetectLanguage("ko")

	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 164)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 164, Height: 6})
	m = next.(chatTUI)

	if got := m.sessionHeaderRowCount(); got == 0 {
		t.Fatal("six-row terminal dropped the compact session header")
	}
	plain := ansi.Strip(m.View().Content)
	header := ansi.Strip(renderSessionHeader(m, 164))
	if !strings.HasPrefix(plain, header+"\n") {
		t.Fatalf("six-row terminal did not keep the session header at the top:\n%s", plain)
	}
	if got := len(strings.Split(plain, "\n")); got != m.height {
		t.Fatalf("six-row terminal rendered %d rows, want %d:\n%s", got, m.height, plain)
	}
}

func TestSessionHeaderUsesTitlebarAndSegmentedFacts(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	t.Cleanup(func() { i18n.DetectLanguage(previousLanguage) })
	i18n.DetectLanguage("ko")

	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 100)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(chatTUI)

	plain := ansi.Strip(renderSessionHeader(m, 100))
	lines := strings.Split(plain, "\n")
	if len(lines) != 3 {
		t.Fatalf("session header has %d rows, want titlebar + segmented facts + border:\n%s", len(lines), plain)
	}
	if strings.Contains(plain, "+-----------------------------+") {
		t.Fatalf("session header must not own launch artwork:\n%s", plain)
	}
	if !strings.Contains(plain, "Patty Code") || !strings.Contains(plain, "╭") || !strings.Contains(plain, "╰") {
		t.Fatalf("session header did not render a titlebar frame:\n%s", plain)
	}
	if !strings.Contains(plain, "모델") || !strings.Contains(plain, "여유") {
		t.Fatalf("segmented status bar dropped live facts:\n%s", plain)
	}
	if got := m.sessionHeaderRowCount(); got != 3 {
		t.Fatalf("session header reserves %d rows, want 3", got)
	}
}

func TestLaunchStageShowsCenteredArtworkWithoutHeaderOwnership(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	t.Cleanup(func() { i18n.DetectLanguage(previousLanguage) })
	i18n.DetectLanguage("ko")

	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 100)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = next.(chatTUI)

	plain := ansi.Strip(m.View().Content)
	header := ansi.Strip(renderSessionHeader(m, 100))
	if strings.Contains(header, "+-----------------------------+") {
		t.Fatalf("header exposed launch artwork:\n%s", header)
	}
	if !strings.Contains(plain, "+-----------------------------+  +-----------------------------+") {
		t.Fatalf("launch stage did not show the two approved marks:\n%s", plain)
	}
	if !strings.Contains(plain, "Patty Code") || !strings.Contains(plain, "입력") {
		t.Fatalf("launch stage lost the titlebar/composer hierarchy:\n%s", plain)
	}
}
