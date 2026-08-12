package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/skill"
)

func (m *chatTUI) runSkillSubcommand(input string) {
	args := tokenizeArgs(input)
	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}
	switch sub {
	case "":
		m.openSkillPicker()
	case "list", "ls":
		m.skillList()
	case "manage", "picker":
		m.openSkillPicker()
	case "show", "cat":
		if len(args) < 3 {
			m.notice("usage: /skills show <name>")
			return
		}
		m.skillShow(args[2])
	case "enable", "disable":
		if len(args) < 3 {
			m.notice("usage: /skills " + sub + " <name>")
			return
		}
		m.skillSetEnabled(args[2], sub == "enable")
	case "new", "init":
		if len(args) < 3 {
			m.notice("usage: /skills new <name> [--global]")
			return
		}
		global := containsArg(args[3:], "--global")
		m.skillNew(args[2], global)
	case "paths":
		m.skillPaths()
	default:
		hint := ""
		if _, ok := m.ctrl.RunSkill("/" + args[1]); ok {
			hint = " (to run it, type /" + args[1] + ")"
		}
		m.notice("unknown /skills subcommand " + args[1] + hint + " — try: /skills, /skills manage, /skills show <name>, /skills enable <name>, /skills disable <name>, /skills new <name>, /skills paths")
	}
}

func (m *chatTUI) skillList() {
	skills := m.skills
	if m.ctrl != nil {
		skills = managementSlashSkills(m.ctrl)
	}
	if len(skills) == 0 {
		m.notice("no skills found. Add SKILL.md / <name>.md under .patty/skills (project) or ~/.patty/skills (global); .agents/.agent/.claude skills dirs also work. Invoke with /<name> or run_skill.")
		return
	}
	m.commitLine(renderSkillList(m.width, sortedSkills(skills), m.disabledSkillNames()))
}

func (m *chatTUI) skillShow(name string) {
	skills := m.skills
	if m.ctrl != nil {
		skills = managementSlashSkills(m.ctrl)
	}
	for _, s := range skills {
		if s.Name == name || s.SlashName() == strings.TrimPrefix(name, "/") {
			disabled := false
			if m.ctrl != nil {
				disabled = !m.ctrl.SkillEnabled(s.Name)
			}
			m.commitLine(renderSkillShow(m.width, s, disabled))
			return
		}
	}
	m.notice("unknown skill: " + name)
}

func managementSlashSkills(ctrl control.SessionAPI) []skill.Skill {
	if ctrl == nil {
		return nil
	}
	// AllSkills preserves disabled entries; SlashSkills adds every enabled
	// package-qualified alias when multiple plugins export the same bare name.
	all := append([]skill.Skill(nil), ctrl.AllSkills()...)
	all = append(all, ctrl.SlashSkills()...)
	return skill.VisibleSlashSkills(all)
}

func (m *chatTUI) disabledSkillNames() map[string]bool {
	out := map[string]bool{}
	if m.ctrl == nil {
		return out
	}
	for _, s := range m.ctrl.DisabledSkills() {
		out[s.Name] = true
	}
	return out
}

func (m *chatTUI) skillSetEnabled(name string, enabled bool) {
	m.skillSaveEnabledChanges(map[string]bool{name: enabled})
}

