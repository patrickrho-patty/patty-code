package cli

import "strings"

// tuiFrameLayout is the render plan for the pinned frame rail. Keeping the
// ordered parts and their measured rows together prevents View, viewport
// sizing, and cursor placement from maintaining separate copies of the same
// layout rules.
//
// Composer rendering remains in View because its vertical padding calculation
// consults the bottom rail. The plan owns the composer reservation and places
// every other part exactly once.
type tuiFrameLayout struct {
	preComposerParts    []string
	statusBlock         string
	rowsAboveComposer   int
	rowsWithoutComposer int
	composerRows        int
	statusRows          int
	hideComposer        bool
}

func (m chatTUI) frameLayout(width int) tuiFrameLayout {
	return m.buildFrameLayout(width, true)
}

func (m chatTUI) buildFrameLayout(width int, includeComposer bool) tuiFrameLayout {
	width = max(width, 10)
	layout := tuiFrameLayout{hideComposer: m.hideComposer()}

	appendPreComposer := func(part string) {
		if part == "" {
			return
		}
		layout.preComposerParts = append(layout.preComposerParts, part)
		rows := renderedRowCount(part)
		layout.rowsAboveComposer += rows
		layout.rowsWithoutComposer += rows
	}

	appendPreComposer(m.renderTodoPanel())
	appendPreComposer(m.renderApprovalBanner())
	appendPreComposer(m.renderChooser())
	appendPreComposer(m.renderRewind())
	appendPreComposer(m.renderMCPImport())
	appendPreComposer(m.renderResumePicker())
	appendPreComposer(m.renderQuickPicker())
	appendPreComposer(m.renderCopyPicker())
	appendPreComposer(m.renderCompletion())
	if m.nativeScrollback {
		appendPreComposer(m.renderMainManager())
	}

	shellMode := strings.HasPrefix(strings.TrimSpace(m.input.Value()), "!")
	cancelRequested := m.cancelRequested()
	working := m.runningWorkingLine(cancelRequested, true)
	if working != "" {
		appendPreComposer(workingStyle.Width(width).MaxWidth(width).Render(wrapStatusLine(working, width)))
	}

	appendPreComposer(m.renderMainManagerFooter())
	if !layout.hideComposer {
		appendPreComposer(m.renderQueueIndicator())
	}

	primaryStatus := m.primaryStatusLine(shellMode, cancelRequested)
	layout.statusBlock = m.renderFrameStatusBlock(primaryStatus, width)
	if layout.statusBlock != "" {
		layout.rowsWithoutComposer += renderedRowCount(layout.statusBlock)
	}
	layout.statusRows = renderedRowCount(wrapStatusLine(working, width)) + renderedRowCount(layout.statusBlock)
	// Before the first resize, focused tests and pre-frame callers may still
	// carry the historical two-row status reservation. Keep that conservative
	// fallback until Update has synchronized statusLineCount from this plan.
	if layout.statusRows < m.statusLineCount {
		layout.rowsWithoutComposer += m.statusLineCount - layout.statusRows
	}

	if includeComposer && !layout.hideComposer {
		layout.composerRows = m.composerRowCount()
	}
	return layout
}

func renderedRowCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
