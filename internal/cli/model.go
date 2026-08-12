package cli

import (
	"fmt"
	"strings"

	"patty/internal/config"
	"patty/internal/extension/providerext"
	"patty/internal/i18n"
	"patty/internal/provider"
)

// runModelSubcommand handles "/model": with no argument it opens the configured
// model picker; "/model <ref>" switches the
// session to that model in place, carrying the conversation across. The actual
// controller build runs asynchronously so it cannot block the TUI event loop.
func (m *chatTUI) runModelSubcommand(input string) {
	args := tokenizeArgs(input) // args[0] == "/model"
	if len(args) < 2 {
		m.openModelPicker()
		return
	}
	ref := args[1]
	if m.unavailableOrBusyNotice(i18n.M.ModelSwitchUnavailable, i18n.M.ModelSwitchBusy) {
		return
	}
	if ref == m.modelRef {
		m.notice(fmt.Sprintf(i18n.M.ModelAlreadyOnFmt, ref))
		return
	}
	// Persist the user's choice to the user config.toml so the next
	// session starts on the same model instead of falling back to the global
	// default. Mirrors the pattern used by /theme (persistTheme), /effort, and
	// /language.
	m.persistModel(ref)
	m.beginRuntimeRebuild(controllerBuildSpec{
		ModelRef:         ref,
		RuntimeProfile:   m.runtimeProfile,
		ToolApprovalMode: m.ctrl.ToolApprovalMode(),
		PlanMode:         m.ctrl.PlanMode(),
	}, "model", fmt.Sprintf(i18n.M.ModelSwitchingFmt, ref), "", "")
}

func (m *chatTUI) openModelPicker() {
	var catalog []provider.Descriptor
	if m.ctrl != nil {
		catalog = m.ctrl.ProviderCatalog()
	}
	refs := mergeExtensionModelRefs(modelRefs(), catalog)
	if len(refs) == 0 {
		m.notice("model: no configured chat models")
		return
	}
	items := make([]quickPickerItem, 0, len(refs))
	descriptorsByRef := make(map[string]provider.Descriptor, len(catalog))
	for _, descriptor := range catalog {
		if ref := strings.TrimSpace(descriptor.Ref); ref != "" {
			descriptorsByRef[ref] = descriptor
		}
	}
	selected := 0
	for _, ref := range refs {
		label, description := modelPickerPresentation(ref, descriptorsByRef)
		status := ""
		if ref == m.modelRef {
			status = i18n.M.QuickPickerActive
			selected = len(items)
		}
		items = append(items, quickPickerItem{ID: ref, Label: label, Description: description, Status: status})
	}
	m.quickPick = &quickPicker{kind: quickPickerModel, title: i18n.M.QuickPickerModelTitle, items: items, selected: selected}
}

func modelPickerPresentation(ref string, descriptorsByRef map[string]provider.Descriptor) (label, description string) {
	if providerext.PluginRefOwner(ref) != "" {
		if descriptor, ok := descriptorsByRef[ref]; ok {
			label = strings.TrimSpace(descriptor.DisplayName)
			if label == "" {
				label = strings.TrimSpace(descriptor.Model)
			}
			if label == "" {
				label = ref
			}
			return label, fmt.Sprintf(i18n.M.QuickPickerExtensionFmt, ref)
		}
		return ref, i18n.M.QuickPickerExtensionModel
	}
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		providerName := strings.TrimSpace(parts[0])
		modelName := strings.TrimSpace(parts[1])
		if providerName != "" && modelName != "" {
			return modelName, fmt.Sprintf(i18n.M.QuickPickerProviderFmt, providerName)
		}
	}
	return ref, ""
}

// persistModel writes ref (a "provider/model" string) to default_model in the
// user config.toml so the next CLI launch starts on the same
// model. The in-memory switch is always allowed to proceed regardless of the
// outcome here, but every step (rejected by validation, save failed, or
// persisted successfully) reports back to the TUI notice channel so the user
// can see whether their /model choice will survive a restart. Run before
// Snapshot/ModelSwitchingFmt so the persistence outcome shows up first in
// the notice area.
func (m *chatTUI) persistModel(ref string) {
	path, applyErr, saveErr := config.EditUserConfigLocked(func(c *config.Config) error {
		return c.SetDefaultModel(ref)
	})
	switch {
	case path == "":
		return
	case applyErr != nil:
		m.notice(fmt.Sprintf("model: persist refused: %v (ref=%s)", applyErr, ref))
	case saveErr != nil:
		m.notice(fmt.Sprintf("model: persist save failed: %v (ref=%s, path=%s)", saveErr, ref, path))
	default:
		m.notice(fmt.Sprintf("model: persisted (ref=%s, path=%s)", ref, path))
	}
}

// modelRefs returns the configured provider/model refs for slash completion.
func modelRefs() []string {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	var out []string
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if !p.Configured() {
			continue
		}
		for _, model := range p.ChatModelList() {
			out = append(out, p.Name+"/"+model)
		}
	}
	return out
}

// mergeExtensionModelRefs folds the session's extension provider catalog into
// the config-backed picker list. Extension refs arrive fully namespaced
// (plugin/<plugin>/<provider>/<model>); entries already listed (or blank) are
// dropped so a claim-replaced config ref never appears twice. A nil catalog
// returns base unchanged — the no-extension path is untouched.
func mergeExtensionModelRefs(base []string, catalog []provider.Descriptor) []string {
	if len(catalog) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(catalog))
	out := make([]string, 0, len(base)+len(catalog))
	for _, ref := range base {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	for _, d := range catalog {
		ref := strings.TrimSpace(d.Ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

// providerNames returns the names of configured providers for slash completion.
func providerNames() []string {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	var out []string
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if !p.Configured() {
			continue
		}
		out = append(out, p.Name)
	}
	return out
}
