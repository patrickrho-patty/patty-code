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
	newScopedPackSink(scanner)(pack)

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
	newScopedPackSink(scanner)(pack)

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
	newScopedPackSink(scanner)(pack)
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

// --- PAT-1432: scope cascade (Harness > User > Team > Org) ---

func scopedPack(level, id string, overrides ...dariproto.DLPRuleOverride) *dariproto.DLPRulePackWire {
	return &dariproto.DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		Scope:         dariproto.DLPRuleScope{Level: level, ID: id},
		RuleOverrides: overrides,
	}
}

func ruleState(scanner *dlp.Scanner, ruleID string) (enabled bool, severity string) {
	for _, r := range scanner.Rules() {
		if r.RuleID == ruleID {
			return !r.Disabled, string(r.Severity)
		}
	}
	return false, ""
}

func TestScopeCascadeUserOverridesOrg(t *testing.T) {
	scanner := dlp.NewScanner()
	sink := newScopedPackSink(scanner)

	// Org pack disables kr-phone via per-rule override.
	sink(scopedPack(dariproto.ScopeOrg, "org-1",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-phone", Enabled: false}))
	if _, sev := ruleState(scanner, "kr-phone"); sev == "" {
		t.Fatal("kr-phone must exist in the built-in lexicon")
	}
	if enabled, _ := ruleState(scanner, "kr-phone"); enabled {
		t.Fatal("org pack must disable kr-phone")
	}

	// User pack re-enables kr-phone: user outranks org.
	sink(scopedPack(dariproto.ScopeUser, "user-7",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-phone", Enabled: true}))
	if enabled, _ := ruleState(scanner, "kr-phone"); !enabled {
		t.Fatal("user pack must outrank org pack and re-enable kr-phone")
	}
	// Behavior-level proof: the phone rule fires again.
	res := scanner.Scan("연락처: 010-1234-5678")
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "kr-phone" {
			found = true
		}
	}
	if !found {
		t.Fatal("re-enabled kr-phone must produce a finding")
	}
}

func TestScopeCascadeHarnessOverridesUser(t *testing.T) {
	scanner := dlp.NewScanner()
	sink := newScopedPackSink(scanner)

	sink(scopedPack(dariproto.ScopeOrg, "org-1",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-rrn", Enabled: true, Severity: "critical"}))
	sink(scopedPack(dariproto.ScopeUser, "user-7",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-rrn", Enabled: false}))
	// Harness re-enables with lowered severity: harness outranks user.
	sink(scopedPack(dariproto.ScopeHarness, "peer-9",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-rrn", Enabled: true, Severity: "low"}))

	enabled, sev := ruleState(scanner, "kr-rrn")
	if !enabled {
		t.Fatal("harness pack must outrank user pack: kr-rrn enabled")
	}
	if sev != "low" {
		t.Fatalf("harness severity override must win, got %q", sev)
	}
	// The finding itself must carry the overridden severity. (The
	// overall verdict may still DENY via the broad kr-bank-account
	// regex matching the same string — that overlap is the separate
	// PAT-1396 tightening item, not a cascade concern.)
	res := scanner.Scan("주민등록번호: 901225-1234567")
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "kr-rrn" {
			found = true
			if string(f.Severity) != "low" {
				t.Fatalf("kr-rrn finding severity must be the harness override, got %q", f.Severity)
			}
		}
	}
	if !found {
		t.Fatal("kr-rrn must still produce a finding under the harness override")
	}
}

func TestScopeCascadeUnscopedPackIsOrg(t *testing.T) {
	scanner := dlp.NewScanner()
	sink := newScopedPackSink(scanner)

	// Pre-PAT-1432 shape: no Scope field at all.
	sink(&dariproto.DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		RuleOverrides: []dariproto.DLPRuleOverride{
			{RuleID: "pii-kr-phone", Enabled: false},
		},
	})
	sink(scopedPack(dariproto.ScopeTeam, "team-1",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-phone", Enabled: true}))
	if enabled, _ := ruleState(scanner, "kr-phone"); !enabled {
		t.Fatal("unscoped pack must rank as org; team pack must outrank it")
	}
}

func TestScopeCascadeIdempotentRepush(t *testing.T) {
	scanner := dlp.NewScanner()
	sink := newScopedPackSink(scanner)

	orgDisable := scopedPack(dariproto.ScopeOrg, "org-1",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-phone", Enabled: false})
	sink(orgDisable)
	// Simulate a re-push of the same org pack (e.g. reconnect).
	sink(orgDisable)
	if enabled, _ := ruleState(scanner, "kr-phone"); enabled {
		t.Fatal("re-push must keep kr-phone disabled (idempotent)")
	}
	// A later user pack flips it; re-pushing the ORG pack must NOT
	// clobber the user-level re-enable.
	sink(scopedPack(dariproto.ScopeUser, "user-7",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-phone", Enabled: true}))
	sink(orgDisable)
	if enabled, _ := ruleState(scanner, "kr-phone"); !enabled {
		t.Fatal("stale org re-push must not override user-level enable")
	}
}

func TestScopeCascadeRejectsUnknownLevel(t *testing.T) {
	scanner := dlp.NewScanner()
	sink := newScopedPackSink(scanner)
	before := scanner.Rules()

	sink(scopedPack("galactic", "x",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-rrn", Enabled: false}))
	after := scanner.Rules()
	for i := range before {
		if before[i].Disabled != after[i].Disabled {
			t.Fatalf("unknown scope level must be ignored, rule %s changed", before[i].RuleID)
		}
	}
}

func TestScopeCascadeClassFallbackPerLevel(t *testing.T) {
	// Class-level toggles still work per scope: org disables the
	// whole korean_pii class; user re-enables one rule by override.
	scanner := dlp.NewScanner()
	sink := newScopedPackSink(scanner)

	sink(&dariproto.DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		Rules: []dariproto.DLPRuleWire{
			{RuleID: "cls-pii", Pattern: "korean_pii", Severity: "critical", Disabled: true},
		},
	})
	if res := scanner.Scan("주민등록번호: 901225-1234567"); !res.Passed {
		t.Fatal("org class-level disable must silence kr-rrn")
	}
	sink(scopedPack(dariproto.ScopeUser, "user-7",
		dariproto.DLPRuleOverride{RuleID: "pii-kr-rrn", Enabled: true}))
	if res := scanner.Scan("주민등록번호: 901225-1234567"); res.Passed {
		t.Fatal("user override must re-enable kr-rrn on top of org class disable")
	}
	// Sibling rule stays disabled (only the named rule was re-enabled).
	if res := scanner.Scan("연락처: 010-1234-5678"); !res.Passed {
		t.Fatal("kr-phone must remain disabled by the org class toggle")
	}
}