func (m *chatTUI) skillSaveEnabledChanges(changes map[string]bool) {
	if len(changes) == 0 {
		return
	}
	if m.buildController == nil {
		m.notice("skill toggle unavailable in this session")
		return
	}
	if m.ctrl == nil {
		m.notice("skill toggle unavailable in this session")
		return
	}
	if m.runtimeSwitchBusy() {
		m.notice("finish or cancel active work and stop background jobs before changing skills")
		return
	}
	if m.modelSwitchPending {
		m.notice("wait for the current runtime switch to finish")
		return
	}
	known := map[string]string{}
	for _, sk := range m.ctrl.AllSkills() {
		known[config.SkillNameKey(sk.Name)] = sk.Name
	}
	for _, sk := range m.ctrl.SlashSkills() {
		known[sk.SlashName()] = sk.Name
	}
	// Lock only the load-modify-save cycle; the session refresh below runs
	// off-lock. The closure returns a non-empty notice on failure.
	if failNotice := func() string {
		var verb string
		_, applyErr, saveErr := config.EditUserConfigLocked(func(c *config.Config) error {
			for name, enabled := range changes {
				verb = enableVerb(enabled)
				key := config.SkillNameKey(name)
				if key == "" {
					key = strings.TrimPrefix(strings.TrimSpace(name), "/")
				}
				canonical, ok := known[key]
				if !ok {
					return fmt.Errorf("unknown skill: %s", name)
				}
				if err := c.SetSkillEnabled(canonical, enabled); err != nil {
					return err
				}
			}
			return nil
		})
		switch {
		case applyErr != nil:
			return "skill " + verb + ": " + applyErr.Error()
		case saveErr != nil:
			return "skill toggle: " + saveErr.Error()
		}
		return ""
	}(); failNotice != "" {
		m.notice(failNotice)
		return
	}
	notice := ""
	if len(changes) == 1 {
		name := ""
		enabled := false
		for n, e := range changes {
			name, enabled = n, e
		}
		if enabled {
			notice = "enabled skill " + name + " — refreshing session"
		} else {
			notice = "disabled skill " + name + " — refreshing session"
		}
	} else {
		notice = fmt.Sprintf("updated %d skills — refreshing session", len(changes))
	}
	m.scheduleSkillSessionRefresh("skill toggle", notice)
}

func (m *chatTUI) scheduleSkillSessionRefresh(reason, notice string) bool {
	if m.unavailableOrBusyNotice("skill refresh unavailable in this session", "finish or cancel active work and stop background jobs before refreshing skills") {
		return false
	}
	m.beginRuntimeRebuild(controllerBuildSpec{
		ModelRef:         m.modelRef,
		RuntimeProfile:   m.runtimeProfile,
		ToolApprovalMode: m.ctrl.ToolApprovalMode(),
		PlanMode:         m.ctrl.PlanMode(),
	}, reason, notice, "", "")
	return m.pendingModelSwitch != nil
}

func enableVerb(enabled bool) string {
	if enabled {
		return "enable"
	}
	return "disable"
}

func (m *chatTUI) skillNew(name string, global bool) {
	st := m.skillStore()
	scope := skill.ScopeProject
	if global || !st.HasProjectScope() {
		scope = skill.ScopeGlobal
	}
	path, err := st.Create(name, scope)
	if err != nil {
		m.notice("skill new: " + err.Error())
		return
	}
	m.notice(fmt.Sprintf("created skill %q at %s — edit it, then /new (or restart) to pick it up", name, path))
}

func (m *chatTUI) skillPaths() {
	st := m.skillStore()
	m.commitLine(renderSkillPaths(m.width, st.Roots()))
}

func (m *chatTUI) skillStore() *skill.Store {
	cwd, _ := os.Getwd()
	var custom []string
	var excluded []string
	var pluginPaths map[string][]string
	var pluginAgentPaths map[string][]string
	maxDepth := 3
	if cfg, err := config.Load(); err == nil {
		custom = cfg.SkillCustomPaths()
		excluded = cfg.SkillExcludedPaths()
		pluginPaths = cfg.PluginPackageSkillOwners()
		pluginAgentPaths = cfg.PluginPackageAgentOwners()
		maxDepth = cfg.SkillMaxDepth()
	}
	return skill.New(skill.Options{ProjectRoot: cwd, CustomPaths: custom, PluginPaths: pluginPaths, PluginAgentPaths: pluginAgentPaths, ExcludedPaths: excluded, MaxDepth: maxDepth})
}

func (m *chatTUI) runHooksSubcommand(input string) {
	args := tokenizeArgs(input)
	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}
	cwd, _ := os.Getwd()
	switch sub {
	case "", "list", "ls":
		m.hooksList(cwd)
	case "trust":
		// Backward-compatible response for old clients and saved commands.
		m.notice("project hooks are enabled automatically; no trust action is required")
	default:
		m.notice("unknown /hooks subcommand " + args[1] + " — try: /hooks or /hooks list")
	}
}

func (m *chatTUI) hooksList(cwd string) {
	active := m.ctrl.HookRunner().Hooks()
	m.commitLine(renderHooks(m.width, active))
}

func containsArg(args []string, flag string) bool {
	return slices.Contains(args, flag)
}
