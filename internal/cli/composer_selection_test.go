package cli

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/i18n"
)

func newComposerMouseTestTUI(t *testing.T, width, height int) chatTUI {
	t.Helper()
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), width)
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(chatTUI)
}

func updateComposerMouseTestTUI(t *testing.T, m chatTUI, msg tea.Msg) chatTUI {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(chatTUI)
}

func overflowingComposerMouseTestTUI(t *testing.T) chatTUI {
	t.Helper()
	m := newComposerMouseTestTUI(t, 50, 18)
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = "composer-line-" + strconv.Itoa(i)
	}
	m.input.SetValue(strings.Join(lines, "\n"))
	return updateComposerMouseTestTUI(t, m, tea.WindowSizeMsg{Width: 50, Height: 18})
}

func TestComposerWheelScrollsViewWithoutMovingInsertionCursor(t *testing.T) {
	m := overflowingComposerMouseTestTUI(t)
	x, y, ok := m.composerOrigin()
	if !ok {
		t.Fatal("overflowing composer should expose a mouse origin")
	}
	inputOffset := m.input.ScrollYOffset()
	row, column := m.input.Line(), m.input.Column()
	transcriptOffset := m.viewport.YOffset()

	m = updateComposerMouseTestTUI(t, m, tea.MouseWheelMsg{
		X: x, Y: y, Button: tea.MouseWheelUp,
	})

	if !m.composerScrollDetached {
		t.Fatal("wheel over overflowing composer should detach its visible viewport")
	}
	if got, want := m.composerViewOffset(), max(inputOffset-composerWheelRows, 0); got != want {
		t.Fatalf("composer view offset = %d, want %d", got, want)
	}
	if got := m.input.ScrollYOffset(); got != inputOffset {
		t.Fatalf("wheel moved textarea-owned offset to %d, want unchanged %d", got, inputOffset)
	}
	if m.input.Line() != row || m.input.Column() != column {
		t.Fatalf("wheel moved insertion cursor to (%d,%d), want (%d,%d)", m.input.Line(), m.input.Column(), row, column)
	}
	if got := m.viewport.YOffset(); got != transcriptOffset {
		t.Fatalf("composer wheel also moved transcript to %d, want %d", got, transcriptOffset)
	}
	firstVisible := strings.TrimSpace(strings.Split(ansi.Strip(m.renderComposerInput()), "\n")[0])
	wantFirst := "composer-line-" + strconv.Itoa(m.composerViewOffset())
	if firstVisible != wantFirst {
		t.Fatalf("first visible composer row = %q, want %q", firstVisible, wantFirst)
	}
	if cur := m.composerCursor(); cur != nil {
		t.Fatalf("cursor scrolled outside the composer should be hidden, got %+v", cur)
	}
}

func TestComposerMouseRegionIncludesWrappedHintRows(t *testing.T) {
	m := newComposerMouseTestTUI(t, 34, 16)
	_, contentY, ok := m.composerOrigin()
	if !ok {
		t.Fatal("composer should expose a mouse origin")
	}
	if hintRows := m.composerHintRowCount(m.width); hintRows < 2 {
		t.Fatalf("test width produced %d hint rows, want wrapped hints", hintRows)
	}
	lastHintY := contentY + m.composerRowCount() - 2
	if !m.mouseOverComposer(2, lastHintY) {
		t.Fatalf("last hint row y=%d is outside composer mouse region", lastHintY)
	}
}

func TestComposerTypingRestoresCaretFollowingAfterWheel(t *testing.T) {
	m := overflowingComposerMouseTestTUI(t)
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseWheelMsg{
		X: x, Y: y, Button: tea.MouseWheelUp,
	})
	if !m.composerScrollDetached {
		t.Fatal("test setup should detach the composer viewport")
	}

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
	if m.composerScrollDetached {
		t.Fatal("typing should restore caret-following composer viewport")
	}
	if got, want := m.composerViewOffset(), m.input.ScrollYOffset(); got != want {
		t.Fatalf("reattached composer offset = %d, want textarea offset %d", got, want)
	}
	if !strings.HasSuffix(m.input.Value(), "x") {
		t.Fatalf("typing after wheel did not edit at insertion cursor: %q", m.input.Value())
	}
	if cur := m.composerCursor(); cur == nil {
		t.Fatal("caret-following should make the real cursor visible again")
	}
}

