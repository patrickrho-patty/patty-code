package cli

import (
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"patty/internal/config"
	"patty/internal/i18n"
)

type cliColor struct {
	hex string
	// Distance-based downsampling collapses the dark, low-chroma diff backgrounds
	// to plain grey and loses the red/green tint that carries their meaning, so
	// the 256-colour fallback stays hand-chosen rather than computed.
	xterm int
}

type cliPalette struct {
	name         string
	style        string
	background   cliColor
	surface      cliColor
	composer     cliColor
	strong       cliColor
	signal       cliColor
	accent       cliColor
	muted        cliColor
	faint        cliColor
	subtle       cliColor
	success      cliColor
	warn         cliColor
	err          cliColor
	danger       cliColor
	info         cliColor
	secondary    cliColor
	border       cliColor
	selection    cliColor
	userBubbleBG cliColor
	diffAddBG    cliColor
	diffDelBG    cliColor
	toolRead     cliColor
	toolProc     cliColor
}

type cliThemeStyle struct {
	name        string
	mode        string
	description string
	palette     cliPalette
}

var (
	cliDarkTheme = cliPalette{
		name:         "dark",
		style:        "seoul-night",
		background:   cliColor{"#07121a", 233},
		surface:      cliColor{"#091923", 234},
		composer:     cliColor{"#061019", 233},
		strong:       cliColor{"#edf6f7", 255},
		signal:       cliColor{"#d1a653", 179},
		accent:       cliColor{"#e16162", 167},
		muted:        cliColor{"#d8e6e8", 253},
		faint:        cliColor{"#789aa1", 66},
		subtle:       cliColor{"#a8bec2", 250},
		success:      cliColor{"#74b87a", 108},
		warn:         cliColor{"#d1a653", 179},
		err:          cliColor{"#e16162", 167},
		danger:       cliColor{"#e5484d", 167},
		info:         cliColor{"#5cb2c0", 80},
		secondary:    cliColor{"#5cb2c0", 80},
		border:       cliColor{"#5f7f89", 66},
		selection:    cliColor{"#d1a653", 179},
		userBubbleBG: cliColor{"#0a1b24", 234},
		diffAddBG:    cliColor{"#14351d", 22},
		diffDelBG:    cliColor{"#3a1619", 52},
		toolRead:     cliColor{"#5cb2c0", 80},
		toolProc:     cliColor{"#c678dd", 176},
	}
	cliLightTheme = cliPalette{
		name:         "light",
		style:        "hanji-light",
		background:   cliColor{"#eee9dc", 255},
		surface:      cliColor{"#e4ddcf", 254},
		composer:     cliColor{"#f6f2e9", 255},
		strong:       cliColor{"#1f211c", 234},
		signal:       cliColor{"#986b2d", 136},
		accent:       cliColor{"#a83e42", 131},
		muted:        cliColor{"#2c2c26", 235},
		faint:        cliColor{"#756f64", 242},
		subtle:       cliColor{"#59554c", 240},
		success:      cliColor{"#5d8f5f", 65},
		warn:         cliColor{"#986b2d", 136},
		err:          cliColor{"#a83e42", 131},
		danger:       cliColor{"#a83e42", 131},
		info:         cliColor{"#285f85", 24},
		secondary:    cliColor{"#285f85", 24},
		border:       cliColor{"#756f64", 242},
		selection:    cliColor{"#986b2d", 136},
		userBubbleBG: cliColor{"#e3dccd", 253},
		diffAddBG:    cliColor{"#dce9d9", 254},
		diffDelBG:    cliColor{"#f0d9d9", 224},
		toolRead:     cliColor{"#285f85", 24},
		toolProc:     cliColor{"#7a5a94", 97},
	}
	cliInkTheme = cliPalette{
		name:         "dark",
		style:        "ink-night",
		background:   cliColor{"#11110f", 233},
		surface:      cliColor{"#181713", 234},
		composer:     cliColor{"#0d0d0b", 232},
		strong:       cliColor{"#f3eee5", 255},
		signal:       cliColor{"#bc9450", 179},
		accent:       cliColor{"#cd5257", 167},
		muted:        cliColor{"#e6e0d4", 253},
		faint:        cliColor{"#938c7e", 244},
		subtle:       cliColor{"#c3bbad", 250},
		success:      cliColor{"#7dae78", 108},
		warn:         cliColor{"#bc9450", 179},
		err:          cliColor{"#cd5257", 167},
		danger:       cliColor{"#e05a60", 167},
		info:         cliColor{"#7295ad", 67},
		secondary:    cliColor{"#7295ad", 67},
		border:       cliColor{"#6f685d", 242},
		selection:    cliColor{"#bc9450", 179},
		userBubbleBG: cliColor{"#1b1914", 234},
		diffAddBG:    cliColor{"#1b321e", 22},
		diffDelBG:    cliColor{"#36191a", 52},
		toolRead:     cliColor{"#7295ad", 67},
		toolProc:     cliColor{"#a584b5", 139},
	}
	cliJadeTheme = cliPalette{
		name:         "dark",
		style:        "jade-night",
		background:   cliColor{"#071510", 233},
		surface:      cliColor{"#0a1f17", 234},
		composer:     cliColor{"#06110d", 232},
		strong:       cliColor{"#edfaf5", 255},
		signal:       cliColor{"#d5ad5d", 179},
		accent:       cliColor{"#dc5b5e", 167},
		muted:        cliColor{"#d9ebe4", 253},
		faint:        cliColor{"#76998b", 66},
		subtle:       cliColor{"#acc7bc", 250},
		success:      cliColor{"#65b88f", 79},
		warn:         cliColor{"#d5ad5d", 179},
		err:          cliColor{"#dc5b5e", 167},
		danger:       cliColor{"#e05a60", 167},
		info:         cliColor{"#55bca3", 79},
		secondary:    cliColor{"#55bca3", 79},
		border:       cliColor{"#4f7a68", 65},
		selection:    cliColor{"#d5ad5d", 179},
		userBubbleBG: cliColor{"#0c211a", 234},
		diffAddBG:    cliColor{"#123821", 22},
		diffDelBG:    cliColor{"#38191b", 52},
		toolRead:     cliColor{"#55bca3", 79},
		toolProc:     cliColor{"#9b83b2", 139},
	}
	cliThemeStyles = buildCLIThemeStyleCatalog()
	activeCLITheme = applyCLIThemeStyle(cliDarkTheme, cliThemeStyles[0])
	// activeBackgroundProbe stays inert unless a caller that owns stdin opts in
	// through withTerminalProbe; terminalProbe is what opting in installs.
	activeBackgroundProbe = noTerminalBackground
	terminalProbe         = queryTerminalBackground
)

