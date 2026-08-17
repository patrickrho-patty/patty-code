//go:build profile_sovereign

package installsource

import "patty/internal/tier"

// defaultGitHubAPIBaseURL returns ErrGitHubFetchUnavailable in sovereign
// builds — air-gapped installs must configure an internal mirror via
// runtime configuration (ADR G5). The compile-time gate is tighter than
// the old var-and-init pattern: under this profile the GitHub fetch is
// simply unreachable, and the call site wraps the sentinel with the
// operator-facing message about install.github_mirror.
func defaultGitHubAPIBaseURL() (string, error) {
	tier.AssertDisallowed(tier.CapOnlineUpdate)
	return "", ErrGitHubFetchUnavailable
}
