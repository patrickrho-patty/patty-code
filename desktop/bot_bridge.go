package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"patty/internal/bot"
	"patty/internal/event"
)

var errDriveBusy = errors.New("desktop session busy")

//
//
type botBridgeHub struct {
	sessions   func() []bot.DesktopSessionInfo
	approveTab func(tabID, id string, allow, session, persist bool)
	answerTab  func(tabID, id string, answers []QuestionAnswer)
	notify     func(ctx context.Context, connectionID, domain string, msg bot.OutboundMessage) (bot.SendResult, error)
	drive func(tabID, text string, route bot.DesktopWatchRoute) error
	announce func(tabID, text string)
	persistWatchers func(routes []bot.DesktopWatchRoute) error
	takeoverChanged func()
	logger          *slog.Logger

	mu       sync.Mutex
	watchers map[string]bot.DesktopWatchRoute
	pending  map[string]desktopPendingPrompt
	takeovers    map[string]bot.DesktopWatchRoute
	takeoverTabs map[string]string
	watchSeq uint64
	watchPersistDirty bool

	persistMu      sync.Mutex
	lastPersistSeq uint64

	queue chan desktopBridgeNotification
}

type desktopPendingPrompt struct {
	tabID     string
	kind      string // "approval" | "ask"
	tool      string
	subject   string
	questions []event.AskQuestion
}

type desktopBridgeNotification struct {
	text  func(route bot.DesktopWatchRoute) string
	card  func(route bot.DesktopWatchRoute) *bot.InteractiveCard
	route *bot.DesktopWatchRoute
}

func isSharedChat(ct bot.ChatType) bool {
	return ct != bot.ChatDM && ct != bot.ChatDirect
}

func constText(s string) func(bot.DesktopWatchRoute) string {
	return func(bot.DesktopWatchRoute) string { return s }
}

const (
	botBridgeQueueSize     = 64
	botBridgeSendTimeout   = 15 * time.Second
	botBridgeSubjectLimit  = 200
	botBridgePendingLimit  = 200
	botBridgeErrTextLimit  = 300
	botBridgePromptPreview = 500
)

type botBridgeDeps struct {
	sessions        func() []bot.DesktopSessionInfo
	approveTab      func(tabID, id string, allow, session, persist bool)
	answerTab       func(tabID, id string, answers []QuestionAnswer)
	notify          func(ctx context.Context, connectionID, domain string, msg bot.OutboundMessage) (bot.SendResult, error)
	drive           func(tabID, text string, route bot.DesktopWatchRoute) error
	announce        func(tabID, text string)
	persistWatchers func(routes []bot.DesktopWatchRoute) error
	takeoverChanged func()
	logger          *slog.Logger
}

func newBotBridgeHub(deps botBridgeDeps) *botBridgeHub {
	logger := deps.logger
	if logger == nil {
		logger = slog.Default()
	}
	h := &botBridgeHub{
		sessions:        deps.sessions,
		approveTab:      deps.approveTab,
		answerTab:       deps.answerTab,
		notify:          deps.notify,
		drive:           deps.drive,
		announce:        deps.announce,
		persistWatchers: deps.persistWatchers,
		takeoverChanged: deps.takeoverChanged,
		logger:          logger.With("component", "bot_bridge"),
		watchers:        make(map[string]bot.DesktopWatchRoute),
		pending:         make(map[string]desktopPendingPrompt),
		takeovers:       make(map[string]bot.DesktopWatchRoute),
		takeoverTabs:    make(map[string]string),
		queue:           make(chan desktopBridgeNotification, botBridgeQueueSize),
	}
	go h.run()
	return h
}

