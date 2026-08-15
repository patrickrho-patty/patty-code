package dari

import (
	"context"
	"errors"

	"patty/internal/provenancewire"
)

var errNoConn = errors.New("dari: no live connection for provenance flush")

// provenance_bridge.go wires the harness's provenance emission (B1)
// into the live DARI transport. The provider owns one emitter per
// session; the harness's tool/evidence layers record envelopes onto
// it, and every completed governed exchange flushes the pending
// family to the relay in dependency order (changeset → span → action
// → commit-binding), matching the relay's ingestion handlers.

// ProvenanceEmitter returns the provider's session emitter, creating
// it on first use. The harness feeds it via BuildChangeSetEnvelope*
// and Emit*; the flush rides the authenticated connection.
func (p *Provider) ProvenanceEmitter() *provenancewire.ProvenanceEmitter {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.provEmitter == nil {
		p.provEmitter = provenancewire.NewProvenanceEmitter()
	}
	return p.provEmitter
}

// flushProvenence pushes the pending envelope family to the relay.
// Best-effort per exchange: a failed flush preserves the pending
// buffers (the dispatcher's contract) so the next exchange retries.
// Called from the stream reader after AI_COMPLETE.
func (p *Provider) flushProvenance() {
	p.mu.Lock()
	emitter := p.provEmitter
	conn := p.conn
	p.mu.Unlock()
	if emitter == nil || conn == nil {
		return
	}
	disp := provenancewire.NewDispatcher(emitter, connAckSender{conn: conn})
	_ = disp.Flush(context.Background())
}
