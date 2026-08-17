//go:build profile_public

package tier

import "testing"

func TestDefaultTierIsPublic(t *testing.T) {
	if Default != Public {
		t.Fatalf("Default = %v, want Public", Default)
	}
}
