package bot

import (
	"context"
	"fmt"
	"strings"

	"patty/internal/event"
)

type DesktopSessionInfo struct {
	TabID         string
	Label         string
	Workspace     string
	Topic         string
	Ready         bool
	Running       bool
	PendingPrompt bool
	Detached      bool
	Pending       []DesktopPendingInfo
}

type DesktopPendingInfo struct {
	ID   string
	Kind string // "approval" | "ask"
	Tool string
}

type DesktopWatchRoute struct {
	ConnectionID string
	Domain       string
	Platform     Platform
	ChatType     ChatType
	ChatID       string
}

func (r DesktopWatchRoute) Key() string {
	return fmt.Sprintf("%s|%s|%s|%s", r.Platform, r.ConnectionID, r.Domain, r.ChatID)
}

type DesktopBridge interface {
	Sessions() []DesktopSessionInfo
	SetWatch(route DesktopWatchRoute, enable bool) error
	Watching(route DesktopWatchRoute) bool
	Approve(approvalID string, allow bool) (string, error)
	AskQuestions(askID string) ([]event.AskQuestion, bool)
	Answer(askID string, answers []event.AskAnswer) (string, error)
	Takeover(route DesktopWatchRoute, tabID string) (string, error)
	Release(route DesktopWatchRoute) (string, error)
	TakeoverTab(route DesktopWatchRoute) string
	DriveInput(route DesktopWatchRoute, text string) (string, error)
}

func desktopRouteFromMessage(msg InboundMessage) DesktopWatchRoute {
	return DesktopWatchRoute{
		ConnectionID: msg.ConnectionID,
		Domain:       msg.Domain,
		Platform:     msg.Platform,
		ChatType:     msg.ChatType,
		ChatID:       msg.ChatID,
	}
}

const desktopCommandUsage = "사용법:\n" +
	"/desktop status - 모든 데스크톱 live 세션 보기\n" +
	"/desktop watch on|off|status - 데스크톱 이벤트 구독/해지(승인 요청, 작업 완료/오류)\n" +
	"/desktop approve <id> - 대기 중인 데스크톱 작업 승인\n" +
	"/desktop deny <id> - 대기 중인 데스크톱 작업 거절\n" +
	"/desktop answer <id> <옵션 번호 또는 텍스트> - 데스크톱 세션 질문 답변하기\n" +
	"/desktop takeover <tab> - 데스크톱 세션 인수, 후속 메시지 직접 운전\n" +
	"/desktop release - 인수 해제, 일반 bot 세션으로 복귀"

func (gw *BotGateway) handleDesktopCommand(msg InboundMessage) string {
	bridge := gw.cfg.Desktop
	if bridge == nil {
		return "이 bot 은 데스크톱 프로세스에서 실행되지 않음, /desktop 명령 사용 불가. 데스크톱 설정에서 bot 활성화."
	}
	fields := strings.Fields(msg.Text)
	sub := ""
	if len(fields) > 1 {
		sub = strings.ToLower(fields[1])
	}
	switch sub {
	case "", "status", "sessions":
		return formatDesktopSessions(bridge.Sessions())
	case "watch":
		arg := ""
		if len(fields) > 2 {
			arg = strings.ToLower(fields[2])
		}
		route := desktopRouteFromMessage(msg)
		switch arg {
		case "on":
			if err := bridge.SetWatch(route, true); err != nil {
				return "이번 실행 중 데스크톱 이벤트를 구독했으나 저장 실패; 데스크톱 재시작 시 다시 구독 필요."
			}
			return "데스크톱 이벤트를 구독했습니다：승인 요청, 작업 완료/오류가 이 채팅으로 전송됩니다。구독이 저장되어 데스크톱을 다시 시작해도 유지됩니다。/desktop watch off로 해지하세요。"
		case "off":
			if err := bridge.SetWatch(route, false); err != nil {
				return "이번 실행 중 데스크톱 구독 해지했으나 저장 실패; 재시작 시 구독 복구 가능."
			}
			return "데스크톱 이벤트 구독 해지됨, 저장된 구독도 제거됨."
		case "", "state":
			if bridge.Watching(route) {
				return "이 채팅은 데스크톱 이벤트 구독 중. /desktop watch off 로 해지."
			}
			return "이 채팅은 데스크톱 이벤트 미구독. /desktop watch on 으로 구독."
		default:
			return desktopCommandUsage
		}
	case "approve", "deny":
		if len(fields) < 3 {
			return desktopCommandUsage
		}
		feedback, err := bridge.Approve(fields[2], sub == "approve")
		if err != nil {
			return err.Error()
		}
		return feedback
	case "answer":
		if len(fields) < 4 {
			return desktopCommandUsage
		}
		askID := fields[2]
		questions, ok := bridge.AskQuestions(askID)
		if !ok {
			return fmt.Sprintf("답변 대기 중인 질문 %s 없음 (이미 데스크톱에서 답변하거나 만료되었음).", askID)
		}
		raw := strings.Join(fields[3:], " ")
		answers := parseAskAnswers(questions, raw)
		feedback, err := bridge.Answer(askID, answers)
		if err != nil {
			return err.Error()
		}
		return feedback
	case "takeover":
		if len(fields) < 3 {
			return desktopCommandUsage
		}
		feedback, err := bridge.Takeover(desktopRouteFromMessage(msg), fields[2])
		if err != nil {
			return err.Error()
		}
		return feedback
	case "release":
		feedback, err := bridge.Release(desktopRouteFromMessage(msg))
		if err != nil {
			return err.Error()
		}
		return feedback
	default:
		return desktopCommandUsage
	}
}

