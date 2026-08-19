package dlp

import "regexp"

// rules_injection.go — prompt-injection detection rules (PRD §16.4).
// Mirror of the relay's injection catalog.
func defaultInjectionRules() []DetectionRule {
	return []DetectionRule{
		// --- existing rules ---
		{
			RuleID:         "injection-override",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`(?i)ignore (all )?previous instructions`),
			Description:    "Prompt injection override attempt",
			RedactTemplate: "",
		},
		{
			RuleID:         "injection-jailbreak",
			Severity:       SeverityCritical,
			// Word-bounded: an unanchored (?i)DAN also matched ordinary
			// English words ("abundance", "redundant data") in serialized
			// prompts, blocking every outbound request.
			Regex:          regexp.MustCompile(`(?i)\b(jailbreak|dan|do anything now)\b`),
			Description:    "Jailbreak attempt",
			RedactTemplate: "",
		},
		{
			RuleID:         "injection-system",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(^|\n)\s*system\s*:\s*you are`),
			Description:    "Fake system-prompt injection",
			RedactTemplate: "",
		},

		// --- new rules (PAT-1429 mirror) ---
		{
			RuleID:         "injection-exfil-email",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(?:email|send|forward)[^\n]{0,30}(?:this|the|my|your)[^\n]{0,20}(?:output|response|answer|conversation)[^\n]{0,20}(?:to|at)\s+[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
			Description:    "Exfiltration via email",
			RedactTemplate: "",
		},
		{
			RuleID:         "injection-exfil-url",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(?:post|upload|exfiltrate)[^\n]{0,40}(?:this|the|my|your|all)[^\n]{0,30}(?:to|at)\s+https?://`),
			Description:    "Exfiltration via URL",
			RedactTemplate: "",
		},
		{
			RuleID:         "injection-base64-decode",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(?:base64|b64)[^\n]{0,20}(?:decode|decrypt|디코딩|실행)[^\n]{0,30}["'\x60][A-Za-z0-9+/]{40,}={0,2}["'\x60]`),
			Description:    "Base64-encoded instruction",
			RedactTemplate: "",
		},
	}
}
