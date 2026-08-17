package dlp

import "regexp"

// rules_korean_pii.go — Korean PII detection rules (PIPA Article 24-2).
// These rules mirror the relay's authoritative catalog (internal/security).
// Rule IDs use the harness convention (kr-*) so the relay's class-level
// toggles map via classPrefixes["korean_pii"] = {"kr-"}.
func defaultKoreanPIIRules() []DetectionRule {
	return []DetectionRule{
		// --- existing rules (kr-rrn, kr-brn, kr-bank-account, kr-rrn-keyword) ---
		{
			RuleID:         "kr-rrn",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`\b\d{6}[-‐]?[1-4]\d{6}\b`),
			Description:    "Korean resident registration number",
			RedactTemplate: "[KR_RRN_REDACTED]",
		},
		{
			RuleID:         "kr-brn",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`\b\d{3}[-‐]?\d{2}[-‐]?\d{5}\b`),
			Description:    "Korean business registration number",
			RedactTemplate: "[KR_BRN_REDACTED]",
		},
		{
			RuleID:         "kr-bank-account",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`\b\d{2,4}[-‐]?\d{2,4}[-‐]?\d{2,6}[-‐]?\d{0,2}\b`),
			Description:    "Korean bank account number",
			RedactTemplate: "[KR_BANK_ACCOUNT_REDACTED]",
		},
		{
			RuleID:         "kr-rrn-keyword",
			Severity:       SeverityLow,
			Regex:          regexp.MustCompile(`주민등록번호|주민번호`),
			Description:    "Korean resident registration number keyword",
			RedactTemplate: "[KR_RRN_KEYWORD_REDACTED]",
		},

		// --- new rules (PAT-1429 mirror) ---
		{
			RuleID:         "kr-foreign-rrn",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`\b\d{6}[-‐]?[5-8]\d{6}\b`),
			Description:    "Korean foreign resident registration number",
			RedactTemplate: "[KR_FOREIGN_RRN_REDACTED]",
		},
		{
			RuleID:         "kr-driver-license",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`\b\d{2}[-‐]\d{2}[-‐]\d{6}[-‐]\d{2}\b`),
			Description:    "Korean driver's license number",
			RedactTemplate: "[KR_DRIVER_LICENSE_REDACTED]",
		},
		{
			RuleID:         "kr-passport",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`\b[A-Z]\d{8}\b`),
			Description:    "Korean passport number",
			RedactTemplate: "[KR_PASSPORT_REDACTED]",
		},
		{
			RuleID:         "kr-phone",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`\b01[016789][-‐]?\d{3,4}[-‐]?\d{4}\b`),
			Description:    "Korean mobile phone number",
			RedactTemplate: "[KR_PHONE_REDACTED]",
		},
		{
			RuleID:         "kr-phone-landline",
			Severity:       SeverityMedium,
			Regex:          regexp.MustCompile(`\b0[2-6][1-5]?[-‐]?\d{3,4}[-‐]?\d{4}\b`),
			Description:    "Korean landline number",
			RedactTemplate: "[KR_LANDLINE_REDACTED]",
		},
		{
			RuleID:         "kr-credit-card",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`\b(?:4\d{3}|5[1-5]\d{2}|3[47]\d{2}|6(?:011|5\d{2}))[-‐]?\d{4}[-‐]?\d{4}[-‐]?\d{4}\b`),
			Description:    "Credit card number",
			RedactTemplate: "[KR_CREDIT_CARD_REDACTED]",
		},
		{
			RuleID:         "kr-health-insurance",
			Severity:       SeverityMedium,
			Regex:          regexp.MustCompile(`(?:건강보험|의료보험|걱보)[^\n]{0,20}\b\d{10}\b`),
			Description:    "Korean health insurance number",
			RedactTemplate: "[KR_HEALTH_INS_REDACTED]",
		},
		{
			RuleID:         "kr-email-with-name",
			Severity:       SeverityMedium,
			Regex:          regexp.MustCompile(`[\p{Hangul}]{2,4}\s*[:：]\s*[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
			Description:    "Email address with Korean name",
			RedactTemplate: "[KR_EMAIL_REDACTED]",
		},
	}
}
