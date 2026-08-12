package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"patty/internal/event"
	"patty/internal/i18n"
	"patty/internal/provider"
)

const (
	statusFooterIndent   = "  "
	statusFooterGroupGap = 2
)

func footerLabel(label string) string {
	return themeFg(activeCLITheme.subtle, label)
}

func footerHint(hint string) string {
	return themeFg(activeCLITheme.subtle, hint)
}

func footerValue(value string) string {
	return themeFg(activeCLITheme.muted, value)
}

func footerInfo(value string) string {
	return themeFg(activeCLITheme.info, value)
}

func footerSecondary(value string) string {
	return themeFg(activeCLITheme.secondary, value)
}

func footerMetric(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return footerLabel(label) + " " + value
}

func renderNoticeLine(glyph, text string) string {
	label := footerLabel("notice")
	if glyph == "!" {
		label = themeFg(activeCLITheme.warn, "setup")
	}
	if i18n.CurrentLanguage() == "ko" {
		label = footerLabel("알림")
		if glyph == "!" {
			label = themeFg(activeCLITheme.warn, "설정")
		}
	}
	return statusFooterIndent + themeFg(activeCLITheme.accent, "◆") + " " + label + "  " + themeFg(activeCLITheme.strong, localizedStartupDiagnostic(text))
}

func isStartupDiagnosticNotice(text string) bool {
	switch strings.TrimSpace(text) {
	case "Selected model is missing its API key.",
		"Config migration did not complete.",
		"An MCP server failed to start.",
		"Some MCP servers failed to start; run /mcp for details.":
		return true
	default:
		return false
	}
}

func localizedStartupDiagnostic(text string) string {
	text = strings.TrimSpace(text)
	if i18n.CurrentLanguage() != "ko" {
		return text
	}
	switch text {
	case "Selected model is missing its API key.":
		return "선택한 모델의 API 키가 설정되어 있지 않습니다."
	case "Config migration did not complete.":
		return "구성 마이그레이션이 완료되지 않았습니다."
	case "An MCP server failed to start.":
		return "MCP 서버를 시작하지 못했습니다."
	case "Some MCP servers failed to start; run /mcp for details.":
		return "일부 MCP 서버를 시작하지 못했습니다. 자세한 내용은 /mcp를 실행하세요."
	default:
		return text
	}
}

func localizedMissingProviderWarning(text string) string {
	text = strings.TrimSpace(text)
	if i18n.CurrentLanguage() != "ko" {
		return text
	}
	providerName, keyName, ok := parseMissingProviderEnv(text)
	if !ok {
		return text
	}
	return fmt.Sprintf("제공자 %q에 필요한 환경 변수 %s가 설정되어 있지 않습니다.", providerName, keyName)
}

