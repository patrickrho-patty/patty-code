package control

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/agent"
	"patty/internal/event"
)

// label — and the rewind picker restores it into the composer — so Checkpoints
// "한국어로 작성해야 합니다…" in the rewind list (#6903).
func TestCheckpointsReturnUserPromptWithoutComposedPrefixes(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	runner := &recordingSessionRunner{session: sess}
	c := New(Options{
		Runner:      runner,
		Executor:    exec,
		SessionDir:  dir,
		SessionPath: filepath.Join(dir, "session.jsonl"),
		Label:       "test",
	})

	const typed = "로그인 로직 수정"
	composed := agent.ReasoningLanguageBlock("ko-KR") + "\n\n" +
		PlanModeMarker + "\n\n" +
		typed
	o := newTurnOrchestrator(c)
	if err := o.runTurnWithRawDisplay(context.Background(), composed, typed, ""); err != nil {
		t.Fatal(err)
	}

	cps := c.Checkpoints()
	if len(cps) != 1 {
		t.Fatalf("checkpoints = %+v, want one", cps)
	}
	if cps[0].Prompt != typed {
		t.Fatalf("checkpoint prompt = %q, want %q", cps[0].Prompt, typed)
	}
	if strings.Contains(cps[0].Prompt, "<reasoning-language>") || strings.Contains(cps[0].Prompt, PlanModeMarker) {
		t.Fatalf("checkpoint prompt still carries composed prefixes: %q", cps[0].Prompt)
	}
}

func TestHeadlessRunOpensCheckpoint(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{
		Runner:      &fakeTurnRunner{},
		Executor:    exec,
		SessionDir:  dir,
		SessionPath: filepath.Join(dir, "headless.jsonl"),
		Label:       "test",
	})

	if err := c.Run(context.Background(), "headless edit"); err != nil {
		t.Fatal(err)
	}
	checkpoints := c.Checkpoints()
	if len(checkpoints) != 1 || checkpoints[0].Turn != 0 || checkpoints[0].Prompt != "headless edit" {
		t.Fatalf("headless checkpoints = %+v, want one checkpoint for the submitted turn", checkpoints)
	}
}
