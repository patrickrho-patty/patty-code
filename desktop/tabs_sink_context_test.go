package main

import (
	"context"
	"sync"
	"testing"

	"patty/internal/event"
)

// All tabEventSink context mutations go through the locked setContext
// emitting — emitRuntimeEvent sees a nil ctx and no-ops — and the queued
// emitter is drained, so a detached/backgrounded session can't flush stale
// events onto the now-rebound tab (#5352: stale "AI가 계속 출력 중" on the visible
func TestTabEventSinkClearContextStopsEmission(t *testing.T) {
	var mu sync.Mutex
	var emitted int
	s := &tabEventSink{tabID: "t"}
	s.runtimeEvents.emit = func(context.Context, string, ...any) {
		mu.Lock()
		emitted++
		mu.Unlock()
	}

	s.setContext(context.Background())
	if s.context() == nil {
		t.Fatal("setContext did not install the context")
	}

	s.clearContext()
	if s.context() != nil {
		t.Fatal("clearContext did not clear the context")
	}

	s.emitRuntimeEvent(eventChannel, toWireTab(event.Event{}, s.tabID))

	mu.Lock()
	defer mu.Unlock()
	if emitted != 0 {
		t.Fatalf("sink emitted %d events after clearContext, want 0", emitted)
	}
}
