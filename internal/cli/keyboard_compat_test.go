package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/control"
	"patty/internal/event"
)

func TestIMEKeyboardCompatibilityCommandDisablesModifyOtherKeys(t *testing.T) {
	cmd := imeKeyboardCompatibilityCommand()
	if cmd == nil {
		t.Fatal("IME keyboard compatibility returned no command")
	}
	msg := cmd()
	raw, ok := msg.(tea.RawMsg)
	if !ok {
		t.Fatalf("IME keyboard compatibility command = %T, want tea.RawMsg", msg)
	}
	if got := raw.Msg; got != ansi.ResetModifyOtherKeys {
		t.Fatalf("startup keyboard compatibility sequence = %q, want %q", got, ansi.ResetModifyOtherKeys)
	}
}

func TestChatTUIResumeDisablesModifyOtherKeysForIME(t *testing.T) {
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 40)
	_, cmd := m.Update(tea.ResumeMsg{})
	if cmd == nil {
		t.Fatal("chat TUI resume returned no keyboard compatibility command")
	}
	resumeMsg := cmd()
	raw, ok := resumeMsg.(tea.RawMsg)
	if !ok {
		t.Fatalf("chat TUI resume command = %T, want tea.RawMsg", resumeMsg)
	}
	if got := raw.Msg; got != ansi.ResetModifyOtherKeys {
		t.Fatalf("resume keyboard compatibility sequence = %q, want %q", got, ansi.ResetModifyOtherKeys)
	}
}