func parseMissingProviderEnv(text string) (string, string, bool) {
	const prefix = `provider "`
	const middle = `": missing env `
	if !strings.HasPrefix(text, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(text, prefix)
	providerName, keyName, ok := strings.Cut(rest, middle)
	if !ok || strings.TrimSpace(providerName) == "" || strings.TrimSpace(keyName) == "" {
		return "", "", false
	}
	return providerName, keyName, true
}

// renderTurnReceipt attaches the completed turn's token breakdown to the
// assistant response. Unlike the persistent footer, this is historical message
// metadata: it stays in transcript scrollback and deliberately uses a quieter
// palette than runtime/session state.
func renderTurnReceipt(u *provider.Usage, d *event.CacheDiagnostics) string {
	if u == nil || u.TotalTokens == 0 {
		return ""
	}

	total := shortTokens(u.TotalTokens) + " tok"
	if u.Estimated {
		total = "≈" + total
	}
	groups := []string{total}
	if u.PromptTokens > 0 {
		cached := u.CacheHitTokens
		fresh := u.CacheMissTokens
		if fresh == 0 {
			fresh = max(u.PromptTokens-cached, 0)
		}
		groups = append(groups,
			i18n.M.ChatTurnReceiptIn+" "+shortTokens(u.PromptTokens),
			i18n.M.ChatTurnReceiptCached+" "+shortTokens(cached),
			i18n.M.ChatTurnReceiptNew+" "+shortTokens(fresh),
		)
	}
	groups = append(groups, i18n.M.ChatTurnReceiptOut+" "+shortTokens(u.CompletionTokens))
	if u.ReasoningTokens > 0 {
		groups = append(groups, i18n.M.ChatTurnReceiptReasoning+" "+shortTokens(u.ReasoningTokens))
	}
	if u.Estimated {
		groups = append(groups, i18n.M.ChatTurnReceiptEstimated)
	}

	separator := footerHint(" · ")
	styled := make([]string, 0, len(groups))
	for _, group := range groups {
		styled = append(styled, footerValue(group))
	}
	receipt := statusFooterIndent + footerLabel(i18n.M.ChatTurnReceiptLabel) + "  " + strings.Join(styled, separator)
	if d != nil && d.PrefixChanged {
		reasons := strings.Join(d.PrefixChangeReasons, "+")
		if reasons == "" {
			reasons = i18n.M.ChatTurnReceiptUnknownReason
		}
		receipt += separator + themeFg(activeCLITheme.warn, fmt.Sprintf(i18n.M.ChatTurnReceiptPrefixChanged, reasons))
	}
	return receipt
}

// primaryStatusLine renders only live interaction state. Stable session facts
// (mode, model, effort, and context headroom) stay co-located in the persistent
// top instrument strip instead of recreating the reference harness's split footer.
func (m chatTUI) primaryStatusLine(shellMode, cancelRequested bool) string {
	var status string
	switch {
	case m.rewind != nil:
		status = "⟲ rewind"
	case m.mcpImport != nil:
		status = "MCP import"
	case m.resumePick != nil:
		status = i18n.M.StatusResumePicker
	case m.quickPick != nil:
		status = m.quickPick.title
	case m.mcp != nil:
		status = "MCP"
	case m.skillPick != nil:
		status = i18n.M.SkillPickerStatusLabel
	case m.chooser != nil:
		status = i18n.M.ChatStatusQuestion
	case m.pendingApproval != nil && m.pendingApproval.Tool == planApprovalTool:
		status = i18n.M.ChatStatusPlanApproval
	case m.pendingApproval != nil:
		status = i18n.M.ChatStatusToolApproval
	case m.clipboardImagePending:
		status = yellow(i18n.M.ClipboardImagePastingHint)
	case m.copyNoticeText != "":
		status = green(m.copyNoticeText)
	case cancelRequested:
		status = i18n.M.CtrlCQuitHint
	case shellMode:
		status = i18n.M.ShellModeHint
	case m.ctrl != nil && m.ctrl.AutoApproveTools():
		status = footerValue(i18n.M.ChatStatusYoloIdle) + " · " + footerHint(i18n.M.ChatStatusCycleHintCompact)
	default:
		status = footerValue(i18n.M.ChatStatusIdle) + " · " + footerHint(i18n.M.ChatStatusCycleHintCompact)
	}
	if mt := m.mouseTag(); mt != "" {
		if status == "" {
			status = mt
		} else {
			status += " · " + mt
		}
	}
	if status == "" {
		return ""
	}
	return statusFooterIndent + status
}

func cacheStatusColor(rate float64) cliColor {
	switch {
	case rate >= 80:
		return activeCLITheme.success
	case rate >= 50:
		return activeCLITheme.info
	default:
		return activeCLITheme.warn
	}
}

func renderContextStatusGroups(used, window int, ratio float64) []string {
	if used == 0 || window == 0 {
		return nil
	}
	pct := used * 100 / window
	ctxValue := fmt.Sprintf("%s (%d%%)", shortTokens(used), pct)

	if ratio <= 0 || ratio >= 1 {
		ctxValue = fmt.Sprintf("%s / %s (%d%%)", shortTokens(used), shortTokens(window), pct)
		color := activeCLITheme.muted
		switch {
		case pct >= 85:
			color = activeCLITheme.danger
		case pct >= 60:
			color = activeCLITheme.warn
		}
		return []string{footerMetric(i18n.M.ChatStatusContextLabel, themeFg(color, ctxValue))}
	}

	threshold := int(ratio * 100)
	left := max(threshold-pct, 0)
	ctxColor := activeCLITheme.muted
	compactColor := activeCLITheme.muted
	switch {
	case pct >= threshold:
		// Preserve two levels of urgency from the selected design: context is a
		// warning, while the exhausted compaction headroom is the actual danger.
		ctxColor = activeCLITheme.warn
		compactColor = activeCLITheme.danger
	case left <= 10:
		ctxColor = activeCLITheme.warn
		compactColor = activeCLITheme.warn
	}
	return []string{
		footerMetric(i18n.M.ChatStatusContextLabel, themeFg(ctxColor, ctxValue)),
		footerMetric(i18n.M.ChatStatusCompactLabel, themeFg(compactColor, fmt.Sprintf("%d%%", left))),
	}
}

// statusTelemetryGroups returns independently placeable session metrics. Git is
// intentionally excluded because it owns the flexible identity slot; keeping
// metrics separate lets narrow layouts wrap only between semantic groups.
func (m chatTUI) statusTelemetryGroups() []string {
	if m.statuslineCmd != "" && m.statuslineOut != "" {
		return []string{m.statuslineOut}
	}
	var data []string
	metadata := m.displayMetadata()
	if m.ctrl != nil {
		if body, rate, ok := m.cacheStatus(); ok {
			data = append(data, footerMetric(i18n.M.ChatStatusCacheLabel, themeFg(cacheStatusColor(rate), body)))
		}
		data = append(data, renderContextStatusGroups(metadata.ContextUsed, metadata.ContextWindow, m.ctrl.CompactRatio())...)
		if jt := m.jobsTag(); jt != "" {
			data = append(data, footerMetric(i18n.M.ChatStatusJobsLabel, footerInfo(ansi.Strip(jt))))
		}
	}
	if m.balance != "" {
		data = append(data, footerMetric(i18n.M.ChatStatusBalanceLabel, footerValue(m.balance)))
	}
	return data
}

// renderStatusBlock owns the complete persistent runtime footer. The optional
// Git/telemetry band is separated from transient interaction state; stable
// session identity is intentionally absent because the masthead owns it.
func (m chatTUI) renderStatusBlock(primary string, width int) string {
	if width <= 0 {
		width = 1
	}
	primary = hideStatusHintWhenKeyNamesCannotFit(primary, width)
	first := wrapStatusGroups(primary, width)
	second := m.layoutGitTelemetry(width)
	if first == "" {
		return second
	}
	if second == "" {
		return first
	}
	return first + "\n" + statusFooterDivider(width) + "\n" + second
}

// renderFrameStatusBlock keeps the short terminal frame legible. At six rows
// there is no room for a second Git/telemetry band, so the live operational line
// stays visible and stable facts remain in the compact masthead. At the most
// constrained sizes the composer and masthead take precedence over the footer.
func (m chatTUI) renderFrameStatusBlock(primary string, width int) string {
	if m.isNaturalStartupFrame() {
		return m.renderStartupInstructionBlock(primary, width)
	}
	if !m.isCompactTerminal() {
		return alignStatusBlockRight(m.renderStatusBlock(primary, width), width)
	}
	if m.isMinimalTerminal() {
		return ""
	}
	return wrapStatusGroups(hideStatusHintWhenKeyNamesCannotFit(primary, width), width)
}

// alignStatusBlockRight keeps the persistent interaction/status bands anchored
// to the same right-side rail as the launch frame. Without this, the first
// completed turn switches to the generic left-packed footer and makes the
// status/Git identity appear to jump horizontally.
func alignStatusBlockRight(block string, width int) string {
	width = max(width, 1)
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		plain := ansi.Strip(line)
		if strings.TrimSpace(plain) == "" || strings.Trim(plain, "─ ") == "" {
			continue
		}
		lines[i] = alignRightStatusLine(line, width)
	}
	return strings.Join(lines, "\n")
}

