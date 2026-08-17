//go:build profile_sovereign

package main

import (
	"context"
	"errors"
	"net/http"

	"patty/internal/tier"
)

// crashEndpoint is empty in sovereign builds (ADR G2): the vendor crash
// collector must not appear in the binary even as a string.
var crashEndpoint = ""

// errSovereignCrashPost is returned by postCrashReport in sovereign builds
// (ADR G2): stack traces must not leave the enclave; queued frontend and
// native crash reports stay local and are surfaced as failed uploads.
var errSovereignCrashPost = errors.New("crash report upload is not available in this build profile")

// postCrashReport is the sovereign no-op twin: it never dials out and always
// fails closed. Callers already treat a non-nil error as a failed upload
// (frontend reports surface the error; flushPendingCrash keeps the file).
func postCrashReport(ctx context.Context, c *http.Client, endpoint string, r crashReport) error {
	tier.AssertDisallowed(tier.CapCrashUpload)
	_ = ctx
	_ = c
	_ = endpoint
	_ = r
	return errSovereignCrashPost
}
