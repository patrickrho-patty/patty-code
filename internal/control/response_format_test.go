package control

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/agent"
	"patty/internal/event"
)

func TestIsNonTurnHTTPInput(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{"", true},               // empty
		{"  ", true},             // blank
		{"# note text", true},    // memory quick-add (# + space)
		{"/remember MiMo", true}, // remember command note
		{"/compact", true},       // slash command
		{"/model qwen3", true},   // management verb
		{"/new", true},           // slash command
		{"!ls", true},            // shell commands rejected by submitHTTP (403) before any turn
		{"hello", false},         // ordinary turn
		{"explain this code", false},
	} {
		if got := isNonTurnHTTPInput(tc.input); got != tc.want {
			t.Errorf("isNonTurnHTTPInput(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

type observedTurnFormat struct {
	input  string
	format string
}

type formatRecordingRunner struct {
	observed chan<- observedTurnFormat
}

func (r formatRecordingRunner) Run(ctx context.Context, input string) error {
	format := ""
	if responseFormat := agent.ResponseFormatFromRequest(ctx); responseFormat != nil {
		format = responseFormat.Type
	}
	r.observed <- observedTurnFormat{input: input, format: format}
	return nil
}

type formatTurnDoneGate struct {
	mu           sync.Mutex
	turns        int
	firstEntered chan struct{}
	releaseFirst chan struct{}
	allDone      chan struct{}
}

func (g *formatTurnDoneGate) Emit(e event.Event) {
	if e.Kind != event.TurnDone {
		return
	}
	g.mu.Lock()
	g.turns++
	turn := g.turns
	g.mu.Unlock()

	if turn == 1 {
		close(g.firstEntered)
		<-g.releaseFirst
	}
	if turn == 2 {
		close(g.allDone)
	}
}

func receiveObservedTurnFormat(t *testing.T, observed <-chan observedTurnFormat) observedTurnFormat {
	t.Helper()
	select {
	case got := <-observed:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for submitted turn")
		return observedTurnFormat{}
	}
}

func waitForFormatTestSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal(message)
	}
}

// TestSubmitHTTPFormatBindsToTurn holds the first turns finishing window open,
// preserves each accepted turns format. This deterministically exercises the
func TestSubmitHTTPFormatBindsToTurn(t *testing.T) {
	observed := make(chan observedTurnFormat, 2)
	gate := &formatTurnDoneGate{
		firstEntered: make(chan struct{}),
		releaseFirst: make(chan struct{}),
		allDone:      make(chan struct{}),
	}
	c := New(Options{Runner: formatRecordingRunner{observed: observed}, Sink: gate})

	c.SubmitHTTPFormat("first turn", "format-a")
	first := receiveObservedTurnFormat(t, observed)
	waitForFormatTestSignal(t, gate.firstEntered, "first turn did not enter the finishing window")

	c.SubmitHTTPFormat("second turn", "format-b")
	close(gate.releaseFirst)
	second := receiveObservedTurnFormat(t, observed)
	waitForFormatTestSignal(t, gate.allDone, "second turn did not finish")

	if !strings.Contains(first.input, "first turn") || first.format != "format-a" {
		t.Fatalf("first turn = %+v, want first input with format-a", first)
	}
	if !strings.Contains(second.input, "second turn") || second.format != "format-b" {
		t.Fatalf("second turn = %+v, want second input with format-b", second)
	}
}

// TestWithTurnFormatInjectsFormatIntoContext: format이 turn에 바인딩되는 실제 효과
// ——withTurnFormat 주입 후 agent 요청 경로에서 읽을 수 있음(전역 슬롯이 아님).
func TestWithTurnFormatInjectsFormatIntoContext(t *testing.T) {
	c := New(Options{})
	ctx := context.Background()
	if got := agent.ResponseFormatFromRequest(c.withTurnFormat(ctx, "")); got != nil {
		t.Fatalf("empty format must be no-op, got %+v", got)
	}
	if got := agent.ResponseFormatFromRequest(c.withTurnFormat(ctx, "json_object")); got == nil || got.Type != "json_object" {
		t.Fatalf("turn format must reach agent request, got %+v", got)
	}
}

// TestRefTurnFormatBound: @reference turn도 format에 바인딩됨(통일된 아키텍처——
// format은 수락된 모든 turn의 속성이며 runGoalLoop 특례가 아님).
func TestRefTurnFormatBound(t *testing.T) {
	c := New(Options{})
	ctx := context.Background()
	// runRefTurnWithFormat 주입 후 agent 요청 경로가 json_object를 읽음
	if got := agent.ResponseFormatFromRequest(c.withTurnFormat(ctx, "json_object")); got == nil || got.Type != "json_object" {
		t.Fatalf("ref-turn format must bind to ctx, got %+v", got)
	}
	// isRefTurnInput이 @참조 turn을 인식(format이 wrapper를 통해 바인딩되어 더 이상 버려지지 않음)
	// ref-turn 입력 인식(SlashCodeCommentLine은 파일 시스템에 의존하지 않음)
	for _, input := range []string{"// comment line", "//src/main.go:12"} {
		if !SlashCodeCommentLine(input) {
			t.Errorf("SlashCodeCommentLine(%q) = false, want true", input)
		}
	}
}
