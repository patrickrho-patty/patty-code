package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/charmbracelet/x/ansi"

	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/i18n"
	"patty/internal/provider"
)

func TestTurnReceiptKeepsCompletePerTurnBreakdown(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	defer i18n.DetectLanguage("en")
	activeColorProfile = colorprofile.NoTTY
	configureCLITheme("dark")
	i18n.DetectLanguage("ko-KR")

	u := &provider.Usage{
		PromptTokens:     13_625,
		CompletionTokens: 392,
		TotalTokens:      14_017,
		CacheHitTokens:   13_184,
		CacheMissTokens:  441,
		ReasoningTokens:  24,
	}
	got := renderTurnReceipt(u, &event.CacheDiagnostics{PrefixChanged: true, PrefixChangeReasons: []string{"tools"}})
	for _, want := range []string{
		"턴", "14.0K tok", "입력 13.6K", "캐시 13.2K", "신규 441",
		"출력 392", "추론 24", "캐시 접두사 변경: tools",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("turn receipt %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "\033[") {
		t.Fatalf("NO_COLOR turn receipt contains escapes: %q", got)
	}
}

func TestTurnReceiptFallsBackToDerivedFreshTokensAndWrapsCleanly(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	defer i18n.DetectLanguage("en")
	activeColorProfile = colorprofile.ANSI256
	configureCLITheme("dark")
	i18n.DetectLanguage("en")

	got := renderTurnReceipt(&provider.Usage{
		PromptTokens: 1_200, CompletionTokens: 80, TotalTokens: 1_280, CacheHitTokens: 900,
	}, &event.CacheDiagnostics{PrefixChanged: true, PrefixChangeReasons: []string{"tools"}})
	plain := ansi.Strip(got)
	for _, want := range []string{"TURN", "cached 900", "new 300", "cache prefix changed: tools"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("turn receipt %q missing %q", plain, want)
		}
	}
	for i, line := range strings.Split(wrapTranscript(got, 32), "\n") {
		if width := visibleWidth(line); width > 32 {
			t.Fatalf("wrapped turn receipt row %d width = %d, want <= 32: %q", i, width, line)
		}
	}
}

func TestTurnReceiptIgnoresEmptyUsage(t *testing.T) {
	if got := renderTurnReceipt(nil, nil); got != "" {
		t.Fatalf("nil usage receipt = %q, want empty", got)
	}
	if got := renderTurnReceipt(&provider.Usage{}, nil); got != "" {
		t.Fatalf("empty usage receipt = %q, want empty", got)
	}
}

func TestTurnReceiptMarksEstimatedUsage(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	defer i18n.DetectLanguage("en")
	activeColorProfile = colorprofile.NoTTY
	configureCLITheme("dark")
	i18n.DetectLanguage("en")

	got := renderTurnReceipt(&provider.Usage{TotalTokens: 1_024, Estimated: true}, nil)
	for _, want := range []string{"≈1.0K tok", "estimated"} {
		if !strings.Contains(got, want) {
			t.Fatalf("estimated turn receipt %q missing %q", got, want)
		}
	}
}

func TestTurnReceiptBandUsesSingleQuietBoundary(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.NoTTY
	configureCLITheme("dark")

	band := renderTurnReceiptBand("  TURN  14.0K tok · in 13.6K", 48)
	lines := strings.Split(band, "\n")
	if len(lines) != 2 {
		t.Fatalf("turn receipt band rows = %d, want top rule and receipt:\n%s", len(lines), band)
	}
	if strings.Trim(lines[0], "─ ") != "" {
		t.Fatalf("turn receipt band boundary is not a rule:\n%s", band)
	}
	if got := visibleWidth(lines[0]); got != 48 {
		t.Fatalf("receipt rule width = %d, want 48: %q", got, lines[0])
	}
	if !strings.Contains(lines[1], "TURN  14.0K tok") {
		t.Fatalf("receipt body missing from quiet band:\n%s", band)
	}
}

func TestTurnReceiptAdaptsContrastAcrossThemes(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	defer i18n.DetectLanguage("en")
	activeColorProfile = colorprofile.ANSI256
	i18n.DetectLanguage("en")

	for _, tt := range []struct {
		mode, borderSGR, labelSGR, valueSGR string
	}{
		{mode: "dark", borderSGR: "\033[38;5;66m", labelSGR: "\033[38;5;250m", valueSGR: "\033[38;5;253m"},
		{mode: "light", borderSGR: "\033[38;5;242m", labelSGR: "\033[38;5;240m", valueSGR: "\033[38;5;235m"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			configureCLITheme(tt.mode)
			receipt := renderTurnReceipt(&provider.Usage{
				PromptTokens: 900, CompletionTokens: 100, TotalTokens: 1_000,
			}, nil)
			band := renderTurnReceiptBand(receipt, 80)
			for _, want := range []string{tt.borderSGR + "─", tt.labelSGR + "TURN", tt.valueSGR + "1.0K tok"} {
				if !strings.Contains(band, want) {
					t.Fatalf("%s receipt %q missing semantic style %q", tt.mode, band, want)
				}
			}
			if strings.Count(ansi.Strip(band), "\n") != 1 {
				t.Fatalf("%s receipt should keep one rule and one body row: %q", tt.mode, ansi.Strip(band))
			}
		})
	}
}

