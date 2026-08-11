package cli

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/control"
	"patty/internal/event"
)

func TestIMEKeyboardCompatibilityCommandDisablesModifyOtherKeys(t *testing.T) {
	cmd := imeKeyboardCompatibilityCommand()
	if cmd == nil {
		t.Fatal("IME keyboard compatibility returned no command")
	}
	msg := cmd()
	raw, ok := msg.(tea.RawMsg)
	if !ok {
		t.Fatalf("IME keyboard compatibility command = %T, want tea.RawMsg", msg)
	}
	if got := raw.Msg; got != ansi.ResetModifyOtherKeys {
		t.Fatalf("startup keyboard compatibility sequence = %q, want %q", got, ansi.ResetModifyOtherKeys)
	}
}

func TestPhysicalBackspaceMonitorRecordsOnlyDownEdges(t *testing.T) {
	monitor := newPhysicalBackspaceMonitor(func() bool { return false }, time.Hour)
	for _, down := range []bool{false, true, true, false, false, true} {
		monitor.observe(down)
	}
	if got := monitor.pressCount(); got != 2 {
		t.Fatalf("physical Backspace press count = %d, want 2", got)
	}
}

func TestPhysicalBackspaceMonitorCommandCanBeStopped(t *testing.T) {
	monitor := newPhysicalBackspaceMonitor(func() bool { return false }, time.Hour)
	cmd := monitor.command()
	monitor.stop()
	done := make(chan struct{})
	go func() {
		_ = cmd()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("physical Backspace monitor did not stop")
	}
}

func TestPhysicalBackspaceMonitorCapturesAReleasedPress(t *testing.T) {
	var down atomic.Bool
	monitor := newPhysicalBackspaceMonitor(down.Load, time.Millisecond)
	done := make(chan struct{})
	go func() {
		_ = monitor.command()()
		close(done)
	}()
	t.Cleanup(func() {
		monitor.stop()
		<-done
	})

	down.Store(true)
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for monitor.pressCount() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("monitor missed a physical Backspace press")
		case <-time.After(time.Millisecond):
		}
	}
	down.Store(false)

	// The level is gone before the terminal key is handled, but the recorded
	// edge remains available to the compatibility decision.
	if monitor.sampleDown() {
		t.Fatal("test Backspace should already be released")
	}
	if got := monitor.pressCount(); got != 1 {
		t.Fatalf("released Backspace press count = %d, want 1", got)
	}
}

func TestPhysicalBackspaceMonitorSleepsWithoutSamplingWhileInactive(t *testing.T) {
	var samples atomic.Int64
	monitor := newPhysicalBackspaceMonitor(func() bool {
		samples.Add(1)
		return false
	}, time.Millisecond)
	monitor.setActive(false)
	done := make(chan struct{})
	go func() {
		_ = monitor.command()()
		close(done)
	}()
	t.Cleanup(func() {
		monitor.stop()
		<-done
	})

	time.Sleep(10 * time.Millisecond)
	if got := samples.Load(); got != 0 {
		t.Fatalf("inactive monitor sampled physical key state %d times", got)
	}
	monitor.setActive(true)
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for samples.Load() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("reactivated monitor did not resume sampling")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestCurrentKeyboardInputSourceIsAvailableOnMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS TIS is Darwin-only")
	}
	if got := currentKeyboardInputSourceID(); got == "" {
		t.Fatal("current macOS keyboard input source ID is empty")
	}
}

func TestPhysicalBackspaceStateIsAvailableOnMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS HID key state is Darwin-only")
	}
	if !physicalBackspaceStateAvailable() {
		t.Fatal("macOS physical Backspace state API is unavailable")
	}
	_ = currentPhysicalBackspaceDown()
}

func TestKoreanInputSourceMatchesAllKoreanLayouts(t *testing.T) {
	for _, id := range []string{
		korean2SetInputSourceID,
		"com.apple.inputmethod.Korean.3SetKorean",
		"com.apple.inputmethod.Korean.GongjinCheongRomaja",
	} {
		if !isKoreanKeyboardInputSource(id) {
			t.Errorf("Korean input source %q was not recognized", id)
		}
	}
	if isKoreanKeyboardInputSource("com.apple.keylayout.ABC") {
		t.Fatal("ABC layout was recognized as Korean")
	}
}

