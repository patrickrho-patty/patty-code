package dlp

import "regexp"

// rules_secret.go — secret detection rules (PRD §16).
// Mirror of the relay's secret catalog. The relay's secret class
// covers a broad prefix set via classPrefixes["secret"].
func defaultSecretRules() []DetectionRule {
	return []DetectionRule{
		// --- existing rules ---
		{
			RuleID:         "aws-access-key",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
			Description:    "AWS access key",
			RedactTemplate: "[AWS_KEY_REDACTED]",
		},
		{
			RuleID:         "aws-secret-key",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`(?i)aws_secret_access_key\s*=\s*[A-Za-z0-9/+=]{40}`),
			Description:    "AWS secret access key",
			RedactTemplate: "[AWS_SECRET_REDACTED]",
		},
		{
			RuleID:         "generic-bearer-token",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(bearer|token)\s+[A-Za-z0-9._~+/=-]{20,}`),
			Description:    "Generic API bearer token",
			RedactTemplate: "[BEARER_TOKEN_REDACTED]",
		},
		{
			RuleID:         "private-key-pem",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`),
			Description:    "PEM private key",
			RedactTemplate: "[PRIVATE_KEY_REDACTED]",
		},

		// --- new rules (PAT-1429 mirror) ---
		{
			RuleID:         "gcp-key",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),
			Description:    "Google Cloud API key",
			RedactTemplate: "[GCP_KEY_REDACTED]",
		},
		{
			RuleID:         "azure-key",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`AccountKey=[A-Za-z0-9+/=]{60,}`),
			Description:    "Azure storage account key",
			RedactTemplate: "[AZURE_KEY_REDACTED]",
		},
		{
			RuleID:         "ncloud-key",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)(?:ncp|ncloud)[_-]?(?:access|secret)[_-]?key["'\s:=]+[A-Za-z0-9+/=]{16,}`),
			Description:    "Naver Cloud Platform key",
			RedactTemplate: "[NCLOUD_KEY_REDACTED]",
		},
		{
			RuleID:         "gitlab-token",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20}\b`),
			Description:    "GitLab personal access token",
			RedactTemplate: "[GITLAB_TOKEN_REDACTED]",
		},
		{
			RuleID:         "openai-key",
			Severity:       SeverityCritical,
			Regex:          regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}\b`),
			Description:    "OpenAI API key",
			RedactTemplate: "[OPENAI_KEY_REDACTED]",
		},
		{
			RuleID:         "slack-webhook",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Za-z0-9]+/B[A-Za-z0-9]+/[A-Za-z0-9]+`),
			Description:    "Slack webhook URL",
			RedactTemplate: "[SLACK_WEBHOOK_REDACTED]",
		},
		{
			RuleID:         "mysql-connstring",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)\bmysql://[^\s:@/]+:[^\s@]+@`),
			Description:    "MySQL connection string",
			RedactTemplate: "[MYSQL_DSN_REDACTED]",
		},
		{
			RuleID:         "postgres-connstring",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)\bpostgres(?:ql)?://[^\s:@/]+:[^\s@]+@`),
			Description:    "PostgreSQL connection string",
			RedactTemplate: "[POSTGRES_DSN_REDACTED]",
		},
		{
			RuleID:         "redis-connstring",
			Severity:       SeverityHigh,
			Regex:          regexp.MustCompile(`(?i)\bredis[s]?://[^\s:@/]+:[^\s@]+@`),
			Description:    "Redis connection string",
			RedactTemplate: "[REDIS_DSN_REDACTED]",
		},
	}
}
