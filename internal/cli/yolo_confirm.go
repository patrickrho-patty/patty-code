package cli

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"patty/internal/i18n"
)

// yoloFrameInterval is the composer border animation cadence while YOLO mode
// is active. The breathe is intentionally slow and subtle; the tick self-renews
// only while YOLO is on, so it stops on its own when the user leaves YOLO.
const yoloFrameInterval = 150 * time.Millisecond

// yoloFrameMsg advances the pulsing red composer border by one frame while YOLO
// mode is active.
type yoloFrameMsg struct{}

// yoloFrameTick schedules the next YOLO border animation frame.
func yoloFrameTick() tea.Cmd {
	return tea.Tick(yoloFrameInterval, func(time.Time) tea.Msg { return yoloFrameMsg{} })
}

// yoloConfirm is the per-folder YOLO authorization overlay. It intercepts a
// Ctrl+Y into YOLO when the current folder has not yet been approved: approving
// activates YOLO and persists the folder so the disclaimer never reappears
// there; cancelling returns to the prior mode without activating.
type yoloConfirm struct {
	confirm int // 0 = approve, 1 = cancel
}

func (m chatTUI) handleYoloConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "j", "k", "tab", "shift+tab":
		if m.yoloConfirm.confirm == 0 {
			m.yoloConfirm.confirm = 1
		} else {
			m.yoloConfirm.confirm = 0
		}
	case "y", "Y":
		return m.confirmYoloActivation()
	case "n", "N", "esc", "ctrl+c":
		m.yoloConfirm = nil
	case "enter":
		if m.yoloConfirm.confirm == 0 {
			return m.confirmYoloActivation()
		}
		m.yoloConfirm = nil
	}
	return m, nil
}

// confirmYoloActivation approves the current folder for YOLO and activates it.
func (m chatTUI) confirmYoloActivation() (tea.Model, tea.Cmd) {
	cmd := m.activateYolo(true)
	return m, cmd
}

func (m chatTUI) renderYoloConfirm() string {
	if m.yoloConfirm == nil {
		return ""
	}
	w := max(viewWidth(m.width), 40)
	var b strings.Builder
	b.WriteString(i18n.M.YoloModePrompt + "\n")
	b.WriteString(viewMeta(i18n.M.YoloModeDisclaimer) + "\n\n")
	b.WriteString(rowLine(m.yoloConfirm.confirm == 0, 1, "", i18n.M.YoloApproveLabel, false) + "\n")
	b.WriteString(rowLine(m.yoloConfirm.confirm == 1, 2, "", i18n.M.CancelLabel, false))
	return choicePanelStyle.Width(w).Render(b.String())
}
