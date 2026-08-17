//go:build !profile_sovereign

package installsource

import "patty/internal/tier"

// defaultGitHubAPIBaseURL is the GitHub Contents API base URL used by
// fetchGitHubContents (ADR G5). Non-sovereign profiles fetch from the
// public API; sovereign builds return ErrGitHubFetchUnavailable from
// the build-tagged twin in plan_github_sovereign.go.
func defaultGitHubAPIBaseURL() (string, error) {
	tier.AssertAllowed(tier.CapOnlineUpdate)
	return "https://api.github.com", nil
}
