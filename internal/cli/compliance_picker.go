package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"patty/internal/i18n"
)

// openCompliancePicker opens the single-choice overlay allowing the user to
// select a specific compliance framework audit or a full combined audit.
func (m *chatTUI) openCompliancePicker() {
	items := []quickPickerItem{
		{
			ID:          "all",
			Label:       i18n.M.ComplianceItemAll,
			Description: i18n.M.ComplianceItemAllDesc,
		},
		{
			ID:          "pipa",
			Label:       i18n.M.ComplianceItemPipa,
			Description: i18n.M.ComplianceItemPipaDesc,
		},
		{
			ID:          "kisa",
			Label:       i18n.M.ComplianceItemKisa,
			Description: i18n.M.ComplianceItemKisaDesc,
		},
		{
			ID:          "csap",
			Label:       i18n.M.ComplianceItemCsap,
			Description: i18n.M.ComplianceItemCsapDesc,
		},
	}
	m.quickPick = &quickPicker{
		kind:     quickPickerCompliance,
		title:    i18n.M.CompliancePickerTitle,
		items:    items,
		selected: 0,
	}
}

// runComplianceSubcommand handles /compliance (or /컴플라이언스). Bare /compliance
// opens the quickPicker single-choice overlay; sub-arguments (/compliance pipa)
// invoke the target audit sub-skill directly.
func (m *chatTUI) runComplianceSubcommand(input string) tea.Cmd {
	fields := strings.Fields(input)
	if len(fields) <= 1 {
		m.openCompliancePicker()
		return nil
	}
	sub := strings.ToLower(fields[1])
	return m.applyComplianceSelection(sub)
}

// applyComplianceSelection dispatches the chosen compliance audit item.
func (m *chatTUI) applyComplianceSelection(choiceID string) tea.Cmd {
	var targetSkill string
	var label string
	switch strings.ToLower(choiceID) {
	case "pipa", "pipa-compliance":
		targetSkill = "pipa-compliance"
		label = "PIPA (개인정보보호법)"
	case "kisa", "kisa-compliance":
		targetSkill = "kisa-compliance"
		label = "KISA (시큐어코딩)"
	case "csap", "csap-compliance":
		targetSkill = "csap-compliance"
		label = "CSAP (클라우드 보안인증)"
	default:
		targetSkill = "korean-compliance"
		label = "종합 규제 준수"
	}
	cmd := "/" + targetSkill + " Perform compliance audit for " + label
	m.notice(fmt.Sprintf(i18n.M.ComplianceRunningFmt, label))
	return m.startControllerTurn(cmd, cmd, func() { m.ctrl.SubmitDisplay(cmd, cmd) })
}
