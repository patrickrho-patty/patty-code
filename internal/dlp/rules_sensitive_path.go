package dlp

import "regexp"

// rules_sensitive_path.go — sensitive file-path detection rules.
// Relay class: sensitive_path → harness prefix path-.
func defaultSensitivePathRules() []DetectionRule {
	return []DetectionRule{
		// --- relay's existing 3 rules (path-env, path-secrets, path-private-key) ---
		{
			RuleID:         "path-env",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)\.env(\.local|\.production|\.development)?\b`),
			Description:    "Environment file reference",
			RedactTemplate: "[ENV_FILE_REDACTED]",
		},
		{
			RuleID:         "path-secrets",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(secrets?|credentials?|passwords?|keys?)\.(ya?ml|json|txt|conf)`),
			Description:    "Secrets file reference",
			RedactTemplate: "[SECRETS_FILE_REDACTED]",
		},
		{
			RuleID:         "path-private-key",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`(?i)(id_rsa|id_ecdsa|id_ed25519)\b`),
			Description:    "SSH private key reference",
			RedactTemplate: "[SSH_KEY_FILE_REDACTED]",
		},

		// --- new rules (PAT-1429 mirror) ---
		{
			RuleID:         "path-etc-passwd",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`(?i)/etc/(?:passwd|shadow|gshadow|sudoers)\b`),
			Description:    "System credential file reference",
			RedactTemplate: "[SYSTEM_CRED_FILE_REDACTED]",
		},
		{
			RuleID:         "path-proc-self",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)/proc/self/(?:environ|cmdline|fd|mem)\b`),
			Description:    "Process memory/environment reference",
			RedactTemplate: "[PROC_SELF_REDACTED]",
		},
		{
			RuleID:         "path-aws-credentials",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`(?i)(?:~?/\.aws/credentials|aws[_-]?credentials)`),
			Description:    "AWS credentials file reference",
			RedactTemplate: "[AWS_CRED_FILE_REDACTED]",
		},
		{
			RuleID:         "path-gcp-key",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`(?i)(?:service[_-]?account[_-]?key|gcp[_-]?credentials)\.json`),
			Description:    "GCP service account key reference",
			RedactTemplate: "[GCP_KEY_FILE_REDACTED]",
		},
		{
			RuleID:         "path-kube-config",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(?:\.kube/config|kubeconfig|/admin\.conf)\b`),
			Description:    "Kubernetes config reference",
			RedactTemplate: "[KUBE_CONFIG_REDACTED]",
		},
		{
			RuleID:         "path-git-config",
			Severity:       SeverityMedium,
			Regex:          regexp.MustCompile(`(?i)(?:\.git/config|\.git-credentials)\b`),
			Description:    "Git config/credentials reference",
			RedactTemplate: "[GIT_CONFIG_REDACTED]",
		},
		{
			RuleID:         "path-npmrc",
			Severity:       SeverityMedium,
			Regex:          regexp.MustCompile(`(?i)\.npmrc\b`),
			Description:    "npm auth file reference",
			RedactTemplate: "[NPMRC_REDACTED]",
		},
		{
			RuleID:         "path-ssh-config",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)\.ssh/(?:config|authorized_keys|known_hosts)\b`),
			Description:    "SSH config reference",
			RedactTemplate: "[SSH_CONFIG_REDACTED]",
		},
	}
}
