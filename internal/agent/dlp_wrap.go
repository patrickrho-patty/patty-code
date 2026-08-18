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
	// C1.3 + PAT-1432: relay-pushed DLP rule packs reach the live
	// scanner. The dari provider decodes each pack; the sink keeps
	// the latest pack per scope level and re-applies the full
	// cascade (Harness > User > Team > Org) from a pristine snapshot
	// of the built-in lexicon, so re-pushes are idempotent and more
	// specific scopes win.
	if dp, ok := inner.(*dari.Provider); ok {
		dp.SetDLPRuleSink(newScopedPackSink(scanner))
	}
	return dlp.NewProvider(inner, hook, inspector)
}

// newScopedPackSink returns the DLP pack consumer installed on the
// dari provider. Each arriving pack is filed under its effective
// scope level (absent scope = org); the scanner is then rebuilt from
// the pristine built-in rules with all collected packs applied in
// ascending specificity (org first, harness last), so the most
// specific scope's overrides win.
func newScopedPackSink(scanner *dlp.Scanner) func(*dariproto.DLPRulePackWire) {
	pristine := scanner.Rules() // built-in lexicon before any pack
	packs := make(map[string]*dariproto.DLPRulePackWire)
	return func(pack *dariproto.DLPRulePackWire) {
		if pack == nil {
			return
		}
		level := pack.Scope.EffectiveLevel()
		if dariproto.ScopeRank(level) < 0 {
			return // unknown scope level: refuse to apply
		}
		packs[level] = pack
		applyScopedPacks(scanner, pristine, packs)
	}
}

// scopeCascadeOrder lists the scope levels in ascending specificity —
// application order for the cascade (most specific wins).
var scopeCascadeOrder = []string{
	dariproto.ScopeOrg, dariproto.ScopeTeam, dariproto.ScopeUser, dariproto.ScopeHarness,
}

// applyScopedPacks resets the scanner to the pristine lexicon and
// applies the collected packs in ascending scope rank (org → team →
// user → harness). Later (more specific) packs overwrite earlier
// (less specific) ones for the rules they name.
func applyScopedPacks(scanner *dlp.Scanner, pristine []dlp.DetectionRule, packs map[string]*dariproto.DLPRulePackWire) {
	if scanner == nil {
		return
	}
	// Fresh copy of the built-in rules: cascade application must not
	// inherit disables from a previous push.
	merged := make([]dlp.DetectionRule, len(pristine))
	for i, r := range pristine {
		merged[i] = r
	}
	for _, level := range scopeCascadeOrder {
		if pack := packs[level]; pack != nil {
			applyPackToRules(merged, pack)
		}
	}
	scanner.SetRules(merged)
}

// applyPackToRules applies one pack's policy onto a rule slice in
// place: per-rule overrides (PAT-1431) when present — including the
// override's severity — else the class-level disabled prefixes
// (backward compat).
func applyPackToRules(rules []dlp.DetectionRule, pack *dariproto.DLPRulePackWire) {
	if pack == nil {
		return
	}
	if len(pack.RuleOverrides) > 0 {
		for _, override := range pack.RuleOverrides {
			harnessIDs := relayRuleIDToHarnessIDs(override.RuleID)
			for _, id := range harnessIDs {
				for i := range rules {
					if rules[i].RuleID != id {
						continue
					}
					rules[i].Disabled = !override.Enabled
					if sev := dlp.ParseSeverity(override.Severity); sev != "" {
						rules[i].Severity = sev
					}
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
	for i := range rules {
		if dariproto.MatchesPrefix(rules[i].RuleID, disabled) {
			rules[i].Disabled = true
		}
	}
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
