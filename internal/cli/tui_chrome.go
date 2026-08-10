package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"patty/internal/i18n"
	pattyassets "patty/products/patty/assets"
)

const (
	launchMarkWidth            = 31
	launchArtworkGap           = "  "
	fullLaunchArtworkWidth     = launchMarkWidth*2 + len(launchArtworkGap)
	compactTerminalRows        = 7
	minimalTerminalRows        = 4
	composerCursorChromeOffset = 2
	launchStageMaxRows         = 34
)

var approvedTaegeukgiRows, approvedPattyRows = loadApprovedLaunchRows()

func loadApprovedLaunchRows() ([]string, []string) {
	rows := strings.Split(pattyassets.Banner(), "\n")
	if len(rows) != 16 {
		panic(fmt.Sprintf("patty launch banner has %d rows, want 16", len(rows)))
	}
	flag := make([]string, len(rows))
	patty := make([]string, len(rows))
	for i, row := range rows {
		if len(row) != fullLaunchArtworkWidth || row[launchMarkWidth:launchMarkWidth+len(launchArtworkGap)] != launchArtworkGap {
			panic(fmt.Sprintf("patty launch banner row %d does not contain two %d-cell marks", i, launchMarkWidth))
		}
		flag[i] = row[:launchMarkWidth]
		patty[i] = row[launchMarkWidth+len(launchArtworkGap):]
	}
	return flag, patty
}

var taegeukRedCells = map[int]int{
	5: 7,
	6: 8,
	7: 6,
	8: 3,
}

// renderLaunchArtwork keeps the two approved marks unsquashed. Standard and
// wide terminals place them side by side; narrow terminals stack the same
// 31-cell grids; minimum-width terminals receive a bounded identity shorthand.
func renderLaunchArtwork(width int) string {
	width = max(width, 1)
	if width < visibleWidth(approvedTaegeukgiRows[0]) {
		return ansi.Truncate("[KR] [PATTY]", width, "")
	}

	flag := make([]string, len(approvedTaegeukgiRows))
	patty := make([]string, len(approvedPattyRows))
	for row := range approvedTaegeukgiRows {
		flag[row] = colorTaegeukgiRow(approvedTaegeukgiRows[row], row)
		patty[row] = colorPattyRow(approvedPattyRows[row])
	}
	if width < fullLaunchArtworkWidth {
		return strings.Join(flag, "\n") + "\n" + strings.Join(patty, "\n")
	}

	rows := make([]string, len(flag))
	for row := range flag {
		rows[row] = flag[row] + launchArtworkGap + patty[row]
	}
	return strings.Join(rows, "\n")
}

func renderLaunchMasthead(m chatTUI, missing string, width int) string {
	width = max(width, 1)
	if m.isNaturalStartupFrame() || m.shouldCompactLaunchArtwork() {
		return renderCompactLaunchMasthead(m, missing, width)
	}
	var b strings.Builder
	b.WriteString(renderLaunchArtwork(width))
	b.WriteByte('\n')
	b.WriteString(dim(i18n.M.ChatTip))
	if strings.TrimSpace(missing) != "" {
		b.WriteByte('\n')
		b.WriteString(wrapForViewport("! "+localizedMissingProviderWarning(missing), width, activeCLITheme.warn))
	}
	return b.String()
}

func (m chatTUI) isNaturalStartupFrame() bool {
	return m.isLaunchChromeOnlyTranscript()
}

func (m chatTUI) shouldCompactLaunchArtwork() bool {
	if m.height <= 0 || m.nativeScrollback || m.usesTallSessionMasthead(max(m.width, 1)) {
		return false
	}
	// The launch mark is an atomic 16-row identity block. If the transcript
	// viewport cannot hold the block and its tip, render only the tip rather than
	// letting tail-follow expose an arbitrary middle slice after resize.
	return m.transcriptHeight() < len(approvedTaegeukgiRows)+1
}

func renderCompactLaunchMasthead(m chatTUI, missing string, width int) string {
	lines := []string{ansi.Truncate(dim(i18n.M.ChatTip), width, "")}
	if strings.TrimSpace(missing) != "" {
		lines = append(lines, wrapForViewport("! "+localizedMissingProviderWarning(missing), width, activeCLITheme.warn))
	}
	return strings.Join(lines, "\n")
}

