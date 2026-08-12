package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/event"
)

func enableKittyTmuxHangulCompatibility(
	m *chatTUI,
	inputSource string,
	sample func() bool,
) *physicalBackspaceMonitor {
	m.kittyHangulIMECompatibility = true
	m.kittyHangulIMEBehindTmux = true
	m.keyboardInputSourceID = func() string { return inputSource }
	monitor := newPhysicalBackspaceMonitor(sample, time.Hour)
	m.physicalBackspaceMonitor = monitor
	return monitor
}

func TestComposerBackspaceDeletesPreviousGraphemeCluster(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	value := "\u1112\u1161\u11ABx" // decomposed 한 + x
	m.input.SetValue(value)
	m.setComposerCursor(3)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.input.Value(); got != "x" {
		t.Fatalf("backspace deleted %q, want only x left", got)
	}
	if got := m.input.Column(); got != 0 {
		t.Fatalf("cursor column after grapheme backspace = %d, want 0", got)
	}
}

func TestComposerBackspaceDeletesStandaloneHangulJamoAtCursor(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("안ㄴ")
	m.setComposerCursor(len([]rune("안ㄴ")))

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.input.Value(); got != "안" {
		t.Fatalf("backspace over standalone Hangul jamo produced %q, want %q", got, "안")
	}
	if got, want := m.composerCursorOffset(), len([]rune("안")); got != want {
		t.Fatalf("cursor offset after deleting standalone jamo = %d, want %d", got, want)
	}
}

func TestComposerBackspaceClearsStaleWideCellWhenDisplayShrinks(t *testing.T) {
	prev := clearWideInputChanges
	clearWideInputChanges = false
	defer func() { clearWideInputChanges = prev }()

	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("안ㄴ")
	m.setComposerCursor(len([]rune("안ㄴ")))

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = next.(chatTUI)

	if got := m.input.Value(); got != "안" {
		t.Fatalf("backspace over standalone Hangul jamo produced %q, want %q", got, "안")
	}
	if cmd == nil {
		t.Fatal("shrinking a wide-character input must clear stale terminal cells on every platform")
	}
}

func TestComposerCode8BackspaceDeletesStandaloneHangulJamoAtCursor(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("안ㄴ")
	m.setComposerCursor(len([]rune("안ㄴ")))

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 8, Text: "\b"})

	if got := m.input.Value(); got != "안" {
		t.Fatalf("code-8 backspace over standalone Hangul jamo produced %q, want %q", got, "안")
	}
}

func TestComposerBackspaceAfterClickInsideHangulJamoDeletesIt(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("안ㄴ")
	x, y, ok := m.composerOrigin()
	if !ok {
		t.Fatal("composer should expose origin")
	}

	// Cell positions in the textarea content: 안 occupies columns 0-1 and ㄴ
	// occupies columns 2-3. Click the right half of ㄴ to model the cursor being
	// visually inside the wide jamo.
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.input.Value(); got != "안" {
		t.Fatalf("backspace after click inside Hangul jamo produced %q, want %q", got, "안")
	}
}

func TestComposerWordDeleteBackspaceBindingsKeepWordSemantics(t *testing.T) {
	// alt+backspace / ctrl+backspace must reach the textarea word binding, not
	// the grapheme-delete path: the raw-code backspace match is command-gated.
	for _, msg := range []tea.KeyPressMsg{
		{Code: tea.KeyBackspace, Mod: tea.ModAlt},
		{Code: tea.KeyBackspace, Mod: tea.ModCtrl},
	} {
		m := newComposerMouseTestTUI(t, 40, 12)
		m.input.SetValue("안녕하세요 world")
		m.setComposerCursor(len([]rune("안녕하세요 world")))
		m = updateComposerMouseTestTUI(t, m, msg)
		if got, want := m.input.Value(), "안녕하세요 "; got != want {
			t.Fatalf("%+v produced %q, want word-delete to leave %q", msg, got, want)
		}
	}
}

func TestKittyTmuxBackspaceCodedResidualReachesJamoCancellation(t *testing.T) {
	// A Backspace-coded event preserved the physical key identity, so even
	// behind tmux it must cancel the jamo instead of being dropped like the
	// legacy text-only residual.
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return true })
	m.input.SetValue("안")
	m.setComposerCursor(1)
	monitor.observe(true)
	monitor.observe(false)
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace, Text: "ㄴ"})
	if got := m.input.Value(); got != "아" {
		t.Fatalf("tmux composition backspace produced %q, want %q", got, "아")
	}
}