func TestComposerWheelChainsToTranscriptAtInternalBoundary(t *testing.T) {
	m := overflowingComposerMouseTestTUI(t)
	notice := agentEventMsg(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: "line"})
	for range 40 {
		m = updateComposerMouseTestTUI(t, m, notice)
	}
	if !m.viewport.AtBottom() {
		t.Fatal("test transcript should start at bottom")
	}
	x, y, _ := m.composerOrigin()
	wheelUp := tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp}
	for m.composerViewOffset() > 0 {
		m = updateComposerMouseTestTUI(t, m, wheelUp)
	}
	bottom := m.viewport.YOffset()
	if !m.viewport.AtBottom() {
		t.Fatal("scrolling within composer should not move transcript before the boundary")
	}

	m = updateComposerMouseTestTUI(t, m, wheelUp)
	if got, want := m.viewport.YOffset(), bottom-composerWheelRows; got != want {
		t.Fatalf("wheel at composer top chained transcript to %d, want %d", got, want)
	}
}

func TestComposerMouseClickMovesCursorAcrossWideRunes(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("한글abc")
	x, y, ok := m.composerOrigin()
	if !ok {
		t.Fatal("composer should expose a mouse origin while visible")
	}

	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	if got := m.input.Column(); got != 2 {
		t.Fatalf("cursor column = %d, want 2 after two wide runes", got)
	}
	if m.composerSel.active {
		t.Fatal("a plain click should position the cursor without leaving a selection")
	}
}

func TestComposerMouseLayoutRoundTripsTextareaCursor(t *testing.T) {
	cases := []struct {
		width int
		value string
	}{
		{40, ""},
		{20, "hello world and wrapped words"},
		{16, "1234567890기능"},
		{18, "alpha  beta\n기능정리검토 mixed\n\nlast"},
		{18, "one\ntwo\nthree\nfour\nfive\nsix\nseven"},
	}
	for _, tc := range cases {
		m := newComposerMouseTestTUI(t, tc.width, 14)
		m.input.SetValue(tc.value)
		for offset := 0; offset <= len([]rune(tc.value)); offset++ {
			m.setComposerCursor(offset)
			local := m.input.Cursor()
			x, y, ok := m.composerOrigin()
			if !ok || local == nil {
				t.Fatalf("value %q offset %d has no composer cursor", tc.value, offset)
			}
			caret, ok := m.composerCaretAt(x+local.X-composerPromptWidth, y+local.Y, false)
			if !ok || caret.offset != offset {
				t.Fatalf("value %q offset %d round-tripped to %+v (ok=%v)", tc.value, offset, caret, ok)
			}
		}
	}
}

func TestComposerMouseDragSelectsAndTypingReplaces(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("한글abc")
	x, y, _ := m.composerOrigin()

	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 4, Y: y, Button: tea.MouseLeft})

	if got := m.selectedComposerText(); got != "한글" {
		t.Fatalf("selected composer text = %q, want %q", got, "한글")
	}
	highlighted := m.renderComposerInput()
	if !strings.Contains(highlighted, selStyle.Render("한글")) {
		t.Fatalf("rendered composer should highlight exactly the selected wide runes: %q", highlighted)
	}

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'X', Text: "X"})
	if got := m.input.Value(); got != "Xabc" {
		t.Fatalf("typing over selection produced %q, want %q", got, "Xabc")
	}
	if m.composerSel.active {
		t.Fatal("typing over a selection should clear the selection")
	}
}

func TestComposerMouseBackwardDragKeepsLogicalSelection(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("alpha beta")
	x, y, _ := m.composerOrigin()

	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 10, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 6, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 6, Y: y, Button: tea.MouseLeft})
	if got := m.selectedComposerText(); got != "beta" {
		t.Fatalf("backward drag selected %q, want %q", got, "beta")
	}
}

func TestComposerMouseSelectionSnapsToGraphemeClusters(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("e\u0301x 👨‍👩‍👧‍👦z")
	x, y, _ := m.composerOrigin()

	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if got := m.selectedComposerText(); got != "e\u0301" {
		t.Fatalf("combining grapheme selection = %q, want %q", got, "e\u0301")
	}

	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 5, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 5, Y: y, Button: tea.MouseLeft})
	if got := m.selectedComposerText(); got != "👨‍👩‍👧‍👦" {
		t.Fatalf("emoji grapheme selection = %q, want family emoji", got)
	}
}

