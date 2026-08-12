package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"patty/internal/i18n"
)

type clearConfirm struct {
	confirm int // 0 = clear, 1 = cancel
}

func (m chatTUI) handleClearConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "j", "k", "tab", "shift+tab":
		if m.clearConfirm.confirm == 0 {
			m.clearConfirm.confirm = 1
		} else {
			m.clearConfirm.confirm = 0
		}
	case "y", "Y":
		return m.confirmClearContext()
	case "n", "N", "esc", "ctrl+c":
		m.clearConfirm = nil
	case "enter":
		if m.clearConfirm.confirm == 0 {
			return m.confirmClearContext()
		}
		m.clearConfirm = nil
	}
	return m, nil
}

func (m chatTUI) confirmClearContext() (tea.Model, tea.Cmd) {
	m.clearConfirm = nil
	if err := m.ctrl.ClearSession(); err != nil {
		m.notice(fmt.Sprintf("%s: %v", i18n.M.SlashClearFailed, err))
		return m, nil
	}
	m.followSessionLease()
	m.resetFreshContextView(true)
	m.notice(i18n.M.SlashClearDone)
	return m, tea.ClearScreen
}

func (m *chatTUI) resetFreshContextView(clearTranscript bool) {
	m.finalizeStreamed()
	m.pending.Reset()
	m.reasoning.Reset()
	m.todoArgs = ""
	m.chooser = nil
	m.pendingApproval = nil
	m.bubblePending = false
	m.turnDiscarded = false
	if clearTranscript {
		m.clearTranscriptDisplay()
		m.sessionSwitch = true
	} else {
		m.commitLine("")
	}
	m.commitTranscriptSource(transcriptSource{kind: transcriptSourceBanner})
	m.transcriptDirty = true
	m.forceGotoBottom = true
}

func (m chatTUI) renderClearConfirm() string {
	if m.clearConfirm == nil {
		return ""
	}
	w := max(viewWidth(m.width), 40)
	var b strings.Builder
	b.WriteString(i18n.M.SlashClearPrompt + "\n")
	b.WriteString(viewMeta(i18n.M.ClearContextDetails) + "\n\n")
	b.WriteString(rowLine(m.clearConfirm.confirm == 0, 1, "", i18n.M.ConfirmLabel, false) + "\n")
	b.WriteString(rowLine(m.clearConfirm.confirm == 1, 2, "", i18n.M.CancelLabel, false))
	return choicePanelStyle.Width(w).Render(b.String())
}