func TestStatusFooterSemanticPaletteAcrossThemes(t *testing.T) {
	t.Setenv("PATTY_THEME", "")
	t.Setenv("PATTY_THEME_STYLE", "")
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.ANSI256

	for _, tt := range []struct {
		mode, labelSGR, valueSGR, infoSGR, secondarySGR string
	}{
		{mode: "dark", labelSGR: "\033[38;5;250m", valueSGR: "\033[38;5;253m", infoSGR: "\033[38;5;80m", secondarySGR: "\033[38;5;80m"},
		{mode: "light", labelSGR: "\033[38;5;240m", valueSGR: "\033[38;5;235m", infoSGR: "\033[38;5;24m", secondarySGR: "\033[38;5;24m"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			configureCLITheme(tt.mode)
			m := newTestChatTUI()
			m.label = "deepseek-v4-flash"
			m.effortLevel = "auto"
			m.runtimeProfile = "full"
			primary := m.primaryStatusLine(false, false)
			if !strings.Contains(primary, tt.valueSGR+i18n.M.ChatStatusIdle) ||
				!strings.Contains(primary, tt.labelSGR+i18n.M.ChatStatusCycleHintCompact) {
				t.Fatalf("%s interaction hints should use readable semantic contrast: %q", tt.mode, primary)
			}
			for _, reject := range []string{"MODEL", "EFFORT", "deepseek-v4-flash"} {
				if strings.Contains(ansi.Strip(primary), reject) {
					t.Fatalf("%s interaction footer repeated masthead fact %q: %q", tt.mode, reject, primary)
				}
			}
		})
	}
}

func TestStatusFooterThemesKeepIdenticalGeometry(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4-flash"
	m.effortLevel = "max"
	m.runtimeProfile = "full"
	m.balance = "¥12.34"
	m.gitStatus = gitStatus{Repo: "DeepSeek-PattyCode", Branch: "feature/theme-footer", Added: 3}

	render := func(mode string, profile colorprofile.Profile) string {
		activeColorProfile = profile
		configureCLITheme(mode)
		primary := m.primaryStatusLine(false, false)
		return ansi.Strip(m.renderStatusBlock(primary, 132))
	}
	dark := render("dark", colorprofile.ANSI256)
	light := render("light", colorprofile.ANSI256)
	plain := render("dark", colorprofile.NoTTY)
	if dark != light || dark != plain {
		t.Fatalf("theme modes changed footer geometry:\ndark:\n%s\nlight:\n%s\nplain:\n%s", dark, light, plain)
	}
}

