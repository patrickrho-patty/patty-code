package agent

import (
	"os"

	"patty/internal/dariproto"
	"patty/internal/dlp"
	"patty/internal/provider"
	"patty/internal/provider/dari"
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
	// C1.3: relay-pushed DLP rule packs reach the live scanner.
	// The dari provider decodes the pack; the sink applies the org's
	// class enables/disables to the built-in lexicon.
	if dp, ok := inner.(*dari.Provider); ok {
		dp.SetDLPRuleSink(func(pack *dariproto.DLPRulePackWire) {
			applyPackToScanner(scanner, pack)
		})
	}
	return dlp.NewProvider(inner, hook, inspector)
}

// applyPackToScanner disables built-in rule groups whose class the
// org turned off (or omitted) from its pack.
func applyPackToScanner(scanner *dlp.Scanner, pack *dariproto.DLPRulePackWire) {
	if scanner == nil || pack == nil {
		return
	}
	disabled := pack.DisabledRulePrefixes()
	if len(disabled) == 0 {
		return
	}
	for _, rule := range scanner.Rules() {
		if dariproto.MatchesPrefix(rule.RuleID, disabled) && !rule.Disabled {
			scanner.DisableRule(rule.RuleID)
		}
	}
}

// dlpEnabled reports whether the DLP wrapper should be active.
// Production deployments get DLP on by default. Tests that
// contain the detector's trigger phrases MUST disable DLP via
// the harness agent's test helper; the dedicated DLP scanner
// tests run the wrapper directly and don't need this escape
// hatch.
//
// DLP is ON BY DEFAULT (governed sessions must scan pre-send —
// harness plan C acceptance). PATTY_DLP_ENABLED=0 is the escape
// hatch for test binaries whose fixtures deliberately contain the
// detector's trigger phrases; affected packages set it in their
// TestMain.
func dlpEnabled() bool {
	if v := os.Getenv("PATTY_DLP_ENABLED"); v != "" {
		return v != "0"
	}
	return true
}
