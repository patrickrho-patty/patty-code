//go:build !profile_sovereign

package main

import (
	"fmt"

	"patty/internal/tier"
)

// Manifest endpoints — R2 CDN first, then the crash worker release gateway,
// then GitHub as the stable channel's last resort. The
// selected update channel picks the rolling pointer; it is user-configurable and
// independent from the build channel embedded for diagnostics/backcompat. The
// gateway still avoids GitHub's repository-wide /releases/latest shortcut so the
// app is not coupled to GitHub's homepage badge semantics.
const (
	r2Base             = "https://dl.patty.io"
	releaseGatewayBase = "https://crash.patty.io/v1/desktop/releases"
)

// githubManifestFallback is the stable channel's last-resort manifest source.
const githubManifestFallback = "https://github.com/pattycorp/PattyCode/releases/latest/download/latest.json"

// manifestEndpoints lists the update-manifest URLs for the channel (ADR G3:
// online update fetch exists only outside sovereign builds).
func manifestEndpoints(selected string) []string {
	tier.AssertAllowed(tier.CapOnlineUpdate)
	return []string{
		r2Base + "/latest/latest.json",
		releaseGatewayBase + "/stable/latest.json",
		githubManifestFallback,
	}
}

// desktopAssetBases lists the download URL prefixes for a release tag.
func desktopAssetBases(selected, version string, allowLegacyPreview bool) []string {
	tier.AssertAllowed(tier.CapOnlineUpdate)
	_ = selected
	_ = allowLegacyPreview
	tag := desktopReleaseTag(selected, version)
	return []string{
		fmt.Sprintf("%s/%s/", r2Base, tag),
		fmt.Sprintf("https://github.com/pattycorp/PattyCode/releases/download/%s/", tag),
		fmt.Sprintf("https://github.com/pattycorp/PattyCode/releases/download/%s/", version),
	}
}