func TestStatusFooterGitAndDividerAdaptToTheme(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.ANSI256

	for _, tt := range []struct {
		mode, gitSGR, borderSGR string
	}{
		{mode: "dark", gitSGR: "\033[38;5;179m", borderSGR: "\033[38;5;66m"},
		{mode: "light", gitSGR: "\033[38;5;136m", borderSGR: "\033[38;5;242m"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			configureCLITheme(tt.mode)
			m := newTestChatTUI()
			m.gitStatus = gitStatus{Repo: "DeepSeek-PattyCode", Branch: "db4be5e6", Detached: true}
			git := m.layoutGitTelemetry(80)
			if !strings.Contains(git, tt.gitSGR+"DeepSeek-PattyCode") {
				t.Fatalf("%s Git identity should use warm semantic colour: %q", tt.mode, git)
			}
			divider := statusFooterDivider(40)
			if !strings.Contains(divider, tt.borderSGR) || visibleWidth(divider) != 40 {
				t.Fatalf("%s divider should use border token at full width: %q", tt.mode, divider)
			}
		})
	}
}

func TestContextFooterColorsOnlyValuesByUrgency(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.ANSI256
	configureCLITheme("dark")

	normal := strings.Join(renderContextStatusGroups(10, 100, .8), " ")
	if !strings.Contains(normal, "\033[38;5;250mCTX") || !strings.Contains(normal, "\033[38;5;253m10 (10%)") {
		t.Fatalf("normal context should use subtle label and neutral value: %q", normal)
	}

	warning := strings.Join(renderContextStatusGroups(75, 100, .8), " ")
	if !strings.Contains(warning, "\033[38;5;250mCOMPACT") || !strings.Contains(warning, "\033[38;5;179m5%") {
		t.Fatalf("near-threshold context should warn only on values: %q", warning)
	}

	critical := strings.Join(renderContextStatusGroups(80, 100, .8), " ")
	if !strings.Contains(critical, "\033[38;5;179m80 (80%)") || !strings.Contains(critical, "\033[38;5;167m0%") {
		t.Fatalf("critical context should keep warning/danger hierarchy: %q", critical)
	}
}

func TestStatusFooterNoColorKeepsOperationalTelemetryOnly(t *testing.T) {
	defer restoreThemeForTest(activeColorProfile, activeCLITheme)
	activeColorProfile = colorprofile.NoTTY
	configureCLITheme("dark")

	m := newTestChatTUI()
	m.label = "deepseek-v4-flash"
	m.effortLevel = "auto"
	m.runtimeProfile = "full"
	m.balance = "¥12.34"
	block := m.renderStatusBlock(m.primaryStatusLine(false, false), 120)
	if strings.Contains(block, "\033[") {
		t.Fatalf("NO_COLOR footer contains escapes: %q", block)
	}
	for _, want := range []string{i18n.M.ChatStatusIdle, "BAL ¥12.34"} {
		if !strings.Contains(block, want) {
			t.Fatalf("NO_COLOR footer missing %q:\n%s", want, block)
		}
	}
	for _, reject := range []string{"MODEL", "EFFORT", "deepseek-v4-flash", "WORK balanced", " Auto "} {
		if strings.Contains(block, reject) {
			t.Fatalf("NO_COLOR operational footer repeated launch fact %q:\n%s", reject, block)
		}
	}
}