func TestKittyTmuxEnvironmentChangeStartsAndStopsBackspaceMonitor(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	next, cmd := m.update(kittyHangulIMEEnvironmentMsg{compatible: true, behindTmux: true, determined: true})
	m = next.(chatTUI)
	if !physicalBackspaceStateAvailable() {
		if m.physicalBackspaceMonitor != nil || cmd != nil {
			t.Fatal("unsupported platform started the physical Backspace monitor")
		}
		return
	}
	if m.physicalBackspaceMonitor == nil || m.keyboardInputSourceID == nil {
		t.Fatal("Kitty tmux environment did not install native compatibility inputs")
	}
	if cmd == nil {
		t.Fatal("Kitty tmux environment did not start the Backspace monitor")
	}
	monitor := m.physicalBackspaceMonitor

	next, cmd = m.update(kittyHangulIMEEnvironmentMsg{determined: true})
	m = next.(chatTUI)
	if m.physicalBackspaceMonitor != nil || m.keyboardInputSourceID != nil {
		t.Fatal("leaving Kitty tmux kept native compatibility inputs")
	}
	if cmd != nil {
		t.Fatal("leaving Kitty tmux unexpectedly started a command")
	}
	select {
	case <-monitor.stopCh:
	default:
		t.Fatal("leaving Kitty tmux did not stop the Backspace monitor")
	}
}

func TestChatTUIFocusRefreshesCurrentTmuxClient(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	_, cmd := m.update(tea.FocusMsg{})
	if cmd == nil {
		t.Fatal("focus did not request a fresh terminal environment probe")
	}
}

func TestStaleKittyEnvironmentProbeIsIgnored(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	m.kittyHangulIMEProbeGeneration = 2
	next, cmd := m.update(kittyHangulIMEEnvironmentMsg{
		compatible: true, behindTmux: true, generation: 1, determined: true,
	})
	m = next.(chatTUI)
	if cmd != nil || m.kittyHangulIMECompatibility || m.physicalBackspaceMonitor != nil {
		t.Fatal("stale focus probe changed the Kitty compatibility environment")
	}
}

func TestBlurPausesPhysicalBackspaceCapture(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	var samples atomic.Int64
	monitor := newPhysicalBackspaceMonitor(func() bool {
		samples.Add(1)
		return true
	}, time.Hour)
	m.physicalBackspaceMonitor = monitor
	next, _ := m.update(tea.BlurMsg{})
	m = next.(chatTUI)
	monitor.poll()
	if monitor.pressCount() != 0 || monitor.active.Load() || samples.Load() != 0 {
		t.Fatal("blurred TUI continued recording global Backspace presses")
	}
}

func TestBlurInvalidatesInFlightKittyEnvironmentProbe(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := newPhysicalBackspaceMonitor(func() bool { return false }, time.Hour)
	m.physicalBackspaceMonitor = monitor

	next, _ := m.update(tea.FocusMsg{})
	m = next.(chatTUI)
	focusGeneration := m.kittyHangulIMEProbeGeneration
	next, _ = m.update(tea.BlurMsg{})
	m = next.(chatTUI)
	next, cmd := m.update(kittyHangulIMEEnvironmentMsg{
		compatible: true, behindTmux: true, generation: focusGeneration, determined: true,
	})
	m = next.(chatTUI)
	if cmd != nil || monitor.active.Load() {
		t.Fatal("late focus probe resumed physical key capture after blur")
	}
}

func TestFailedKittyEnvironmentProbePreservesCurrentState(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	monitor := newPhysicalBackspaceMonitor(func() bool { return false }, time.Hour)
	m.kittyHangulIMECompatibility = true
	m.kittyHangulIMEBehindTmux = true
	m.physicalBackspaceMonitor = monitor
	m.kittyHangulIMEProbeGeneration = 3

	next, cmd := m.update(kittyHangulIMEEnvironmentMsg{generation: 3})
	m = next.(chatTUI)
	if cmd != nil || !m.kittyHangulIMECompatibility || !m.kittyHangulIMEBehindTmux ||
		m.physicalBackspaceMonitor != monitor || !monitor.active.Load() {
		t.Fatal("failed tmux probe changed the current Kitty compatibility state")
	}
}

func TestOrdinaryKeyDoesNotSynchronouslySamplePhysicalBackspace(t *testing.T) {
	m := newComposerMouseTestTUI(t, 40, 12)
	var samples atomic.Int64
	m.physicalBackspaceMonitor = newPhysicalBackspaceMonitor(func() bool {
		samples.Add(1)
		return false
	}, time.Hour)

	m = updateComposerMouseTestTUI(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if got := samples.Load(); got != 0 {
		t.Fatalf("ordinary key made %d synchronous native key-state calls", got)
	}
}

func TestChatTUIResumeDisablesModifyOtherKeysForIME(t *testing.T) {
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 40)
	_, cmd := m.Update(tea.ResumeMsg{})
	if cmd == nil {
		t.Fatal("chat TUI resume returned no keyboard compatibility command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("chat TUI resume command did not batch keyboard reset and environment probe")
	}
	foundReset := false
	foundProbe := false
	for _, batchedCmd := range batch {
		switch msg := batchedCmd().(type) {
		case tea.RawMsg:
			foundReset = msg.Msg == ansi.ResetModifyOtherKeys
		case kittyHangulIMEEnvironmentMsg:
			foundProbe = true
		}
	}
	if !foundReset || !foundProbe {
		t.Fatalf("resume batch reset=%v probe=%v, want both", foundReset, foundProbe)
	}
}
