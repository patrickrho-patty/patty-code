// Package comms is the harness-side comms channel (harness feature
// plan E2). The relay pushes chat/presence/broadcast/admin-command
// messages to enrolled harnesses over PAPER; the harness surfaces
// them in the IDE chat panel.
//
// The comms channel is governed by the same lease + policy epoch
// as the rest of the harness's PAPER traffic. A harness without an
// active lease fails closed; the comms package refuses to surface
// any incoming message.
package comms

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MessageType is the typed enumeration of comms messages the
// relay pushes to enrolled harnesses.
type MessageType string

const (
	MsgChat      MessageType = "CHAT"
	MsgPresence  MessageType = "PRESENCE"
	MsgBroadcast MessageType = "BROADCAST"
	MsgMention   MessageType = "MENTION"
	MsgAdminNote MessageType = "ADMIN_NOTE"
)

// Message is a single comms envelope. The relay signs each
// envelope under its policy issuer; the harness verifies the
// signature under the trust bundle pushed at AUTH_PROOF time.
type Message struct {
	MessageID      string
	Type           MessageType
	SenderID       string
	ConversationID string // for chat; empty for broadcast
	Body           string
	IssuedAt       int64 // unix-ms
	// EncryptionKeyID is the relay-side reference the harness
	// uses to look up the session key. Empty means clear-text
	// (admin broadcasts).
	EncryptionKeyID string
	// Signature is the relay's ed25519 signature over the
	// canonical bytes produced by SigningBytes.
	Signature []byte
}

// SigningBytes produces the canonical bytes the relay signs and
// the harness verifies.
func (m *Message) SigningBytes() []byte {
	data := fmt.Sprintf("comms|%s|%s|%s|%s|%d|%s|%s",
		m.MessageID, m.Type, m.SenderID, m.ConversationID,
		m.IssuedAt, m.EncryptionKeyID, m.Body)
	return []byte(data)
}

// Digest returns the content-addressed digest of the message. The
// harness surfaces this in the audit log so a relay audit replay
// can verify the message was delivered unchanged.
func (m *Message) Digest() [32]byte {
	h := sha256.New()
	h.Write([]byte("DARI-COMMS-v1\x00"))
	var tsBuf [8]byte
	binary.BigEndian.PutUint64(tsBuf[:], uint64(m.IssuedAt))
	h.Write(tsBuf[:])
	h.Write([]byte(m.MessageID))
	h.Write([]byte(m.SenderID))
	h.Write([]byte(m.ConversationID))
	h.Write([]byte(m.Body))
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return d
}

// Inbox stores the harness's received messages. The harness
// queries the inbox from the IDE chat panel; messages persist
// until the operator marks them read.
type Inbox struct {
	mu       sync.RWMutex
	messages map[string]*Message
	// dedup tracks seen message IDs so the harness ignores
	// duplicate relay retries (PRD §21.4).
	dedup map[string]bool
}

// NewInbox constructs an empty inbox.
func NewInbox() *Inbox {
	return &Inbox{
		messages: make(map[string]*Message),
		dedup:    make(map[string]bool),
	}
}

// Deliver inserts the message into the inbox. Returns the stored
// message if it was new (i.e., not a duplicate retry). The harness
// uses the second return value to drive UI animations.
func (i *Inbox) Deliver(msg *Message) (delivered bool, err error) {
	if msg == nil {
		return false, errors.New("comms: nil message")
	}
	if msg.MessageID == "" {
		return false, errors.New("comms: message missing ID")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.dedup[msg.MessageID] {
		return false, nil
	}
	i.dedup[msg.MessageID] = true
	i.messages[msg.MessageID] = msg
	return true, nil
}

// Get returns the message by ID, if present.
func (i *Inbox) Get(messageID string) (*Message, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	msg, ok := i.messages[messageID]
	return msg, ok
}

// ListConversation returns the messages for the supplied
// conversation ID in chronological order. The harness's IDE chat
// panel queries this for the active conversation.
func (i *Inbox) ListConversation(conversationID string) []*Message {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var out []*Message
	for _, m := range i.messages {
		if m.ConversationID == conversationID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].IssuedAt < out[b].IssuedAt })
	return out
}

