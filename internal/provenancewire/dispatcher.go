package provenancewire

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"patty/internal/paperproto"
)

// Dispatcher sends accumulated provenance envelopes over the PAPER
// connection. The harness owns one Dispatcher per session; after
// each turn, the harness calls `Flush(ctx)` to push the
// accumulated envelopes to the relay. The relay's
// `provenance.CreateChangeSet` / `provenance.CreateProvenanceSpan` /
// `provenance.RecordAction` paths consume them.
type Dispatcher struct {
	mu        sync.Mutex
	conn      DispatchConn
	emitter   *ProvenanceEmitter
	flushErr  error
	flushCount int
}

// DispatchConn is the subset of paperproto.TransportConn the
// dispatcher needs. The harness can substitute a fake connection
// for tests; production wires this to the real conn.
type DispatchConn interface {
	SendRecord(rec *paperproto.Record) error
}

// NewDispatcher wires the emitter to a transport. The harness
// constructs one per session.
func NewDispatcher(emitter *ProvenanceEmitter, conn DispatchConn) *Dispatcher {
	return &Dispatcher{emitter: emitter, conn: conn}
}

// Flush pushes all pending envelopes to the relay over PAPER. The
// method orders the envelopes by type: change sets first (which
// the relay's GORM layer needs as parent rows for spans), then
// spans (which need the change set ID), then actions. After a
// successful flush the emitter's pending buffers are cleared.
// A non-empty flush failure preserves the emitter's pending
// buffers so the next turn can retry.
func (d *Dispatcher) Flush(ctx context.Context) error {
	if d == nil {
		return errors.New("provenancewire: nil dispatcher")
	}
	if err := ctx.Err(); err != nil {
		d.flushErr = err
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn == nil {
		err := errors.New("provenancewire: dispatcher has no transport")
		d.flushErr = err
		return err
	}
	if d.emitter == nil {
		err := errors.New("provenancewire: dispatcher has no emitter")
		d.flushErr = err
		return err
	}

	changeSets, spans, actions := d.emitter.Pending()

	// Validate the envelope family before sending. A bug that
	// references an unknown change set would only surface at the
	// relay, by which point we've spent the round-trip.
	if err := ValidateEnvelopeFamily(actions, changeSets, spans); err != nil {
		wrapped := fmt.Errorf("provenancewire: envelope family validation: %w", err)
		d.flushErr = wrapped
		return wrapped
	}

	// 1. Change sets first.
	for _, cs := range changeSets {
		data, err := EncodeChangeSetEnvelope(cs)
		if err != nil {
			wrapped := fmt.Errorf("provenancewire: encode changeset %s: %w", cs.ChangeSetID, err)
			d.flushErr = wrapped
			return wrapped
		}
		rec := &paperproto.Record{
			Kind:        paperproto.KindMessage,
			MessageType: uint16(paperproto.MsgProvenanceChangeSet),
			Payload:     data,
		}
		if err := d.conn.SendRecord(rec); err != nil {
			wrapped := fmt.Errorf("provenancewire: send changeset %s: %w", cs.ChangeSetID, err)
			d.flushErr = wrapped
			return wrapped
		}
	}

	// 2. Spans second (depend on change sets).
	for _, sp := range spans {
		data, err := EncodeSpanEnvelope(sp)
		if err != nil {
			wrapped := fmt.Errorf("provenancewire: encode span %s: %w", sp.SpanID, err)
			d.flushErr = wrapped
			return wrapped
		}
		rec := &paperproto.Record{
			Kind:        paperproto.KindMessage,
			MessageType: uint16(paperproto.MsgProvenanceSpan),
			Payload:     data,
		}
		if err := d.conn.SendRecord(rec); err != nil {
			wrapped := fmt.Errorf("provenancewire: send span %s: %w", sp.SpanID, err)
			d.flushErr = wrapped
			return wrapped
		}
	}

	// 3. Actions last.
	for _, act := range actions {
		data, err := EncodeActionEnvelope(act)
		if err != nil {
			wrapped := fmt.Errorf("provenancewire: encode action %s: %w", act.ActionID, err)
			d.flushErr = wrapped
			return wrapped
		}
		rec := &paperproto.Record{
			Kind:        paperproto.KindMessage,
			MessageType: uint16(paperproto.MsgActionEnvelope),
			Payload:     data,
		}
		if err := d.conn.SendRecord(rec); err != nil {
			wrapped := fmt.Errorf("provenancewire: send action %s: %w", act.ActionID, err)
			d.flushErr = wrapped
			return wrapped
		}
	}

	// Bind any commit bindings the emitter collected.
	bindings := d.emitter.PendingBindings()
	for _, b := range bindings {
		data, err := EncodeCommitBindingEnvelope(b)
		if err != nil {
			wrapped := fmt.Errorf("provenancewire: encode commit binding %s: %w", b.BindingID, err)
			d.flushErr = wrapped
			return wrapped
		}
		rec := &paperproto.Record{
			Kind:        paperproto.KindMessage,
			MessageType: uint16(paperproto.MsgProvenanceCommitBind),
			Payload:     data,
		}
		if err := d.conn.SendRecord(rec); err != nil {
			wrapped := fmt.Errorf("provenancewire: send commit binding %s: %w", b.BindingID, err)
			d.flushErr = wrapped
			return wrapped
		}
	}

	d.emitter.Clear()
	d.flushCount++
	d.flushErr = nil
	return nil
}

// LastFlushError returns the error from the most recent Flush. The
// harness surfaces this in the audit log so operators see flush
// drift even when the harness retried.
func (d *Dispatcher) LastFlushError() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.flushErr
}

// FlushCount returns the number of successful flushes since the
// dispatcher was created. The harness exposes this in the E1
// status bar.
func (d *Dispatcher) FlushCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.flushCount
}

// PendingCount returns the number of un-flushed envelopes. The
// harness exposes this in the E1 status bar.
func (d *Dispatcher) PendingCount() (int, int, int) {
	if d == nil || d.emitter == nil {
		return 0, 0, 0
	}
	cs, sp, act := d.emitter.Pending()
	return len(cs), len(sp), len(act)
}
