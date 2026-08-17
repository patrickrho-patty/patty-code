//go:build profile_sovereign

package main

import (
	"context"
	"testing"

	"patty/internal/config"
)

// ADR G3 in the desktop module: the online update channel — manifest
// endpoints and download asset bases — is compiled out of sovereign builds.
func TestSovereignUpdaterHasNoOnlineChannel(t *testing.T) {
	if got := manifestEndpoints("stable"); got != nil {
		t.Fatalf("manifestEndpoints = %v, want nil in sovereign builds", got)
	}
	if got := desktopAssetBases("stable", "v1.0.0", false); got != nil {
		t.Fatalf("desktopAssetBases = %v, want nil in sovereign builds", got)
	}
	c, err := httpClient()
	if err != nil {
		t.Fatalf("httpClient: %v", err)
	}
	if _, err := fetchManifest(context.Background(), c, nil, "stable"); err != ErrUpdateUnavailable {
		t.Fatalf("fetchManifest err = %v, want ErrUpdateUnavailable", err)
	}
	// The tier-aware default keeps the updater dormant at launch (ADR G3).
	cfg := config.Default()
	if cfg.DesktopCheckUpdates() {
		t.Fatal("DesktopCheckUpdates default = true, want false in sovereign builds")
	}
}
