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
	"io"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"patty/internal/agent"
	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/jobs"
)

// TestReloadRuntimeSwapsAndClosesOldAfterSwap covers the success path through
// boot.Rebuild: the tab's controller is replaced, the session (grants, file)
// migrates, the outgoing controller is closed only after the swap published
// the replacement, and the frontend fence is emitted.
func TestReloadRuntimeSwapsAndClosesOldAfterSwap(t *testing.T) {
	dir := reloadRuntimeFixture(t)

	oldPath := filepath.Join(dir, "old.jsonl")
	oldExec := agent.New(nil, nil, agent.NewSession("old system prompt"), agent.Options{}, event.Discard)
	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	var fenceMu sync.Mutex
	var fenceNames []string
	app.runtimeEvents.emit = func(_ context.Context, name string, _ ...any) {
		fenceMu.Lock()
		fenceNames = append(fenceNames, name)
		fenceMu.Unlock()
	}

	closed := false
	var ctrlAtClose control.SessionAPI
	oldCtrl := control.New(control.Options{
		Executor:    oldExec,
		SessionDir:  dir,
		SessionPath: oldPath,
		Label:       "old",
		Sink:        event.Discard,
		Cleanup: func() {
			closed = true
			app.mu.RLock()
			ctrlAtClose = app.tabs["tab_a"].Ctrl
			app.mu.RUnlock()
		},
	})
	oldCtrl.RestoreSessionAuthorizations(control.SessionAuthorizations{
		Grants:                   []string{"bash|go test ./..."},
		PlanModeReadOnlyCommands: []string{"go test ./..."},
	})
	tab := reloadRuntimeTab(t, app, dir, oldCtrl)

	if err := app.ReloadRuntime(tab.ID); err != nil {
		t.Fatalf("ReloadRuntime: %v", err)
	}

	newCtrl, ok := tab.Ctrl.(*control.Controller)
	if !ok {
		t.Fatalf("tab.Ctrl = %T, want *control.Controller", tab.Ctrl)
	}
	if newCtrl == oldCtrl {
		t.Fatal("ReloadRuntime kept the outgoing controller installed")
	}
	if got := newCtrl.SessionPath(); got != oldPath {
		t.Fatalf("session path = %q, want the carried session file %q", got, oldPath)
	}
	got := newCtrl.SessionAuthorizations()
	if len(got.Grants) != 1 || got.Grants[0] != "bash|go test ./..." {
		t.Fatalf("migrated grants = %+v, want [\"bash|go test ./...\"]", got.Grants)
	}
	if !closed {
		t.Fatal("outgoing controller was not closed")
	}
	if ctrlAtClose != newCtrl {
		t.Fatal("outgoing controller closed before the swap published the replacement")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		fenceMu.Lock()
		found := slices.Contains(fenceNames, "runtime:rebuilt")
		fenceMu.Unlock()
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no runtime:rebuilt fence emitted")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestReloadRuntimeBusyQueuesDeferred covers the busy contract: active work
// queues exactly one reload on the deferred-rebuild loop (coalesced), and the
// retry runs the boot.Rebuild reload once the tab is idle.
func TestReloadRuntimeBusyQueuesDeferred(t *testing.T) {
	dir := reloadRuntimeFixture(t)

	jm := jobs.NewManager(event.Discard)
	releaseJob := make(chan struct{})
	jm.Start("test", "blocking job", func(ctx context.Context, _ io.Writer) (string, error) {
		select {
		case <-releaseJob:
			return "", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	oldCtrl := control.New(control.Options{
		SessionDir:  dir,
		SessionPath: filepath.Join(dir, "old.jsonl"),
		Label:       "old",
		Sink:        event.Discard,
		Jobs:        jm,
	})
	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	tab := reloadRuntimeTab(t, app, dir, oldCtrl)

	if err := app.ReloadRuntime(tab.ID); err != nil {
		t.Fatalf("busy ReloadRuntime returned %v, want nil (queued)", err)
	}
	if tab.Ctrl != oldCtrl {
		t.Fatal("busy reload swapped the controller")
	}
	if !app.deferredRebuildPending(tab.ID) {
		t.Fatal("busy reload was not queued on the deferred loop")
	}
	// A second request while busy coalesces into the same queued reload.
	if err := app.ReloadRuntime(tab.ID); err != nil {
		t.Fatalf("second busy ReloadRuntime returned %v, want nil", err)
	}
	app.deferredRebuild.mu.Lock()
	label, ok := app.deferredRebuild.pending[tab.ID]
	count := len(app.deferredRebuild.pending)
	app.deferredRebuild.mu.Unlock()
	if !ok || label != deferredRuntimeReloadLabel {
		t.Fatalf("pending label = %q (present=%t), want %q", label, ok, deferredRuntimeReloadLabel)
	}
	if count != 1 {
		t.Fatalf("pending entries = %d, want exactly 1 (coalesced)", count)
	}

	// The work finishes; the retry path reloads the now-idle tab.
	close(releaseJob)
	deadline := time.Now().Add(2 * time.Second)
	for oldCtrl.RuntimeStatus().BackgroundJobs > 0 {
		if time.Now().After(deadline) {
			t.Fatal("background job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	app.retryDeferredRuntimeReload(tab.ID, tab)
	if tab.Ctrl == oldCtrl {
		t.Fatal("deferred retry did not reload the idle tab")
	}
	if app.deferredRebuildPending(tab.ID) {
		t.Fatal("deferred entry survived a successful retry")
	}
}