func (m chatTUI) renderStartupInstructionBlock(primary string, width int) string {
	width = max(width, 1)
	primary = hideStatusHintWhenKeyNamesCannotFit(primary, width)
	primary = strings.TrimSpace(primary)
	git := strings.TrimSpace(m.layoutGitTelemetry(width))
	line := primary
	if git != "" {
		if line != "" {
			line += " · "
		}
		line += git
	}
	if strings.TrimSpace(ansi.Strip(line)) == "" {
		return ""
	}
	line = ansi.Truncate(line, width, "")
	if m.height >= 30 {
		return "\n" + alignRightStatusLine(line, width)
	}
	return alignRightStatusLine(line, width)
}

func alignRightStatusLine(line string, width int) string {
	width = max(width, 1)
	line = ansi.Truncate(line, width, "")
	return strings.Repeat(" ", max(width-visibleWidth(line), 0)) + line
}

// hideStatusHintWhenKeyNamesCannotFit keeps the readable Shift+Tab/Ctrl+Y
// spelling on normal terminals without hard-wrapping a single shortcut on an
// extremely narrow terminal. In that case the idle state remains visible and
// the optional shortcut help yields space to the composer.
func hideStatusHintWhenKeyNamesCannotFit(primary string, width int) string {
	hint := i18n.M.ChatStatusCycleHintCompact
	for group := range strings.SplitSeq(hint, " · ") {
		if visibleWidth(statusFooterIndent+group) > width {
			return strings.Replace(primary, " · "+footerHint(hint), "", 1)
		}
	}
	return primary
}

