//go:build profile_sovereign

package crashreport

import (
	"context"
	"errors"

	"patty/internal/netclient"
	"patty/internal/tier"
)

// ErrSendUnavailable is returned by Send in sovereign builds (ADR G2):
// stack traces must not leave the enclave; local list/show/delete remain.
var ErrSendUnavailable = errors.New("crash report upload is not available in this build profile — inspect locally with `patcode report show`")

// Send is the sovereign fail-closed twin (ADR G2): stack traces must not
// leave the enclave; local list/show/delete remain available.
func Send(ctx context.Context, report Report, proxy netclient.ProxySpec) error {
	tier.AssertDisallowed(tier.CapCrashUpload)
	_ = ctx
	_ = report
	_ = proxy
	return ErrSendUnavailable
}
