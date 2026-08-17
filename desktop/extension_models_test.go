package main

import (
	"strings"
	"testing"

	"patty/internal/control"
	"patty/internal/provider"
)

// extensionCatalogCtrl stubs only what the model discovery/switch paths read
// from a tab controller; everything else falls through the embedded nil
// SessionAPI and must not be called.
type extensionCatalogCtrl struct {
	control.SessionAPI
	catalog []provider.Descriptor
}

func (c *extensionCatalogCtrl) ProviderCatalog() []provider.Descriptor { return c.catalog }

// TestModelsForTabIncludesExtensionProviders: the desktop model switcher
// merges the tab controller's extension catalog into the config-backed list —
// plugin refs arrive fully namespaced, duplicates collapse, and the active
// plugin ref is marked current.
func TestModelsForTabIncludesExtensionProviders(t *testing.T) {
	reloadRuntimeFixture(t) // one configured provider "old" with model old-model

	app := NewApp()
	tab := &WorkspaceTab{
		ID:    "tab_a",
		Scope: "global",
		Ready: true,
		model: "plugin/demo/fake/x",
		Ctrl: &extensionCatalogCtrl{catalog: []provider.Descriptor{
			{Ref: "plugin/demo/fake/x", Model: "x"},
			{Ref: "plugin/demo/fake/y", Model: "y"},
			{Ref: "old/old-model"}, // claim-replaced duplicate must not repeat
		}},
		disabledMCP: map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	models := app.ModelsForTab(tab.ID)
	var pluginX, pluginY, oldCount *ModelInfo
	oldSeen := 0
	for i := range models {
		switch models[i].Ref {
		case "plugin/demo/fake/x":
			pluginX = &models[i]
		case "plugin/demo/fake/y":
			pluginY = &models[i]
		case "old/old-model":
			oldSeen++
			oldCount = &models[i]
		}
	}
	if pluginX == nil || pluginY == nil {
		t.Fatalf("ModelsForTab = %+v, want both plugin refs", models)
	}
	if pluginX.Provider != "plugin/demo" || pluginX.Model != "fake/x" {
		t.Fatalf("plugin ref display = %+v, want provider plugin/demo model fake/x", pluginX)
	}
	if !pluginX.Current {
		t.Fatalf("active plugin ref not marked current: %+v", pluginX)
	}
	if oldSeen != 1 || oldCount == nil {
		t.Fatalf("config ref repeated %d times, want exactly one", oldSeen)
	}

	// A tab whose controller has no extension catalog (nil → no sidecar
	// declared providers) enumerates config only, as before.
	tab.Ctrl = &extensionCatalogCtrl{}
	for _, info := range app.ModelsForTab(tab.ID) {
		if strings.HasPrefix(info.Ref, "plugin/") {
			t.Fatalf("empty catalog produced plugin ref %+v", info)
		}
	}
}
