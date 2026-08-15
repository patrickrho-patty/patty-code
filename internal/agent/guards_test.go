package agent

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"patty/internal/event"
	"patty/internal/provider"
	"patty/internal/tool"
	_ "patty/internal/tool/builtin"
)

// TestTruncateToolOutputUnderCap leaves small payloads alone  the cap should
func TestTruncateToolOutputUnderCap(t *testing.T) {
	in := strings.Repeat("a", maxToolOutputBytes)
	got, notice := truncateToolOutput(in)
	if got != in {
		t.Errorf("payload at exactly the cap was rewritten")
	}
	if notice != "" {
		t.Errorf("at-cap payload should not emit a notice, got %q", notice)
	}
}

func TestTruncateToolOutputHeadTail(t *testing.T) {
	head := strings.Repeat("H", maxToolOutputBytes)
	tail := strings.Repeat("T", maxToolOutputBytes)
	in := head + tail
	out, notice := truncateToolOutput(in)
	if !strings.HasPrefix(out, "H") || !strings.HasSuffix(out, "T") {
		t.Errorf("head/tail not preserved at the edges: %q…%q", out[:20], out[len(out)-20:])
	}
	if !strings.Contains(out, "truncated") {
		t.Errorf("truncation marker missing: %q", out)
	}
	if len(out) >= len(in) {
		t.Errorf("output not shorter than input: in=%d out=%d", len(in), len(out))
	}
	if !strings.Contains(notice, "truncated") {
		t.Errorf("notice missing: %q", notice)
	}
}

func TestTruncateToolOutputRuneBoundaries(t *testing.T) {
	in := strings.Repeat("한", maxToolOutputBytes) // 3 bytes each — guarantees a cut inside a rune
	out, _ := truncateToolOutput(in)
	if !utf8.ValidString(out) {
		t.Errorf("truncated output is not valid UTF-8")
	}
}

func TestFinishReasonMessage(t *testing.T) {
	silent := []string{"", "stop", "tool_calls"}
	for _, r := range silent {
		if msg, ok := finishReasonMessage(&provider.Usage{FinishReason: r}); ok {
			t.Errorf("finish_reason=%q should be silent, got %q", r, msg)
		}
	}
	loud := map[string]string{
		"length":                 "max output",
		"client_reasoning_limit": "client reasoning safety limit",
		"content_filter":         "content filter",
		"repetition_truncation":  "repetition",
	}
	for reason, fragment := range loud {
		msg, ok := finishReasonMessage(&provider.Usage{FinishReason: reason})
		if !ok || !strings.Contains(msg, fragment) {
			t.Errorf("finish_reason=%q: got (%q, %v), want fragment %q", reason, msg, ok, fragment)
		}
	}
}

func TestEmptyFinalNotice(t *testing.T) {
	msg := emptyFinalNotice()
	for _, hidden := range []string{"blocked", "finish=", "reasoning="} {
		if strings.Contains(msg, hidden) {
			t.Errorf("notice %q should not expose internal diagnostic %q", msg, hidden)
		}
	}
	detail := emptyFinalNoticeDetail("deepseek-flash", &provider.Usage{FinishReason: "stop"}, 512)
	for _, want := range []string{"deepseek-flash", "finish=stop", "reasoning=512"} {
		if !strings.Contains(detail, want) {
			t.Errorf("notice detail %q missing %q", detail, want)
		}
	}
	if got := emptyFinalNoticeDetail("p", nil, 0); !strings.Contains(got, "finish=unknown") {
		t.Errorf("nil usage should report finish=unknown, got %q", got)
	}
}

type fakeTool struct {
	name     string
	readOnly bool
	delay    time.Duration
	err      error
	calls    *int32 // shared counter to assert all dispatched
}

func (f fakeTool) Name() string            { return f.name }
func (f fakeTool) Description() string     { return "" }
func (f fakeTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (f fakeTool) ReadOnly() bool          { return f.readOnly }
func (f fakeTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	if f.calls != nil {
		atomic.AddInt32(f.calls, 1)
	}
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if f.err != nil {
		return "", f.err
	}
	return f.name + " done", nil
}

func TestPartitionToolCallsAllReadOnly(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "ro1", readOnly: true})
	reg.Add(fakeTool{name: "ro2", readOnly: true})
	calls := []provider.ToolCall{{Name: "ro1"}, {Name: "ro2"}}
	got := partitionToolCalls(reg, calls)
	want := []toolCallBatch{{start: 0, end: 2, parallel: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionToolCalls = %+v, want %+v", got, want)
	}
}

