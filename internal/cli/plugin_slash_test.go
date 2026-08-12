package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/i18n"
)

func writePluginTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func typeLine(t *testing.T, m chatTUI, line string) chatTUI {
	t.Helper()
	for _, ch := range line {
		m0, _ := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = m0.(chatTUI)
	}
	return m
}

// TestPluginsSlashShowsComingSoonNotice pins the /plugins placeholder: it is a
// local command with no controller rebuild, and its coming-soon line lands in
// the transcript right after the echoed command.
func TestPluginsSlashShowsComingSoonNotice(t *testing.T) {
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 140)
	m0, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = m0.(chatTUI)
	m.runSlashCommand("/plugins")
	if m.pendingModelSwitch != nil {
		t.Fatalf("/plugins must not schedule a controller rebuild, got %v", m.pendingModelSwitch)
	}
	out := strings.Join(m.transcript, "\n")
	if !strings.Contains(out, i18n.M.PluginComingSoon) {
		t.Fatalf("coming-soon notice missing from the transcript:\n%s", out)
	}
}

// TestPluginsFlowKeepsArtworkCentered pins the /plugins flow against the
// original layout: the command echoes as a user bubble and the harness answers
// with the coming-soon line, while the banner artwork stays horizontally
// centered in scrollback instead of jumping left. It drives the real key
// path so the viewport sync runs like in production.
func TestPluginsFlowKeepsArtworkCentered(t *testing.T) {
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 140)
	m0, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = m0.(chatTUI)
	m = typeLine(t, m, "/plugins")
	// Close the completion menu, then submit the command.
	m0, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m0.(chatTUI)
	m0, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = m0.(chatTUI)

	plain := ansi.Strip(m.View().Content)
	lines := strings.Split(plain, "\n")
	var artwork, echo, notice int
	for i, line := range lines {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "+-"):
			artwork = i
		case strings.Contains(line, "/plugins"):
			echo = i
		case strings.Contains(line, i18n.M.PluginComingSoon):
			notice = i
		}
	}
	if echo == 0 || notice == 0 {
		t.Fatalf("expected the echo bubble and the coming-soon line in the transcript:\n%s", plain)
	}
	if notice <= echo {
		t.Fatalf("coming-soon line must follow the echoed command:\n%s", plain)
	}
	if artwork == 0 {
		t.Fatalf("banner artwork missing from the view:\n%s", plain)
	}
	artLine := lines[artwork]
	leading := len(artLine) - len(strings.TrimLeft(artLine, " "))
	if leading < 10 {
		t.Fatalf("banner artwork is left-aligned (leading=%d); it must stay centered:\n%s", leading, plain)
	}
}

// TestLaunchFrameBreathingRow pins the startup spacing: a blank row must sit
// between the artwork and the context line in the centered launch frame.
func TestLaunchFrameBreathingRow(t *testing.T) {
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 140)
	m0, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = m0.(chatTUI)
	if !m.isLaunchChromeOnlyTranscript() {
		t.Fatal("startup precondition failed: transcript is not launch chrome")
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	lastArtwork, tip := -1, -1
	for i, line := range lines {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "+-"):
			lastArtwork = i
		case strings.Contains(line, "Context is kept across turns"):
			tip = i
		}
	}
	if lastArtwork < 0 || tip < 0 {
		t.Fatalf("artwork or context line missing from the launch frame:\n%s", strings.Join(lines, "\n"))
	}
	if tip <= lastArtwork+1 {
		t.Fatalf("no blank row between artwork (line %d) and context line (line %d):\n%s", lastArtwork, tip, strings.Join(lines, "\n"))
	}
}
