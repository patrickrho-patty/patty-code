// [Package bot  Patty Code  IM bot ， QQ、、WeChat。]
// [Hermes Agent  gateway/adapter/session 。]
package bot

import (
	"context"
	"slices"
	"strings"
)

// [Platform  IM 。]
type Platform string

// [ChatType]
type ChatType string

const (
	ChatDM     ChatType = "dm"
	ChatGroup  ChatType = "group"
	ChatGuild  ChatType = "guild"
	ChatDirect ChatType = "direct"
	ChatThread ChatType = "thread"
)

// [SessionSource ， session key。]
type SessionSource struct {
	Platform     Platform `json:"platform"`
	ConnectionID string   `json:"connection_id,omitempty"`
	Domain       string   `json:"domain,omitempty"`
	ChatType     ChatType `json:"chat_type"`
	ChatID       string   `json:"chat_id"`
	UserID       string   `json:"user_id"`
	ThreadID     string   `json:"thread_id,omitempty"`
}

type InboundMedia struct {
	Name        string                                        `json:"name,omitempty"`
	MIME        string                                        `json:"mime,omitempty"`
	Data        []byte                                        `json:"-"`
	Load        func(context.Context) ([]byte, string, error) `json:"-"`
	FailureText string                                        `json:"-"`
}

// [InboundMessage]
type InboundMessage struct {
	Platform     Platform `json:"platform"`
	ConnectionID string   `json:"connection_id,omitempty"`
	Domain       string   `json:"domain,omitempty"`
	ChatType     ChatType `json:"chat_type"`
	ChatID       string   `json:"chat_id"`
	UserID       string   `json:"user_id"`
	UserName     string   `json:"user_name"`
	OperatorID string         `json:"operator_id,omitempty"`
	Text       string         `json:"text"`
	MessageID  string         `json:"message_id"`
	ThreadID   string         `json:"thread_id,omitempty"`
	MediaURLs  []string       `json:"media_urls,omitempty"`
	Media      []InboundMedia `json:"-"`
	ResolveUserName func(context.Context) string `json:"-"`
	Raw             any                          `json:"-"`
}

func (m InboundMessage) Session() SessionSource {
	return SessionSource{
		Platform:     m.Platform,
		ConnectionID: m.ConnectionID,
		Domain:       m.Domain,
		ChatType:     m.ChatType,
		ChatID:       m.ChatID,
		UserID:       m.UserID,
		ThreadID:     m.ThreadID,
	}
}

// [OutboundMessage]
type OutboundMessage struct {
	ConnectionID string           `json:"connection_id,omitempty"`
	Domain       string           `json:"domain,omitempty"`
	ChatID       string           `json:"chat_id"`
	ChatType     ChatType         `json:"chat_type,omitempty"`
	Text         string           `json:"text,omitempty"`
	MediaURLs    []string         `json:"media_urls,omitempty"`
	ReplyToMsgID string           `json:"reply_to_msg_id,omitempty"`
	Keyboard     *InlineKeyboard  `json:"keyboard,omitempty"`
	Card         *InteractiveCard `json:"card,omitempty"`
}

// [InlineKeyboard （ QQ ）。]
type InlineKeyboard struct {
	Rows []InlineKeyboardRow `json:"rows"`
}

// [InlineKeyboardRow]
type InlineKeyboardRow struct {
	Buttons []InlineKeyboardButton `json:"buttons"`
}

// [InlineKeyboardButton]
type InlineKeyboardButton struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Style      int    `json:"style,omitempty"` // 0=default, 1=primary, 2=danger
	CallbackID string `json:"callback_id,omitempty"`
}

// [InteractiveCard]
type InteractiveCard struct {
	Header   string                   `json:"header"`
	Elements []InteractiveCardElement `json:"elements"`
}

// [InteractiveCardElement]
type InteractiveCardElement struct {
	Tag     string         `json:"tag"`
	Content string         `json:"content,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// [SendResult]
type SendResult struct {
	MessageID  string   `json:"message_id,omitempty"`
	MessageIDs []string `json:"message_ids,omitempty"`
	Err        error    `json:"err,omitempty"`
}

func (r SendResult) DeliveredMessageIDs() []string {
	ids := make([]string, 0, len(r.MessageIDs)+1)
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if slices.Contains(ids, id) {
			return
		}
		ids = append(ids, id)
	}
	for _, id := range r.MessageIDs {
		add(id)
	}
	add(r.MessageID)
	return ids
}

func (r *SendResult) Merge(delivered SendResult) {
	for _, id := range delivered.DeliveredMessageIDs() {
		duplicate := slices.Contains(r.MessageIDs, id)
		if !duplicate {
			r.MessageIDs = append(r.MessageIDs, id)
		}
		r.MessageID = id
	}
}

// [Adapter]
type Adapter interface {
// [Platform]
	Platform() Platform

// [Start 어댑터， gateway。]
	Start(ctx context.Context) error

// [Stop]
	Stop() error

// [Send]
	Send(ctx context.Context, msg OutboundMessage) (SendResult, error)

// [SendTyping]
	SendTyping(ctx context.Context, chatID string) error

// [Messages]
	Messages() <-chan InboundMessage

// [Name]
	Name() string
}

// [MessageHandler  BotGateway 처리。]
type MessageHandler func(ctx context.Context, msg InboundMessage)