func noTerminalBackground() (terminalRGB, bool) { return terminalRGB{}, false }

// cliCursorShape is the active cursor shape for the textarea input, configured
// via [ui] cursor_shape. Defaults to the slim bar used by the chat composer.
var cliCursorShape = "bar"

func configureCLITheme(mode string) {
	configureCLIThemeWithStyle(mode, "")
}

func configureCLIThemeWithStyle(mode, style string) {
	if env := strings.TrimSpace(os.Getenv("PATTY_THEME")); env != "" {
		if st, ok := cliThemeStyleByName(env); ok {
			mode = st.mode
			style = st.name
		} else {
			mode = env
		}
	}
	if env := strings.TrimSpace(os.Getenv("PATTY_THEME_STYLE")); env != "" {
		style = env
	}
	activeCLITheme = resolveCLIThemeWithStyle(mode, style)
	refreshCLIStyles()
}

func resolveCLITheme(mode string) cliPalette {
	return resolveCLIThemeWithStyle(mode, "")
}

func resolveCLIThemeWithStyle(mode, style string) cliPalette {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if st, ok := cliThemeStyleByName(mode); ok {
		return buildCLITheme(st.mode, st.name)
	}
	resolvedMode := resolveCLIThemeMode(mode)
	st, ok := cliThemeStyleByName(style)
	if !ok || st.mode != resolvedMode {
		st = defaultCLIThemeStyle(resolvedMode)
	}
	return buildCLITheme(resolvedMode, st.name)
}

func resolveCLIThemeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "light":
		return "light"
	case "dark":
		return "dark"
	case "auto", "":
		if rgb, ok := activeBackgroundProbe(); ok {
			if rgb.looksLight() {
				return "light"
			}
			return "dark"
		}
		if colorFGBGLooksLight() {
			return "light"
		}
		return "dark"
	default:
		return "dark"
	}
}

