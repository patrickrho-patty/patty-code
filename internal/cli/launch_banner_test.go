package cli

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"patty/internal/control"
	"patty/internal/event"
	"patty/internal/i18n"
	"patty/internal/provider"
)

func TestLaunchMastheadCombinesApprovedArtworkAndTip(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("ko")
	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.label = "deepseek-v4"
	m.effortLevel = "medium"

	out := ansi.Strip(renderLaunchMasthead(m, "", 100))
	for _, want := range []string{
		"+-----------------------------+  +-----------------------------+",
		i18n.M.ChatTip,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("launch masthead missing %q:\n%s", want, out)
		}
	}
}

func TestLiveFactChangesDoNotRewriteLaunchScrollback(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("ko")
	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{})
	m.width = 100
	m.label = "deepseek-v4"
	m.effortLevel = "medium"
	m.commitTranscriptSource(transcriptSource{kind: transcriptSourceBanner})
	m.commitLine("preserve this transcript block")
	m.refreshLaunchMasthead() // establish the artwork/theme revision
	before := m.transcript[0]

	m.label = "deepseek-r2"
	m.effortLevel = "high"
	if changed := m.refreshLaunchMasthead(); changed {
		t.Fatal("changing live facts should not rewrite launch scrollback")
	}
	if m.transcript[0] != before {
		t.Fatal("launch artwork changed after a live fact update")
	}
	header := ansi.Strip(renderSessionHeader(m, 100))
	if !strings.Contains(header, "deepseek-r2") || !strings.Contains(header, "높음") {
		t.Fatalf("persistent session header did not render live facts:\n%s", header)
	}
	if got := m.transcript[1]; got != "preserve this transcript block" {
		t.Fatalf("refresh changed an unrelated transcript block: %q", got)
	}
	if changed := m.refreshLaunchMasthead(); changed {
		t.Fatal("unchanged session facts should not invalidate the transcript again")
	}
}

func TestStartupReplayKeepsLiveFactsInPersistentHeader(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("ko")
	ctrl := control.New(control.Options{})
	m := newChatTUI(ctrl, "", make(chan event.Event, 1), 100)
	m.label = "deepseek-v4"
	m.history = []provider.Message{{Role: provider.RoleUser, Content: "이전 질문"}}

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(chatTUI)
	if len(m.transcriptSources) < 2 || m.transcriptSources[0].kind != transcriptSourceBanner || m.transcriptSources[1].kind != transcriptSourceReplayHistory {
		t.Fatalf("startup sources = %+v, want independent banner and replay history", m.transcriptSources)
	}

	m.label = "deepseek-r2"
	m.effortLevel = "high"
	if changed := m.refreshLaunchMasthead(); changed {
		t.Fatal("startup replay artwork should not be rewritten after live fact changes")
	}
	header := ansi.Strip(renderSessionHeader(m, 100))
	for _, want := range []string{"deepseek-r2", "높음"} {
		if !strings.Contains(header, want) {
			t.Fatalf("persistent replay header missing %q:\n%s", want, header)
		}
	}
	if history := ansi.Strip(m.transcript[1]); !strings.Contains(history, "이전 질문") {
		t.Fatalf("replay history changed or disappeared:\n%s", history)
	}
}
