//go:build profile_public

// Public-surface desktop tests (ADR G4): every test here needs generic
// OpenAI/Anthropic-kind provider fixtures (BYOK flows, official-provider
// surfaces, model/effort/token-mode rebuilds against generic endpoints) or
// another public-only capability (e.g. balance fetch). The enterprise and
// sovereign tier locks refuse those configs at boot by design, so these
// assertions compile and run only in the public-profile leg (see Makefile
// test-profiles).
package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"patty/internal/agent"
	"patty/internal/boot"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/provider"
)

// TestSetModelForTabPluginRefReachesBoot: with a controller whose merged
// catalog declares the plugin ref, SetModelForTab's config gate accepts the
// ref and hands it to boot — which is the resolver of record (here it fails
// only because no sidecar is installed in this test's PATTY_HOME). A
// non-plugin unknown ref must still fail at the desktop gate, before boot.
func TestSetModelForTabPluginRefReachesBoot(t *testing.T) {
	dir := reloadRuntimeFixture(t)

	oldExec := agent.New(nil, nil, agent.NewSession("old system prompt"), agent.Options{}, event.Discard)
	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	oldCtrl := control.New(control.Options{
		Executor:   oldExec,
		SessionDir: dir,
		Label:      "old",
		Sink:       event.Discard,
		// The merged catalog this controller would expose with the demo
		// sidecar live: plugin/demo/fake/x is a valid switch target.
		ProviderResolver: &provider.StaticResolver{Descriptors: []provider.Descriptor{
			{Ref: "plugin/demo/fake/x", Model: "x"},
		}},
	})
	reloadRuntimeTab(t, app, dir, oldCtrl)

	err := app.SetModelForTab("tab_a", "plugin/demo/fake/x")
	if err == nil {
		t.Fatal("SetModelForTab succeeded without an installed sidecar; expected boot to gate the plugin ref")
	}
	if !errors.Is(err, boot.ErrUnknownModel) {
		t.Fatalf("SetModelForTab error = %v, want boot.ErrUnknownModel (the desktop gate must pass plugin refs through to boot)", err)
	}

	// The desktop config gate still rejects unknown NON-plugin refs itself.
	err = app.SetModelForTab("tab_a", "ghost/never-existed")
	if err == nil || !strings.Contains(err.Error(), `unknown model "ghost/never-existed"`) {
		t.Fatalf("SetModelForTab non-plugin error = %v, want the desktop unknown-model gate", err)
	}
	if errors.Is(err, boot.ErrUnknownModel) {
		t.Fatalf("non-plugin unknown ref reached boot: %v", err)
	}
}