func TestComposerBackspaceOverCommittedHangulDeletesWholeSyllable(t *testing.T) {
	// A Backspace without associated text operates on committed text: one
	// stroke deletes the whole syllable, never the IME jamo steps.
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("안녕")
	m.setComposerCursor(len([]rune("안녕")))

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := m.input.Value(); got != "안" {
		t.Fatalf("backspace over committed Hangul produced %q, want %q", got, "안")
	}
	if got, want := m.composerCursorOffset(), len([]rune("안")); got != want {
		t.Fatalf("cursor offset after committed backspace = %d, want %d", got, want)
	}
}

func TestComposerBackspaceWithAssociatedTextDecomposesHangulSyllable(t *testing.T) {
	// Kitty protocol mode attaches the live preedit to the Backspace key as
	// associated text; each stroke then cancels one jamo, IME style.
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("안녕")
	m.setComposerCursor(len([]rune("안녕")))

	for _, want := range []string{"안녀", "안ㄴ", "안"} {
		m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace, Text: "안녀"})
		if got := m.input.Value(); got != want {
			t.Fatalf("composition backspace step produced %q, want %q", got, want)
		}
		if got, wantOffset := m.composerCursorOffset(), len([]rune(want)); got != wantOffset {
			t.Fatalf("cursor offset after value %q = %d, want %d", want, got, wantOffset)
		}
	}
}

func TestIMETraceRecordsKeyAndComposerTransitionWhenEnabled(t *testing.T) {
	var log bytes.Buffer
	m := newComposerMouseTestTUI(t, 40, 12)
	m.imeTraceMode = imeTraceFull
	m.diagnostics = &tuiDiagnostics{writer: &log}
	m.input.SetValue("안ㄴ")
	m.setComposerCursor(len([]rune("안ㄴ")))

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	got := log.String()
	for _, want := range []string{
		`ime_input`,
		`key="backspace"`,
		`code=127`,
		`text=""`,
		`handled=true`,
		`before="안ㄴ"`,
		`before_cursor=0:2`,
		`after="안"`,
		`after_cursor=0:1`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("IME trace %q does not contain %q", got, want)
		}
	}
}

func TestIMETraceRecordsOnlyComposerShapeWhenRequested(t *testing.T) {
	var log bytes.Buffer
	m := newComposerMouseTestTUI(t, 40, 12)
	m.imeTraceMode = imeTraceShape
	m.diagnostics = &tuiDiagnostics{writer: &log}
	m.input.SetValue("안ㄴ")
	m.setComposerCursor(len([]rune("안ㄴ")))

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	got := log.String()
	for _, want := range []string{
		`ime_shape`,
		`code=127`,
		`handled=true`,
		`before_runes=2`,
		`before_cursor=0:2`,
		`after_runes=1`,
		`after_cursor=0:1`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("automatic IME shape trace %q does not contain %q", got, want)
		}
	}
	for _, sensitive := range []string{`before=`, `after=`, "안", "ㄴ"} {
		if strings.Contains(got, sensitive) {
			t.Fatalf("automatic IME shape trace leaked %q in %q", sensitive, got)
		}
	}
}

func TestIMETraceIsDisabledByDefault(t *testing.T) {
	var log bytes.Buffer
	m := newComposerMouseTestTUI(t, 40, 12)
	m.diagnostics = &tuiDiagnostics{writer: &log}
	m.input.SetValue("안ㄴ")
	m.setComposerCursor(len([]rune("안ㄴ")))

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	if got := log.String(); strings.Contains(got, "ime_input") || strings.Contains(got, "ime_shape") {
		t.Fatalf("disabled IME trace wrote an IME event: %q", got)
	}
}