func TestStatusFooterUsesReadableLocalizedHintAndWrapsCleanly(t *testing.T) {
	defer i18n.DetectLanguage("en")
	for _, tt := range []struct {
		lang, compact string
	}{
		{lang: "en", compact: "Shift+Tab ask/auto/plan · Ctrl+Y YOLO Mode"},
		{lang: "ko", compact: "Shift+Tab 확인/자동/계획 · Ctrl+Y YOLO 모드"},
		{lang: "en-US", compact: "Shift+Tab ask/auto/plan · Ctrl+Y YOLO Mode"},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			i18n.DetectLanguage(tt.lang)
			m := newTestChatTUI()
			m.ctrl = control.New(control.Options{})
			m.label = "deepseek-v4-flash"
			m.runtimeProfile = "full"
			m.effortLevel = "auto"

			primary := m.primaryStatusLine(false, false)
			block := ansi.Strip(m.renderStatusBlock(primary, 100))
			lines := strings.Split(block, "\n")
			// With session identity moved to the masthead, idle interaction hints
			// remain a single compact row.
			if len(lines) != 1 {
				t.Fatalf("localized footer rows = %d, want a single row:\n%s", len(lines), block)
			}
			if !strings.Contains(block, tt.compact) {
				t.Fatalf("localized footer did not keep readable shortcut hints:\n%s", block)
			}
			for _, reject := range []string{"MODEL", "EFFORT", "deepseek-v4-flash"} {
				if strings.Contains(block, reject) {
					t.Fatalf("localized footer repeated masthead fact %q:\n%s", reject, block)
				}
			}
			if strings.Contains(block, "⇧Tab") || strings.Contains(block, "^Y") {
				t.Fatalf("localized footer fell back to symbolic shortcut notation:\n%s", block)
			}
			for row, line := range lines {
				if width := visibleWidth(line); width > 100 {
					t.Fatalf("localized footer row %d width = %d, want <= 100: %q", row, width, line)
				}
			}

			narrow := ansi.Strip(m.renderStatusBlock(primary, 24))
			if strings.Contains(narrow, "Shift+Tab") || strings.Contains(narrow, "Ctrl+Y") {
				t.Fatalf("shortcut help should yield when readable key names cannot fit:\n%s", narrow)
			}
			if !strings.Contains(narrow, ansi.Strip(footerValue(i18n.M.ChatStatusIdle))) {
				t.Fatalf("narrow footer should preserve the idle state:\n%s", narrow)
			}
		})
	}
}

func TestStatusFooterLocalizesMetricLabelsAndKeepsNarrowRows(t *testing.T) {
	defer i18n.DetectLanguage("en")
	for _, tt := range []struct {
		lang      string
		telemetry []string
	}{
		{
			lang:      "ko",
			telemetry: []string{"캐시", "컨텍스트", "압축", "백그라운드", "잔액"},
		},
		{
			lang:      "en",
			telemetry: []string{"CACHE", "CTX", "COMPACT", "JOBS", "BAL"},
		},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			i18n.DetectLanguage(tt.lang)
			m := newTestChatTUI()
			m.label = "deepseek-v4-flash"
			m.effortLevel = "auto"
			m.runtimeProfile = "full"
			groups := []string{
				footerMetric(i18n.M.ChatStatusCacheLabel, footerValue("90%")),
			}
			groups = append(groups, renderContextStatusGroups(75, 100, .8)...)
			groups = append(groups,
				footerMetric(i18n.M.ChatStatusJobsLabel, footerInfo("2")),
				footerMetric(i18n.M.ChatStatusBalanceLabel, footerValue("¥12.34")),
			)
			packed := ansi.Strip(packStatusGroups(groups, 22))
			for _, label := range tt.telemetry {
				if !strings.Contains(packed, label+" ") {
					t.Fatalf("localized telemetry missing %q:\n%s", label, packed)
				}
			}
			for row, line := range strings.Split(packed, "\n") {
				if width := visibleWidth(line); width > 22 {
					t.Fatalf("localized telemetry row %d width = %d, want <= 22: %q", row, width, line)
				}
			}
		})
	}
}

