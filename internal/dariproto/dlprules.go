package dariproto

import (
	"errors"
	"fmt"
	"strings"
)

// dlprules.go is the connector-side decode of the relay's DLP rule
// pack (harness plan C1.3). The wire struct mirrors the relay's
// internal/relay/dari_wire.go wireDLPRulePack field-for-field; the
// cross-repo conformance suite pins the bytes.
//
// The relay's rules carry CLASSES (korean_pii / secret /
// prompt_injection / sensitive_path) — the detection regexes live in
// each side's engine. The connector maps the org's enabled/disabled
// classes onto its built-in lexicon so both sides enforce the same
// policy surface.

// DLPRuleWire is one synced detection rule (mirrors the relay).
type DLPRuleWire struct {
	RuleID     string `cbor:"1,keyasint"`
	Pattern    string `cbor:"2,keyasint"`
	Severity   string `cbor:"3,keyasint"`
	RedactWith string `cbor:"4,keyasint,omitempty"`
	Disabled   bool   `cbor:"5,keyasint,omitempty"`
}

// DLPRulePackWire is the epoch-bound pack push (mirrors the relay).
type DLPRulePackWire struct {
	Version    uint16        `cbor:"1,keyasint"`
	EpochID    string        `cbor:"2,keyasint"`
	OrgID      string        `cbor:"3,keyasint"`
	NotAfterMs int64         `cbor:"4,keyasint"`
	Rules      []DLPRuleWire `cbor:"5,keyasint"`
	Digest     [32]byte      `cbor:"6,keyasint"`
	// RuleOverrides carries per-rule enabled/severity/action overrides.
	// When present, these take precedence over class-level toggles.
	RuleOverrides []DLPRuleOverride `cbor:"7,keyasint,omitempty"`
	// Scope names the administrative level this pack applies to
	// (PAT-1432). Absent scope = org (byte-compatible with the
	// pre-scope wire format; field is omitempty).
	Scope DLPRuleScope `cbor:"8,keyasint,omitempty"`
}

// DLPRuleOverride is a per-rule override (PAT-1431).
type DLPRuleOverride struct {
	RuleID   string `cbor:"1,keyasint"`
	Enabled  bool   `cbor:"2,keyasint"`
	Severity string `cbor:"3,keyasint"`
	Action   string `cbor:"4,keyasint"`
}

// DLP scope levels (PAT-1432). Precedence when multiple scoped
// packs arrive: Harness > User > Team > Organization.
const (
	ScopeOrg     = "org"
	ScopeTeam    = "team"
	ScopeUser    = "user"
	ScopeHarness = "harness"
)

// DLPRuleScope names the administrative level a rule pack applies
// to. ID is the level's subject: the org ID, team ID, user ID, or
// harness peer ID. A zero Scope (or Level == "") means org —
// pre-PAT-1432 packs decode that way and stay byte-compatible.
type DLPRuleScope struct {
	Level string `cbor:"1,keyasint"`
	ID    string `cbor:"2,keyasint"`
}

// EffectiveLevel normalizes an absent level to org.
func (s DLPRuleScope) EffectiveLevel() string {
	if s.Level == "" {
		return ScopeOrg
	}
	return s.Level
}

// scopeRank orders levels by specificity (higher wins). Unknown
// levels sort below org so a malformed pack can never shadow an
// explicit org pack.
var scopeRank = map[string]int{
	ScopeOrg:     0,
	ScopeTeam:    1,
	ScopeUser:    2,
	ScopeHarness: 3,
}

// ScopeRank reports the precedence rank of a level (org lowest,
// harness highest; unknown levels rank below org).
func ScopeRank(level string) int {
	if r, ok := scopeRank[level]; ok {
		return r
	}
	return -1
}

// DecodeDLPRulePack parses a DLP_RULE_PACK body.
func DecodeDLPRulePack(data []byte) (*DLPRulePackWire, error) {
	if len(data) == 0 {
		return nil, errors.New("dari: empty DLP rule pack body")
	}
	var pack DLPRulePackWire
	if err := UnmarshalCBOR(data, &pack); err != nil {
		return nil, fmt.Errorf("dari: decode DLP rule pack: %w", err)
	}
	if pack.EpochID == "" {
		return nil, errors.New("dari: DLP rule pack missing epoch binding")
	}
	return &pack, nil
}

// classPrefixes maps the relay's rule classes onto the connector's
// built-in lexicon rule-ID prefixes.
var classPrefixes = map[string][]string{
	"korean_pii":       {"kr-"},
	"secret":           {"aws-", "private-key", "generic-bearer-", "gcp-", "azure-", "ncloud-", "gitlab-", "openai-", "slack-webhook", "mysql-", "postgres-", "redis-"},
	"prompt_injection": {"injection-"},
	"sensitive_path":   {"path-"},
}

// DisabledRulePrefixes derives the built-in rule-ID prefixes the org
// has disabled from the pack (a class present with Disabled=true, or a
// class entirely absent from an otherwise non-empty pack, disables
// its prefix group).
func (p *DLPRulePackWire) DisabledRulePrefixes() []string {
	if p == nil || len(p.Rules) == 0 {
		return nil
	}
	present := map[string]bool{}
	disabled := map[string]bool{}
	for _, r := range p.Rules {
		present[r.Pattern] = true
		if r.Disabled {
			disabled[r.Pattern] = true
		}
	}
	var out []string
	for class, prefixes := range classPrefixes {
		if len(prefixes) == 0 {
			continue
		}
		if disabled[class] || !present[class] {
			out = append(out, prefixes...)
		}
	}
	return out
}

// IsClassEnabled reports whether the pack enables a rule class.
func (p *DLPRulePackWire) IsClassEnabled(class string) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Rules {
		if r.Pattern == class {
			return !r.Disabled
		}
	}
	return false
}

// Describe renders the pack for the audit log.
func (p *DLPRulePackWire) Describe() string {
	if p == nil {
		return "<nil pack>"
	}
	return fmt.Sprintf("dlp-pack epoch=%s org=%s rules=%d", p.EpochID, p.OrgID, len(p.Rules))
}

// MatchesPrefix reports whether a connector rule ID falls under one of
// the prefixes (case-insensitive).
func MatchesPrefix(ruleID string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(ruleID), p) {
			return true
		}
	}
	return false
}
