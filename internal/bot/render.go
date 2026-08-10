package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"patty/internal/event"
)

type messageEditor interface {
	EditMessage(ctx context.Context, messageID string, msg OutboundMessage) error
}

type renderSink struct {
	ctx        context.Context
	adapter    Adapter
	editor     messageEditor //
	connID     string
	domain     string
	chatID     string
	chatType   ChatType
	userID     string
	replyTo    string
	logger     *slog.Logger
	ctrl       botController
	onApproval func(event.Approval)
	onAsk      func(event.Ask)

	buf           strings.Builder
	thinking      strings.Builder
	inThinking    bool
	toolNames     map[string]string // tool ID -> name
	lastFlush     time.Time
	lastProgress  time.Time
	progressCount int

	liveMsgID     string    // 지금 편집 중인 메시지 ID；비어 있으면 현재 블록이 아직 메시지를 생성하지 않았음을 의미
	liveSentBytes int       // buf 앞부분에서 이미 live 메시지로 전송된 바이트 수
	lastEdit      time.Time // 마지막으로 create/edit에 성공한 시간(속도 제한용)
}

const (
	renderSoftFlushAfter      = 1200 * time.Millisecond
	renderMaxChunkRunes       = 1800
	renderHardChunkRunes      = 3500
	renderProgressMinInterval = 2 * time.Second
	renderMaxProgressMessages = 3
)

func newRenderSink(ctx context.Context, adapter Adapter, connID, domain, chatID string, chatType ChatType, userID string, replyTo string, logger *slog.Logger, onApproval func(event.Approval), onAsk func(event.Ask)) *renderSink {
	editor, _ := adapter.(messageEditor)
	return &renderSink{
		ctx:        ctx,
		adapter:    adapter,
		editor:     editor,
		connID:     connID,
		domain:     domain,
		chatID:     chatID,
		chatType:   chatType,
		userID:     userID,
		replyTo:    replyTo,
		logger:     logger,
		onApproval: onApproval,
		onAsk:      onAsk,
		toolNames:  make(map[string]string),
		lastFlush:  time.Now(),
	}
}

func (s *renderSink) Emit(e event.Event) {
	switch e.Kind {
	case event.TurnStarted:
		s.buf.Reset()
		s.thinking.Reset()
		s.inThinking = false
		s.toolNames = make(map[string]string)
		s.progressCount = 0
		s.lastProgress = time.Time{}
		s.liveMsgID = ""
		s.liveSentBytes = 0
		s.lastEdit = time.Time{}

	case event.Reasoning:
		if !s.inThinking {
			s.inThinking = true
		}
		s.thinking.WriteString(e.Text)

	case event.Text:
		if s.inThinking {
			s.inThinking = false
		}
		s.buf.WriteString(e.Text)
		s.maybeStream()

	case event.Message:

	case event.ToolDispatch:
		if e.Tool.Refreshed {
			break
		}
		name := renderToolName(e.Tool)
		s.toolNames[e.Tool.ID] = name
		s.sendProgress(fmt.Sprintf("실행 중: %s", name), false)

	case event.ToolResult:
		name := s.toolNames[e.Tool.ID]
		if name == "" {
			name = renderToolName(e.Tool)
		}
		if e.Tool.Err != "" {
			s.sendProgress(fmt.Sprintf("%s 실행 실패，나중에 결과에서 설명하겠습니다。", name), true)
		}

	case event.ToolProgress:

	case event.ApprovalRequest:
		if s.onApproval != nil {
			s.onApproval(e.Approval)
		}
		approvalText := renderApprovalText(e.Approval)
		msg := OutboundMessage{
			ConnectionID: s.connID,
			Domain:       s.domain,
			ChatID:       s.chatID,
			ChatType:     s.chatType,
			Text:         approvalText,
			ReplyToMsgID: s.replyTo,
		}
		_ = s.send(msg)

	case event.AskRequest:
		if s.onAsk != nil {
			s.onAsk(e.Ask)
		}
		askText := renderAskText(e.Ask)
		msg := OutboundMessage{
			ConnectionID: s.connID,
			Domain:       s.domain,
			ChatID:       s.chatID,
			ChatType:     s.chatType,
			Text:         askText,
			ReplyToMsgID: s.replyTo,
		}
		_ = s.send(msg)

	case event.TurnDone:
		s.flush()
		if e.Err != nil {
			if !strings.Contains(e.Err.Error(), "context canceled") {
				_ = s.send(OutboundMessage{
					ConnectionID: s.connID,
					Domain:       s.domain,
					ChatID:       s.chatID,
					ChatType:     s.chatType,
					Text:         fmt.Sprintf("❌ 실행 오류: %v", e.Err),
					ReplyToMsgID: s.replyTo,
				})
			}
		}

	case event.Notice:
		if e.Audience == event.NoticeAudienceOperator {
			s.logger.Debug("bot suppressed operator notice", "code", e.Code)
			break
		}
		if e.Level == event.LevelWarn {
			_ = s.send(OutboundMessage{
				ConnectionID: s.connID,
				Domain:       s.domain,
				ChatID:       s.chatID,
				ChatType:     s.chatType,
				Text:         fmt.Sprintf("⚠️ %s", e.Text),
				ReplyToMsgID: s.replyTo,
			})
		}

	case event.CompactionStarted:
		_ = s.send(OutboundMessage{
			ConnectionID: s.connID,
			Domain:       s.domain,
			ChatID:       s.chatID,
			ChatType:     s.chatType,
			Text:         "🔄 컨텍스트 압축 중...",
			ReplyToMsgID: s.replyTo,
		})
	}
}