func TestConfiguredIMETraceMode(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  imeTraceMode
	}{
		{value: "", want: imeTraceDisabled},
		{value: "0", want: imeTraceDisabled},
		{value: "shape", want: imeTraceShape},
		{value: "1", want: imeTraceFull},
		{value: "true", want: imeTraceFull},
		{value: "full", want: imeTraceFull},
	} {
		if got := configuredIMETraceMode(func(string) string { return tc.value }); got != tc.want {
			t.Fatalf("configuredIMETraceMode(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestComposerDeleteRemovesNextGraphemeCluster(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	value := "x\u1112\u1161\u11ABy" // x + decomposed 한 + y
	m.input.SetValue(value)
	m.setComposerCursor(1)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyDelete})

	if got := m.input.Value(); got != "xy" {
		t.Fatalf("delete removed %q, want xy", got)
	}
	if got := m.input.Column(); got != 1 {
		t.Fatalf("cursor column after grapheme delete = %d, want 1", got)
	}
}

func TestComposerArrowsMoveByGraphemeCluster(t *testing.T) {
	m := newComposerMouseTestTUI(t, 60, 12)
	value := "a👨‍👩‍👧‍👦b"
	m.input.SetValue(value)
	m.setComposerCursor(len([]rune("a👨‍👩‍👧‍👦")))

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if got, want := m.input.Column(), 1; got != want {
		t.Fatalf("left moved cursor to rune column %d, want cluster start %d", got, want)
	}

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if got, want := m.input.Column(), len([]rune("a👨‍👩‍👧‍👦")); got != want {
		t.Fatalf("right moved cursor to rune column %d, want cluster end %d", got, want)
	}
}

func TestComposerTypedTextNormalizesToNFC(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: '\u1112', Text: "\u1112\u1161\u11AB"})

	if got := m.input.Value(); got != "한" {
		t.Fatalf("typed decomposed Hangul stored %q, want NFC 한", got)
	}
	if got := m.input.Column(); got != 1 {
		t.Fatalf("cursor column after normalized insert = %d, want 1", got)
	}
}

func TestKittyCompositionBackspaceCancelsLastJamo(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMECompatibility = true

	// Kitty's associated-text reporting delivers the composition Backspace
	// with the residual jamo attached; the app must cancel that jamo from the
	// preedit instead of dropping the key or inserting the residual as text.
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: '안', Text: "안"})
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace, Text: "ㄴ"})

	if got := m.input.Value(); got != "아" {
		t.Fatalf("Kitty composition backspace produced %q, want %q", got, "아")
	}
	if rendered := ansi.Strip(m.renderComposerInput()); strings.Contains(rendered, "ㄴ") {
		t.Fatalf("Kitty composition backspace left residual jamo in rendered composer: %q", rendered)
	}
}

func TestKittyAssociatedTextSequencePreservesBackspaceIdentity(t *testing.T) {
	var decoder uv.EventDecoder
	n, event := decoder.Decode([]byte("\x1b[127;1;12596u")) // Backspace with associated text ㄴ.
	if n != len("\x1b[127;1;12596u") {
		t.Fatalf("decoded %d bytes, want %d", n, len("\x1b[127;1;12596u"))
	}
	keyEvent, ok := event.(uv.KeyPressEvent)
	if !ok {
		t.Fatalf("decoded event type = %T, want ultraviolet.KeyPressEvent", event)
	}
	key := tea.KeyPressMsg(keyEvent)
	if key.Code != tea.KeyBackspace || key.Text != "ㄴ" {
		t.Fatalf("decoded key = %+v, want Backspace with associated text ㄴ", key)
	}

	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMECompatibility = true
	m.input.SetValue("안")
	m.setComposerCursor(1)
	m = updateComposerMouseTestTUI(t, m, key)

	if got := m.input.Value(); got != "아" {
		t.Fatalf("parsed Kitty Backspace replay produced %q, want %q", got, "아")
	}
}

func TestKittyTmuxSuppressesJamoAfterPhysicalBackspaceWasReleased(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return false })
	m.input.SetValue("안")
	m.setComposerCursor(1)
	monitor.observe(true)
	monitor.observe(false)

	// In Kitty's legacy stream the final Backspace is replaced by the residual
	// pre-edit jamo. Reproduce transport delay by releasing Backspace before the
	// terminal event reaches Update; the recorded down edge must survive it.
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})

	if got := m.input.Value(); got != "안" {
		t.Fatalf("physical Backspace residual produced %q, want 안", got)
	}
}