func TestComposerSelectionTracksSoftWrapAndNewlines(t *testing.T) {
	m := newComposerMouseTestTUI(t, 16, 14)
	m.input.SetValue("1234567890기능\nsecond")
	x, y, _ := m.composerOrigin()

	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 10, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 3, Y: y + 2, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 3, Y: y + 2, Button: tea.MouseLeft})

	if got, want := m.selectedComposerText(), "기능\nsec"; got != want {
		t.Fatalf("wrapped multi-line selection = %q, want %q", got, want)
	}

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got, want := m.input.Value(), "1234567890ond"; got != want {
		t.Fatalf("backspace over wrapped selection = %q, want %q", got, want)
	}
}

func TestComposerSelectionPasteAndCopyTakePrecedence(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("alpha beta")
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 6, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 10, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 10, Y: y, Button: tea.MouseLeft})

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m = next.(chatTUI)
	if cmd == nil {
		t.Fatal("Ctrl+C with a composer selection should issue a clipboard command")
	}
	if got := m.input.Value(); got != "alpha beta" {
		t.Fatalf("Ctrl+C changed composer value to %q", got)
	}
	if got := m.selectedComposerText(); got != "beta" {
		t.Fatalf("Ctrl+C should preserve composer selection, got %q", got)
	}

	m = updateComposerMouseTestTUI(t, m, tea.PasteMsg{Content: "gamma"})
	if got := ansi.Strip(m.input.Value()); got != "alpha gamma" {
		t.Fatalf("paste over selection produced %q, want %q", got, "alpha gamma")
	}
}

func TestComposerDragReleaseAutoCopies(t *testing.T) {
	setLocalClipboardSession(t)
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("alpha beta")
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 6, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 10, Y: y, Button: tea.MouseLeft})

	previous := writeNativeClipboardText
	t.Cleanup(func() { writeNativeClipboardText = previous })
	writeNativeClipboardText = func(text string) error {
		if text != "beta" {
			t.Fatalf("composer drag copied %q, want beta", text)
		}
		return nil
	}

	next, cmd := m.Update(tea.MouseReleaseMsg{X: x + 10, Y: y, Button: tea.MouseLeft})
	m = next.(chatTUI)
	if cmd == nil {
		t.Fatal("composer drag release should copy the selected text")
	}
	if got := m.selectedComposerText(); got != "beta" {
		t.Fatalf("composer selection after drag copy = %q, want beta", got)
	}

	result := clipboardCopyResultFromCmd(t, cmd)
	next, _ = m.Update(result)
	m = next.(chatTUI)
	if m.copyNoticeText != i18n.M.MouseCopiedHint {
		t.Fatalf("composer drag copy notice = %q, want %q", m.copyNoticeText, i18n.M.MouseCopiedHint)
	}
}

func TestComposerPlainClickReleaseDoesNotCopy(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("alpha beta")
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 3, Y: y, Button: tea.MouseLeft})

	next, cmd := m.Update(tea.MouseReleaseMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
	m = next.(chatTUI)
	if cmd != nil {
		t.Fatal("plain composer click must not copy an empty selection")
	}
	if m.validComposerSelection() {
		t.Fatal("plain composer click should clear its empty selection")
	}
}

func TestComposerSelectionDoesNotTurnCommandShortcutIntoText(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("keep this")
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 4, Y: y, Button: tea.MouseLeft})

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	if got := m.input.Value(); got != "keep this" {
		t.Fatalf("Ctrl+Y changed selected composer text to %q", got)
	}
}