func (s *renderSink) flush() {
	for strings.TrimSpace(s.buf.String()) != "" {
		raw := s.buf.String()
		if s.editor != nil && s.liveMsgID != "" && len([]rune(raw)) < renderHardChunkRunes {
			s.flushPrefix(len(raw))
			continue
		}
		idx := renderFlushIndex(raw, renderSoftFlushAfter)
		if idx <= 0 {
			idx = byteIndexForRuneLimit(raw, renderMaxChunkRunes)
		}
		if idx <= 0 || idx > len(raw) {
			idx = len(raw)
		}
		s.flushPrefix(idx)
	}
}

func (s *renderSink) flushPrefix(idx int) {
	raw := s.buf.String()
	if idx <= 0 || idx > len(raw) {
		idx = len(raw)
	}
	text := strings.TrimSpace(raw[:idx])
	if text == "" {
		remaining := raw[idx:]
		s.buf.Reset()
		s.buf.WriteString(remaining)
		s.lastFlush = time.Now()
		return
	}
	resumeFrom := idx
	if s.liveMsgID != "" {
		if err := s.editLive(text); err != nil {
			s.logger.Warn("bot live message final edit failed; sending tail as new message", "err", err)
			if tail := strings.TrimSpace(raw[min(s.liveSentBytes, idx):idx]); tail != "" {
				_ = s.send(s.textMessage(tail))
			}
			if s.liveSentBytes > resumeFrom {
				resumeFrom = s.liveSentBytes
			}
		}
		s.liveMsgID = ""
		s.liveSentBytes = 0
	} else {
		_ = s.send(s.textMessage(text))
	}
	if resumeFrom > len(raw) {
		resumeFrom = len(raw)
	}
	remaining := raw[resumeFrom:]
	s.buf.Reset()
	s.buf.WriteString(remaining)
	s.lastFlush = time.Now()
}

