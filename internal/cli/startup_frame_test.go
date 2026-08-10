package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/provider"
)

func TestMissingKeyStartupWithInvisibleHistoryStaysNatural(t *testing.T) {
	m := newChatTUI(control.New(control.Options{}), `provider "deepseek-flash": missing env DEEPSEEK_API_KEY`, make(chan event.Event, 1), 140)
	m.history = append(m.history, provider.Message{Role: provider.RoleSystem, Content: "runtime metadata"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = next.(chatTUI)
	m.ingestEvent(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "Selected model is missing its API key."})
	if !m.isNaturalStartupFrame() {
		t.Fatalf("missing-key startup with invisible history should stay natural: sources=%+v transcript=%q", m.transcriptSources, m.transcript)
	}
}

func TestVisibleReplayHistoryDisablesNaturalStartupFrame(t *testing.T) {
	m := newChatTUI(control.New(control.Options{}), "", make(chan event.Event, 1), 140)
	m.history = append(m.history, provider.Message{Role: provider.RoleUser, Content: "이전 질문"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = next.(chatTUI)
	if m.isNaturalStartupFrame() {
		t.Fatalf("visible replay history should use the full transcript frame: sources=%+v transcript=%q", m.transcriptSources, m.transcript)
	}
}
