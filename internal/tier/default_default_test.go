//go:build !profile_public && !profile_sovereign

package tier

import "testing"

func TestDefaultTierIsEnterprise(t *testing.T) {
	if Default != Enterprise {
		t.Fatalf("Default = %v, want Enterprise", Default)
	}
}