func (s *renderSink) maybeStream() {
	if s.editor == nil {
		return
	}
	raw := s.buf.String()
	if len([]rune(raw)) >= renderHardChunkRunes {
		idx := lastSemanticBoundary(raw, renderMaxChunkRunes)
		if idx <= 0 {
			idx = byteIndexForRuneLimit(raw, renderMaxChunkRunes)
		}
		s.flushPrefix(idx)
		return
	}
	last := s.lastEdit
	if s.liveMsgID == "" {
		last = s.lastFlush
	}
	if time.Since(last) < renderSoftFlushAfter {
		return
	}
	text := strings.TrimSpace(raw)
	if text == "" {
		return
	}
	if s.liveMsgID == "" {
		res, err := s.adapter.Send(s.ctx, s.textMessage(text))
		if err != nil {
			s.logger.Warn("bot live message create failed", "err", err)
			s.lastFlush = time.Now()
			return
		}
		if strings.TrimSpace(res.MessageID) == "" {
			s.editor = nil
			s.cutBufPrefix(len(raw))
			return
		}
		s.liveMsgID = res.MessageID
		s.liveSentBytes = len(raw)
		s.lastEdit = time.Now()
		return
	}
	if err := s.editLive(text); err != nil {
		s.logger.Warn("bot live message edit failed; rotating to new message", "err", err)
		s.cutBufPrefix(s.liveSentBytes)
		s.liveMsgID = ""
		s.liveSentBytes = 0
		return
	}
	s.liveSentBytes = len(raw)
	s.lastEdit = time.Now()
}

func (s *renderSink) editLive(text string) error {
	err := s.editor.EditMessage(s.ctx, s.liveMsgID, s.textMessage(text))
	if err == nil {
		s.lastEdit = time.Now()
	}
	return err
}

func (s *renderSink) cutBufPrefix(n int) {
	raw := s.buf.String()
	if n <= 0 {
		return
	}
	if n > len(raw) {
		n = len(raw)
	}
	remaining := raw[n:]
	s.buf.Reset()
	s.buf.WriteString(remaining)
	s.lastFlush = time.Now()
}

func (s *renderSink) textMessage(text string) OutboundMessage {
	return OutboundMessage{
		ConnectionID: s.connID,
		Domain:       s.domain,
		ChatID:       s.chatID,
		ChatType:     s.chatType,
		Text:         text,
		ReplyToMsgID: s.replyTo,
	}
}

func (s *renderSink) sendProgress(text string, force bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	now := time.Now()
	if s.progressCount >= renderMaxProgressMessages {
		return
	}
	if !force && !s.lastProgress.IsZero() && now.Sub(s.lastProgress) < renderProgressMinInterval {
		return
	}
	_ = s.send(OutboundMessage{
		ConnectionID: s.connID,
		Domain:       s.domain,
		ChatID:       s.chatID,
		ChatType:     s.chatType,
		Text:         text,
		ReplyToMsgID: s.replyTo,
	})
	s.progressCount++
	s.lastProgress = now
}

func renderToolName(t event.Tool) string {
	if name := strings.TrimSpace(t.Name); name != "" {
		return name
	}
	if id := strings.TrimSpace(t.ID); id != "" {
		return id
	}
	return "tool"
}

func renderFlushIndex(text string, elapsed time.Duration) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	runes := []rune(text)
	if len(runes) >= renderHardChunkRunes {
		if idx := lastSemanticBoundary(text, renderHardChunkRunes); idx > 0 {
			return idx
		}
		return byteIndexForRuneLimit(text, renderMaxChunkRunes)
	}
	if len(runes) >= renderMaxChunkRunes {
		if idx := lastSemanticBoundary(text, renderMaxChunkRunes); idx > 0 {
			return idx
		}
	}
	if elapsed < renderSoftFlushAfter {
		return 0
	}
	return lastSemanticBoundary(text, len(runes))
}