func buildCLITheme(mode, style string) cliPalette {
	base := cliDarkTheme
	if mode == "light" {
		base = cliLightTheme
	}
	st, ok := cliThemeStyleByName(style)
	if !ok || st.mode != base.name {
		st = defaultCLIThemeStyle(base.name)
	}
	return applyCLIThemeStyle(base, st)
}

func applyCLIThemeStyle(base cliPalette, style cliThemeStyle) cliPalette {
	palette := style.palette
	if palette.background.hex == "" {
		palette = base
	}
	palette.name = style.mode
	palette.style = style.name
	return palette
}

func cliThemeStyleByName(name string) (cliThemeStyle, bool) {
	descriptor, ok := config.ResolveCLIThemeStyle(name)
	if !ok {
		return cliThemeStyle{}, false
	}
	for _, st := range cliThemeStyles {
		if st.name == descriptor.Name {
			return st, true
		}
	}
	return cliThemeStyle{}, false
}

func buildCLIThemeStyleCatalog() []cliThemeStyle {
	palettes := map[string]cliPalette{
		"seoul-night": cliDarkTheme,
		"ink-night":   cliInkTheme,
		"hanji-light": cliLightTheme,
		"jade-night":  cliJadeTheme,
	}
	descriptors := config.CLIThemeStyles()
	styles := make([]cliThemeStyle, 0, len(descriptors))
	for _, descriptor := range descriptors {
		styles = append(styles, cliThemeStyle{
			name: descriptor.Name, mode: descriptor.Mode,
			description: descriptor.Description, palette: palettes[descriptor.Name],
		})
	}
	return styles
}

func defaultCLIThemeStyle(mode string) cliThemeStyle {
	if mode == "light" {
		for _, st := range cliThemeStyles {
			if st.name == "hanji-light" {
				return st
			}
		}
	}
	return cliThemeStyles[0]
}

// withTerminalProbe resolves "auto" against a live OSC 11 query. Probing reads
// stdin in raw mode, so only a caller that owns stdin may opt in; everyone else
// gets the COLORFGBG fallback and never fights the TUI's input reader.
func withTerminalProbe(fn func()) {
	prev := activeBackgroundProbe
	activeBackgroundProbe = terminalProbe
	defer func() { activeBackgroundProbe = prev }()
	fn()
}

func setCLIThemeMode(mode string) cliPalette {
	activeCLITheme = resolveCLIThemeWithStyle(mode, activeCLITheme.style)
	refreshCLIStyles()
	return activeCLITheme
}

func setCLIThemeStyle(name string) (cliPalette, bool) {
	st, ok := cliThemeStyleByName(name)
	if !ok {
		return cliPalette{}, false
	}
	activeCLITheme = resolveCLIThemeWithStyle(st.mode, st.name)
	refreshCLIStyles()
	return activeCLITheme, true
}

type terminalRGB struct {
	r int
	g int
	b int
}

func (c terminalRGB) looksLight() bool {
	luma := 0.2126*float64(c.r) + 0.7152*float64(c.g) + 0.0722*float64(c.b)
	return luma >= 150
}

func parseOSC11Response(s string) (terminalRGB, bool) {
	_, after, ok := strings.Cut(s, "]11;")
	if !ok {
		return terminalRGB{}, false
	}
	payload := after
	if end := strings.IndexByte(payload, '\a'); end >= 0 {
		payload = payload[:end]
	} else if end := strings.Index(payload, "\x1b\\"); end >= 0 {
		payload = payload[:end]
	}
	payload = strings.TrimSpace(payload)
	if strings.HasPrefix(payload, "#") {
		r, g, b, ok := parseHexColor(payload)
		return terminalRGB{r, g, b}, ok
	}
	for _, prefix := range []string{"rgb:", "rgba:"} {
		if after, ok := strings.CutPrefix(payload, prefix); ok {
			return parseOSCColorTriplet(after)
		}
	}
	return terminalRGB{}, false
}

func parseOSCColorTriplet(s string) (terminalRGB, bool) {
	parts := strings.Split(s, "/")
	if len(parts) < 3 {
		return terminalRGB{}, false
	}
	r, okR := parseOSCColorComponent(parts[0])
	g, okG := parseOSCColorComponent(parts[1])
	b, okB := parseOSCColorComponent(parts[2])
	return terminalRGB{r, g, b}, okR && okG && okB
}