func (h *botBridgeHub) observe(tabID string, e event.Event) {
	switch e.Kind {
	case event.ApprovalRequest:
		h.mu.Lock()
		h.rememberPendingLocked(e.Approval.ID, desktopPendingPrompt{
			tabID:   tabID,
			kind:    "approval",
			tool:    e.Approval.Tool,
			subject: truncateForBridge(e.Approval.Subject, botBridgeSubjectLimit),
		})
		watching := len(h.watchers) > 0
		h.mu.Unlock()
		if watching {
			h.enqueue(h.approvalNotification(tabID, e.Approval))
		}
	case event.AskRequest:
		h.mu.Lock()
		h.rememberPendingLocked(e.Ask.ID, desktopPendingPrompt{
			tabID:     tabID,
			kind:      "ask",
			questions: e.Ask.Questions,
		})
		watching := len(h.watchers) > 0
		h.mu.Unlock()
		if watching {
			h.enqueue(h.askNotification(tabID, e.Ask))
		}
	case event.TurnDone:
		h.mu.Lock()
		for id, p := range h.pending {
			if p.tabID == tabID {
				delete(h.pending, id)
			}
		}
		watching := len(h.watchers) > 0
		h.mu.Unlock()
		if !watching {
			return
		}
		if e.Err != nil && strings.Contains(e.Err.Error(), "context canceled") {
			return
		}
		h.enqueue(h.turnDoneNotification(tabID, e))
	}
}

func (h *botBridgeHub) rememberPendingLocked(id string, p desktopPendingPrompt) {
	if strings.TrimSpace(id) == "" {
		return
	}
	if len(h.pending) >= botBridgePendingLimit {
		h.pending = make(map[string]desktopPendingPrompt)
	}
	h.pending[id] = p
}

func (h *botBridgeHub) enqueue(n desktopBridgeNotification) {
	select {
	case h.queue <- n:
	default:
		h.logger.Warn("desktop bridge notification queue full; dropping")
	}
}

func (h *botBridgeHub) run() {
	for n := range h.queue {
		h.deliver(n)
	}
}

func (h *botBridgeHub) deliver(n desktopBridgeNotification) {
	h.mu.Lock()
	var routes []bot.DesktopWatchRoute
	if n.route != nil {
		routes = []bot.DesktopWatchRoute{*n.route}
	} else {
		routes = h.watcherRoutesLocked()
	}
	notify := h.notify
	h.mu.Unlock()
	if notify == nil || len(routes) == 0 {
		return
	}
	var wg sync.WaitGroup
	for _, route := range routes {
		wg.Add(1)
		go func(route bot.DesktopWatchRoute) {
			defer wg.Done()
			msg := bot.OutboundMessage{
				ChatID:   route.ChatID,
				ChatType: route.ChatType,
			}
			if n.text != nil {
				msg.Text = n.text(route)
			}
			if n.card != nil {
				msg.Card = n.card(route)
			}
			ctx, cancel := context.WithTimeout(context.Background(), botBridgeSendTimeout)
			defer cancel()
			if _, err := notify(ctx, route.ConnectionID, route.Domain, msg); err != nil {
				h.logger.Warn("desktop bridge notification send failed", "platform", route.Platform, "err", err)
			}
		}(route)
	}
	wg.Wait()
}

func (h *botBridgeHub) tabLabel(tabID string) string {
	if s, ok := h.sessionByTabID(tabID); ok {
		if label := strings.TrimSpace(s.Label); label != "" {
			return label
		}
		if title := strings.TrimSpace(s.Topic); title != "" {
			return title
		}
	}
	return "(이름 없음 세션)"
}

func (h *botBridgeHub) sessionByTabID(tabID string) (bot.DesktopSessionInfo, bool) {
	if h.sessions == nil {
		return bot.DesktopSessionInfo{}, false
	}
	for _, s := range h.sessions() {
		if s.TabID == tabID {
			return s, true
		}
	}
	return bot.DesktopSessionInfo{}, false
}

