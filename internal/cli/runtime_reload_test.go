package cli

import (
	"context"
	"errors"
	"testing"

	"patty/internal/boot"
	"patty/internal/control"
	"patty/internal/event"
)

// stubRuntimeRebuilder records how often the /reload build seam ran and
// returns a fixed result (or error).
func stubRuntimeRebuilder(res *boot.BuildResult, err error) (runtimeRebuilder, *int) {
	calls := new(int)
	return func(context.Context, controllerBuildSpec, *control.Controller) (*boot.BuildResult, error) {
		*calls++
		return res, err
	}, calls
}

func reloadTestModel(ctrl control.SessionAPI, rebuild runtimeRebuilder) *chatTUI {
	pending := []string{}
	return &chatTUI{ctrl: ctrl, rebuildRuntime: rebuild, pendingCommit: &pending}
}

// TestReloadRebuilderSeesLiveOverrides pins the pointer capture in
// bindRuntimeRebuilder: a runtime switch (/effort, /output-style) mutates the
// cliBuildOverrides the /reload seam must see, not the launch-time snapshot.
func TestReloadRebuilderSeesLiveOverrides(t *testing.T) {
	m := newTestChatTUI()
	overrides := cliBuildOverrides{OutputStyle: "explanatory"}
	var got cliBuildOverrides
	m.bindRuntimeRebuilder(10, nil, false, &overrides, func(_ string, _ int, _ bool, _ event.Sink, _ string, o cliBuildOverrides) boot.Options {
		got = o
		return boot.Options{}
	})
	// A runtime switch mutates the live overrides after the seam was bound.
	overrides.OutputStyle = "concise"
	effort := "max"
	overrides.Effort = &effort

	// Nil-controller BuildResult makes RebuildFrom fail fast after buildOpts
	// recorded the effective overrides.
	m.lastBuildResult = &boot.BuildResult{}
	old := control.New(control.Options{})
	if _, err := m.rebuildRuntime(context.Background(), controllerBuildSpec{ModelRef: "m"}, old); err == nil {
		t.Fatal("expected the stub rebuild to fail")
	}
	if got.OutputStyle != "concise" {
		t.Fatalf("reload carried OutputStyle = %q, want the live value concise", got.OutputStyle)
	}
	if got.Effort == nil || *got.Effort != "max" {
		t.Fatalf("reload carried Effort = %v, want the live value max", got.Effort)
	}
}

// TestReloadDispositionDecisionTable pins the /reload queue semantics: no
// rebuild seam reports unavailable, idle rebuilds now, and a turn/approval or
// runtime switch in flight queues exactly one reload.
func TestReloadDispositionDecisionTable(t *testing.T) {
	rebuilder, _ := stubRuntimeRebuilder(nil, errors.New("unused"))
	idleCtrl := func() *control.Controller { return control.New(control.Options{}) }

	// No rebuild seam (headless/test surface): unavailable even when idle.
	m := reloadTestModel(idleCtrl(), nil)
	if got := m.reloadDisposition(); got != reloadUnavailable {
		t.Fatalf("no rebuildRuntime: reloadDisposition = %v, want reloadUnavailable", got)
	}
	// Idle with a seam: rebuild immediately.
	m = reloadTestModel(idleCtrl(), rebuilder)
	if got := m.reloadDisposition(); got != reloadNow {
		t.Fatalf("idle: reloadDisposition = %v, want reloadNow", got)
	}
	// A runtime switch in flight queues.
	m.modelSwitchPending = true
	if got := m.reloadDisposition(); got != reloadQueued {
		t.Fatalf("switch pending: reloadDisposition = %v, want reloadQueued", got)
	}
	// Active work (here: a pending approval) queues too.
	m = reloadTestModel(idleCtrl(), rebuilder)
	m.pendingApproval = &event.Approval{}
	if got := m.reloadDisposition(); got != reloadQueued {
		t.Fatalf("pending approval: reloadDisposition = %v, want reloadQueued", got)
	}
}