// renderSessionHeader is the persistent top instrument strip from the Seoul
// flow-board. Artwork remains a launch moment in transcript scrollback, while
// live mode, model, effort, and headroom stay visible above every turn.
func renderSessionHeader(m chatTUI, width int) string {
	width = max(width, 1)
	if m.isCompactTerminal() || width < 48 {
		return renderCompactSessionHeader(m, width)
	}
	return renderTitlebar(width) + "\n" + renderInstrumentBar(m, width)
}

func (m chatTUI) usesTallSessionMasthead(width int) bool {
	return false
}

func renderTallSessionMasthead(m chatTUI, width int) string {
	artRows := strings.Split(renderLaunchArtwork(fullLaunchArtworkWidth), "\n")
	rightWidth := max(width-fullLaunchArtworkWidth-len(launchArtworkGap), 1)
	title := themeStyle(activeCLITheme.accent).Bold(true).Render(i18n.M.ChatMastheadTitle)
	rightRows := []string{ansi.Truncate(title, rightWidth, "")}
	if facts := renderSessionFacts(m, rightWidth); facts != "" {
		rightRows = append(rightRows, strings.Split(facts, "\n")...)
	}

	for row := range artRows {
		right := ""
		if row < len(rightRows) {
			right = ansi.Truncate(rightRows[row], rightWidth, "")
		}
		artRows[row] += launchArtworkGap + padRight(right, rightWidth)
	}
	return strings.Join(artRows, "\n")
}

func renderTitlebar(width int) string {
	width = max(width, 1)
	if width < 4 {
		return ansi.Truncate("Patty Code", width, "")
	}
	left := themeFg(activeCLITheme.border, "╭─")
	right := themeFg(activeCLITheme.border, "╮")
	title := themeStyle(activeCLITheme.accent).Bold(true).Render(" Patty Code ")
	fillWidth := max(width-visibleWidth(left)-visibleWidth(title)-visibleWidth(right), 0)
	return left + title + themedRule(fillWidth, activeCLITheme.border) + right
}

func renderInstrumentBar(m chatTUI, width int) string {
	width = max(width, 1)
	facts := strings.Split(renderSessionFacts(m, max(width-2, 1)), "\n")
	rows := make([]string, 0, len(facts)+1)
	for _, row := range facts {
		rows = append(rows, boxedHeaderRow(row, width))
	}
	rows = append(rows, themeFg(activeCLITheme.border, "╰")+themedRule(max(width-2, 0), activeCLITheme.border)+themeFg(activeCLITheme.border, "╯"))
	return strings.Join(rows, "\n")
}

func boxedHeaderRow(row string, width int) string {
	width = max(width, 1)
	if width < 3 {
		return ansi.Truncate(ansi.Strip(row), width, "")
	}
	inner := max(width-2, 1)
	row = ansi.Truncate(row, inner, "")
	padding := strings.Repeat(" ", max(inner-visibleWidth(row), 0))
	return themeFg(activeCLITheme.border, "│") + row + padding + themeFg(activeCLITheme.border, "│")
}

func (m chatTUI) isCompactTerminal() bool {
	return m.height > 0 && m.height <= compactTerminalRows
}

func (m chatTUI) isMinimalTerminal() bool {
	return m.height > 0 && m.height <= minimalTerminalRows
}

func renderCompactSessionHeader(m chatTUI, width int) string {
	title := themeStyle(activeCLITheme.accent).Bold(true).Render(i18n.M.ChatMastheadTitle)
	facts := strings.ReplaceAll(renderSessionFacts(m, width), "\n", "  ")
	line := title + "  " + facts
	if visibleWidth(line) <= width {
		return line
	}
	// The compact rail is a priority surface. If a narrow terminal cannot fit
	// every fact, keep the title and the first semantic groups rather than
	// allowing a wrapped fact strip to become an accidental second masthead.
	return ansi.Truncate(title+"  "+ansi.Strip(facts), width, "")
}

func (m chatTUI) sessionHeaderRowCount() int {
	rows := strings.Count(renderSessionHeader(m, max(m.width, 10)), "\n") + 1
	if m.height <= 0 {
		return rows
	}
	minimumBottom := m.bottomRows()
	// On very short panes, preserve the masthead and one transcript row before
	// preserving a second transcript row. The header is the user's orientation
	// anchor; a one-row transcript is still useful, while a missing header makes
	// the entire frame look like an unstyled input prompt.
	if rows+minimumBottom+1 > m.height {
		return 0
	}
	return rows
}

