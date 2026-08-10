package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"patty/internal/bot"
	"patty/internal/config"
	"patty/internal/control"
	"patty/internal/event"
)


func (a *App) newBotBridge() *botBridgeHub {
	return newBotBridgeHub(botBridgeDeps{
		sessions:        a.bridgeSessions,
		approveTab:      a.bridgeApprove,
		answerTab:       a.bridgeAnswer,
		notify:          a.botRuntime.SendToAdapter,
		drive:           a.bridgeDrive,
		announce:        a.bridgeAnnounce,
		persistWatchers: a.bridgePersistWatchers,
		takeoverChanged: a.emitProjectTreeChanged,
		logger:          slog.Default(),
	})
}

func (a *App) bridgeSessions() []bot.DesktopSessionInfo {
	tabs := a.ListTabs()
	out := make([]bot.DesktopSessionInfo, 0, len(tabs)+4)
	seen := make(map[string]bool, len(tabs))
	for _, t := range tabs {
		seen[t.ID] = true
		out = append(out, bot.DesktopSessionInfo{
			TabID:         t.ID,
			Label:         t.Label,
			Workspace:     t.WorkspaceName,
			Topic:         t.TopicTitle,
			Ready:         t.Ready,
			Running:       t.Running,
			PendingPrompt: t.PendingPrompt,
		})
	}
	a.mu.RLock()
	for _, tab := range a.detachedSessions {
		if tab == nil || seen[tab.ID] {
			continue
		}
		seen[tab.ID] = true
		out = append(out, bot.DesktopSessionInfo{
			TabID:    tab.ID,
			Label:    tab.TopicTitle,
			Topic:    tab.TopicTitle,
			Ready:    tab.Ctrl != nil,
			Running:  strings.TrimSpace(tab.ActivityStatus) != "",
			Detached: true,
		})
	}
	a.mu.RUnlock()
	return out
}

func (a *App) bridgeCtrlByTabID(tabID string) control.SessionAPI {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if tab := a.tabByEventSinkIDLocked(tabID); tab != nil {
		return tab.Ctrl
	}
	return nil
}

func (a *App) bridgeApprove(tabID, id string, allow, session, persist bool) {
	if ctrl := a.bridgeCtrlByTabID(tabID); ctrl != nil {
		ctrl.Approve(id, allow, session, persist)
	}
}

func (a *App) bridgeAnswer(tabID, id string, answers []QuestionAnswer) {
	ctrl := a.bridgeCtrlByTabID(tabID)
	if ctrl == nil {
		return
	}
	out := make([]event.AskAnswer, len(answers))
	for i, an := range answers {
		out[i] = event.AskAnswer{QuestionID: an.QuestionID, Selected: an.Selected}
	}
	ctrl.AnswerQuestion(id, out)
}

func (a *App) bridgeAnnounce(tabID, text string) {
	a.mu.RLock()
	tab := a.tabByEventSinkIDLocked(tabID)
	var sink *tabEventSink
	if tab != nil {
		sink = tab.sink
	}
	a.mu.RUnlock()
	if sink == nil {
		return
	}
	sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: text})
}

func (a *App) bridgeDrive(tabID, text string, route bot.DesktopWatchRoute) error {
	admission, ctrl, err := a.beginTabTurn(tabID, false)
	if err != nil {
		if errors.Is(err, control.ErrTurnRunning) {
			return errDriveBusy
		}
		return err
	}
	defer admission.abort()
	tab := admission.tab
	if tab.sink == nil {
		return fmt.Errorf("세션 이벤트 채널을 사용할 수 없습니다.")
	}
	if a.botBridge == nil || a.botBridge.TakeoverTab(route) != tabID {
		return fmt.Errorf("인수 해제됨, 세션을 다시 인수하세요.")
	}
	target := botForwardTarget{
		ConnID:   route.ConnectionID,
		Domain:   route.Domain,
		ChatID:   route.ChatID,
		ChatType: route.ChatType,
	}
	generation := tab.sink.SetBotSink(newBotEventForwarder(a.botRuntime, []botForwardTarget{target}))
	a.ensureTabTopicIndexedForUserTurn(tab)
	ctrl.SubmitDisplay(text, text)
	if !admission.finish(ctrl) {
		tab.sink.clearBotSink(generation)
		return errDriveBusy
	}
	return nil
}

func (a *App) bridgePersistWatchers(routes []bot.DesktopWatchRoute) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		watchers := make([]config.BotDesktopWatcherConfig, 0, len(routes))
		for _, r := range routes {
			watchers = append(watchers, config.BotDesktopWatcherConfig{
				Platform:     string(r.Platform),
				ConnectionID: r.ConnectionID,
				Domain:       r.Domain,
				ChatType:     string(r.ChatType),
				ChatID:       r.ChatID,
			})
		}
		c.Bot.DesktopWatchers = watchers
		return nil
	})
}

func bridgeRoutesFromConfig(watchers []config.BotDesktopWatcherConfig) []bot.DesktopWatchRoute {
	routes := make([]bot.DesktopWatchRoute, 0, len(watchers))
	for _, w := range watchers {
		routes = append(routes, bot.DesktopWatchRoute{
			Platform:     bot.Platform(strings.TrimSpace(w.Platform)),
			ConnectionID: strings.TrimSpace(w.ConnectionID),
			Domain:       strings.TrimSpace(w.Domain),
			ChatType:     bot.ChatType(strings.TrimSpace(w.ChatType)),
			ChatID:       strings.TrimSpace(w.ChatID),
		})
	}
	return routes
}
