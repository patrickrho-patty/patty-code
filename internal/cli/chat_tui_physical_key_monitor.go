package cli

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
)

const physicalBackspacePollInterval = 5 * time.Millisecond

const physicalBackspaceEvidenceTTL = 500 * time.Millisecond

// physicalBackspaceMonitor records down edges near the OS input boundary.
// Kitty's legacy stream can turn the last Korean-IME Backspace into a plain
// compatibility jamo, and tmux discards Kitty's associated-text metadata. A
// durable edge count survives the terminal/PTY/UI scheduling delay that makes
// a one-off CGEventSourceKeyState sample unreliable.
type physicalBackspaceMonitor struct {
	sample            func() bool
	interval          time.Duration
	stopCh            chan struct{}
	activityCh        chan struct{}
	stopOnce          sync.Once
	presses           atomic.Uint64
	lastPressUnixNano atomic.Int64
	active            atomic.Bool
	down              atomic.Bool
}

func newPhysicalBackspaceMonitor(sample func() bool, interval time.Duration) *physicalBackspaceMonitor {
	monitor := &physicalBackspaceMonitor{
		sample:     sample,
		interval:   interval,
		stopCh:     make(chan struct{}),
		activityCh: make(chan struct{}, 1),
	}
	monitor.active.Store(true)
	return monitor
}

func (m *physicalBackspaceMonitor) observe(down bool) {
	if !m.active.Load() {
		m.down.Store(false)
		return
	}
	wasDown := m.down.Swap(down)
	if down && !wasDown {
		m.lastPressUnixNano.Store(time.Now().UnixNano())
		m.presses.Add(1)
	}
}

func (m *physicalBackspaceMonitor) pressCount() uint64 {
	if m == nil {
		return 0
	}
	return m.presses.Load()
}

func (m *physicalBackspaceMonitor) sampleDown() bool {
	return m != nil && m.active.Load() && m.sample != nil && m.sample()
}

func (m *physicalBackspaceMonitor) cachedDown() bool {
	return m != nil && m.active.Load() && m.down.Load()
}

func (m *physicalBackspaceMonitor) hasRecentPress(now time.Time) bool {
	if m == nil {
		return false
	}
	pressedAt := m.lastPressUnixNano.Load()
	if pressedAt == 0 {
		return false
	}
	age := now.UnixNano() - pressedAt
	return age >= 0 && age <= physicalBackspaceEvidenceTTL.Nanoseconds()
}

func (m *physicalBackspaceMonitor) setActive(active bool) {
	if m == nil || m.active.Swap(active) == active {
		return
	}
	select {
	case m.activityCh <- struct{}{}:
	default:
	}
}

func (m *physicalBackspaceMonitor) poll() {
	if !m.active.Load() {
		m.observe(false)
		return
	}
	m.observe(m.sample())
}

func (m *physicalBackspaceMonitor) command() tea.Cmd {
	if m == nil || m.sample == nil {
		return nil
	}
	return func() tea.Msg {
		var ticker *time.Ticker
		var tick <-chan time.Time
		defer func() {
			if ticker != nil {
				ticker.Stop()
			}
		}()
		for {
			if m.active.Load() && ticker == nil {
				m.poll()
				ticker = time.NewTicker(m.interval)
				tick = ticker.C
			} else if !m.active.Load() && ticker != nil {
				ticker.Stop()
				ticker = nil
				tick = nil
				m.observe(false)
			}
			select {
			case <-m.stopCh:
				return nil
			case <-m.activityCh:
				continue
			case <-tick:
				m.poll()
			}
		}
	}
}

func (m *physicalBackspaceMonitor) stop() {
	if m != nil {
		m.stopOnce.Do(func() { close(m.stopCh) })
	}
}

func isKoreanKeyboardInputSource(id string) bool {
	return strings.HasPrefix(id, "com.apple.inputmethod.Korean.")
}

func isKittyHangulResidualCandidate(msg tea.KeyPressMsg) bool {
	key := msg.Key()
	if key.Mod != 0 {
		return false
	}
	text := []rune(key.Text)
	return len(text) == 1 && text[0] >= '\u3131' && text[0] <= '\u314e'
}

// capturePhysicalBackspaceEvidence is called once for every terminal key
// event, before modal routing. Non-candidate keys consume old edges so a later
// intentional jamo cannot be mistaken for an IME residual.
func (m *chatTUI) capturePhysicalBackspaceEvidence(msg tea.KeyPressMsg) bool {
	monitor := m.physicalBackspaceMonitor
	if monitor == nil {
		return false
	}
	candidate := isKittyHangulResidualCandidate(msg)
	key := msg.Key()
	down := monitor.cachedDown()
	if candidate || key.Code == tea.KeyBackspace {
		// A live sample closes the gap before the next poll, but only the two
		// relevant event shapes pay for a native CoreGraphics call.
		down = monitor.sampleDown()
	}
	count := monitor.pressCount()
	if candidate && m.physicalBackspaceAwaitingRelease && !down {
		// A candidate was already suppressed from the live key level before the
		// polling goroutine recorded its edge. Retire that late count now.
		m.physicalBackspaceConsumed = count
		m.physicalBackspaceAwaitingRelease = false
	}
	recentRecordedPress := count > m.physicalBackspaceConsumed && monitor.hasRecentPress(time.Now())
	evidence := candidate && (down || recentRecordedPress)
	m.physicalBackspaceConsumed = count
	m.physicalBackspaceAwaitingRelease = down
	return evidence
}

func (m *chatTUI) discardPhysicalBackspaceEvidence() {
	monitor := m.physicalBackspaceMonitor
	if monitor == nil {
		return
	}
	m.physicalBackspaceConsumed = monitor.pressCount()
	m.physicalBackspaceAwaitingRelease = false
}

func (m *chatTUI) applyKittyHangulIMEEnvironment(compatible, behindTmux bool) tea.Cmd {
	m.kittyHangulIMECompatibility = compatible
	m.kittyHangulIMEBehindTmux = behindTmux
	if !behindTmux {
		m.physicalBackspaceMonitor.stop()
		m.physicalBackspaceMonitor = nil
		m.keyboardInputSourceID = nil
		m.physicalBackspaceConsumed = 0
		m.physicalBackspaceAwaitingRelease = false
		return nil
	}
	if m.physicalBackspaceMonitor != nil {
		m.physicalBackspaceMonitor.setActive(true)
		m.discardPhysicalBackspaceEvidence()
		return nil
	}
	prewarmKeyboardCompatibilityAPIs()
	if !physicalBackspaceStateAvailable() {
		m.keyboardInputSourceID = nil
		return nil
	}
	m.keyboardInputSourceID = currentKeyboardInputSourceID
	m.physicalBackspaceMonitor = newPhysicalBackspaceMonitor(
		currentPhysicalBackspaceDown,
		physicalBackspacePollInterval,
	)
	m.discardPhysicalBackspaceEvidence()
	return m.physicalBackspaceMonitor.command()
}
