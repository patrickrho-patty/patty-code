//go:build profile_sovereign

package telemetry

import "testing"

func TestSovereignProfileDisablesTelemetry(t *testing.T) {
	if Enabled("on", "1.0.0", true) {
		t.Fatal("sovereign builds must never enable vendor telemetry regardless of mode")
	}
	if got := (&Client{}).post(nil, "/ping", nil); got != nil {
		t.Fatalf("sovereign post must be a no-op, got %v", got)
	}
}