// transcriptLocalY translates a terminal row into the viewport's coordinate
// system. Native-scrollback mode has no application-owned transcript viewport.
func (m chatTUI) transcriptLocalY(screenY int) (int, bool) {
	if m.nativeScrollback {
		return 0, false
	}
	y := screenY - m.sessionHeaderRowCount()
	return y, y >= 0 && y < m.viewport.Height()
}

func (m *chatTUI) refreshLaunchMasthead() bool {
	revision := launchMastheadRevision(*m)
	if revision == m.launchMastheadRevision {
		return false
	}
	m.launchMastheadRevision = revision
	m.ensureTranscriptSources()
	changed := false
	for i, source := range m.transcriptSources {
		var rendered string
		switch source.kind {
		case transcriptSourceBanner:
			rendered = renderLaunchMasthead(*m, source.raw, transcriptContentWidth(m.width, m.nativeScrollback))
		case transcriptSourceReplayBundle:
			rendered = m.renderTranscriptSource(source, m.width)
		default:
			continue
		}
		if m.transcript[i] == rendered {
			continue
		}
		m.setTranscriptBlock(i, rendered, source)
		changed = true
	}
	return changed
}

func launchMastheadRevision(m chatTUI) string {
	return fmt.Sprintf("%s\x1f%s\x1f%s\x1f%d\x1f%d\x1f%d\x1f%t\x1f%t\x1f%d",
		i18n.CurrentLanguage(),
		activeCLITheme.name,
		activeCLITheme.style,
		activeColorProfile,
		m.width,
		m.height,
		m.started,
		m.isLaunchChromeOnlyTranscript(),
		len(m.transcriptSources),
	)
}

func colorTaegeukgiRow(row string, rowIndex int) string {
	red := cliColor{"#cd2e3a", 196}
	blue := cliColor{"#0047a0", 21}
	redCells := taegeukRedCells[rowIndex]
	taegeukCell := 0
	return colorASCIIRuns(row, func(r rune) (cliColor, bool) {
		switch r {
		case '@':
			color := blue
			if taegeukCell < redCells {
				color = red
			}
			taegeukCell++
			return color, true
		case '=', '/', '\\':
			return activeCLITheme.strong, true
		}
		return cliColor{}, false
	})
}

func colorPattyRow(row string) string {
	return colorASCIIRuns(row, func(r rune) (cliColor, bool) {
		switch r {
		case '=', '/', '\\':
			return activeCLITheme.accent, true
		default:
			return cliColor{}, false
		}
	})
}

func colorASCIIRuns(row string, colorFor func(rune) (cliColor, bool)) string {
	if !colorOn() {
		return row
	}
	var b strings.Builder
	var run strings.Builder
	var runColor cliColor
	flush := func() {
		if run.Len() == 0 {
			return
		}
		b.WriteString(themeFg(runColor, run.String()))
		run.Reset()
	}
	for _, r := range row {
		color, ok := colorFor(r)
		if !ok {
			flush()
			b.WriteRune(r)
			continue
		}
		if run.Len() > 0 && color != runColor {
			flush()
		}
		runColor = color
		run.WriteRune(r)
	}
	flush()
	return b.String()
}

func renderSessionFacts(m chatTUI, width int) string {
	mode := localizedModeFact(m)
	model := strings.TrimSpace(m.label)
	if model == "" {
		model = "—"
	}
	effort := localizedEffortFact(m.effortLevel)

	used, window := 0, 0
	if m.ctrl != nil {
		used, window = m.ctrl.ContextSnapshot()
	}
	groups := []string{
		sessionFactPill(i18n.M.ChatStatusModeLabel, themeFg(activeCLITheme.accent, mode)),
		sessionFactPill(i18n.M.ChatStatusModelLabel, themeFg(activeCLITheme.strong, model)),
		sessionFactPill(i18n.M.ChatStatusEffortLabel, themeFg(activeCLITheme.strong, effort)),
		sessionFactPill(i18n.M.ChatStatusHeadroomLabel, themeFg(activeCLITheme.signal, contextHeadroom(used, window))),
	}
	return packStatusGroups(groups, width)
}

