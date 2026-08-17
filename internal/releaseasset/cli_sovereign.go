//go:build profile_sovereign

package releaseasset

import (
	"context"
	"errors"
	"net/http"

	"patty/internal/tier"
)

// ErrDownloadUnavailable is returned by DownloadCLI in sovereign builds
// (ADR G3): remote CLI asset downloads from vendor/GitHub hosts are
// compiled out; air-gapped installs use offline media.
var ErrDownloadUnavailable = errors.New("remote CLI asset download is not available in this build profile")

// DownloadCLI is the sovereign fail-closed twin (ADR G3): the GitHub release
// fetch is compiled out of air-gapped builds; callers surface the sentinel.
func DownloadCLI(ctx context.Context, client *http.Client, version, goos, goarch string) ([]byte, error) {
	tier.AssertDisallowed(tier.CapOnlineUpdate)
	_ = ctx
	_ = client
	_ = version
	_ = goos
	_ = goarch
	return nil, ErrDownloadUnavailable
}