func parseOSCColorComponent(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 4 {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0, false
	}
	max := int64(1)<<(4*len(s)) - 1
	if max <= 0 {
		return 0, false
	}
	return int(v * 255 / max), true
}

func colorFGBGLooksLight() bool {
	parts := strings.Split(os.Getenv("COLORFGBG"), ";")
	if len(parts) == 0 {
		return false
	}
	bg, err := strconv.Atoi(parts[len(parts)-1])
	return err == nil && (bg == 7 || bg == 15)
}

func fgSGR(c cliColor) string {
	if trueColorTerminal() {
		if r, g, b, ok := parseHexColor(c.hex); ok {
			return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
		}
	}
	return fmt.Sprintf("\033[38;5;%dm", c.xterm)
}

func bgSGR(c cliColor) string {
	if trueColorTerminal() {
		if r, g, b, ok := parseHexColor(c.hex); ok {
			return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
		}
	}
	return fmt.Sprintf("\033[48;5;%dm", c.xterm)
}

func parseHexColor(hex string) (int, int, int, bool) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	r, errR := strconv.ParseUint(hex[0:2], 16, 8)
	g, errG := strconv.ParseUint(hex[2:4], 16, 8)
	b, errB := strconv.ParseUint(hex[4:6], 16, 8)
	return int(r), int(g), int(b), errR == nil && errG == nil && errB == nil
}

func themeFg(c cliColor, s string) string {
	if !colorOn() {
		return s
	}
	return sgr(fgSGR(c), s)
}

func themedRule(width int, color cliColor) string {
	if width <= 0 {
		return ""
	}
	return themeFg(color, strings.Repeat("─", width))
}

// themeLipColor pre-resolves the fallback rather than handing lipgloss a 24-bit
// value: the bubbletea renderer would otherwise downsample it with the same
// distance metric the hand-chosen xterm indices exist to avoid.
func themeLipColor(c cliColor) color.Color {
	if trueColorTerminal() {
		return lipgloss.Color(c.hex)
	}
	return lipgloss.Color(strconv.Itoa(c.xterm))
}

func themeStyle(c cliColor) lipgloss.Style {
	if !colorOn() {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(themeLipColor(c))
}

func withThemeBorderFG(st lipgloss.Style, c cliColor) lipgloss.Style {
	if !colorOn() {
		return st
	}
	return st.BorderForeground(themeLipColor(c))
}

func init() {
	refreshCLIStyles()
}

func refreshCLIStyles() {
	composerInputStyle = lipgloss.NewStyle()
	selStyle = lipgloss.NewStyle().Reverse(true)
	if colorOn() {
		// The composer is rendered as an ANSI string, not a cell buffer. A
		// full-width background here is unsafe because nested textarea/foreground
		// spans emit resets that would punch holes through the surface. Keep the
		// canvas terminal-native and style only the input text.
		composerInputStyle = composerInputStyle.Foreground(themeLipColor(activeCLITheme.strong))
		selStyle = lipgloss.NewStyle().
			Foreground(themeLipColor(activeCLITheme.background)).
			Background(themeLipColor(activeCLITheme.selection)).
			Bold(true)
	}
	todoPanelStyle = withThemeBorderFG(lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false), activeCLITheme.border).
		PaddingLeft(1)
	statusBlockStyle = themeStyle(activeCLITheme.faint)
	workingStyle = themeStyle(activeCLITheme.faint)
	compSelStyle = themeStyle(activeCLITheme.accent).Bold(true)
	choicePanelStyle = withThemeBorderFG(lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, true, false), activeCLITheme.accent).
		PaddingLeft(1)
	scrollThumbStyle = themeStyle(activeCLITheme.accent)
	scrollTrackStyle = themeStyle(activeCLITheme.faint)
}

