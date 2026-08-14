package dari

import (
	"context"
	"testing"
)

// TestPrewarmWithoutConnectionFailsClean covers the F2 boundary: on a
// fresh provider with no relay reachable, Prewarm surfaces the dial
// failure (no panic, no silent success).
func TestPrewarmWithoutConnectionFailsClean(t *testing.T) {
	p := NewForTest(&testConfig{RelayAddr: "127.0.0.1:1", Model: "m"})
	if err := p.Prewarm(context.Background()); err == nil {
		t.Fatal("prewarm against an unreachable relay must fail")
	}
}