func TestStatusFooterKeepsSessionFactsOutOfOperationalRows(t *testing.T) {
	i18n.DetectLanguage("en")

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4-flash"
	m.runtimeProfile = "full"
	m.effortLevel = "auto"
	m.balance = "¥12.34"
	m.gitStatus = gitStatus{
		Repo:      "DeepSeek-PattyCode",
		Branch:    "feature/responsive-footer",
		Added:     1199,
		Removed:   244,
		Untracked: 3,
	}

	primary := m.primaryStatusLine(false, false)
	lines := strings.Split(ansi.Strip(m.renderStatusBlock(primary, 160)), "\n")
	if len(lines) != 3 {
		t.Fatalf("wide status block lines = %d, want two data rows plus divider:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for _, reject := range []string{"MODEL", "EFFORT", "deepseek-v4-flash", "WORK balanced", " Auto "} {
		if strings.Contains(lines[0], reject) {
			t.Fatalf("first operational row repeated masthead fact %q:\n%s", reject, strings.Join(lines, "\n"))
		}
	}
	if strings.Contains(lines[0], "DeepSeek-PattyCode@") {
		t.Fatalf("first row should not contain Git identity:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Trim(lines[1], "─ ") != "" {
		t.Fatalf("middle row should be a divider:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[2], "PattyCode@feature/responsive-footer") || strings.Contains(lines[2], "…") {
		t.Fatalf("second row should preserve the full Git identity when it fits:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[2], "+1199 -244 ?3") || !strings.HasSuffix(lines[2], "BAL ¥12.34") {
		t.Fatalf("second row should preserve Git changes and right-anchor telemetry:\n%s", strings.Join(lines, "\n"))
	}
	for i, line := range lines {
		if got := visibleWidth(line); got > 160 {
			t.Fatalf("row %d width = %d, want <= 160: %q", i, got, line)
		}
	}
}

func TestStatusFooterWithoutGitLeftAlignsTelemetry(t *testing.T) {
	defer i18n.DetectLanguage("en")
	i18n.DetectLanguage("en")

	m := newTestChatTUI()
	m.balance = "¥12.34"
	line := ansi.Strip(m.layoutGitTelemetry(120))
	if !strings.HasPrefix(line, statusFooterIndent+"BAL ¥12.34") {
		t.Fatalf("non-Git telemetry should be left aligned, got %q", line)
	}
	if visibleWidth(line) >= 120 {
		t.Fatalf("non-Git telemetry unexpectedly retained right-alignment padding: %q", line)
	}
}

func TestStatusFooterOmitsEmptyDataBand(t *testing.T) {
	m := newTestChatTUI()
	primary := "  Auto · ready"
	block := ansi.Strip(m.renderStatusBlock(primary, 120))
	if block != primary {
		t.Fatalf("empty Git/telemetry status block = %q, want only %q", block, primary)
	}
	if strings.Contains(block, "─") {
		t.Fatalf("empty Git/telemetry status block retained a divider: %q", block)
	}
}

func TestStatusFooterMediumLayoutKeepsOneInteractionRow(t *testing.T) {
	i18n.DetectLanguage("en")

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4-flash"
	m.runtimeProfile = "full"
	m.effortLevel = "auto"

	primary := m.primaryStatusLine(false, false)
	lines := strings.Split(ansi.Strip(m.renderStatusBlock(primary, 82)), "\n")
	if len(lines) != 1 {
		t.Fatalf("medium footer rows = %d, want one interaction row without an empty data band:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for _, reject := range []string{"MODEL", "EFFORT", "deepseek-v4-flash", "WORK balanced"} {
		if strings.Contains(lines[0], reject) {
			t.Fatalf("medium operational footer repeated masthead fact %q: %q", reject, lines[0])
		}
	}
}

func TestStatusFooterStacksGitAndTelemetryWithoutFloatingContinuation(t *testing.T) {
	i18n.DetectLanguage("en")

	m := newTestChatTUI()
	m.gitStatus = gitStatus{
		Repo: "DeepSeek-PattyCode", Branch: "feature/responsive-footer", Added: 20, Removed: 4,
	}
	m.balance = "¥123.45"

	lines := strings.Split(ansi.Strip(m.layoutGitTelemetry(56)), "\n")
	if len(lines) != 2 {
		t.Fatalf("stacked Git/telemetry rows = %d, want 2:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[0], statusFooterIndent+"DeepSeek-PattyCode@") || !strings.Contains(lines[0], "+20 -4") {
		t.Fatalf("Git should own the complete first row:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[1], statusFooterIndent+"BAL ¥123.45") {
		t.Fatalf("stacked telemetry should be left aligned, got %q", lines[1])
	}
}

func TestStatusFooterNarrowLayoutBreaksBetweenGroups(t *testing.T) {
	i18n.DetectLanguage("en")

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "provider/" + strings.Repeat("long-model-", 8)
	m.runtimeProfile = "delivery"
	m.balance = "¥123.45"
	m.gitStatus = gitStatus{
		Repo:    "PattyCode-Workspace",
		Branch:  "feature/" + strings.Repeat("long-branch-", 8),
		Added:   20,
		Removed: 4,
	}

	primary := m.primaryStatusLine(false, false)
	block := ansi.Strip(m.renderStatusBlock(primary, 40))
	lines := strings.Split(block, "\n")
	if len(lines) <= 2 {
		t.Fatalf("narrow status block lines = %d, want semantic wrapping:\n%s", len(lines), block)
	}
	for i, line := range lines {
		if got := visibleWidth(line); got > 40 {
			t.Fatalf("row %d width = %d, want <= 40: %q", i, got, line)
		}
	}
	if !strings.Contains(block, "@") || !strings.Contains(block, "+20 -4") || !strings.Contains(block, "¥123.45") {
		t.Fatalf("narrow layout dropped required information:\n%s", block)
	}
}

func TestStatusFooterCustomLineStillReplacesBuiltInData(t *testing.T) {
	i18n.DetectLanguage("en")

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4-flash"
	m.runtimeProfile = "delivery"
	m.balance = "¥12.34"
	m.statuslineCmd = "custom-status"
	m.statuslineOut = "custom telemetry"
	m.gitStatus = gitStatus{Repo: "Patty Code", Branch: "main"}

	primary := m.primaryStatusLine(false, false)
	block := ansi.Strip(m.renderStatusBlock(primary, 120))
	if strings.Contains(block, "deepseek-v4-flash") || strings.Contains(block, "work delivery") || strings.Contains(block, "¥12.34") {
		t.Fatalf("custom statusline should replace built-in data fields:\n%s", block)
	}
	if !strings.Contains(block, "Patty Code@main") || !strings.Contains(block, "custom telemetry") {
		t.Fatalf("custom statusline should coexist with Git identity:\n%s", block)
	}
}

func TestStatusFooterHeightCountUsesRenderedLayout(t *testing.T) {
	i18n.DetectLanguage("en")

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.width = 34
	m.label = "provider/" + strings.Repeat("long-model-", 6)
	m.runtimeProfile = "delivery"
	m.gitStatus = gitStatus{Repo: "VeryLongWorkspaceName", Branch: strings.Repeat("branch/", 8)}
	m.balance = "¥12.34"

	primary := m.primaryStatusLine(false, false)
	want := strings.Count(m.renderStatusBlock(primary, m.width), "\n") + 1
	if got := m.computeStatusLineCount(m.width); got != want {
		t.Fatalf("computed status rows = %d, rendered rows = %d", got, want)
	}
}

func TestAlignStatusBlockRight(t *testing.T) {
	width := 40
	divider := themedRule(width, activeCLITheme.border)
	block := "left text\n" + divider + "\n\x1b[2mdimmed\x1b[0m"
	got := alignStatusBlockRight(block, width)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("alignStatusBlockRight produced %d lines, want 3:\n%s", len(lines), got)
	}
	if plain := ansi.Strip(lines[0]); len(plain) != width || !strings.HasSuffix(plain, "left text") {
		t.Fatalf("content line not right-aligned to %d columns: %q", width, plain)
	}
	if lines[1] != divider {
		t.Fatalf("divider line was modified: %q", lines[1])
	}
	if plain := ansi.Strip(lines[2]); len(plain) != width || !strings.HasSuffix(plain, "dimmed") {
		t.Fatalf("styled line not right-aligned: %q", plain)
	}
	if !strings.Contains(lines[2], "\x1b[2m") {
		t.Fatalf("ANSI styling lost on aligned line: %q", lines[2])
	}
}
