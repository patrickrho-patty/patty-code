package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"patty/internal/bot"
	"patty/internal/event"
)

const botForwardSendTimeout = 30 * time.Second
const botForwardQueueSize = 64


type botForwardTarget struct {
	ConnID   string
	Domain   string
	ChatID   string
	ChatType bot.ChatType
}


//
type botEventForwarder struct {
	runtime *desktopBotRuntime
	targets []botForwardTarget

	mu        sync.Mutex
	buf       strings.Builder
	queueMu   sync.Mutex
	queue     chan string
	closed    bool
	closeOnce sync.Once
}

func newBotEventForwarder(runtime *desktopBotRuntime, targets []botForwardTarget) *botEventForwarder {
	f := &botEventForwarder{
		runtime: runtime,
		targets: targets,
		queue:   make(chan string, botForwardQueueSize),
	}
	go f.run()
	return f
}

func (f *botEventForwarder) Emit(e event.Event) {
	if f.runtime == nil || len(f.targets) == 0 {
		return
	}
	switch e.Kind {
	case event.TurnStarted:
		f.mu.Lock()
		f.buf.Reset()
		f.mu.Unlock()

	case event.Text:
		f.mu.Lock()
		f.buf.WriteString(e.Text)
		size := f.buf.Len()
		f.mu.Unlock()
		if size >= 400 {
			f.flush()
		}

	case event.TurnDone:
		f.flush()
		f.Close()

	case event.ApprovalRequest:
		text := "⚠️ Patty Code 데스크톱에서 승인 필요: " + e.Approval.Tool + " — " + e.Approval.Subject
		text += "\n데스크톱 창으로 돌아가 처리하세요."
		f.sendToAll(text)

	case event.AskRequest:
		var qb strings.Builder
		qb.WriteString("❓ Patty Code 데스크톱에서 질문에 답변해야 합니다:\n")
		for i, q := range e.Ask.Questions {
			if i > 0 {
				qb.WriteString("\n")
			}
			qb.WriteString(q.Prompt)
		}
		qb.WriteString("\n데스크톱 창으로 돌아가 처리하세요.")
		f.sendToAll(qb.String())

	case event.Notice:
		if e.Audience == event.NoticeAudienceOperator {
			break
		}
		if e.Level == event.LevelWarn {
			f.sendToAll("⚠️ " + e.Text)
		}

	case event.CompactionStarted:
		f.sendToAll("🔄 컨텍스트 압축 중...")
	}
}

func (f *botEventForwarder) flush() {
	f.mu.Lock()
	text := strings.TrimSpace(f.buf.String())
	if text == "" {
		f.mu.Unlock()
		return
	}
	f.buf.Reset()
	f.mu.Unlock()

	f.sendToAll(text)
}

func (f *botEventForwarder) sendToAll(text string) {
	text = strings.TrimSpace(text)
	if f.runtime == nil || len(f.targets) == 0 || text == "" {
		return
	}
	f.queueMu.Lock()
	defer f.queueMu.Unlock()
	if f.closed {
		return
	}
	select {
	case f.queue <- text:
	default:
		log.Printf("[bot-forward] send queue full; dropping message for %d target(s)", len(f.targets))
	}
}

func (f *botEventForwarder) run() {
	for text := range f.queue {
		f.sendToAllNow(text)
	}
}

func (f *botEventForwarder) sendToAllNow(text string) {
	for _, tgt := range f.targets {
		ctx, cancel := context.WithTimeout(context.Background(), botForwardSendTimeout)
		_, err := f.runtime.SendToAdapter(ctx, tgt.ConnID, tgt.Domain, bot.OutboundMessage{
			ChatID:   tgt.ChatID,
			ChatType: tgt.ChatType,
			Text:     text,
		})
		cancel()
		if err != nil {
			log.Printf("[bot-forward] send to %s/%s failed: %v", tgt.ConnID, tgt.ChatType, err)
		}
	}
}

func (f *botEventForwarder) Close() {
	f.closeOnce.Do(func() {
		f.flush()
		f.queueMu.Lock()
		f.closed = true
		close(f.queue)
		f.queueMu.Unlock()
	})
}