// TestRunReloadCommandQueuesOnceAndDrainsWhenIdle covers the full busy → queue
// → idle → rebuild arc: exactly one queued reload, exactly one build, and the
// swap message carrying both controllers (the old one retires after the swap).
func TestRunReloadCommandQueuesOnceAndDrainsWhenIdle(t *testing.T) {
	oldCtrl := control.New(control.Options{Label: "old"})
	newCtrl := control.New(control.Options{Label: "new"})
	rebuilder, calls := stubRuntimeRebuilder(&boot.BuildResult{Controller: newCtrl}, nil)
	m := reloadTestModel(oldCtrl, rebuilder)
	m.modelSwitchPending = true

	if cmd := m.runReloadCommand(); cmd != nil {
		t.Fatal("busy /reload returned a cmd, want nil (queued instead)")
	}
	if !m.pendingReload {
		t.Fatal("busy /reload did not queue a reload")
	}
	// A second /reload while busy coalesces into the same queued reload.
	if cmd := m.runReloadCommand(); cmd != nil {
		t.Fatal("second busy /reload returned a cmd, want nil")
	}
	if *calls != 0 {
		t.Fatalf("rebuild ran %d times while busy, want 0", *calls)
	}

	// The switch settles; the drain schedules exactly one rebuild.
	m.modelSwitchPending = false
	cmd := m.drainQueuedRuntimeReload()
	if cmd == nil {
		t.Fatal("drainQueuedRuntimeReload returned nil, want the rebuild cmd")
	}
	if m.pendingReload {
		t.Fatal("queued reload flag survived the drain")
	}
	msg := cmd()
	swMsg, ok := msg.(modelSwitchMsg)
	if !ok {
		t.Fatalf("rebuild cmd returned %T, want modelSwitchMsg", msg)
	}
	if swMsg.err != nil {
		t.Fatalf("rebuild failed: %v", swMsg.err)
	}
	if swMsg.ctrl != newCtrl {
		t.Fatal("modelSwitchMsg did not carry the rebuilt controller")
	}
	if swMsg.oldCtrl != oldCtrl {
		t.Fatal("modelSwitchMsg did not carry the outgoing controller for the post-swap close")
	}
	if *calls != 1 {
		t.Fatalf("rebuild ran %d times, want exactly 1", *calls)
	}
}

// TestScheduleRuntimeReloadFailureKeepsOldController pins the fail-atomic
// contract: a failed build produces a "reload"-prefixed error message and
// leaves the running model on the old controller (the swap only happens in
// the modelSwitchMsg success branch).
func TestScheduleRuntimeReloadFailureKeepsOldController(t *testing.T) {
	oldCtrl := control.New(control.Options{Label: "old"})
	rebuilder, calls := stubRuntimeRebuilder(nil, errors.New("build exploded"))
	m := reloadTestModel(oldCtrl, rebuilder)

	cmd := m.scheduleRuntimeReload()
	if cmd == nil {
		t.Fatal("scheduleRuntimeReload returned nil cmd")
	}
	msg := cmd()
	swMsg, ok := msg.(modelSwitchMsg)
	if !ok {
		t.Fatalf("rebuild cmd returned %T, want modelSwitchMsg", msg)
	}
	if swMsg.err == nil {
		t.Fatal("failed build produced a nil error")
	}
	if swMsg.failurePrefix != "reload" {
		t.Fatalf("failurePrefix = %q, want %q", swMsg.failurePrefix, "reload")
	}
	if m.ctrl != oldCtrl {
		t.Fatal("failed reload replaced the controller before the swap handler ran")
	}
	if *calls != 1 {
		t.Fatalf("rebuild ran %d times, want 1", *calls)
	}
}

// TestRunReloadCommandWithoutSeamDoesNotQueue: a session with no rebuild seam
// reports unavailable instead of queueing a reload that could never drain.
func TestRunReloadCommandWithoutSeamDoesNotQueue(t *testing.T) {
	m := reloadTestModel(control.New(control.Options{}), nil)
	if cmd := m.runReloadCommand(); cmd != nil {
		t.Fatal("/reload without a rebuild seam returned a cmd")
	}
	if m.pendingReload {
		t.Fatal("/reload without a rebuild seam queued a reload")
	}
}
