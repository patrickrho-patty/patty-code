package cli

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"patty/internal/config"
	"patty/internal/provider"
)

// TestModelRefsFromConfig verifies the /model picker enumerates configured
// provider/model refs (built-in defaults when no patty.toml is present), and
// only those whose provider API key is set.
func TestModelRefsFromConfig(t *testing.T) {
	isolateUserConfig(t) // no patty.toml -> built-in default providers
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	refs := modelRefs()
	if got, want := strings.Join(refs, ","), "patty/patty-code-standard"; got != want {
		t.Fatalf("default model refs = %q, want %q", got, want)
	}
}

func TestBareModelOpensKeyboardPicker(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatal(err)
	}
	m := newTestChatTUI()
	m.runModelSubcommand("/model")
	if m.quickPick == nil || m.quickPick.kind != quickPickerModel {
		t.Fatalf("bare /model picker = %+v", m.quickPick)
	}
	if len(m.quickPick.items) != 1 || m.quickPick.items[0].ID != "patty/patty-code-standard" || m.quickPick.items[0].Label != "patty-code-standard" {
		t.Fatalf("picker items = %+v", m.quickPick.items)
	}
	if m.renderQuickPicker() == "" || !m.hideComposer() {
		t.Fatal("model picker should render as a modal panel")
	}
	next, _ := m.handleQuickPickerKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if next.(chatTUI).quickPick != nil {
		t.Fatal("Esc should close model picker")
	}
}

// TestModelRefsSkipsUnconfigured verifies that with no provider keys set, the
// picker offers nothing rather than listing models the user can't select.
func TestModelRefsSkipsUnconfigured(t *testing.T) {
	isolateUserConfig(t)
	t.Setenv("DARI_HARNESS_KEY", "")
	if refs := modelRefs(); len(refs) != 0 {
		t.Errorf("no keys set → no refs, got %v", refs)
	}
}

// TestModelArgCompletion verifies "/model " completes to the configured refs
// through the shared completion path.
func TestModelArgCompletion(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	m := newTestChatTUI()
	items, _, ok := m.slashArgItems("/model ")
	if !ok || len(items) == 0 {
		t.Fatalf("/model arg completion should offer refs, ok=%v n=%d", ok, len(items))
	}
}

// TestPersistModelWritesDefaultModel verifies that calling persistModel with a
// "provider/model" ref writes default_model = "<ref>" to the user config file
// in TOML form. This is the fix for the "default model resets on every launch"
// regression: previously /model only mutated the in-memory controller and the
// next startup read the global default.
func TestPersistModelWritesDefaultModel(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}

	m := newTestChatTUI()
	m.persistModel("patty/patty-code-standard")

	body, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(body), `default_model = "patty/patty-code-standard"`) {
		t.Fatalf("saved config missing default_model ref:\n%s", body)
	}
}

// TestPersistModelRejectsUnknownRef verifies that an unresolvable ref is
// silently dropped (logged to slog, not pushed to the TUI notice channel)
// and never lands in the config file. Reason: surface a "persist failed"
// notice on the input box would make /model feel broken to users whose
// stored config doesn't list the exact model ref they picked; the in-
// memory switch still goes through.
func TestPersistModelRejectsUnknownRef(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}

	m := newTestChatTUI()
	m.persistModel("ghost/never-existed")

	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("unknown ref must not create config file, stat err=%v", err)
	}
}

// TestPersistModelAcceptsPluginRef: a plugin-namespaced ref picked in /model
// persists like any config ref — the config catalog cannot vouch for it, but
// boot's merged resolver gates it at the next launch.
func TestPersistModelAcceptsPluginRef(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("DARI_HARNESS_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}

	m := newTestChatTUI()
	m.persistModel("plugin/demo/fake/x")

	body, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(body), `default_model = "plugin/demo/fake/x"`) {
		t.Fatalf("saved config missing plugin default_model ref:\n%s", body)
	}
}

// TestMergeExtensionModelRefs pins the /model picker merge: extension
// descriptors join the config-backed list with their plugin/... refs intact,
// duplicates collapse, and a nil catalog leaves the base list untouched.
func TestMergeExtensionModelRefs(t *testing.T) {
	base := []string{"deepseek/deepseek-v4", "mimo/mimo-v2"}
	catalog := []provider.Descriptor{
		{Ref: "plugin/demo/fake/x"},
		{Ref: "plugin/demo/fake/y"},
		{Ref: "deepseek/deepseek-v4"}, // claim-replaced duplicate must not repeat
		{Ref: "  "},                   // blank entries are dropped
	}
	got := mergeExtensionModelRefs(base, catalog)
	want := []string{"deepseek/deepseek-v4", "mimo/mimo-v2", "plugin/demo/fake/x", "plugin/demo/fake/y"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("merge = %v, want %v", got, want)
	}
	if out := mergeExtensionModelRefs(base, nil); strings.Join(out, ",") != strings.Join(base, ",") {
		t.Fatalf("nil catalog changed the base list: %v", out)
	}
}

func TestModelPickerPresentationPreservesExtensionIdentity(t *testing.T) {
	descriptor := provider.Descriptor{
		Ref:         "plugin/demo/fake/x",
		DisplayName: "Demo X",
		Model:       "x",
	}
	descriptorsByRef := map[string]provider.Descriptor{descriptor.Ref: descriptor}
	label, description := modelPickerPresentation("plugin/demo/fake/x", descriptorsByRef)
	if label != "Demo X" || description != "Extension: plugin/demo/fake/x" {
		t.Fatalf("extension presentation = %q/%q", label, description)
	}

	label, description = modelPickerPresentation("plugin/demo/fake/y", descriptorsByRef)
	if label != "plugin/demo/fake/y" || description != "Extension model" {
		t.Fatalf("unknown extension presentation = %q/%q", label, description)
	}

	label, description = modelPickerPresentation("plugin/local-model", descriptorsByRef)
	if label != "local-model" || description != "Provider: plugin" {
		t.Fatalf("ordinary plugin-named provider presentation = %q/%q", label, description)
	}

	for _, malformed := range []string{"patty/", "/medium"} {
		label, description = modelPickerPresentation(malformed, descriptorsByRef)
		if label != malformed || description != "" {
			t.Fatalf("malformed ref %q presentation = %q/%q", malformed, label, description)
		}
	}
}