func lastSemanticBoundary(text string, maxRunes int) int {
	if maxRunes <= 0 {
		return 0
	}
	count := 0
	lastBoundary := 0
	lastNonSpaceBoundary := 0
	inFence := false
	for idx, r := range text {
		if strings.HasPrefix(text[idx:], "```") {
			inFence = !inFence
		}
		count++
		if count > maxRunes {
			break
		}
		next := idx + len(string(r))
		if r == '\n' && !inFence {
			lastNonSpaceBoundary = next
			lastBoundary = next
			continue
		}
		if unicode.IsSpace(r) {
			if lastNonSpaceBoundary > 0 {
				lastBoundary = next
			}
			continue
		}
		if inFence {
			continue
		}
		if isSemanticBoundaryRune(r) {
			lastNonSpaceBoundary = next
			lastBoundary = next
		}
	}
	return lastBoundary
}

func isSemanticBoundaryRune(r rune) bool {
	switch r {
	case '.', '!', '?', ';', '。', '！', '？', '；', '…':
		return true
	default:
		return false
	}
}

func byteIndexForRuneLimit(text string, maxRunes int) int {
	if maxRunes <= 0 {
		return 0
	}
	count := 0
	for idx, r := range text {
		count++
		if count >= maxRunes {
			return idx + len(string(r))
		}
	}
	return len(text)
}

func (s *renderSink) send(msg OutboundMessage) error {
	_, err := s.adapter.Send(s.ctx, msg)
	return err
}

func approvalKeyboard(id string) *InlineKeyboard {
	return &InlineKeyboard{Rows: []InlineKeyboardRow{{
		Buttons: []InlineKeyboardButton{
			{ID: "allow_once", Label: "한 번 허용", Style: 1, CallbackID: "/approve " + id},
			{ID: "deny", Label: "거절", Style: 2, CallbackID: "/deny " + id},
		},
	}}}
}

func recoveryKeyboard(a event.Approval) *InlineKeyboard {
	if isRecoveryPlanChange(a) {
		return &InlineKeyboard{Rows: []InlineKeyboardRow{{Buttons: []InlineKeyboardButton{
			{ID: "recovery_continue", Label: "1 채택하고 계속", Style: 0, CallbackID: "/recovery-continue " + a.ID},
			{ID: "recovery_revise", Label: "2 채택하지 않고 조정", Style: 0, CallbackID: "/recovery-revise " + a.ID},
		}}}}
	}
	buttons := []InlineKeyboardButton{{ID: "recovery_continue", Label: "1 한 번 계속", Style: 1, CallbackID: "/recovery-continue " + a.ID}}
	if a.Recovery != nil && a.Recovery.CanGrantTask {
		buttons = append(buttons, InlineKeyboardButton{ID: "recovery_continue_task", Label: "2 본 작업에서 동일 유형 허용", Style: 0, CallbackID: "/recovery-continue-task " + a.ID})
		return &InlineKeyboard{Rows: []InlineKeyboardRow{{Buttons: buttons}, {Buttons: []InlineKeyboardButton{{ID: "recovery_revise", Label: "3 다른 방법으로", Style: 0, CallbackID: "/recovery-revise " + a.ID}}}}}
	}
	buttons = append(buttons, InlineKeyboardButton{ID: "recovery_revise", Label: "2 다른 방법으로", Style: 0, CallbackID: "/recovery-revise " + a.ID})
	return &InlineKeyboard{Rows: []InlineKeyboardRow{{Buttons: buttons}}}
}

func isRecoveryApproval(a event.Approval) bool {
	return strings.EqualFold(strings.TrimSpace(a.Kind), "recovery") || a.Recovery != nil
}

func isRecoveryPlanChange(a event.Approval) bool {
	if !isRecoveryApproval(a) || a.Recovery == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(a.Recovery.ChangeKind)) {
	case "strategy", "scope":
		return true
	default:
		return false
	}
}

func renderApprovalText(a event.Approval) string {
	if isRecoveryApproval(a) {
		return renderRecoveryText(a)
	}
	return fmt.Sprintf("⚠️ 승인 필요:\n도구: %s\n작업: %s\n\nID: `%s`\n1을 답변하면 승인，2를 답변하면 거절；/approve %s 또는 /deny %s를 사용할 수도 있습니다.",
		a.Tool, a.Subject, a.ID, a.ID, a.ID)
}