func statusFooterDivider(width int) string {
	width = max(width, 1)
	if width <= visibleWidth(statusFooterIndent) {
		return themedRule(width, activeCLITheme.border)
	}
	ruleWidth := width - visibleWidth(statusFooterIndent)
	return statusFooterIndent + themedRule(ruleWidth, activeCLITheme.border)
}

func wrapStatusGroups(line string, width int) string {
	if width <= 0 || line == "" || visibleWidth(line) <= width {
		return line
	}
	groups := strings.Split(line, " · ")
	if len(groups) < 2 {
		return wrapStatusLine(line, width)
	}

	var rows []string
	current := groups[0]
	for _, group := range groups[1:] {
		candidate := current + " · " + group
		if visibleWidth(candidate) <= width {
			current = candidate
			continue
		}
		rows = append(rows, wrapStatusLine(current, width))
		current = statusFooterIndent + group
	}
	rows = append(rows, wrapStatusLine(current, width))
	return strings.Join(rows, "\n")
}

func (m chatTUI) layoutGitTelemetry(width int) string {
	telemetryGroups := m.statusTelemetryGroups()
	telemetry := strings.Join(telemetryGroups, "  ")
	hasGit := strings.TrimSpace(m.gitStatus.Repo) != "" && strings.TrimSpace(m.gitStatus.Branch) != ""
	if !hasGit {
		// Without a Git identity there is no left-hand peer to balance. Keep the
		// telemetry anchored to the normal footer indent instead of leaving a
		// repo-sized visual hole across most of a wide terminal.
		return packStatusGroups(telemetryGroups, width)
	}

	fullGitBudget := max(width-visibleWidth(statusFooterIndent), 1)
	git := m.gitStatus.RenderWithin(fullGitBudget, activeCLITheme.warn)
	gitLine := statusFooterIndent + git
	if telemetry == "" {
		return gitLine
	}

	telemetryWidth := visibleWidth(telemetry)
	if visibleWidth(gitLine)+statusFooterGroupGap+telemetryWidth <= width {
		return gitLine + strings.Repeat(" ", width-visibleWidth(gitLine)-telemetryWidth) + telemetry
	}

	// Under width pressure Git gets its own full row instead of being shortened
	// merely to keep telemetry beside it. Telemetry then packs left-to-right by
	// semantic group, so no right-aligned fragment floats on a continuation row.
	return gitLine + "\n" + packStatusGroups(telemetryGroups, width)
}

func packStatusGroups(groups []string, width int) string {
	width = max(width, 1)
	if len(groups) == 0 {
		return ""
	}
	indent := statusFooterIndent
	if width <= visibleWidth(indent) {
		indent = ""
	}

	var rows []string
	current := indent
	for _, group := range groups {
		if strings.TrimSpace(ansi.Strip(group)) == "" {
			continue
		}
		candidate := current + group
		if strings.TrimSpace(ansi.Strip(current)) != "" {
			candidate = current + "  " + group
		}
		if visibleWidth(candidate) <= width {
			current = candidate
			continue
		}
		if strings.TrimSpace(ansi.Strip(current)) != "" {
			rows = append(rows, current)
		}
		current = indent + group
		if visibleWidth(current) > width {
			rows = append(rows, wrapStatusLine(current, width))
			current = indent
		}
	}
	if strings.TrimSpace(ansi.Strip(current)) != "" {
		rows = append(rows, current)
	}
	return strings.Join(rows, "\n")
}