func (h *botBridgeHub) approvalNotification(tabID string, approval event.Approval) desktopBridgeNotification {
	label := h.tabLabel(tabID)
	subjectFor := func(route bot.DesktopWatchRoute) string {
		if isSharedChat(route.ChatType) {
			return "(명령 상세 정보 데스크톱 또는 DM에서만 표시)"
		}
		return truncateForBridge(approval.Subject, botBridgeSubjectLimit)
	}
	return desktopBridgeNotification{
		text: func(route bot.DesktopWatchRoute) string {
			return fmt.Sprintf("⚠️ 데스크톱 세션 「%s」의 승인이 필요합니다:\n도구: %s\n작업: %s\n\nID: `%s`\n/desktop approve %s로 승인하세요. /desktop deny %s로 거절합니다. 먼저 처리한 쪽이 우선됩니다.",
				label, approval.Tool, subjectFor(route), approval.ID, approval.ID, approval.ID)
		},
		card: func(route bot.DesktopWatchRoute) *bot.InteractiveCard {
			return &bot.InteractiveCard{
				Header: "데스크톱 세션 승인 필요",
				Elements: []bot.InteractiveCardElement{
					{Tag: "markdown", Content: fmt.Sprintf("**세션**: %s\n\n**도구**: %s\n\n**작업**: %s\n\nID: `%s`", label, approval.Tool, subjectFor(route), approval.ID)},
					{Tag: "action", Extra: map[string]any{
						"actions": []map[string]any{
							desktopCardButton("한 번 허용", "primary", "/desktop approve "+approval.ID, route),
							desktopCardButton("거절", "danger", "/desktop deny "+approval.ID, route),
						},
					}},
				},
			}
		},
	}
}

func (h *botBridgeHub) askNotification(tabID string, ask event.Ask) desktopBridgeNotification {
	label := h.tabLabel(tabID)
	var b strings.Builder
	fmt.Fprintf(&b, "❓ 데스크톱 세션 「%s」이(가) 답변을 기다리고 있습니다:\n", label)
	for i, q := range ask.Questions {
		fmt.Fprintf(&b, "\n**%d. %s**\n", i+1, truncateForBridge(q.Prompt, botBridgePromptPreview))
		for j, opt := range q.Options {
			fmt.Fprintf(&b, "  %d. %s\n", j+1, opt.Label)
		}
	}
	fmt.Fprintf(&b, "\nID: `%s`\n/desktop answer %s <옵션 번호 또는 텍스트> 로 답변하세요. 먼저 처리한 쪽 승리.", ask.ID, ask.ID)
	privateText := b.String()
	sharedText := fmt.Sprintf("❓ 데스크톱 세션 「%s」답변을 기다리고 있습니다 (질문 상세 정보는 데스크톱 또는 DM에서만 표시됩니다)。\n\nID: `%s`", label, ask.ID)
	textFor := func(route bot.DesktopWatchRoute) string {
		if isSharedChat(route.ChatType) {
			return sharedText
		}
		return privateText
	}

	var card func(route bot.DesktopWatchRoute) *bot.InteractiveCard
	if len(ask.Questions) == 1 && len(ask.Questions[0].Options) > 0 {
		options := ask.Questions[0].Options
		card = func(route bot.DesktopWatchRoute) *bot.InteractiveCard {
			if isSharedChat(route.ChatType) {
				return nil
			}
			actions := make([]map[string]any, 0, len(options))
			for i, opt := range options {
				optLabel := strings.TrimSpace(opt.Label)
				if optLabel == "" {
					optLabel = fmt.Sprintf("옵션 %d", i+1)
				}
				actions = append(actions, desktopCardButton(optLabel, "primary", fmt.Sprintf("/desktop answer %s %d", ask.ID, i+1), route))
			}
			return &bot.InteractiveCard{
				Header: "데스크톱 세션 답변 대기 중",
				Elements: []bot.InteractiveCardElement{
					{Tag: "markdown", Content: privateText},
					{Tag: "action", Extra: map[string]any{"actions": actions}},
				},
			}
		}
	}
	return desktopBridgeNotification{text: textFor, card: card}
}

