package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"

	"patty/internal/secrets"
)

const (
	imeTraceEnv             = "PATTY_IME_TRACE"
	korean2SetInputSourceID = "com.apple.inputmethod.Korean.2SetKorean"
)

type imeTraceMode uint8

const (
	imeTraceDisabled imeTraceMode = iota
	imeTraceShape
	imeTraceFull
)

func configuredIMETraceMode(getenv func(string) string) imeTraceMode {
	switch strings.ToLower(strings.TrimSpace(getenv(imeTraceEnv))) {
	case "1", "true", "full":
		return imeTraceFull
	case "shape":
		return imeTraceShape
	default:
		return imeTraceDisabled
	}
}

var (
	kittyCompatibilityOnce       sync.Once
	kittyCompatibility           bool
	kittyCompatibilityBehindTmux bool
)

func detectKittyHangulIMEEnvironment() (compatible, behindTmux bool) {
	kittyCompatibilityOnce.Do(func() {
		kittyCompatibility, kittyCompatibilityBehindTmux, _ = probeKittyHangulIMEEnvironment()
	})
	return kittyCompatibility, kittyCompatibilityBehindTmux
}

type kittyHangulIMEEnvironmentMsg struct {
	compatible bool
	behindTmux bool
	generation uint64
	determined bool
}

func probeKittyHangulIMEEnvironment() (compatible, behindTmux, determined bool) {
	return kittyHangulIMEEnvironmentFor(
		runtime.GOOS,
		term.IsTerminal(int(os.Stdin.Fd())),
		os.Getenv,
		currentTmuxClientTerminal,
	)
}

func currentTmuxClientTerminal() string {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-p", "#{client_termname}")
	cmd.Env = secrets.ProcessEnv()
	clientTerm, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(clientTerm)
}

func refreshKittyHangulIMEEnvironment(generation uint64) tea.Cmd {
	return func() tea.Msg {
		compatible, behindTmux, determined := probeKittyHangulIMEEnvironment()
		return kittyHangulIMEEnvironmentMsg{
			compatible: compatible,
			behindTmux: behindTmux,
			generation: generation,
			determined: determined,
		}
	}
}

func kittyHangulIMEEnvironmentFor(
	goos string,
	stdinTTY bool,
	getenv func(string) string,
	tmuxClientTerm func() string,
) (compatible, behindTmux, determined bool) {
	if goos != "darwin" || !stdinTTY {
		return false, false, true
	}
	insideTmux := getenv("TMUX") != ""
	if insideTmux {
		// Pane environment survives detach/reattach. The current tmux client is
		// authoritative; inherited TERM_PROGRAM/KITTY_WINDOW_ID may be stale.
		clientTerm := strings.TrimSpace(tmuxClientTerm())
		if clientTerm == "" {
			return false, false, false
		}
		isKitty := strings.Contains(strings.ToLower(clientTerm), "kitty")
		return isKitty, isKitty, true
	}
	for _, value := range []string{
		getenv("TERM_PROGRAM"),
		getenv("TERM"),
		getenv("KITTY_WINDOW_ID"),
	} {
		if strings.Contains(strings.ToLower(value), "kitty") {
			return true, false, true
		}
	}
	return false, false, true
}

func (m *chatTUI) shouldSuppressKittyHangulResidualCommit(msg tea.KeyPressMsg, physicalBackspaceEvidence bool) bool {
	if !m.kittyHangulIMECompatibility || m.validComposerSelection() {
		return false
	}
	if !isKittyHangulResidualCandidate(msg) {
		return false
	}
	// A Backspace-coded event preserved the physical key identity, so the
	// terminal handed the composition to the app: let the composer cancel the
	// jamo. Only text-only legacy residuals are suppressed behind tmux.
	if msg.Key().Code == tea.KeyBackspace {
		return false
	}
	return m.kittyHangulIMEBehindTmux && physicalBackspaceEvidence &&
		m.keyboardInputSourceID != nil &&
		isKoreanKeyboardInputSource(m.keyboardInputSourceID())
}

func applyKittyHangulKeyboardEnhancements(view *tea.View, enabled bool) {
	if view == nil || !enabled {
		return
	}
	// Kitty normally emits only the IME-produced text, losing the physical key
	// identity when macOS clears a Hangul pre-edit with Backspace. Associated-text
	// reporting keeps both: Backspace with text "ㄴ" is not a deliberate commit.
	view.KeyboardEnhancements.ReportAllKeysAsEscapeCodes = true
	view.KeyboardEnhancements.ReportAssociatedText = true
}

// traceComposerInputTransition records the terminal event and the textarea
// state on both sides of an edit. It is deliberately opt-in because composer
// contents may be sensitive; the trace is written only to the existing private,
// bounded TUI diagnostic log.
func (m *chatTUI) traceComposerInputTransition(
	msg tea.Msg,
	before string,
	beforeLine, beforeColumn int,
	handled bool,
) {
	if m.diagnostics == nil || m.imeTraceMode == imeTraceDisabled {
		return
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	key := keyMsg.Key()
	if m.imeTraceMode == imeTraceShape {
		if !composerBackspaceKey(keyMsg, m.input.KeyMap) && !containsHangul(key.Text) {
			return
		}
		_, _ = fmt.Fprintf(
			m.diagnostics.Writer(),
			"ime_shape t=%s code=%d text_runes=%d mod=%d shifted=%d base=%d repeat=%t handled=%t before_runes=%d before_cursor=%d:%d after_runes=%d after_cursor=%d:%d\n",
			m.diagnostics.now().UTC().Format("15:04:05.000000000"),
			key.Code,
			utf8.RuneCountInString(key.Text),
			key.Mod,
			key.ShiftedCode,
			key.BaseCode,
			key.IsRepeat,
			handled,
			utf8.RuneCountInString(before),
			beforeLine,
			beforeColumn,
			utf8.RuneCountInString(m.input.Value()),
			m.input.Line(),
			m.input.Column(),
		)
		return
	}
	_, _ = fmt.Fprintf(
		m.diagnostics.Writer(),
		"ime_input t=%s key=%q code=%d text=%q mod=%d shifted=%d base=%d repeat=%t handled=%t before=%q before_cursor=%d:%d after=%q after_cursor=%d:%d\n",
		m.diagnostics.now().UTC().Format("15:04:05.000000000"),
		keyMsg.String(),
		key.Code,
		key.Text,
		key.Mod,
		key.ShiftedCode,
		key.BaseCode,
		key.IsRepeat,
		handled,
		before,
		beforeLine,
		beforeColumn,
		m.input.Value(),
		m.input.Line(),
		m.input.Column(),
	)
}

func containsHangul(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Hangul) {
			return true
		}
	}
	return false
}
