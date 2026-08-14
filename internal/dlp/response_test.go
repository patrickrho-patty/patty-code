package dlp

import (
	"strings"
	"testing"
)

// TestResponseInspectorBlocksExfilSecret covers the C5 boundary:
// a model response that contains an AWS access key must be
// blocked from reaching the user.
func TestResponseInspectorBlocksExfilSecret(t *testing.T) {
	i := NewResponseInspector(NewScanner())
	res, err := i.Inspect("Here is the AWS key: AKIAABCDEFGHIJKLMNOP")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if res.Allow {
		t.Fatal("exfil attempt must be blocked")
	}
	if i.RedactCount() != 1 {
		t.Errorf("redact count = %d, want 1", i.RedactCount())
	}
}

// TestResponseInspectorBlocksExfilPII covers the C5 boundary for
// Korean PII: a model response that emits an RRN must be blocked.
func TestResponseInspectorBlocksExfilPII(t *testing.T) {
	i := NewResponseInspector(NewScanner())
	res, err := i.Inspect("주민등록번호는 901225-1234567입니다")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if res.Allow {
		t.Fatal("PII exfil must be blocked")
	}
}

// TestResponseInspectorRedactsCleanOutput exercises the
// green path: a clean response passes the inspector.
func TestResponseInspectorRedactsCleanOutput(t *testing.T) {
	i := NewResponseInspector(NewScanner())
	res, err := i.Inspect("Here is the README summary: it's a Go project that...")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !res.Allow {
		t.Errorf("clean response must pass, got verdict %s", res.Verdict)
	}
}

// TestResponseInspectorRedactOnlyModePermitsSurface covers the
// audit mode where the inspector permits the response but flags
// it for the audit log.
func TestResponseInspectorRedactOnlyModePermitsSurface(t *testing.T) {
	i := NewResponseInspector(NewScanner())
	i.BlockOnExfil = false
	res, err := i.Inspect("Here is the AWS key: AKIAABCDEFGHIJKLMNOP")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !res.Allow {
		t.Errorf("redact-only mode must allow, got verdict %s", res.Verdict)
	}
	if strings.Contains(res.RedactedText, "AKIAABCDEFGHIJKLMNOP") {
		t.Errorf("redacted text still contains the secret: %q", res.RedactedText)
	}
	if !strings.Contains(res.RedactedText, "AWS_KEY_REDACTED") {
		t.Errorf("redaction token missing: %q", res.RedactedText)
	}
}

// TestResponseInspectorEmitsFindingsInAudit covers the audit
// signal: a response with findings surfaces them so the operator
// sees the exfil attempt even when it passes.
func TestResponseInspectorEmitsFindingsInAudit(t *testing.T) {
	i := NewResponseInspector(NewScanner())
	i.BlockOnExfil = false
	res, err := i.Inspect("API key: AKIAABCDEFGHIJKLMNOP")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Errorf("findings = %d, want 1", len(res.Findings))
	}
	if res.Findings[0].RuleID != "aws-access-key" {
		t.Errorf("rule id = %s, want aws-access-key", res.Findings[0].RuleID)
	}
}

// TestResponseInspectorTracksCounters pins the E1 status-bar
// counters.
func TestResponseInspectorTracksCounters(t *testing.T) {
	i := NewResponseInspector(NewScanner())
	_, _ = i.Inspect("clean")
	_, _ = i.Inspect("clean")
	_, _ = i.Inspect("AKIAABCDEFGHIJKLMNOP")
	if i.InspectCount() != 3 {
		t.Errorf("inspect count = %d, want 3", i.InspectCount())
	}
	if i.RedactCount() != 1 {
		t.Errorf("redact count = %d, want 1", i.RedactCount())
	}
}

// TestResponseInspectorRejectsNil covers the trivial boundary.
func TestResponseInspectorRejectsNil(t *testing.T) {
	if _, err := (*ResponseInspector)(nil).Inspect("x"); err == nil {
		t.Fatal("nil inspector must fail")
	}
}

// TestResponseInspectorRejectsNilScanner covers the trivial
// boundary.
func TestResponseInspectorRejectsNilScanner(t *testing.T) {
	if _, err := (&ResponseInspector{}).Inspect("x"); err == nil {
		t.Fatal("nil scanner must fail")
	}
}