func applyTextareaTheme(ti *textarea.Model) {
	plain := lipgloss.NewStyle()
	weak := themeStyle(activeCLITheme.faint)
	placeholder := weak.Faint(true)
	if !colorOn() {
		weak = plain
		placeholder = plain
	}

	styles := ti.Styles()
	styles.Focused = textarea.StyleState{
		Base:             plain,
		Text:             plain,
		CursorLine:       plain,
		CursorLineNumber: weak,
		EndOfBuffer:      weak,
		LineNumber:       weak,
		Placeholder:      placeholder,
		Prompt:           weak,
	}
	styles.Blurred = textarea.StyleState{
		Base:             plain,
		Text:             plain,
		CursorLine:       plain,
		CursorLineNumber: weak,
		EndOfBuffer:      weak,
		LineNumber:       weak,
		Placeholder:      placeholder,
		Prompt:           weak,
	}
	if colorOn() {
		styles.Cursor.Color = themeLipColor(activeCLITheme.accent)
	} else {
		styles.Cursor.Color = nil
	}
	switch cliCursorShape {
	case "block":
		styles.Cursor.Shape = tea.CursorBlock
	case "underline":
		styles.Cursor.Shape = tea.CursorUnderline
	default:
		styles.Cursor.Shape = tea.CursorBar
	}
	ti.SetStyles(styles)
}

func (m *chatTUI) runThemeSubcommand(input string) tea.Cmd {
	args := tokenizeArgs(input)
	if len(args) < 2 {
		m.notice(i18n.M.ThemeHeader + "\n" + describeCLIThemes() + "\n" + i18n.M.ThemeHint)
		return nil
	}
	name := strings.ToLower(args[1])
	previous := activeCLITheme
	var theme cliPalette
	switch name {
	case "auto", "light", "dark":
		theme = setCLIThemeMode(name)
	default:
		next, ok := setCLIThemeStyle(name)
		if !ok {
			m.notice(fmt.Sprintf(i18n.M.ThemeUnknownFmt, name) + "\n" + describeCLIThemes())
			return nil
		}
		theme = next
	}
	m.refreshRuntimeTheme()
	m.notice(fmt.Sprintf(i18n.M.ThemeChangedFmt, theme.name, theme.style))

	// Persist to user config so the choice survives restart.
	m.persistTheme(name)
	return m.startThemeSweep(previous, theme)
}

func (m *chatTUI) persistTheme(inputName string) {
	path, _, saveErr := config.EditUserConfigLocked(func(c *config.Config) error {
		switch inputName {
		case "auto", "light", "dark":
			c.UI.Theme = inputName
			c.UI.ThemeStyle = activeCLITheme.style
		default:
			c.UI.Theme = activeCLITheme.name
			c.UI.ThemeStyle = activeCLITheme.style
		}
		return nil
	})
	if saveErr != nil {
		slog.Warn("theme: failed to persist", "path", path, "err", saveErr)
	}
}

func (m *chatTUI) refreshRuntimeTheme() {
	m.spinner.Style = themeStyle(activeCLITheme.accent)
	applyTextareaTheme(&m.input)
}

func describeCLIThemes() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  auto · light · dark\n", dim("modes:"))
	for _, st := range cliThemeStyles {
		marker := "  "
		if st.name == activeCLITheme.style {
			marker = accent("› ")
		}
		fmt.Fprintf(&b, "%s%-10s %s  %s\n", marker, st.name, dim(st.mode), dim(st.description))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *chatTUI) themeArgItems(val string) ([]compItem, int, bool) {
	cmdEnd := strings.IndexAny(val, " \t")
	if cmdEnd < 0 || canonicalBuiltinSlashCommand(val[:cmdEnd]) != "/theme" {
		return nil, 0, false
	}
	from := strings.LastIndexAny(val, " \t") + 1
	prior := strings.Fields(val[:from])
	if len(prior) != 1 {
		return nil, from, true
	}
	cur := strings.ToLower(val[from:])
	items := []struct {
		label string
		mode  string
		desc  string
	}{
		{label: "auto", mode: "mode", desc: "detect terminal background"},
		{label: "light", mode: "mode", desc: "force light shell"},
		{label: "dark", mode: "mode", desc: "force dark shell"},
	}
	var out []compItem
	for _, it := range items {
		if cur != "" && !strings.HasPrefix(it.label, cur) {
			continue
		}
		out = append(out, compItem{label: it.label, insert: it.label, hint: it.mode + " · " + it.desc})
	}
	for _, st := range cliThemeStyles {
		if cur != "" && !strings.HasPrefix(st.name, cur) {
			continue
		}
		hint := st.mode + " · " + st.description
		if st.name == activeCLITheme.style {
			hint = i18n.M.ArgThemeCurrent
		}
		out = append(out, compItem{label: st.name, insert: st.name, hint: hint})
	}
	return out, from, true
}
