//go:build !profile_public && !profile_sovereign

package tier

import (
	"strings"
	"testing"
)

func TestDefaultTierIsEnterprise(t *testing.T) {
	if Default != Enterprise {
		t.Fatalf("Default = %v, want Enterprise", Default)
	}
}

// The enterprise leg of the audit-trail panic coverage: enterprise ALLOWS
// CapVendorTelemetry, so AssertDisallowed must panic here — the loud
// failure mode that surfaces a wrong-tag-set programmer error.
func TestAssertDisallowedPanicsWhenEnterpriseAllowsCapability(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("AssertDisallowed(CapVendorTelemetry) did not panic under enterprise — audit-trail regression")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, CapVendorTelemetry.String()) {
			t.Fatalf("panic message %v does not name the capability", r)
		}
	}()
	AssertDisallowed(CapVendorTelemetry)
}
