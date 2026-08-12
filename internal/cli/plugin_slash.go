package cli

import "patty/internal/i18n"

// runPluginSubcommand shows the coming-soon notice for every /plugins form:
// the plugin package surface is not user-facing yet, so no subcommand
// instructions are offered.
func (m *chatTUI) runPluginSubcommand(input string) {
	m.notice(i18n.M.PluginComingSoon)
}
