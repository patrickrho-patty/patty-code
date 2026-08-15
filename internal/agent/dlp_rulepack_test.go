package agent

import (
	"testing"

	"patty/internal/dariproto"
	"patty/internal/dlp"
)

// dlp_rulepack_test.go pins the relay class→connector rule-ID mapping
// against the REAL built-in lexicon (the "korean-"/"bearer-" prefix
// bug matched zero rules and silently disabled nothing).
func TestClassPrefixesMatchRealRuleIDs(t *testing.T) {
	scanner := dlp.NewScanner()
	if len(scanner.Rules()) < 10 {
		t.Fatalf("built-in lexicon suspiciously small: %d rules", len(scanner.Rules()))
	}
	for class, prefixes := range map[string][]string{
		// mirrors dariproto.classPrefixes (unexported); kept literal so
		// drift between the map and the lexicon fails HERE.
		"korean_pii":       {"kr-"},
		"secret":           {"aws-", "private-key", "generic-bearer-"},
		"prompt_injection": {"injection-"},
	} {
		for _, p := range prefixes {
			hit := false
			for _, r := range scanner.Rules() {
				if dariproto.MatchesPrefix(r.RuleID, []string{p}) {
					hit = true
					break
				}
			}
			if !hit {
				t.Errorf("class %s prefix %q matches ZERO built-in rule IDs", class, p)
			}
		}
	}
}

func TestDisabledPrefixesActuallyDisableScannerRules(t *testing.T) {
	scanner := dlp.NewScanner()
	pack := &dariproto.DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		Rules: []dariproto.DLPRuleWire{
			{RuleID: "cls-sec", Pattern: "secret", Severity: "critical"},
			{RuleID: "cls-pii", Pattern: "korean_pii", Severity: "critical", Disabled: true},
		},
	}
	applyPackToScanner(scanner, pack)
	for _, r := range scanner.Rules() {
		if dariproto.MatchesPrefix(r.RuleID, pack.DisabledRulePrefixes()) && !r.Disabled {
			t.Fatalf("rule %s must be disabled after applying the pack", r.RuleID)
		}
	}
	// Korean PII must now PASS (disabled class), secrets still BLOCK.
	if res := scanner.Scan("주민등록번호 900101-1234567 입니다"); !res.Passed {
		t.Fatalf("korean_pii disabled but RRN still blocked: %+v", res)
	}
	if res := scanner.Scan("AKIAIOSFODNN7EXAMPLE secret w/ aws-secret-key abcdefghijklmnopqrstuvwxyz1234567890ABCD"); res.Passed {
		t.Fatal("secret class still enabled and must block")
	}
}