func TestComposerImagePasteShortcutKeepsSelectionUntilImageArrives(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("alpha beta")
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + 6, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 10, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 10, Y: y, Button: tea.MouseLeft})

	shortcut := tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}
	shortcutName := "Ctrl+V"
	if runtime.GOOS == "windows" {
		shortcut.Mod = tea.ModAlt
		shortcutName = "Alt+V"
	}
	next, cmd := m.Update(shortcut)
	m = next.(chatTUI)
	if cmd == nil {
		t.Fatalf("%s should issue an async image clipboard read", shortcutName)
	}
	if !m.clipboardImagePending {
		t.Fatalf("%s should show the image paste as pending", shortcutName)
	}
	if got := m.selectedComposerText(); got != "beta" {
		t.Fatalf("%s should keep the selection until the clipboard arrives, got %q", shortcutName, got)
	}

	m = updateComposerMouseTestTUI(t, m, clipboardImageMsg{path: ".patty/attachments/test.png"})
	if got := m.input.Value(); got != "alpha [image #1] " {
		t.Fatalf("image paste over selection produced %q, want %q", got, "alpha [image #1] ")
	}
	if m.validComposerSelection() && !m.composerSel.empty() {
		t.Fatal("image paste should consume the selection")
	}
	if m.clipboardImagePending {
		t.Fatal("image result should clear the pending state")
	}
}

func TestImagePasteShortcutIsDistinctFromTerminalTextPaste(t *testing.T) {
	tests := []struct {
		key  string
		goos string
		want bool
	}{
		{key: "ctrl+v", goos: "darwin", want: true},
		{key: "ctrl+v", goos: "linux", want: true},
		{key: "ctrl+v", goos: "windows", want: false},
		{key: "alt+v", goos: "windows", want: true},
		{key: "alt+v", goos: "darwin", want: false},
		{key: "ctrl+shift+v", goos: "linux", want: false},
		{key: "super+v", goos: "darwin", want: false},
		{key: "meta+v", goos: "darwin", want: false},
	}
	for _, tt := range tests {
		if got := imagePasteShortcut(tt.key, tt.goos); got != tt.want {
			t.Errorf("imagePasteShortcut(%q, %q) = %v, want %v", tt.key, tt.goos, got, tt.want)
		}
	}
}

func TestComposerArrowKeysCollapseSelection(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("alpha beta")
	x, y, _ := m.composerOrigin()
	drag := func(m chatTUI, from, to int) chatTUI {
		m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x + from, Y: y, Button: tea.MouseLeft})
		m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + to, Y: y, Button: tea.MouseLeft})
		return updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + to, Y: y, Button: tea.MouseLeft})
	}

	// Left/Right collapse to the ordered selection start/end without moving
	m = drag(m, 6, 10)
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.composerSel.active {
		t.Fatal("Left should dismiss the selection")
	}
	if got := m.input.Column(); got != 6 {
		t.Fatalf("Left collapsed cursor to column %d, want 6 (selection start)", got)
	}

	m = drag(m, 6, 10)
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if got := m.input.Column(); got != 10 {
		t.Fatalf("Right collapsed cursor to column %d, want 10 (selection end)", got)
	}

	m = drag(m, 10, 6)
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if got := m.input.Column(); got != 6 {
		t.Fatalf("Left after backward drag collapsed to column %d, want 6", got)
	}

	m = drag(m, 10, 6)
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if got := m.input.Column(); got != 10 {
		t.Fatalf("Right after backward drag collapsed to column %d, want 10", got)
	}

	if got := m.input.Value(); got != "alpha beta" {
		t.Fatalf("arrow keys changed composer value to %q", got)
	}
}

func TestComposerNewlineSelectionHighlightsTrailingSpace(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("abc\ndef")

	m.composerSel = composerSelection{active: true, anchor: 3, head: 4, value: m.input.Value()}
	highlighted := m.renderComposerInput()
	if strings.Contains(highlighted, selStyle.Render("c")) {
		t.Fatalf("newline-only selection must not highlight the previous character: %q", highlighted)
	}
	if !strings.Contains(highlighted, "abc"+selStyle.Render(" ")) {
		t.Fatalf("newline-only selection should highlight the trailing caret space: %q", highlighted)
	}

	m.composerSel = composerSelection{active: true, anchor: 2, head: 4, value: m.input.Value()}
	highlighted = m.renderComposerInput()
	if !strings.Contains(highlighted, selStyle.Render("c ")) {
		t.Fatalf("selecting the last char plus newline should highlight both cells: %q", highlighted)
	}
}