func (h *botBridgeHub) turnDoneNotification(tabID string, e event.Event) desktopBridgeNotification {
	label := h.tabLabel(tabID)
	if e.Outcome == event.TurnOutcomeRecoveryPaused {
		return desktopBridgeNotification{text: constText(fmt.Sprintf(
			"⏸️ 데스크톱 세션 '%s' 자동 재시도 일시 중지. 완료된 작업 유지; '계속' 전송으로 새 라운드 시작, 방향 조정 요구도 가능.",
			label,
		))}
	}
	if e.Err != nil {
		return desktopBridgeNotification{text: func(route bot.DesktopWatchRoute) string {
			if isSharedChat(route.ChatType) {
				return fmt.Sprintf("❌ 데스크톱 세션 '%s' 작업 오류 (상세 Desktop 또는 DM 참조).", label)
			}
			return fmt.Sprintf("❌ 데스크톱 세션 '%s' 작업 오류: %s", label, truncateForBridge(e.Err.Error(), botBridgeErrTextLimit))
		}}
	}
	return desktopBridgeNotification{text: constText(fmt.Sprintf("✅ 데스크톱 세션 '%s' 작업 완료.", label))}
}

func desktopCardButton(label, style, command string, route bot.DesktopWatchRoute) map[string]any {
	return map[string]any{
		"tag":  "button",
		"text": map[string]string{"tag": "plain_text", "content": label},
		"type": style,
		"value": map[string]string{
			"command":   command,
			"chat_type": string(route.ChatType),
		},
	}
}

func truncateForBridge(s string, limit int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}


func (h *botBridgeHub) Sessions() []bot.DesktopSessionInfo {
	if h.sessions == nil {
		return nil
	}
	sessions := h.sessions()
	h.mu.Lock()
	byTab := make(map[string][]bot.DesktopPendingInfo, len(h.pending))
	for id, p := range h.pending {
		byTab[p.tabID] = append(byTab[p.tabID], bot.DesktopPendingInfo{ID: id, Kind: p.kind, Tool: p.tool})
	}
	h.mu.Unlock()
	for i := range sessions {
		if pend := byTab[sessions[i].TabID]; len(pend) > 0 {
			sort.Slice(pend, func(a, b int) bool { return pend[a].ID < pend[b].ID })
			sessions[i].Pending = pend
		}
	}
	return sessions
}

func (h *botBridgeHub) SetWatch(route bot.DesktopWatchRoute, enable bool) error {
	h.mu.Lock()
	if enable {
		h.watchers[route.Key()] = route
	} else {
		delete(h.watchers, route.Key())
	}
	h.watchSeq++
	h.watchPersistDirty = true
	seq := h.watchSeq
	routes := h.watcherRoutesLocked()
	persist := h.persistWatchers
	h.mu.Unlock()
	if persist == nil {
		return nil
	}
	h.persistMu.Lock()
	defer h.persistMu.Unlock()
	if seq <= h.lastPersistSeq {
		return nil
	}
	if err := persist(routes); err != nil {
		return err
	}
	h.lastPersistSeq = seq
	h.mu.Lock()
	if h.watchSeq == seq {
		h.watchPersistDirty = false
	}
	h.mu.Unlock()
	return nil
}

func (h *botBridgeHub) watcherVersion() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.watchSeq
}

func (h *botBridgeHub) seedWatchers(routes []bot.DesktopWatchRoute, expectedSeq uint64) {
	h.persistMu.Lock()
	defer h.persistMu.Unlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.watchSeq != expectedSeq || h.watchPersistDirty {
		return
	}
	h.watchers = make(map[string]bot.DesktopWatchRoute, len(routes))
	for _, r := range routes {
		if strings.TrimSpace(r.ChatID) == "" {
			continue
		}
		h.watchers[r.Key()] = r
	}
}

func (h *botBridgeHub) watcherRoutesLocked() []bot.DesktopWatchRoute {
	routes := make([]bot.DesktopWatchRoute, 0, len(h.watchers))
	for _, r := range h.watchers {
		routes = append(routes, r)
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Key() < routes[j].Key() })
	return routes
}

func (h *botBridgeHub) Watching(route bot.DesktopWatchRoute) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.watchers[route.Key()]
	return ok
}

