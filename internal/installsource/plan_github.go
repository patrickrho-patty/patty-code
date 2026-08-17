//go:build !profile_sovereign

package installsource

import "patty/internal/tier"

// defaultGitHubAPIBaseURL is the GitHub Contents API base URL used by
// fetchGitHubContents (ADR G5). Non-sovereign profiles fetch from the
// public API; sovereign builds return ErrGitHubFetchUnavailable from
// the build-tagged twin in plan_github_sovereign.go.
//
// NOTE: this consults CapOnlineUpdate (G3) for what is operationally a G5
// row (plugin/extension sources). The verdicts coincide today (sovereign
// excludes both); when G5 lands with per-profile source allowlists, split
// this into CapPublicPluginSources — see ADR gate inventory.
func defaultGitHubAPIBaseURL() (string, error) {
	tier.AssertAllowed(tier.CapOnlineUpdate)
	return "https://api.github.com", nil
}