func sessionFactPill(label, value string) string {
	return themeFg(activeCLITheme.border, "[ ") +
		footerLabel(label) +
		" " +
		value +
		themeFg(activeCLITheme.border, " ]")
}

func localizedModeFact(m chatTUI) string {
	if strings.HasPrefix(strings.TrimSpace(m.input.Value()), "!") {
		return i18n.M.ChatModeShell
	}
	if m.ctrl == nil {
		return i18n.M.ChatModeAuto
	}
	parts := strings.Split(m.modeTagText(), "+")
	for i, part := range parts {
		switch part {
		case "Ask":
			parts[i] = i18n.M.ChatModeAsk
		case "Auto":
			parts[i] = i18n.M.ChatModeAuto
		case "Plan":
			parts[i] = i18n.M.ChatModePlan
		case "Goal":
			parts[i] = i18n.M.ChatModeGoal
		case "YOLO":
			parts[i] = i18n.M.ChatModeYOLO
		case "Don't Ask":
			parts[i] = i18n.M.ChatModeDontAsk
		case "Approve":
			parts[i] = i18n.M.ChatModeApprove
		}
	}
	return strings.Join(parts, "+")
}

func localizedEffortFact(effort string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	switch effort {
	case "", "auto":
		return i18n.M.ChatEffortAuto
	case "low":
		return i18n.M.ChatEffortLow
	case "medium":
		return i18n.M.ChatEffortMedium
	case "high":
		return i18n.M.ChatEffortHigh
	case "xhigh":
		return i18n.M.ChatEffortXHigh
	case "max":
		return i18n.M.ChatEffortMax
	default:
		return effort
	}
}

func contextHeadroom(used, window int) string {
	if window <= 0 {
		return "—"
	}
	remaining := max(window-used, 0)
	return fmt.Sprintf("%d%%", remaining*100/window)
}

const composerChromeInset = 2
const minTranscriptRowsWithComposerHints = 3

// renderComposerChrome gives the input a padded label and a secondary
// capability legend without drawing a box around the conversation. The textarea
// keeps its own height between these fixed rows, so cursor and viewport
// accounting remain exact as the input grows.
func renderComposerChrome(m chatTUI, width int) string {
	width = max(width, 1)
	titleText := "메시지 입력"
	if i18n.CurrentLanguage() == "en" {
		titleText = "MESSAGE INPUT"
	}
	title := themeStyle(activeCLITheme.signal).Bold(true).Render(titleText)
	header := renderComposerTopBorder(title, width)

	inputWidth := max(width-4, 1)
	inputRows := strings.Split(m.renderComposerInput(), "\n")
	if h := max(m.input.Height(), 1); len(inputRows) > h {
		inputRows = inputRows[:h]
	}
	for row := range inputRows {
		// The textarea owns soft wrapping and its visible height. Lip Gloss must
		// not create additional unaccounted rows from a long placeholder.
		inputRows[row] = ansi.Truncate(inputRows[row], inputWidth, "")
	}
	input := renderComposerInputPlate(strings.Join(inputRows, "\n"), inputWidth)
	bottom := renderComposerBottomBorder(width)
	if m.isMinimalTerminal() {
		return ansi.Truncate(title+"  "+ansi.Strip(input), width, "")
	}
	if m.isCompactTerminal() {
		return header + "\n" + input + "\n" + bottom
	}
	if m.isNaturalStartupFrame() && !m.completion.active {
		return header + "\n" + input + "\n" + bottom
	}
	if m.composerHintRowCount(width) == 0 {
		return header + "\n" + input + "\n" + bottom
	}
	hints := renderComposerHints(width)
	if m.composerHintSpacerRows(width) == 0 {
		return header + "\n" + input + "\n" + bottom + "\n" + hints
	}
	return header + "\n" + input + "\n" + bottom + "\n" + strings.Repeat(" ", min(composerChromeInset, width)) + "\n" + hints
}

