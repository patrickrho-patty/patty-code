package agent

import (
	"os"
	"strings"

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

// relayRuleIDToHarnessIDs maps a relay RuleID to the set of harness
// RuleIDs that should be toggled. The relay's per-rule override
// semantically names a specific detection (e.g. "pii-kr-phone"). The
// harness's built-in lexicon uses shorter names (e.g. "kr-phone").
// korean_pii relays use the "pii-*" prefix that the harness drops;
// secret/injection/path classes share the suffix exactly.
func relayRuleIDToHarnessIDs(ruleID string) []string {
	// "pii-*" -> strip "pii-" prefix and pass through directly
	if strings.HasPrefix(ruleID, "pii-") {
		return []string{strings.TrimPrefix(ruleID, "pii-")}
	}
	// Explicit name differences for secrets: relay uses
	// "secret-aws-key" / "secret-github-pat" / "secret-jwt" etc.;
	// harness uses "aws-access-key" / "aws-secret-key" / "generic-bearer-token" etc.
	switch ruleID {
	case "secret-aws-key":
		return []string{"aws-access-key", "aws-secret-key"}
	case "secret-github-pat":
		return []string{"generic-bearer-token"} // harness has no dedicated github rule
	case "secret-jwt":
		return []string{"generic-bearer-token"}
	case "secret-private-key":
		return []string{"private-key-pem"}
	case "secret-generic-api-key":
		return []string{"generic-bearer-token"}
	case "secret-gcp-key", "secret-azure-key", "secret-ncloud-key",
		"secret-gitlab-token", "secret-openai-key", "secret-slack-webhook",
		"secret-mysql-connstring", "secret-postgres-connstring", "secret-redis-connstring":
		return nil // harness doesn't have dedicated rules for these; skip
	}
	// injection-* and path-* match harness IDs by suffix
	if strings.HasPrefix(ruleID, "injection-") || strings.HasPrefix(ruleID, "path-") {
		return []string{ruleID}
	}
	return nil
}

// applyPackToScanner applies the relay's DLP rule pack to the scanner.
// It handles both class-level toggles (backward compat) and per-rule
// overrides (PAT-1431). Per-rule overrides take precedence.
func applyPackToScanner(scanner *dlp.Scanner, pack *dariproto.DLPRulePackWire) {
	if scanner == nil || pack == nil {
		return
	}

	// Per-rule overrides (PAT-1431): translate relay RuleIDs to the
	// harness's built-in lexicon, then toggle the matched rules.
	if len(pack.RuleOverrides) > 0 {
		for _, override := range pack.RuleOverrides {
			harnessIDs := relayRuleIDToHarnessIDs(override.RuleID)
			for _, id := range harnessIDs {
				if override.Enabled {
					scanner.EnableRule(id)
				} else {
					scanner.DisableRule(id)
				}
			}
		}
		return // per-rule overrides applied; skip class-level fallback
	}

	// Fallback: class-level toggles (backward compat).
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
