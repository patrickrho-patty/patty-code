package cli

import (
	"fmt"
	"strings"

	"patty/internal/i18n"
)

func renderModels(width int, refs []string, active string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", viewHeader("%s", i18n.M.ModelListHeader))
	for _, ref := range refs {
		status := ""
		if ref == active {
			status = "  " + viewStatus(i18n.M.QuickPickerActive)
		}
		fmt.Fprintf(&b, "  %s%s\n", viewCompactText(ref, viewBudget(width, 2+visibleWidth(status))), status)
	}
	b.WriteString(viewHint(viewCompactText(i18n.M.ViewModelHint, viewBudget(width, 2))))
	return strings.TrimRight(b.String(), "\n")
}
