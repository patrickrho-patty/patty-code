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
		"secret":           {"aws-", "private-key", "generic-bearer-", "gcp-", "azure-", "ncloud-", "gitlab-", "openai-", "slack-webhook", "mysql-", "postgres-", "redis-"},
		"prompt_injection": {"injection-"},
		"sensitive_path":   {"path-"},
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

func TestPerRuleOverridesTakePrecedence(t *testing.T) {
	// PAT-1431: per-rule overrides must take precedence over class-level toggles.
	scanner := dlp.NewScanner()
	pack := &dariproto.DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		Rules: []dariproto.DLPRuleWire{
			{RuleID: "cls-pii", Pattern: "korean_pii", Severity: "critical", Disabled: false},
		},
		RuleOverrides: []dariproto.DLPRuleOverride{
			{RuleID: "pii-kr-phone", Enabled: false, Severity: "high", Action: "block"},
			{RuleID: "pii-kr-rrn", Enabled: true, Severity: "critical", Action: "block"},
		},
	}
	applyPackToScanner(scanner, pack)

	// Per-rule override disabled kr-phone: scanning a phone number must not
	// find a kr-phone finding. Pre-existing broad kr-bank-account regex also
	// matches phone numbers (from original PAT-1396 note), so we assert
	// specifically that kr-phone is absent from the findings list.
	res := scanner.Scan("연락처: 010-1234-5678")
	hasPhone := false
	for _, f := range res.Findings {
		if f.RuleID == "kr-phone" {
			hasPhone = true
			break
		}
	}
	if hasPhone {
		t.Fatalf("kr-phone should be disabled by per-rule override, got findings %v", res.Findings)
	}
	// kr-rrn should still be enabled: scanning an RRN must find it.
	resRRN := scanner.Scan("주민등록번호: 901225-1234567")
	foundRRN := false
	for _, f := range resRRN.Findings {
		if f.RuleID == "kr-rrn" {
			foundRRN = true
		}
	}
	if !foundRRN {
		t.Fatalf("kr-rrn should remain enabled by per-rule override, got findings %v", resRRN.Findings)
	}
}

func TestClassLevelFallbackWhenNoOverrides(t *testing.T) {
	// Backward compat: when no per-rule overrides exist, fall back to class-level.
	scanner := dlp.NewScanner()
	pack := &dariproto.DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		Rules: []dariproto.DLPRuleWire{
			{RuleID: "cls-pii", Pattern: "korean_pii", Severity: "critical", Disabled: true},
		},
		// No RuleOverrides
	}
	applyPackToScanner(scanner, pack)

	// All kr-* rules should be disabled (class-level toggle)
	if res := scanner.Scan("주민등록번호: 901225-1234567"); !res.Passed {
		t.Fatal("kr-rrn should be disabled by class-level toggle")
	}
	if res := scanner.Scan("연락처: 010-1234-5678"); !res.Passed {
		t.Fatal("kr-phone should be disabled by class-level toggle")
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