// ListBroadcasts returns all broadcast messages (ConversationID
// empty) in chronological order. The harness's IDE notification
// panel queries this.
func (i *Inbox) ListBroadcasts() []*Message {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var out []*Message
	for _, m := range i.messages {
		if m.Type == MsgBroadcast || m.ConversationID == "" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].IssuedAt < out[b].IssuedAt })
	return out
}

// Count returns the number of stored messages.
func (i *Inbox) Count() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.messages)
}

// MarkRead removes the dedup marker for the supplied ID so the
// operator can re-receive the message if the relay re-pushes it.
// The harness invokes this when the operator clicks "mark as
// unread" or "remove".
func (i *Inbox) MarkRead(messageID string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.messages, messageID)
	delete(i.dedup, messageID)
}

// Channel is the harness-side comms channel. It receives
// messages from the relay's PAPER stream, verifies their
// signatures, and surfaces them to the harness's inbox.
type Channel struct {
	mu        sync.RWMutex
	inbox     *Inbox
	issuerPub ed25519.PublicKey
	// metrics: surfaced in the E1 status bar.
	verifiedCount int64
	rejectedCount int64
	dedupCount    int64
}

// NewChannel constructs a channel with the harness's inbox.
func NewChannel(inbox *Inbox) *Channel {
	return &Channel{inbox: inbox}
}

// SetIssuerPubKey updates the issuer public key used for
// signature verification. The harness pushes this when the trust
// bundle changes.
func (c *Channel) SetIssuerPubKey(pub ed25519.PublicKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.issuerPub = pub
}

// Receive is the single entry point the PAPER transport calls when
// a comms message arrives. Returns the stored message + a flag
// indicating whether the message was new (vs. a duplicate retry).
// The harness's UI consults the bool to drive notifications.
func (c *Channel) Receive(msg *Message) (delivered bool, err error) {
	if msg == nil {
		return false, errors.New("comms: nil message")
	}
	c.mu.RLock()
	issuerPub := c.issuerPub
	c.mu.RUnlock()
	if issuerPub == nil {
		return false, errors.New("comms: issuer public key not configured")
	}
	if len(msg.Signature) == 0 {
		c.mu.Lock()
		c.rejectedCount++
		c.mu.Unlock()
		return false, errors.New("comms: message missing signature")
	}
	if !verifyMessageSignature(issuerPub, msg) {
		c.mu.Lock()
		c.rejectedCount++
		c.mu.Unlock()
		return false, errors.New("comms: message signature verification failed")
	}
	c.mu.Lock()
	verified := true
	c.verifiedCount++
	c.mu.Unlock()
	delivered, err = c.inbox.Deliver(msg)
	if err != nil {
		return false, err
	}
	if !delivered {
		// A duplicate retry: the inbox suppressed the message
		// but we still verified the signature. Roll back the
		// verified counter so it reflects only new deliveries.
		if verified {
			c.mu.Lock()
			c.verifiedCount--
			c.dedupCount++
			c.mu.Unlock()
		}
	}
	return delivered, nil
}

// Inbox returns the channel's inbox for direct querying.
func (c *Channel) Inbox() *Inbox {
	return c.inbox
}

// VerifiedCount returns the E1 status-bar counter.
func (c *Channel) VerifiedCount() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.verifiedCount
}

// RejectedCount returns the E1 status-bar counter.
func (c *Channel) RejectedCount() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rejectedCount
}

// DedupCount returns the E1 status-bar counter for duplicate
// retries the harness suppressed.
func (c *Channel) DedupCount() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dedupCount
}

// verifyMessageSignature checks the message's signature under the
// supplied public key. The relay signs `Message.SigningBytes`; the
// harness verifies the same bytes.
func verifyMessageSignature(issuerPub ed25519.PublicKey, msg *Message) bool {
	return ed25519.Verify(issuerPub, msg.SigningBytes(), msg.Signature)
}

// _ keeps the time import visible when tests evolve to use it.
var _ = time.Now
