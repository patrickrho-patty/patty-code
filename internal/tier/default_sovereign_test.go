//go:build profile_sovereign

package tier

import "testing"

func TestDefaultTierIsSovereign(t *testing.T) {
	if Default != Sovereign {
		t.Fatalf("Default = %v, want Sovereign", Default)
	}
}
