package cli

import (
	"fmt"
	"strings"

	"patty/internal/config"
	"patty/internal/i18n"
	"patty/internal/outputstyle"
)

// openOutputStylePicker opens the single-choice overlay over the builtin and
// custom output styles. Builtin styles get localized display labels; the
// active style is marked and preselected.
func (m *chatTUI) openOutputStylePicker() {
	styles := outputstyle.List(outputstyle.Dirs())
	if len(styles) == 0 {
		m.notice(i18n.M.OutputStyleNone)
		return
	}
	items := make([]quickPickerItem, 0, len(styles))
	selected := 0
	for i, st := range styles {
		label, description := outputStylePickerPresentation(st)
		status := ""
		if strings.EqualFold(st.Name, m.outputStyle) {
			status = i18n.M.QuickPickerActive
			selected = i
		}
		items = append(items, quickPickerItem{ID: st.Name, Label: label, Description: description, Status: status})
	}
	m.quickPick = &quickPicker{kind: quickPickerOutputStyle, title: i18n.M.OutputStylePickerTitle, items: items, selected: selected}
}

// outputStylePickerPresentation localizes the three builtin style names and
// descriptions; custom styles keep their own metadata.
func outputStylePickerPresentation(st outputstyle.OutputStyle) (label, description string) {
	if !st.Builtin {
		return st.Name, st.Description
	}
	switch st.Name {
	case "explanatory":
		return i18n.M.OutputStyleExplanatory, i18n.M.OutputStyleExplanatoryDesc
	case "learning":
		return i18n.M.OutputStyleLearning, i18n.M.OutputStyleLearningDesc
	case "concise":
		return i18n.M.OutputStyleConcise, i18n.M.OutputStyleConciseDesc
	}
	return st.Name, st.Description
}

// applyOutputStyle persists the chosen style to the user config and rebuilds
// the controller so it lands in the live system prompt now. display is the
// localized picker label used in notices; the rebuild carries the canonical
// style name so a project-pinned config can never override the in-session pick.
func (m *chatTUI) applyOutputStyle(name, display string) {
	if m.unavailableOrBusyNotice(i18n.M.OutputStyleSwitchUnavailable, i18n.M.OutputStyleSwitchBusy) {
		return
	}
	if strings.EqualFold(name, m.outputStyle) {
		m.notice(fmt.Sprintf(i18n.M.OutputStyleAlreadyFmt, display))
		return
	}
	m.persistOutputStyle(name)
	m.beginRuntimeRebuild(controllerBuildSpec{
		ModelRef:         m.modelRef,
		RuntimeProfile:   m.runtimeProfile,
		ToolApprovalMode: m.ctrl.ToolApprovalMode(),
		PlanMode:         m.ctrl.PlanMode(),
		OutputStyle:      name,
	}, "output-style", fmt.Sprintf(i18n.M.OutputStyleSwitchingFmt, display), fmt.Sprintf(i18n.M.OutputStyleSwitchedFmt, display), name)
}

// persistOutputStyle writes the style name to output_style in the user
// config.toml so the next CLI launch starts with the same style. The
// in-memory switch proceeds regardless of the outcome; failures report to the
// notice channel so the user can see whether their choice will survive a
// restart.
func (m *chatTUI) persistOutputStyle(name string) {
	path, applyErr, saveErr := config.EditUserConfigLocked(func(c *config.Config) error {
		return c.SetOutputStyle(name)
	})
	switch {
	case path == "":
		return
	case applyErr != nil:
		m.notice(fmt.Sprintf("output-style: persist refused: %v (style=%s)", applyErr, name))
	case saveErr != nil:
		m.notice(fmt.Sprintf("output-style: persist save failed: %v (style=%s, path=%s)", saveErr, name, path))
	}
}
