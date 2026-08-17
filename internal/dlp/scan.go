// Package dlp is the connector-side data-loss prevention scanner.
// It runs on the harness before DARI dispatch (PRD §16.3, §16.5)
// so secrets and PII never leave the host. The detection logic
// mirrors the relay's `internal/security/service.go::CheckContext`
// so the harness and the control plane agree on what constitutes
// a finding; the harness also subscribes to the relay's per-org
// rule-pack via the policy epoch (A4) so admin toggles take
// effect inline.
package dlp

import (
	"regexp"
	"strings"
	"sync"
)

// Severity mirrors the relay's SecurityFinding severity values.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Verdict mirrors the relay's CheckContext verdict values.
type Verdict string

const (
	VerdictAllow         Verdict = "ALLOW"
	VerdictDeny          Verdict = "DENY"
	VerdictRequireReview Verdict = "REQUIRE_REVIEW"
)

// Finding is a single DLP detection. The connector surfaces the
// findings to the harness UI and the audit log.
type Finding struct {
	RuleID   string
	Severity Severity
	// Offset is the byte index in the scanned text where the
	// finding starts. The connector uses it to highlight the
	// finding in the harness UI.
	Offset int
	// Length is the byte length of the matched content. The
	// connector uses it to highlight the finding and to redact
	// the matched content from outbound messages.
	Length int
	// Description is a short human-readable label. The relay and
	// the connector MUST agree on this string for stable UX.
	Description string
	// Sample is a redacted preview of the matched content. The
	// connector never logs the raw content; the relay rule pack
	// controls how much is shown.
	Sample string
}

// ScanResult is the connector-side verdict for one outbound
// message. The harness consults this before every AI_OPEN.
type ScanResult struct {
	Passed   bool
	Verdict  Verdict
	Findings []Finding
	// RedactedText is the input text with the matched content
	// replaced by redaction tokens. Empty if no redaction is
	// required.
	RedactedText string
}

// DetectionRule is a single regex-based scanner. The connector
// ships a built-in lexicon (Korean PII + secrets + injection)
// and accepts per-org rule overrides via the policy epoch.
type DetectionRule struct {
	RuleID      string
	Severity    Severity
	Regex       *regexp.Regexp
	Description string
	// RedactTemplate replaces matched content. The default
	// `[$1_REDACTED]` keeps the rule name visible to the model
	// without leaking the matched value.
	RedactTemplate string
	// Disabled controls whether the rule fires. The relay's
	// per-org admin toggles flip this; tests use it to gate
	// edge-case detectors.
	Disabled bool
}

// --- Rule definitions are in rules_*.go ---

// Scanner is the harness-side DLP engine. It owns the active
// rule pack and runs every outbound message through it before
// DARI dispatch.
type Scanner struct {
	mu    sync.RWMutex
	rules []DetectionRule
}

// NewScanner constructs a scanner with the connector's built-in
// lexicon (Korean PII + secrets + injection + sensitive paths).
func NewScanner() *Scanner {
	return &Scanner{
		rules: append(append(append(defaultKoreanPIIRules(), defaultSecretRules()...), defaultInjectionRules()...), defaultSensitivePathRules()...),
	}
}

// DefaultKoreanPIIRules returns the connector's built-in Korean
// PII detector lexicon. The lexicon covers the most common
// Korean government identifiers (PRD §16.3) and is intentionally
// minimal — production deployments override the lexicon via the
// relay's rule pack.
func DefaultKoreanPIIRules() []DetectionRule { return defaultKoreanPIIRules() }

// DefaultSecretRules returns the connector's built-in secret
// detector lexicon (AWS keys, generic API tokens, private keys).
func DefaultSecretRules() []DetectionRule { return defaultSecretRules() }

// DefaultInjectionRules returns the connector's built-in
// prompt-injection detector (PRD §16.4). The detector flags
// canonical override/jailbreak phrases the relay's
// `detectInjection` also catches.
func DefaultInjectionRules() []DetectionRule { return defaultInjectionRules() }

// Rules returns a copy of the active rules for diagnostics.
func (s *Scanner) Rules() []DetectionRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DetectionRule, len(s.rules))
	copy(out, s.rules)
	return out
}

// SetRules replaces the rule pack. The harness calls this when
// the relay pushes a refreshed rule pack via the policy epoch
// (A4 + C1.3).
func (s *Scanner) SetRules(rules []DetectionRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = append([]DetectionRule(nil), rules...)
}

// DisableRule marks a rule as disabled by ID. The relay's
// per-org admin toggles flow through this.
func (s *Scanner) DisableRule(ruleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rules {
		if s.rules[i].RuleID == ruleID {
			s.rules[i].Disabled = true
		}
	}
}

// EnableRule marks a rule as enabled by ID.
func (s *Scanner) EnableRule(ruleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rules {
		if s.rules[i].RuleID == ruleID {
			s.rules[i].Disabled = false
		}
	}
}

// Scan runs the rule pack against the supplied text. The verdict
// is the most restrictive decision across all findings:
//   - any critical or high -> DENY
//   - any medium -> REQUIRE_REVIEW
//   - otherwise ALLOW
//
// The method returns a copy of the input text with each finding's
// matched bytes replaced by the rule's redaction template.
func (s *Scanner) Scan(text string) ScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var findings []Finding
	verdict := VerdictAllow
	mutated := []byte(text)
	// Apply redactions right-to-left so byte offsets remain
	// valid through the mutation loop.
	type pendingRedaction struct {
		offset int
		length int
		redact string
	}
	pending := make([]pendingRedaction, 0)

	for _, rule := range s.rules {
		if rule.Disabled {
			continue
		}
		matches := rule.Regex.FindAllStringIndex(text, -1)
		for _, m := range matches {
			findings = append(findings, Finding{
				RuleID:      rule.RuleID,
				Severity:    rule.Severity,
				Offset:      m[0],
				Length:      m[1] - m[0],
				Description: rule.Description,
				Sample:      redactForSample(text[m[0]:m[1]]),
			})
			if rule.RedactTemplate != "" {
				pending = append(pending, pendingRedaction{
					offset: m[0],
					length: m[1] - m[0],
					redact: rule.RedactTemplate,
				})
			}
			if rule.Severity == SeverityCritical || rule.Severity == SeverityHigh {
				verdict = VerdictDeny
			} else if rule.Severity == SeverityMedium {
				if verdict == VerdictAllow {
					verdict = VerdictRequireReview
				}
			}
		}
	}

	// Apply redactions right-to-left to preserve offsets.
	// Skip overlapping redactions (earlier rule wins).
	for i := len(pending) - 1; i >= 0; i-- {
		p := pending[i]
		if p.offset+p.length > len(mutated) {
			continue // overlapping match, skip
		}
		mutated = append(mutated[:p.offset], append([]byte(p.redact), mutated[p.offset+p.length:]...)...)
	}

	return ScanResult{
		Passed:       verdict == VerdictAllow,
		Verdict:      verdict,
		Findings:     findings,
		RedactedText: string(mutated),
	}
}

// redactForSample returns a privacy-preserving preview of the
// matched content. The connector never logs the raw content; the
// preview shows the prefix and length only.
func redactForSample(s string) string {
	const previewLen = 4
	if len(s) <= previewLen {
		return strings.Repeat("*", len(s))
	}
	return s[:previewLen] + strings.Repeat("*", len(s)-previewLen)
}