func TestPartitionToolCallsSegmentsAroundWriters(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "ro", readOnly: true})
	reg.Add(fakeTool{name: "rw", readOnly: false})
	calls := []provider.ToolCall{{Name: "ro"}, {Name: "rw"}, {Name: "ro"}}
	got := partitionToolCalls(reg, calls)
	want := []toolCallBatch{
		{start: 0, end: 1, parallel: true},
		{start: 1, end: 2},
		{start: 2, end: 3, parallel: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionToolCalls = %+v, want %+v", got, want)
	}
}

func TestPartitionToolCallsUnknownToolSerial(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "ro", readOnly: true})
	calls := []provider.ToolCall{{Name: "ro"}, {Name: "vanished"}, {Name: "ro"}}
	got := partitionToolCalls(reg, calls)
	want := []toolCallBatch{
		{start: 0, end: 1, parallel: true},
		{start: 1, end: 2},
		{start: 2, end: 3, parallel: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionToolCalls = %+v, want %+v", got, want)
	}
}

// parallel read-only run: it reads the turns receipts, so the prior reads must
func TestPartitionToolCallsCompleteStepSerial(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	reg.Add(fakeTool{name: "complete_step", readOnly: true})

	calls := []provider.ToolCall{{Name: "read_file"}, {Name: "complete_step"}}
	got := partitionToolCalls(reg, calls)
	want := []toolCallBatch{
		{start: 0, end: 1, parallel: true},
		{start: 1, end: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionToolCalls = %+v, want %+v", got, want)
	}
}

func TestPartitionToolCallsTodoWriteSerial(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	reg.Add(fakeTool{name: "todo_write", readOnly: true})

	calls := []provider.ToolCall{{Name: "read_file"}, {Name: "todo_write"}, {Name: "read_file"}}
	got := partitionToolCalls(reg, calls)
	want := []toolCallBatch{
		{start: 0, end: 1, parallel: true},
		{start: 1, end: 2},
		{start: 2, end: 3, parallel: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionToolCalls = %+v, want %+v", got, want)
	}
}

func TestPartitionToolCallsBackgroundCollectorsSerial(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	reg.Add(fakeTool{name: "wait", readOnly: true})
	reg.Add(fakeTool{name: "bash_output", readOnly: true})

	calls := []provider.ToolCall{{Name: "read_file"}, {Name: "wait"}, {Name: "bash_output"}, {Name: "read_file"}}
	got := partitionToolCalls(reg, calls)
	want := []toolCallBatch{
		{start: 0, end: 1, parallel: true},
		{start: 1, end: 2},
		{start: 2, end: 3},
		{start: 3, end: 4, parallel: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionToolCalls = %+v, want %+v", got, want)
	}
}

// complete in well under 3×80ms — the wall-clock proof of true parallelism.
func TestExecuteBatchParallelReadOnly(t *testing.T) {
	const delay = 80 * time.Millisecond
	calls := int32(0)
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "a", readOnly: true, delay: delay, calls: &calls})
	reg.Add(fakeTool{name: "b", readOnly: true, delay: delay, calls: &calls})
	reg.Add(fakeTool{name: "c", readOnly: true, delay: delay, calls: &calls})

	a := New(nil, reg, NewSession(""), Options{}, event.Discard)

	start := time.Now()
	batch := a.executeBatch(context.Background(), []provider.ToolCall{{Name: "a"}, {Name: "b"}, {Name: "c"}})
	results := batch.results
	elapsed := time.Since(start)

	if calls != 3 {
		t.Errorf("dispatched %d calls, want 3", calls)
	}
	if len(results) != 3 || results[0] != "a done" || results[1] != "b done" || results[2] != "c done" {
		t.Errorf("results out of order or wrong: %v", results)
	}
	if elapsed >= 2*delay {
		t.Errorf("read-only batch took %v (>= %v) — not parallel", elapsed, 2*delay)
	}
}

func TestExecuteBatchStampsToolResultTimestamps(t *testing.T) {
	const delay = 30 * time.Millisecond
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "a", readOnly: true, delay: delay})

	sink := &recordSink{}
	a := New(nil, reg, NewSession(""), Options{}, sink)

	before := time.Now().UnixMilli()
	a.executeBatch(context.Background(), []provider.ToolCall{{Name: "a"}})
	after := time.Now().UnixMilli()

	results := sink.kinds(event.ToolResult)
	if len(results) != 1 {
		t.Fatalf("got %d tool results, want 1", len(results))
	}
	tr := results[0].Tool
	if tr.StartedAt < before || tr.StartedAt > after {
		t.Errorf("StartedAt = %d, want within [%d, %d]", tr.StartedAt, before, after)
	}
	if tr.EndedAt != tr.StartedAt+tr.DurationMs {
		t.Errorf("EndedAt = %d, want StartedAt+DurationMs = %d", tr.EndedAt, tr.StartedAt+tr.DurationMs)
	}
	if tr.DurationMs < delay.Milliseconds() {
		t.Errorf("DurationMs = %d, want >= %d", tr.DurationMs, delay.Milliseconds())
	}
}