func renderComposerInputPlate(input string, width int) string {
	width = max(width, 1)
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		line = composerInputStyle.Render(line)
		line = ansi.Truncate(line, width, "")
		padding := strings.Repeat(" ", max(width-visibleWidth(line), 0))
		if width <= 1 {
			lines[i] = themeFg(activeCLITheme.border, "│") + ansi.Truncate(ansi.Strip(line), 1, "") + themeFg(activeCLITheme.border, "│")
			continue
		}
		lines[i] = themeFg(activeCLITheme.border, "│") + " " + line + padding + " " + themeFg(activeCLITheme.border, "│")
	}
	return strings.Join(lines, "\n")
}

func renderComposerTopBorder(title string, width int) string {
	width = max(width, 1)
	if width < 4 {
		return ansi.Truncate("╭"+ansi.Strip(title), width, "")
	}
	left := themeFg(activeCLITheme.border, "╭─")
	right := themeFg(activeCLITheme.border, "╮")
	title = " " + title + " "
	fillWidth := max(width-visibleWidth(left)-visibleWidth(title)-visibleWidth(right), 0)
	return left + title + themedRule(fillWidth, activeCLITheme.border) + right
}

func renderComposerBottomBorder(width int) string {
	width = max(width, 1)
	if width < 3 {
		return ansi.Truncate("╰╯", width, "")
	}
	return themeFg(activeCLITheme.border, "╰") + themedRule(max(width-2, 0), activeCLITheme.border) + themeFg(activeCLITheme.border, "╯")
}

func renderComposerHints(width int) string {
	label := "도움말"
	if i18n.CurrentLanguage() == "en" {
		label = "HELP"
	}
	hintGroups := []string{
		themeFg(activeCLITheme.faint, label),
		themeFg(activeCLITheme.muted, "/ "+strings.TrimSpace(strings.TrimPrefix(i18n.M.ChatComposerCommandsHint, "/"))),
		themeFg(activeCLITheme.muted, "@ "+strings.TrimSpace(strings.TrimPrefix(i18n.M.ChatComposerFilesHint, "@"))),
		themeFg(activeCLITheme.muted, "! "+strings.TrimSpace(strings.TrimPrefix(i18n.M.ChatComposerShellHint, "!"))),
		themeFg(activeCLITheme.muted, "? "+strings.TrimSpace(strings.TrimPrefix(i18n.M.ChatComposerShortcutsHint, "?"))),
	}
	if width < 28 {
		// Labels would consume four or more rows here and crowd out the actual
		// editor. The symbols are the stable command language across locales.
		hintGroups = []string{
			themeFg(activeCLITheme.faint, label),
			themeFg(activeCLITheme.muted, "/"),
			themeFg(activeCLITheme.muted, "@"),
			themeFg(activeCLITheme.muted, "!"),
			themeFg(activeCLITheme.muted, "?"),
		}
	}
	return insetComposerLine(packStatusGroups(hintGroups, max(width-composerChromeInset, 1)), width)
}

func (m chatTUI) composerHintRowCount(width int) int {
	width = max(width, 1)
	hintRows := strings.Count(renderComposerHints(width), "\n") + 1
	if m.height > 0 {
		fixedBottom := m.bottomRowsWithoutComposer()
		headerRows := strings.Count(renderSessionHeader(m, max(m.width, 10)), "\n") + 1
		composerRowsWithHints := 2 + m.input.Height() + hintRows
		if headerRows+fixedBottom+composerRowsWithHints+minTranscriptRowsWithComposerHints > m.height {
			return 0
		}
	}
	return m.composerHintSpacerRows(width) + hintRows
}

func (m chatTUI) composerHintSpacerRows(width int) int {
	width = max(width, 1)
	if m.height <= 0 {
		return 1
	}
	hintRows := strings.Count(renderComposerHints(width), "\n") + 1
	composerRowsWithSpacer := 2 + m.input.Height() + hintRows + 1
	fixedBottom := m.bottomRowsWithoutComposer()
	headerRows := strings.Count(renderSessionHeader(m, max(m.width, 10)), "\n") + 1
	if headerRows+fixedBottom+composerRowsWithSpacer+minTranscriptRowsWithComposerHints > m.height {
		return 0
	}
	return 1
}

func insetComposerLine(s string, width int) string {
	width = max(width, 1)
	if width <= composerChromeInset {
		return ansi.Truncate(s, width, "")
	}
	inset := strings.Repeat(" ", composerChromeInset)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = inset + ansi.Truncate(line, width-composerChromeInset, "")
	}
	return strings.Join(lines, "\n")
}
