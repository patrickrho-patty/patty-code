package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"patty/internal/config"
)

func (m *chatTUI) runEffortCommand(input string) tea.Cmd {
	entry, ref, err := m.currentConfigProvider()
	if err != nil {
		m.notice("effort: " + err.Error())
		return nil
	}
	cap := config.EffortCapabilityForEntry(entry)
	if !cap.Supported {
		m.notice(fmt.Sprintf("effort is not configurable for %s", entry.Name))
		return nil
	}

	args := tokenizeArgs(input)
	if len(args) < 2 {
		current := config.EffortDisplay(entry)
		options := strings.Join(cap.Levels, "|")
		m.notice(fmt.Sprintf("effort for %s: %s (default: %s; options: %s)", entry.Name, current, cap.Default, options))
		return nil
	}
	if len(args) > 2 {
		m.notice("usage: /effort " + strings.Join(cap.Levels, "|"))
		return nil
	}
	effort, err := config.NormalizeEffort(entry, args[1])
	if err != nil {
		m.notice(err.Error())
		return nil
	}
	if m.unavailableOrBusyNotice("effort: model switching is unavailable in this session", "finish or cancel active work and stop background jobs before changing effort") {
		return nil
	}

	// Lock only the load-modify-save cycle; the snapshot and controller
	// rebuild below run off-lock.
	path, applyErr, saveErr := config.EditUserConfigLocked(func(c *config.Config) error {
		if _, ok := c.Provider(entry.Name); !ok {
			if err := c.UpsertProvider(*entry); err != nil {
				return err
			}
		}
		if entry.Kind == "anthropic" && effort != "" && entry.Thinking == "" {
			if err := c.SetProviderThinking(entry.Name, "adaptive"); err != nil {
				return err
			}
		}
		return c.SetProviderEffort(entry.Name, effort)
	})
	if path == "" {
		m.notice("effort: cannot resolve user config directory")
		return nil
	}
	if applyErr != nil || saveErr != nil {
		err := applyErr
		if saveErr != nil {
			err = saveErr
		}
		m.notice("effort: " + err.Error())
		return nil
	}

	display := effort
	if display == "" {
		display = "auto"
	}
	m.beginRuntimeRebuild(controllerBuildSpec{
		ModelRef:         ref,
		RuntimeProfile:   m.runtimeProfile,
		ToolApprovalMode: m.ctrl.ToolApprovalMode(),
		PlanMode:         m.ctrl.PlanMode(),
		EffortOverride:   &effort,
	}, "effort", fmt.Sprintf("setting effort for %s to %s…", entry.Name, display), fmt.Sprintf("effort for %s set to %s", entry.Name, display), "")
	return m.pendingModelSwitch
}

func (m *chatTUI) currentConfigProvider() (*config.ProviderEntry, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}
	// When the per-tab ref is empty we are inheriting the configured
	// default — let resolveModelForCLI fall through a keyless default to
	// the next configured provider (issue #6996). When m.modelRef is
	// already set we honor it verbatim: the user picked that model
	// explicitly (via /model, on the model switcher, or in the bootstrap
	// step) and we must not silently swap to a different provider just
	// because the entry happens to be keyless.
	ref := m.modelRef
	if strings.TrimSpace(ref) == "" {
		var rerr error
		ref, _, rerr = resolveModelForCLI("", cfg)
		if rerr != nil {
			return nil, "", rerr
		}
	}
	entry, ok := cfg.ResolveModel(ref)
	if !ok {
		return nil, "", fmt.Errorf("unknown model %q", ref)
	}
	if ref == entry.Name || !strings.Contains(ref, "/") {
		ref = entry.Name + "/" + entry.Model
	}
	return entry, ref, nil
}

func (m *chatTUI) refreshEffortStatus() {
	m.effortLevel = ""
	entry, _, err := m.currentConfigProvider()
	if err != nil {
		return
	}
	if !config.EffortCapabilityForEntry(entry).Supported {
		return
	}
	m.effortLevel = config.EffortDisplay(entry)
}
