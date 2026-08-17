//go:build !profile_sovereign

// Permissive-policy assertions (ADR G2): telemetry may be enabled
// only in profiles that allow CapVendorTelemetry. The sovereign expectation
// (Enabled always false) lives in sovereign_test.go.

package telemetry

import "testing"

func TestEnabledPolicyOnlineProfile(t *testing.T) {
	clearPolicyEnv(t)
	if !Enabled("auto", "v1.20.0", true) {
		t.Fatal("auto should enable a release build on an interactive terminal")
	}
	if !Enabled("on", "v1.20.0", false) {
		t.Fatal("on should permit a local headless release run")
	}
}