func (gw *BotGateway) divertToDesktopTakeover(ctx context.Context, adapter Adapter, msg InboundMessage) bool {
	if strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
		return false
	}
	bridge := gw.cfg.Desktop
	if bridge == nil {
		return false
	}
	route := desktopRouteFromMessage(msg)
	if bridge.TakeoverTab(route) == "" {
		return false
	}
	if !gw.checkCommandRole(msg.Platform, msg, "admin") {
		_, _ = bridge.Release(route)
		_ = gw.sendText(ctx, adapter, msg, "데스크톱 인수 해제됨: 현재 계정 bot 관리자 권한 없음.")
		return true
	}
	if strings.TrimSpace(msg.Text) == "" {
		_ = gw.sendText(ctx, adapter, msg, "인수 모드에서는 임시로 파일 보내기 지원 안됨, 텍스트만 전송.")
		return true
	}
	feedback, err := bridge.DriveInput(route, msg.Text)
	if err != nil {
		_ = gw.sendText(ctx, adapter, msg, err.Error())
		return true
	}
	if strings.TrimSpace(feedback) != "" {
		_ = gw.sendText(ctx, adapter, msg, feedback)
	}
	return true
}

func formatDesktopSessions(sessions []DesktopSessionInfo) string {
	if len(sessions) == 0 {
		return "현재 데스크톱에 live 세션 없음."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "데스크톱 live 세션 (%d 개):\n", len(sessions))
	for _, s := range sessions {
		state := "대기중"
		switch {
		case s.PendingPrompt:
			state = "⚠️ 승인/답변 대기"
		case s.Running:
			state = "▶️ 실행 중"
		case !s.Ready:
			state = "시작 중"
		}
		label := strings.TrimSpace(s.Label)
		if label == "" {
			label = strings.TrimSpace(s.Topic)
		}
		if label == "" {
			label = "(이름 없음)"
		}
		if s.Detached {
			state += "·백그라운드"
		}
		fmt.Fprintf(&b, "\n- %s [%s]", label, state)
		if ws := strings.TrimSpace(s.Workspace); ws != "" {
			fmt.Fprintf(&b, "\n  프로젝트: %s", ws)
		}
		fmt.Fprintf(&b, "\n  tab: %s", s.TabID)
		for _, p := range s.Pending {
			kind := "승인"
			if p.Kind == "ask" {
				kind = "질문"
			}
			line := fmt.Sprintf("\n  대기 중 %s: %s", kind, p.ID)
			if tool := strings.TrimSpace(p.Tool); tool != "" {
				line += " (" + tool + ")"
			}
			b.WriteString(line)
		}
	}
	b.WriteString("\n\n/desktop watch on으로 승인과 완료 이벤트를 구독하세요；/desktop takeover <tab>으로 세션을 인수하세요。")
	return b.String()
}