func (h *botBridgeHub) Approve(approvalID string, allow bool) (string, error) {
	approvalID = strings.TrimSpace(approvalID)
	h.mu.Lock()
	p, ok := h.pending[approvalID]
	if ok && p.kind == "approval" {
		delete(h.pending, approvalID)
	}
	h.mu.Unlock()
	if !ok || p.kind != "approval" {
		return "", fmt.Errorf("대기 중인 승인을 찾을 수 없습니다 %s (이미 데스크톱에서 처리되었거나 만료되었습니다). /desktop status로 현재 세션을 확인하세요.", approvalID)
	}
	if h.approveTab == nil {
		return "", fmt.Errorf("데스크톱 승인 채널을 사용할 수 없습니다.")
	}
	h.approveTab(p.tabID, approvalID, allow, false, false)
	action := "승인"
	if !allow {
		action = "거절"
	}
	return fmt.Sprintf("%s 「%s」의 작업 제출 완료 (%s).데스크톱에서 먼저 처리한 쪽이 우선됩니다。", action, h.tabLabel(p.tabID), p.tool), nil
}

func (h *botBridgeHub) AskQuestions(askID string) ([]event.AskQuestion, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.pending[strings.TrimSpace(askID)]
	if !ok || p.kind != "ask" {
		return nil, false
	}
	return p.questions, true
}

func (h *botBridgeHub) Answer(askID string, answers []event.AskAnswer) (string, error) {
	askID = strings.TrimSpace(askID)
	h.mu.Lock()
	p, ok := h.pending[askID]
	if ok && p.kind == "ask" {
		delete(h.pending, askID)
	}
	h.mu.Unlock()
	if !ok || p.kind != "ask" {
		return "", fmt.Errorf("답변 대기 중인 질문 %s 없음 (이미 데스크톱에서 답변하거나 만료되었음).", askID)
	}
	if h.answerTab == nil {
		return "", fmt.Errorf("Desktop 질답 채널 사용 불가.")
	}
	out := make([]QuestionAnswer, 0, len(answers))
	for _, an := range answers {
		out = append(out, QuestionAnswer{QuestionID: an.QuestionID, Selected: an.Selected})
	}
	h.answerTab(p.tabID, askID, out)
	return fmt.Sprintf("「%s」에 대한 답변 제출 완료。데스크톱에서 먼저 처리한 쪽이 우선됩니다。", h.tabLabel(p.tabID)), nil
}


func (h *botBridgeHub) Takeover(route bot.DesktopWatchRoute, tabID string) (string, error) {
	tabID = strings.TrimSpace(tabID)
	if route.ChatType != bot.ChatDM {
		return "", fmt.Errorf("인수는 DM만 지원: 그룹에서 인수하면 다른 구성원도 데스크톱 세션 운전 가능. bot 과의 DM 에서 인수.")
	}
	session, ok := h.sessionByTabID(tabID)
	if !ok {
		return "", fmt.Errorf("세션 %s 없음. /desktop status 로 인수 가능한 세션 확인.", tabID)
	}
	if session.Detached {
		return "", fmt.Errorf("세션 「%s」이 백그라운드에서 실행 중이며, 현재 인수할 수 없습니다. 먼저 데스크톱에서 열어주세요.", h.tabLabel(tabID))
	}
	h.mu.Lock()
	if holder, held := h.takeovers[tabID]; held && holder.Key() != route.Key() {
		h.mu.Unlock()
		return "", fmt.Errorf("세션 '%s' 는 이미 다른 채팅에 의해 인수됨.", h.tabLabel(tabID))
	}
	released := ""
	if prev, ok := h.takeoverTabs[route.Key()]; ok && prev != tabID {
		delete(h.takeovers, prev)
		released = prev
	}
	h.takeovers[tabID] = route
	h.takeoverTabs[route.Key()] = tabID
	announce := h.announce
	changed := h.takeoverChanged
	h.mu.Unlock()
	if announce != nil {
		if released != "" {
			announce(released, "IM 원격 인수 해제됨 (다른 채팅으로 전환되었습니다).")
		}
		announce(tabID, "이 세션은 이미 IM 원격에 의해 인수됨 (bot 관리자). 여기서 로컬 전송 임의 메시지로 제어 회수.")
	}
	if changed != nil {
		changed()
	}
	label := h.tabLabel(tabID)
	return fmt.Sprintf("「%s」을(를) 인수했습니다. 이제 직접 메시지를 보내서 구동할 수 있으며, 출력은 이 채팅으로 표시됩니다. /desktop release로 인수 해제. 데스크톱 로컬 입력이 자동으로 권한을 반환합니다.", label), nil
}

