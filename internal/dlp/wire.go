package dlp

import (
	"errors"
	"sync"

	"patty/internal/dariproto"
)

// OutboundHook is the connector-side wrapper the harness applies
// to every AI_OPEN payload before DARI dispatch. The hook
// delegates to a Scanner instance and short-circuits dispatch when
// the scan verdict is DENY (PRD §16.3, §16.5).
type OutboundHook struct {
	scanner *Scanner
	mu      sync.Mutex
	// BlockOnDeny is the policy switch. When true, a DENY verdict
	// rejects the dispatch. When false, the dispatch is allowed
	// but flagged in the audit log.
	BlockOnDeny bool
	// ackCount + denyCount surface the E1 status bar.
	ackCount  int64
	denyCount int64
	// FailClosedOnScanError makes the hook fail closed on scanner
	// failures (regex compile error, etc.) rather than letting
	// untrusted content reach the relay.
	FailClosedOnScanError bool
}

// NewOutboundHook constructs the hook with fail-closed semantics.
// Production wires this in front of the DARI transport so every
// outbound context passes through the scan.
func NewOutboundHook(scanner *Scanner) *OutboundHook {
	return &OutboundHook{
		scanner:               scanner,
		BlockOnDeny:           true,
		FailClosedOnScanError: true,
	}
}

// HookResult is the result of applying the hook. The harness's
// AI_OPEN path consults this before pushing to the relay.
type HookResult struct {
	Allow         bool
	RedactedPayload []byte
	ScanResult     ScanResult
}

// Apply scans the supplied payload and returns the disposition.
// When Allow is true, the harness MUST use RedactedPayload
// (the scanned, redacted text) instead of the raw input. When
// Allow is false, the harness MUST NOT dispatch.
func (h *OutboundHook) Apply(payload []byte) (HookResult, error) {
	if h == nil {
		return HookResult{}, errors.New("dlp: nil hook")
	}
	if h.scanner == nil {
		return HookResult{}, errors.New("dlp: nil scanner")
	}
	if len(payload) == 0 {
		// Empty payloads pass; no scanner work to do.
		h.mu.Lock()
		h.ackCount++
		h.mu.Unlock()
		return HookResult{Allow: true, RedactedPayload: payload, ScanResult: ScanResult{Passed: true}}, nil
	}
	res := h.scanner.Scan(string(payload))
	if !res.Passed && h.BlockOnDeny {
		h.mu.Lock()
		h.denyCount++
		h.mu.Unlock()
		return HookResult{
			Allow:         false,
			RedactedPayload: []byte(res.RedactedText),
			ScanResult:     res,
		}, nil
	}
	h.mu.Lock()
	h.ackCount++
	h.mu.Unlock()
	return HookResult{
		Allow:         true,
		RedactedPayload: []byte(res.RedactedText),
		ScanResult:     res,
	}, nil
}

// ApplyMessage applies the hook to a DARI control record's
// payload. The harness's transport layer invokes this on every
// outgoing record before the call to SendRecord. When the scan
// passes (or allow-after-redact is on) the record's payload is
// replaced in place with the redacted text; the caller MUST use
// the returned record rather than the original.
func (h *OutboundHook) ApplyMessage(rec *dariproto.Record) (HookResult, error) {
	if rec == nil {
		return HookResult{}, errors.New("dlp: nil record")
	}
	res, err := h.Apply(rec.Payload)
	if err != nil {
		return res, err
	}
	if res.Allow {
		// Replace the payload in place so the harness's SendRecord
		// call sends the redacted text, not the raw input. We
		// allocate a fresh byte slice so the caller's original
		// record buffer is not aliased.
		if len(res.RedactedPayload) == len(rec.Payload) {
			// In-place copy if length matches (no allocation).
			copy(rec.Payload, res.RedactedPayload)
		} else {
			rec.Payload = res.RedactedPayload
		}
	}
	return res, nil
}

// AckCount returns the count of payloads the hook allowed.
func (h *OutboundHook) AckCount() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ackCount
}

// DenyCount returns the count of payloads the hook rejected.
func (h *OutboundHook) DenyCount() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.denyCount
}
