package dlp

import (
	"context"
	"fmt"

	"patty/internal/paperproto"
	"patty/internal/provider"
)

// Provider wraps an inner provider.Provider with DLP and
// outbound-hook enforcement. The wrapper scans the request
// before forwarding to the inner provider, and scans each chunk
// the inner provider emits so secrets/PII never leave the
// harness.
type Provider struct {
	inner     provider.Provider
	hook      *OutboundHook
	inspector *ResponseInspector
}

// NewProvider wraps the supplied inner provider with DLP checks.
// The hook + inspector run across every stream; the request
// stage uses the hook (`BlockOnDeny=true` by default), and the
// response stage uses the inspector (`BlockOnExfil=true` by
// default).
func NewProvider(inner provider.Provider, hook *OutboundHook, inspector *ResponseInspector) *Provider {
	if hook == nil {
		hook = NewOutboundHook(NewScanner())
	}
	if inspector == nil {
		inspector = NewResponseInspector(hook.scanner)
	}
	return &Provider{inner: inner, hook: hook, inspector: inspector}
}

// Name returns the inner provider's name.
func (p *Provider) Name() string { return p.inner.Name() }

// Stream applies the DLP + workflow gates around the inner
// provider's Stream call. The harness's agent loop calls this
// instead of the raw provider.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	// Outbound: scan the request context for secrets/PII.
	if p.hook != nil {
		// Build the candidate text from the request's user +
		// system messages. The harness's agent loop already
		// assembles these into a single prompt before calling
		// Stream.
		candidate := req.SerializeForScan()
		hookResult, err := p.hook.Apply([]byte(candidate))
		if err != nil {
			return nil, fmt.Errorf("dlp: outbound hook: %w", err)
		}
		if !hookResult.Allow {
			return nil, fmt.Errorf("dlp: outbound blocked by rule %s: %s",
				hookResult.ScanResult.Findings[0].RuleID,
				hookResult.ScanResult.Findings[0].Description)
		}
		// The hook returned a redacted payload; the inner provider
		// gets the request with redacted content. (The harness's
		// agent keeps the original request for the audit log; the
		// scanned redacted payload is what reaches the relay.)
		redacted := string(hookResult.RedactedPayload)
		if redacted != candidate {
			req = req.WithRedactedPrompt(redacted)
		}
	}

	ch, err := p.inner.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	// Wrap the chunk channel so each response chunk is scanned
	// before the harness sees it.
	out := make(chan provider.Chunk, 16)
	go func() {
		defer close(out)
		for chunk := range ch {
			if chunk.Type == provider.ChunkText && chunk.Text != "" {
				inspResult, err := p.inspector.Inspect(chunk.Text)
				if err != nil {
					// Inspector error: fail closed by emitting
					// an error chunk and terminating the stream.
					out <- provider.Chunk{
						Type: provider.ChunkError,
						Err:  fmt.Errorf("dlp: response inspector: %w", err),
					}
					return
				}
				if !inspResult.Allow {
					out <- provider.Chunk{
						Type: provider.ChunkError,
						Err:  fmt.Errorf("dlp: response exfil blocked by rule %s: %s", inspResult.Findings[0].RuleID, inspResult.Findings[0].Description),
					}
					return
				}
				// Use the redacted text so the harness never sees
				// the raw exfiltrated content.
				chunk.Text = inspResult.RedactedText
			}
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// Ensure the wrapper satisfies the provider interface used by the
// harness. paperproto is imported so the linter doesn't strip the
// same-package use; the wrapper itself proxies the inner provider.
var _ provider.Provider = (*Provider)(nil)
var _ = paperproto.ALPNProtocol