func (h *botBridgeHub) Release(route bot.DesktopWatchRoute) (string, error) {
	h.mu.Lock()
	tabID, ok := h.takeoverTabs[route.Key()]
	if ok {
		delete(h.takeoverTabs, route.Key())
		delete(h.takeovers, tabID)
	}
	announce := h.announce
	changed := h.takeoverChanged
	h.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("이 채팅은 현재 어떤 데스크톱 세션도 인수하지 않음.")
	}
	if announce != nil {
		announce(tabID, "IM 원격 인수 해제됨。")
	}
	if changed != nil {
		changed()
	}
	return fmt.Sprintf("'%s' 의 인수를 해제했습니다.", h.tabLabel(tabID)), nil
}

func (h *botBridgeHub) TakeoverTab(route bot.DesktopWatchRoute) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.takeoverTabs[route.Key()]
}

func (h *botBridgeHub) DriveInput(route bot.DesktopWatchRoute, text string) (string, error) {
	h.mu.Lock()
	tabID := h.takeoverTabs[route.Key()]
	h.mu.Unlock()
	if tabID == "" {
		return "", fmt.Errorf("이 채팅은 어떤 데스크톱 세션도 인수하지 않음.")
	}
	session, ok := h.sessionByTabID(tabID)
	if !ok || session.Detached {
		h.mu.Lock()
		delete(h.takeoverTabs, route.Key())
		delete(h.takeovers, tabID)
		h.mu.Unlock()
		if changed := h.takeoverChanged; changed != nil {
			changed()
		}
		return "", fmt.Errorf("인수된 세션이 닫히거나 백그라운드로 전환됨, 인수가 자동 해제됨.")
	}
	if session.Running {
		return "", h.busyError(tabID)
	}
	if h.drive == nil {
		return "", fmt.Errorf("Desktop 구동 채널 사용 불가.")
	}
	if err := h.drive(tabID, text, route); err != nil {
		if errors.Is(err, errDriveBusy) {
			return "", h.busyError(tabID)
		}
		return "", fmt.Errorf("구동 실패: %w", err)
	}
	return "", nil
}

func (h *botBridgeHub) busyError(tabID string) error {
	return fmt.Errorf("세션 「%s」이(가) 실행 중입니다. 완료가 된 후 다시 시도해주세요. 또는 /desktop watch on으로 완료 알림을 구독하세요.", h.tabLabel(tabID))
}

func (h *botBridgeHub) reclaimFromDesktop(tabID string) {
	h.mu.Lock()
	route, ok := h.takeovers[tabID]
	if ok {
		delete(h.takeovers, tabID)
		delete(h.takeoverTabs, route.Key())
	}
	notify := h.notify
	changed := h.takeoverChanged
	h.mu.Unlock()
	if !ok {
		return
	}
	if changed != nil {
		changed()
	}
	if notify == nil {
		return
	}
	label := h.tabLabel(tabID)
	h.enqueue(desktopBridgeNotification{
		text:  constText(fmt.Sprintf("🔓 데스크톱에서 세션 「%s」의 권한을 회수했습니다，인수 해제됨。", label)),
		route: &route,
	})
}

func (h *botBridgeHub) remoteControlledTabs() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.takeovers) == 0 {
		return nil
	}
	out := make(map[string]bool, len(h.takeovers))
	for tabID := range h.takeovers {
		out[tabID] = true
	}
	return out
}