func TestExecuteBatchCancelledCallsCarryNoTimestamps(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "a", readOnly: true})

	sink := &recordSink{}
	a := New(nil, reg, NewSession(""), Options{}, sink)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.executeBatch(ctx, []provider.ToolCall{{Name: "a"}})

	results := sink.kinds(event.ToolResult)
	if len(results) != 1 {
		t.Fatalf("got %d tool results, want 1", len(results))
	}
	tr := results[0].Tool
	if tr.StartedAt != 0 || tr.EndedAt != 0 {
		t.Errorf("never-ran call has StartedAt=%d EndedAt=%d, want both zero", tr.StartedAt, tr.EndedAt)
	}
}

func TestExecuteBatchSegmentsAroundWrites(t *testing.T) {
	const delay = 150 * time.Millisecond
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "ro1", readOnly: true, delay: delay})
	reg.Add(fakeTool{name: "ro2", readOnly: true, delay: delay})
	reg.Add(fakeTool{name: "ro3", readOnly: true, delay: delay})
	reg.Add(fakeTool{name: "ro4", readOnly: true, delay: delay})
	reg.Add(fakeTool{name: "rw", readOnly: false, delay: delay})

	a := New(nil, reg, NewSession(""), Options{}, event.Discard)

	start := time.Now()
	batch := a.executeBatch(context.Background(), []provider.ToolCall{
		{Name: "ro1"},
		{Name: "ro2"},
		{Name: "rw"},
		{Name: "ro3"},
		{Name: "ro4"},
	})
	results := batch.results
	elapsed := time.Since(start)

	want := []string{"ro1 done", "ro2 done", "rw done", "ro3 done", "ro4 done"}
	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d: %v", len(results), len(want), results)
	}
	for i := range want {
		if results[i] != want[i] {
			t.Fatalf("results out of order or wrong: got %v want %v", results, want)
		}
	}
	// Desired shape is roughly 3delay: (ro1|ro2), then rw, then (ro3|ro4).
	// Old all-serial behaviour is roughly 5delay and should fail this bound.
	if elapsed >= 4*delay {
		t.Errorf("mixed batch took %v (>= %v) — read-only segments did not parallelise", elapsed, 4*delay)
	}
	if elapsed < 2*delay {
		t.Errorf("mixed batch took only %v — write call appears to have overlapped a read-only segment", elapsed)
	}
}

func TestExecuteBatchFeedsReceiptsToCompleteStep(t *testing.T) {
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(completeStep)
	a := New(nil, reg, NewSession(""), Options{}, event.Discard)

	batch := a.executeBatch(context.Background(), []provider.ToolCall{
		{Name: "bash", Arguments: `{"command":"go test ./internal/..."}`},
		{Name: "complete_step", Arguments: `{
			"step":"Run checks",
			"result":"checks passed",
			"evidence":[{"kind":"verification","summary":"tests passed","command":"go test ./internal/..."}]
		}`},
	})
	results := batch.results

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if !strings.Contains(results[1], "host-verified 1") {
		t.Fatalf("complete_step did not see bash receipt: %q", results[1])
	}
}

func TestExecuteOneFailedReceiptDoesNotVerify(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false, err: errors.New("boom")})
	a := New(nil, reg, NewSession(""), Options{}, event.Discard)

	out := a.executeOne(context.Background(), provider.ToolCall{Name: "bash", Arguments: `{"command":"go test ./..."}`})
	if out.errMsg == "" {
		t.Fatal("failing fake tool should return an error outcome")
	}
	if a.evidence.HasSuccessfulCommand("go test ./...") {
		t.Fatal("failed bash receipt must not verify")
	}
}

func TestRunResetsEvidenceLedger(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false})
	prov := &mockProvider{name: "p", chunks: []provider.Chunk{{Type: provider.ChunkText, Text: "done"}}}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	a.executeOne(context.Background(), provider.ToolCall{Name: "bash", Arguments: `{"command":"go test ./..."}`})
	if !a.evidence.HasSuccessfulCommand("go test ./...") {
		t.Fatal("setup failed to record evidence")
	}

	if err := a.Run(context.Background(), "next turn"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if a.evidence.HasSuccessfulCommand("go test ./...") {
		t.Fatal("new user turn should not inherit previous receipts")
	}
}
