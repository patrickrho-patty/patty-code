//go:build !profile_sovereign

package installsource

import "patty/internal/tier"

// githubAPIBaseURL is the GitHub Contents API base URL used by
// fetchGitHubContents (ADR G5). Configured at build time so non-sovereign
// profiles can fetch from the public API; sovereign builds use the empty
// default in plan_github_sovereign.go and must be pointed at an internal
// mirror via runtime configuration.
func init() {
	tier.AssertAllowed(tier.CapOnlineUpdate)
	githubAPIBaseURL = "https://api.github.com"
}