func TestClipboardPasteOverWideSelectionRequestsClearScreen(t *testing.T) {
	prev := clearWideInputChanges
	clearWideInputChanges = true
	defer func() { clearWideInputChanges = prev }()

	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("한글abc")
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 4, Y: y, Button: tea.MouseLeft})

	next, cmd := m.Update(tea.PasteMsg{Content: "hi"})
	m = next.(chatTUI)
	if got := m.input.Value(); got != "hiabc" {
		t.Fatalf("clipboard paste over wide selection produced %q, want %q", got, "hiabc")
	}
	if cmd == nil {
		t.Fatal("replacing a wide selection should request a full redraw")
	}
}

func TestFailedImagePastePreservesComposerSelection(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("keep this")
	x, y, _ := m.composerOrigin()
	m = updateComposerMouseTestTUI(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseMotionMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	m = updateComposerMouseTestTUI(t, m, tea.MouseReleaseMsg{X: x + 4, Y: y, Button: tea.MouseLeft})

	m = updateComposerMouseTestTUI(t, m, tea.PasteMsg{Content: "/path/that/does/not/exist.png"})
	if got := m.input.Value(); got != "keep this" {
		t.Fatalf("failed image paste changed composer value to %q", got)
	}
	if got := m.selectedComposerText(); got != "keep" {
		t.Fatalf("failed image paste should preserve selection, got %q", got)
	}
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

func TestComposerBackspaceDecomposesHangulSyllableThenDeletesInitialJamo(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.input.SetValue("안녕")
	m.setComposerCursor(len([]rune("안녕")))

	for _, want := range []string{"안녀", "안ㄴ", "안"} {
		m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
		if got := m.input.Value(); got != want {
			t.Fatalf("backspace step produced %q, want %q", got, want)
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

func TestCapturedKittyMacOSKoreanBackspaceResidualCommitIsSuppressed(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMECompatibility = true

	// Kitty's ordinary PTY stream loses the physical Backspace and sends only
	// "ㄴ". Associated-text keyboard reporting preserves both fields, making the
	// faulty residual distinguishable from deliberate standalone jamo input.
	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: '안', Text: "안"})
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace, Text: "ㄴ"})
	m = next.(chatTUI)

	if got := m.input.Value(); got != "안" {
		t.Fatalf("captured Kitty IME replay produced %q, want %q", got, "안")
	}
	if cmd == nil {
		t.Fatal("suppressing Kitty's residual IME commit must invalidate the terminal frame")
	}
	if rendered := ansi.Strip(m.renderComposerInput()); strings.Contains(rendered, "ㄴ") {
		t.Fatalf("captured Kitty IME replay left residual jamo in rendered composer: %q", rendered)
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

	if got := m.input.Value(); got != "안" {
		t.Fatalf("parsed Kitty Backspace replay produced %q, want 안", got)
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
	m.kittyHangulIMECompatibility = true
	m.input.SetValue("안")
	m.setComposerCursor(1)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'ㄴ', Text: "ㄴ"})

	if got := m.input.Value(); got != "안ㄴ" {
		t.Fatalf("Kitty compatibility erased deliberate standalone jamo: %q", got)
	}
}

func TestKittyMacOSViewRequestsLosslessKeyboardIdentity(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMECompatibility = true

	view := m.View()
	if !view.KeyboardEnhancements.ReportAllKeysAsEscapeCodes || !view.KeyboardEnhancements.ReportAssociatedText {
		t.Fatalf("Kitty compatibility keyboard enhancements = %+v", view.KeyboardEnhancements)
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
		name       string
		goos       string
		stdinTTY   bool
		env        map[string]string
		clientTerm string
		want       bool
	}{
		{
			name: "direct Kitty on macOS", goos: "darwin", stdinTTY: true,
			env: map[string]string{"TERM": "xterm-kitty"}, want: true,
		},
		{
			name: "Kitty client behind tmux with stale outer metadata", goos: "darwin", stdinTTY: true,
			env:        map[string]string{"TERM": "tmux-256color", "TERM_PROGRAM": "WarpTerminal", "TMUX": "/tmp/tmux"},
			clientTerm: "xterm-kitty", want: true,
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
			got := kittyHangulIMECompatibilityFor(
				tc.goos,
				tc.stdinTTY,
				func(key string) string { return tc.env[key] },
				func() string { return tc.clientTerm },
			)
			if got != tc.want {
				t.Fatalf("compatibility detection = %v, want %v", got, tc.want)
			}
		})
	}
}