func renderRecoveryText(a event.Approval) string {
	var b strings.Builder
	if isRecoveryPlanChange(a) {
		b.WriteString("⚠️ 실행 계획 결정 필요\n")
	} else {
		b.WriteString("⚠️ 실행 전 확인\n")
	}
	rec := a.Recovery
	if rec != nil {
		if isRecoveryPlanChange(a) && (strings.TrimSpace(rec.PlanBefore) != "" || strings.TrimSpace(rec.PlanAfter) != "") {
			if before := clipBotPlan(rec.PlanBefore); before != "" {
				fmt.Fprintf(&b, "기존 계획:\n%s\n", before)
			}
			if after := clipBotPlan(rec.PlanAfter); after != "" {
				fmt.Fprintf(&b, "새 계획:\n%s\n", after)
			}
		} else if next := firstNonEmptyBot(rec.NextAction, a.Subject, a.Tool); next != "" {
			fmt.Fprintf(&b, "곧 실행: %s\n", next)
		}
		why := firstNonEmptyBot(rec.ChangeRationale, rec.ReviewRationale, a.Reason)
		if why != "" {
			fmt.Fprintf(&b, "이유: %s\n", why)
		}
	} else {
		fmt.Fprintf(&b, "곧 실행: %s\n", firstNonEmptyBot(a.Subject, a.Tool))
	}
	if isRecoveryPlanChange(a) {
		fmt.Fprintf(&b, "\nID: `%s`\n1을 답변하여 새 계획을 채택하고 계속하세요，2 채택하지 않고 Auto에게 조정 요청。구체적인 의견이 필요하면 `/recovery-revise %s <조정 의견>`을 사용하세요。", a.ID, a.ID)
	} else if rec != nil && rec.CanGrantTask {
		if scope := strings.TrimSpace(rec.TaskGrantScope); scope != "" {
			fmt.Fprintf(&b, "권한 범위: %s\n", scope)
		}
		fmt.Fprintf(&b, "\nID: `%s`\n1을 답변하면 한 번 계속，2를 답변하면 이 작업 내에서 동일한 유형의 작업 허용，3을 답변하면 다른 방법으로。범위 확대나 위험 레벨 상승 시 다시 확인할 수 있습니다。", a.ID)
	} else {
		fmt.Fprintf(&b, "\nID: `%s`\n1을 답변하면 계속，2를 답변하면 다른 방법으로。", a.ID)
	}
	return b.String()
}

func approvalCard(a event.Approval, chatType ChatType, userID string) *InteractiveCard {
	return &InteractiveCard{
		Header: "승인 필요",
		Elements: []InteractiveCardElement{
			{Tag: "markdown", Content: fmt.Sprintf("**도구**: %s\n\n**작업**: %s\n\nID: `%s`", a.Tool, a.Subject, a.ID)},
			{Tag: "action", Extra: map[string]any{
				"actions": []map[string]any{
					{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "한 번 허용"}, "type": "primary", "value": cardActionValue("/approve "+a.ID, chatType, userID)},
					{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "거절"}, "type": "danger", "value": cardActionValue("/deny "+a.ID, chatType, userID)},
				},
			}},
		},
	}
}