func TestKittyTmuxSuppressesDecodedLegacyJamoAfterBackspaceRelease(t *testing.T) {
	var decoder uv.EventDecoder
	raw := []byte("ㄴ")
	n, event := decoder.Decode(raw)
	if n != len(raw) {
		t.Fatalf("decoded %d bytes, want %d", n, len(raw))
	}
	keyEvent, ok := event.(uv.KeyPressEvent)
	if !ok {
		t.Fatalf("legacy PTY event type = %T, want ultraviolet.KeyPressEvent", event)
	}

	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return false })
	m.input.SetValue("안")
	m.setComposerCursor(1)
	monitor.observe(true)
	monitor.observe(false)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg(keyEvent))
	if got := m.input.Value(); got != "안" {
		t.Fatalf("decoded Kitty/tmux legacy replay produced %q, want 안", got)
	}
}

func TestKittyTmuxSuppressesJamoWhilePhysicalBackspaceIsStillDown(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return true })

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})

	if got := m.input.Value(); got != "" {
		t.Fatalf("live physical Backspace residual produced %q, want empty input", got)
	}
}

func TestKittyTmuxDoesNotSuppressPhysicalBackspaceJamoOutsideKoreanInput(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := enableKittyTmuxHangulCompatibility(&m, "com.apple.keylayout.ABC", func() bool { return false })
	m.input.SetValue("안")
	m.setComposerCursor(1)
	monitor.observe(true)
	monitor.observe(false)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})

	if got := m.input.Value(); got != "안ㄴ" {
		t.Fatalf("non-Korean input source produced %q, want 안ㄴ", got)
	}
}

func TestKittyMacOSCompatibilityPreservesChosungInput(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMECompatibility = true

	for range 2 {
		m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㅇ', Text: "ㅇ"})
	}

	if got := m.input.Value(); got != "ㅇㅇ" {
		t.Fatalf("Kitty compatibility changed first-class chosung input to %q", got)
	}
}

func TestKittyMacOSCompatibilityPreservesStandaloneJamoAfterSyllable(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return false })
	m.input.SetValue("안")
	m.setComposerCursor(1)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})

	if got := m.input.Value(); got != "안ㄴ" {
		t.Fatalf("Kitty compatibility erased deliberate standalone jamo: %q", got)
	}
}

func TestKittyTmuxDoesNotReuseSuppressedBackspaceForIntentionalJamo(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	down := true
	monitor := enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return down })

	// Update sees the live level before the polling goroutine records the edge.
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})
	monitor.observe(true)
	down = false
	monitor.observe(false)

	// The monitor's late edge belongs to the already-suppressed Backspace. It
	// must not erase the next deliberate two-set jamo.
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})
	if got := m.input.Value(); got != "ㄴ" {
		t.Fatalf("late Backspace edge changed deliberate jamo to %q, want ㄴ", got)
	}
}

func TestKittyTmuxDoesNotSuppressJamoFromExpiredBackspaceEvidence(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return false })
	monitor.observe(true)
	monitor.observe(false)
	monitor.lastPressUnixNano.Store(time.Now().Add(-physicalBackspaceEvidenceTTL - time.Second).UnixNano())

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})
	if got := m.input.Value(); got != "ㄴ" {
		t.Fatalf("expired Backspace evidence changed deliberate jamo to %q, want ㄴ", got)
	}
}

func TestPasteConsumesUnmatchedPhysicalBackspaceEvidence(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return false })
	monitor.observe(true)
	monitor.observe(false)

	m = updateComposerMouseTestTUI(t, m, tea.PasteMsg{Content: "붙여넣기"})
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})
	if got := m.input.Value(); got != "붙여넣기ㄴ" {
		t.Fatalf("paste left stale Backspace evidence, composer = %q", got)
	}
}

func TestChooserFreeTextSuppressesDelayedKittyTmuxResidualJamo(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := enableKittyTmuxHangulCompatibility(&m, korean2SetInputSourceID, func() bool { return false })
	m.chooser = newChooser(event.Ask{
		ID: "ask-1",
		Questions: []event.AskQuestion{{
			ID: "q1", Prompt: "답변", Options: []event.AskOption{{Label: "선택"}},
		}},
	})
	m.chooser.typing = true
	m.input.SetValue("안")
	m.setComposerCursor(1)
	monitor.observe(true)
	monitor.observe(false)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})
	if got := m.input.Value(); got != "안" {
		t.Fatalf("chooser residual replay produced %q, want 안", got)
	}
}

