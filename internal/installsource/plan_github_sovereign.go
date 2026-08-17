//go:build profile_sovereign

package installsource

import "patty/internal/tier"

// githubAPIBaseURL is empty in sovereign builds — air-gapped installs must
// configure internal mirrors via runtime configuration (ADR G5). With the
// default empty, fetchGitHubContents fails closed (URL parse succeeds but
// the subsequent HTTP fetch rejects the hostless URL).
func init() {
	tier.AssertDisallowed(tier.CapOnlineUpdate)
	githubAPIBaseURL = ""
}