func recoveryCard(a event.Approval, chatType ChatType, userID string) *InteractiveCard {
	if isRecoveryPlanChange(a) {
		return &InteractiveCard{
			Header: "실행 계획 결정 필요",
			Elements: []InteractiveCardElement{
				{Tag: "markdown", Content: renderRecoveryText(a)},
				{Tag: "action", Extra: map[string]any{
					"actions": []map[string]any{
						{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "채택하고 계속"}, "type": "default", "value": cardActionValue("/recovery-continue "+a.ID, chatType, userID)},
						{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "채택하지 않고 조정"}, "type": "default", "value": cardActionValue("/recovery-revise "+a.ID, chatType, userID)},
					},
				}},
			},
		}
	}
	actions := []map[string]any{
		{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "한 번 계속"}, "type": "primary", "value": cardActionValue("/recovery-continue "+a.ID, chatType, userID)},
	}
	if a.Recovery != nil && a.Recovery.CanGrantTask {
		actions = append(actions, map[string]any{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "본 작업에서 동일 유형 허용"}, "type": "default", "value": cardActionValue("/recovery-continue-task "+a.ID, chatType, userID)})
	}
	actions = append(actions, map[string]any{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "다른 방법으로"}, "type": "default", "value": cardActionValue("/recovery-revise "+a.ID, chatType, userID)})
	return &InteractiveCard{
		Header: "실행 전 확인",
		Elements: []InteractiveCardElement{
			{Tag: "markdown", Content: renderRecoveryText(a)},
			{Tag: "action", Extra: map[string]any{
				"actions": actions,
			}},
		},
	}
}

func clipBotPlan(plan string) string {
	plan = strings.TrimSpace(plan)
	const maxRunes = 800
	runes := []rune(plan)
	if len(runes) <= maxRunes {
		return plan
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "…"
}

func firstNonEmptyBot(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func cardActionValue(command string, chatType ChatType, userID string) map[string]string {
	value := map[string]string{
		"command":   command,
		"chat_type": string(chatType),
	}
	if strings.TrimSpace(userID) != "" {
		value["user_id"] = strings.TrimSpace(userID)
	}
	return value
}

func renderAskText(ask event.Ask) string {
	var qb strings.Builder
	qb.WriteString("❓ 다음 질문에 답해 주세요:\n")
	for i, q := range ask.Questions {
		fmt.Fprintf(&qb, "\n**%d. %s**\n", i+1, q.Prompt)
		for j, opt := range q.Options {
			fmt.Fprintf(&qb, "  %d. %s", j+1, opt.Label)
			if opt.Description != "" {
				fmt.Fprintf(&qb, " — %s", opt.Description)
			}
			qb.WriteString("\n")
		}
		if q.Multi {
			qb.WriteString("  (여러 개 선택 가능)\n")
		}
	}
	fmt.Fprintf(&qb, "\nID: `%s`", ask.ID)
	if askSupportsNumericShortcut(ask) {
		fmt.Fprintf(&qb, "\n옵션 번호를 직접 회신하여 답변하세요；/answer %s <옵션 번호 또는 텍스트>를 사용할 수도 있습니다。", ask.ID)
	} else {
		fmt.Fprintf(&qb, "\n/answer %s <옵션 번호 또는 텍스트>로 답변하세요；여러 문제에는 q1=1;q2=2。", ask.ID)
	}
	return qb.String()
}

func askCard(ask event.Ask, fallback string, chatType ChatType, userID string) *InteractiveCard {
	card := &InteractiveCard{
		Header: "답변 필요",
		Elements: []InteractiveCardElement{
			{Tag: "markdown", Content: fallback},
		},
	}
	if !askSupportsNumericShortcut(ask) {
		return card
	}
	question := ask.Questions[0]
	actions := make([]map[string]any, 0, len(question.Options))
	for i, opt := range question.Options {
		label := strings.TrimSpace(opt.Label)
		if label == "" {
			label = fmt.Sprintf("옵션 %d", i+1)
		}
		actions = append(actions, map[string]any{
			"tag":   "button",
			"text":  map[string]string{"tag": "plain_text", "content": label},
			"type":  "primary",
			"value": cardActionValue(fmt.Sprintf("/answer %s %d", ask.ID, i+1), chatType, userID),
		})
	}
	if len(actions) > 0 {
		card.Elements = append(card.Elements, InteractiveCardElement{Tag: "action", Extra: map[string]any{"actions": actions}})
	}
	return card
}

func askSupportsNumericShortcut(ask event.Ask) bool {
	return len(ask.Questions) == 1 && len(ask.Questions[0].Options) > 0
}