func TestKittyMacOSViewRequestsLosslessKeyboardIdentity(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMECompatibility = true

	view := m.View()
	if !view.ReportFocus {
		t.Fatal("chat view must report focus so global Backspace capture pauses while blurred")
	}
	if !view.KeyboardEnhancements.ReportAllKeysAsEscapeCodes || !view.KeyboardEnhancements.ReportAssociatedText {
		t.Fatalf("Kitty compatibility keyboard enhancements = %+v", view.KeyboardEnhancements)
	}

	m.kittyHangulIMEBehindTmux = true
	view = m.View()
	if view.KeyboardEnhancements.ReportAllKeysAsEscapeCodes || view.KeyboardEnhancements.ReportAssociatedText {
		t.Fatalf("tmux view requested enhancements whose associated text tmux discards: %+v", view.KeyboardEnhancements)
	}

	m.kittyHangulIMECompatibility = false
	view = m.View()
	if view.KeyboardEnhancements.ReportAllKeysAsEscapeCodes || view.KeyboardEnhancements.ReportAssociatedText {
		t.Fatalf("non-Kitty view unexpectedly requested compatibility enhancements: %+v", view.KeyboardEnhancements)
	}
}

func TestOtherTerminalsKeepStandaloneJamoAfterHangulSyllable(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMECompatibility = false
	m.input.SetValue("안")
	m.setComposerCursor(1)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})

	if got := m.input.Value(); got != "안ㄴ" {
		t.Fatalf("normal terminal input produced %q, want %q", got, "안ㄴ")
	}
}

func TestKittyHangulIMECompatibilityDetection(t *testing.T) {
	tests := []struct {
		name         string
		goos         string
		stdinTTY     bool
		env          map[string]string
		clientTerm   string
		want         bool
		wantTmux     bool
		undetermined bool
	}{
		{
			name: "direct Kitty on macOS", goos: "darwin", stdinTTY: true,
			env: map[string]string{"TERM": "xterm-kitty"}, want: true,
		},
		{
			name: "Kitty client behind tmux with stale outer metadata", goos: "darwin", stdinTTY: true,
			env:        map[string]string{"TERM": "tmux-256color", "TERM_PROGRAM": "WarpTerminal", "TMUX": "/tmp/tmux"},
			clientTerm: "xterm-kitty", want: true, wantTmux: true,
		},
		{
			name: "non-Kitty tmux client overrides stale Kitty pane metadata", goos: "darwin", stdinTTY: true,
			env: map[string]string{
				"TERM": "tmux-256color", "TERM_PROGRAM": "kitty", "KITTY_WINDOW_ID": "7", "TMUX": "/tmp/tmux",
			},
			clientTerm: "xterm-ghostty", want: false, wantTmux: false,
		},
		{
			name: "tmux client probe failure is not a negative detection", goos: "darwin", stdinTTY: true,
			env:          map[string]string{"TERM": "tmux-256color", "TMUX": "/tmp/tmux"},
			undetermined: true,
		},
		{
			name: "non-Kitty macOS terminal", goos: "darwin", stdinTTY: true,
			env: map[string]string{"TERM": "xterm-256color"}, want: false,
		},
		{
			name: "Linux Kitty remains native", goos: "linux", stdinTTY: true,
			env: map[string]string{"TERM": "xterm-kitty"}, want: false,
		},
		{
			name: "headless process cannot own IME", goos: "darwin", stdinTTY: false,
			env: map[string]string{"TERM": "xterm-kitty"}, want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, gotTmux, determined := kittyHangulIMEEnvironmentFor(
				tc.goos,
				tc.stdinTTY,
				func(key string) string { return tc.env[key] },
				func() string { return tc.clientTerm },
			)
			if got != tc.want {
				t.Fatalf("compatibility detection = %v, want %v", got, tc.want)
			}
			if gotTmux != tc.wantTmux {
				t.Fatalf("tmux detection = %v, want %v", gotTmux, tc.wantTmux)
			}
			if determined == tc.undetermined {
				t.Fatalf("determined = %v, want %v", determined, !tc.undetermined)
			}
		})
	}
}
