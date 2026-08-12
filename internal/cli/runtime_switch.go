package cli

import (
	tea "charm.land/bubbletea/v2"

	"patty/internal/i18n"
)

// beginRuntimeRebuild runs the part of a runtime switch every trigger shares:
// snapshot the live controller (which may retarget it to a recovery branch),
// capture the carried history and session path, move the session lease, then
// fire the async controller rebuild. Callers run the trigger-specific guards
// (busy, already-on, persistence) first.
func (m *chatTUI) beginRuntimeRebuild(spec controllerBuildSpec, failurePrefix, switchingNotice, successNotice, outputStyle string) tea.Cmd {
	if err := m.ctrl.Snapshot(); err != nil {
		m.notice(failurePrefix + ": snapshot failed: " + err.Error())
		return nil
	}
	// Capture the resume path and history only after Snapshot: a snapshot
	// conflict can retarget the controller to a recovery branch, and a
	// pre-snapshot capture would bind the rebuild to the stale original file.
	carried := m.ctrl.History()
	prevPath := m.ctrl.SessionPath()
	// Move the lease before the rebuilt controller binds prevPath for writing
	// (AdoptHistory resumes there): after a snapshot retarget the lease still
	// guards the old path, so the async build must not write unguarded.
	if err := m.rebindSessionLease(prevPath); err != nil {
		m.notice(failurePrefix + ": " + sessionLeaseHeldNotice(err))
		return nil
	}
	if switchingNotice != "" {
		m.notice(switchingNotice)
	}

	// Capture old controller for cleanup after the async build succeeds.
	oldCtrl := m.ctrl
	build := m.buildController

	// Fire the build off the event loop; the result arrives as a tea.Cmd. The
	// old controller's Close kills plugin subprocesses and corrupts the
	// terminal's raw mode if run synchronously inside Update.
	m.modelSwitchPending = true
	m.pendingModelSwitch = func() tea.Msg {
		c, err := build(spec, carried, prevPath, oldCtrl)
		if err != nil {
			return modelSwitchMsg{ref: spec.ModelRef, err: err, failurePrefix: failurePrefix}
		}
		// Pass the old controller back in the message: its SessionEnd hooks
		// and plugin subprocess teardown must not run from a goroutine, so
		// Update defers the close as a tea.Cmd after the next render.
		return modelSwitchMsg{
			ref:           spec.ModelRef,
			ctrl:          c,
			oldCtrl:       oldCtrl,
			label:         c.Label(),
			commands:      c.Commands(),
			skills:        c.SlashSkills(),
			host:          c.Host(),
			successNotice: successNotice,
			outputStyle:   outputStyle,
		}
	}
	return m.pendingModelSwitch
}

// unavailableOrBusyNotice reports the trigger-specific reason a runtime switch
// cannot start: no builder, active work, or a switch already in flight. It
// emits the matching notice and returns true when the switch must not start.
func (m *chatTUI) unavailableOrBusyNotice(unavailable, busy string) bool {
	if m.buildController == nil || m.ctrl == nil {
		m.notice(unavailable)
		return true
	}
	if m.runtimeSwitchBusy() {
		m.notice(busy)
		return true
	}
	if m.modelSwitchPending {
		m.notice(i18n.M.RuntimeSwitchPending)
		return true
	}
	return false
}
