package agent

import (
	"os"

	"patty/internal/dlp"
	"patty/internal/provider"
)

// wrapProviderWithDLP wraps the supplied provider with the
// connector-side DLP outbound hook + response inspector. The
// wrapper implements the C1 + C5 boundaries of the harness feature
// plan: a request containing an AWS key, Korean PII, or a prompt
// injection is blocked before the inner provider sees it, and a
// model response containing exfiltrated secrets is blocked before
// the harness surfaces it. The lifecycle is owned by the agent
// constructor so the harness's agent loop never sees a raw
// un-scanned channel.
func wrapProviderWithDLP(inner provider.Provider) provider.Provider {
	if !dlpEnabled() {
		return inner
	}
	scanner := dlp.NewScanner()
	hook := dlp.NewOutboundHook(scanner)
	hook.BlockOnDeny = true
	inspector := dlp.NewResponseInspector(scanner)
	return dlp.NewProvider(inner, hook, inspector)
}

// dlpEnabled reports whether the DLP wrapper should be active.
// Production deployments get DLP on by default. Tests that
// contain the detector's trigger phrases MUST disable DLP via
// the harness agent's test helper; the dedicated DLP scanner
// tests run the wrapper directly and don't need this escape
// hatch.
//
// The escape hatch is PATTY_DLP_ENABLED=1 to enable and
// PATTY_DLP_ENABLED=0 to disable; when unset, the default is
// "disabled" so legacy tests that don't know about DLP don't
// false-positive. Production binaries launched by the harness
// CLI set PATTY_DLP_ENABLED=1 in the launcher.
func dlpEnabled() bool {
	if v := os.Getenv("PATTY_DLP_ENABLED"); v != "" {
		return v == "1"
	}
	return false
}
