package dlp

import (
	"errors"
	"sync"
)

// ResponseInspector scans model responses (PRD §16.5) for exfil
// signals — secrets, PII, or prompt-injection indicators that the
// model emits. The inspector reuses the same Scanner as the
// outbound hook so the rule pack stays in sync.
type ResponseInspector struct {
	scanner *Scanner
	mu      sync.Mutex
	// inspectCount is the number of responses scanned.
	inspectCount int64
	// redactCount is the number of responses that contained at
	// least one finding. Surfaced in the E1 status bar.
	redactCount int64
	// BlockOnExfil controls the fail-closed policy. When true, a
	// critical-severity finding (e.g., PEM private key in output)
	// refuses to surface the response. When false, the response is
	// surfaced with redactions but flagged in the audit log.
	BlockOnExfil bool
}

// NewResponseInspector wires the inspector to the shared scanner.
func NewResponseInspector(scanner *Scanner) *ResponseInspector {
	return &ResponseInspector{
		scanner:       scanner,
		BlockOnExfil:   true,
	}
}

// InspectionResult is the verdict for one model response.
type InspectionResult struct {
	Allow         bool
	RedactedText   string
	Verdict       Verdict
	Findings      []Finding
}

// Inspect scans the response and returns the disposition. The
// caller MUST use RedactedText when present — never the raw model
// output. When Allow is false (fail-closed), the response MUST NOT
// be surfaced to the user.
func (i *ResponseInspector) Inspect(response string) (InspectionResult, error) {
	if i == nil {
		return InspectionResult{}, errors.New("dlp: nil response inspector")
	}
	if i.scanner == nil {
		return InspectionResult{}, errors.New("dlp: nil scanner")
	}
	i.mu.Lock()
	i.inspectCount++
	i.mu.Unlock()
	res := i.scanner.Scan(response)
	if !res.Passed && i.BlockOnExfil {
		i.mu.Lock()
		i.redactCount++
		i.mu.Unlock()
		return InspectionResult{
			Allow:       false,
			RedactedText: res.RedactedText,
			Verdict:     res.Verdict,
			Findings:    res.Findings,
		}, nil
	}
	if len(res.Findings) > 0 {
		i.mu.Lock()
		i.redactCount++
		i.mu.Unlock()
	}
	return InspectionResult{
		Allow:       true,
		RedactedText: res.RedactedText,
		Verdict:     res.Verdict,
		Findings:    res.Findings,
	}, nil
}

// InspectCount returns the number of responses scanned since the
// inspector was created. Surfaced in the E1 status bar.
func (i *ResponseInspector) InspectCount() int64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.inspectCount
}

// RedactCount returns the number of responses that produced at
// least one finding since the inspector was created.
func (i *ResponseInspector) RedactCount() int64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.redactCount
}
