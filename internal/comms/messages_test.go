package comms

import (
	"crypto/ed25519"
	"strings"
	"sync"
	"testing"
)

// newIssuerPair generates an ed25519 issuer key for tests.
func newIssuerPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	return pub, priv
}

func sampleChat(id, sender string) *Message {
	return &Message{
		MessageID:      id,
		Type:           MsgChat,
		SenderID:       sender,
		ConversationID: "conv-1",
		Body:           "hello",
		IssuedAt:       1_700_000_000_000,
	}
}

func sampleBroadcast(id string) *Message {
	return &Message{
		MessageID: id,
		Type:      MsgBroadcast,
		SenderID:  "ops-team",
		Body:      "scheduled maintenance",
		IssuedAt:  1_700_000_000_000,
	}
}

func signMessage(t *testing.T, msg *Message, priv ed25519.PrivateKey) {
	t.Helper()
	msg.Signature = ed25519.Sign(priv, msg.SigningBytes())
}

// TestInboxDeliverAndDedup covers the E2 green path: a new
// message is stored, a duplicate retry is suppressed.
func TestInboxDeliverAndDedup(t *testing.T) {
	inbox := NewInbox()
	if delivered, err := inbox.Deliver(sampleChat("m-1", "alice")); err != nil || !delivered {
		t.Fatalf("first deliver: delivered=%v err=%v", delivered, err)
	}
	if delivered, err := inbox.Deliver(sampleChat("m-1", "alice")); err != nil || delivered {
		t.Fatalf("dup deliver: delivered=%v err=%v", delivered, err)
	}
	if inbox.Count() != 1 {
		t.Errorf("count = %d, want 1", inbox.Count())
	}
}

// TestInboxListConversationOrdersByIssuedAt covers the IDE chat
// panel's expected sort order.
func TestInboxListConversationOrdersByIssuedAt(t *testing.T) {
	inbox := NewInbox()
	for _, msg := range []*Message{
		{MessageID: "m-1", Type: MsgChat, SenderID: "alice", ConversationID: "c1", Body: "first", IssuedAt: 300},
		{MessageID: "m-2", Type: MsgChat, SenderID: "alice", ConversationID: "c1", Body: "second", IssuedAt: 100},
		{MessageID: "m-3", Type: MsgChat, SenderID: "alice", ConversationID: "c1", Body: "third", IssuedAt: 200},
	} {
		if _, err := inbox.Deliver(msg); err != nil {
			t.Fatalf("deliver: %v", err)
		}
	}
	got := inbox.ListConversation("c1")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].MessageID != "m-2" || got[1].MessageID != "m-3" || got[2].MessageID != "m-1" {
		t.Errorf("sort order wrong: %v %v %v", got[0].MessageID, got[1].MessageID, got[2].MessageID)
	}
}

// TestInboxListBroadcasts covers the IDE notification panel:
// broadcasts are surfaced separately from chat.
func TestInboxListBroadcasts(t *testing.T) {
	inbox := NewInbox()
	_, _ = inbox.Deliver(sampleChat("m-1", "alice"))
	_, _ = inbox.Deliver(sampleBroadcast("b-1"))
	got := inbox.ListBroadcasts()
	if len(got) != 1 || got[0].MessageID != "b-1" {
		t.Errorf("broadcasts = %v, want one (b-1)", got)
	}
}

// TestInboxMarkReadRemoves covers the operator-driven read state.
func TestInboxMarkReadRemoves(t *testing.T) {
	inbox := NewInbox()
	_, _ = inbox.Deliver(sampleChat("m-1", "alice"))
	inbox.MarkRead("m-1")
	if inbox.Count() != 0 {
		t.Errorf("count after MarkRead = %d, want 0", inbox.Count())
	}
	if _, ok := inbox.Get("m-1"); ok {
		t.Errorf("message must be removed after MarkRead")
	}
}

// TestInboxRejectsNil covers the trivial boundary.
func TestInboxRejectsNil(t *testing.T) {
	inbox := NewInbox()
	if _, err := inbox.Deliver(nil); err == nil {
		t.Fatal("nil message must fail")
	}
	if _, err := inbox.Deliver(&Message{}); err == nil {
		t.Fatal("empty message ID must fail")
	}
}

// TestChannelReceivesSignedMessage covers the E2 green path:
// a properly-signed message lands in the inbox.
func TestChannelReceivesSignedMessage(t *testing.T) {
	inbox := NewInbox()
	ch := NewChannel(inbox)
	issuerPub, issuerPriv := newIssuerPair(t)
	ch.SetIssuerPubKey(issuerPub)
	msg := sampleChat("m-1", "alice")
	signMessage(t, msg, issuerPriv)
	delivered, err := ch.Receive(msg)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if !delivered {
		t.Error("first receive must deliver")
	}
	if ch.VerifiedCount() != 1 {
		t.Errorf("verified count = %d, want 1", ch.VerifiedCount())
	}
	if inbox.Count() != 1 {
		t.Errorf("inbox count = %d, want 1", inbox.Count())
	}
}

// TestChannelRejectsUnsignedMessage covers the trust boundary:
// a message without a signature is rejected before reaching
// the inbox.
func TestChannelRejectsUnsignedMessage(t *testing.T) {
	inbox := NewInbox()
	ch := NewChannel(inbox)
	issuerPub, _ := newIssuerPair(t)
	ch.SetIssuerPubKey(issuerPub)
	_, err := ch.Receive(sampleChat("m-1", "alice"))
	if err == nil {
		t.Fatal("unsigned message must fail")
	}
	if ch.RejectedCount() != 1 {
		t.Errorf("rejected count = %d, want 1", ch.RejectedCount())
	}
	if inbox.Count() != 0 {
		t.Errorf("unsigned message must not reach inbox")
	}
}

// TestChannelRejectsTamperedMessage covers the trust-boundary
// drift: a message with a signature from a different issuer is
// rejected.
func TestChannelRejectsTamperedMessage(t *testing.T) {
	inbox := NewInbox()
	ch := NewChannel(inbox)
	issuerPub, _ := newIssuerPair(t)
	ch.SetIssuerPubKey(issuerPub)
	_, otherPriv := newIssuerPair(t)
	msg := sampleChat("m-1", "alice")
	signMessage(t, msg, otherPriv) // wrong issuer
	_, err := ch.Receive(msg)
	if err == nil {
		t.Fatal("rogue-issuer message must fail")
	}
}

// TestChannelDedupCountsCover the dedup-counter metric: a
// duplicate retry increments DedupCount, not VerifiedCount.
func TestChannelDedupCountsCover(t *testing.T) {
	inbox := NewInbox()
	ch := NewChannel(inbox)
	issuerPub, issuerPriv := newIssuerPair(t)
	ch.SetIssuerPubKey(issuerPub)
	msg := sampleChat("m-1", "alice")
	signMessage(t, msg, issuerPriv)
	if _, err := ch.Receive(msg); err != nil {
		t.Fatalf("first receive: %v", err)
	}
	if _, err := ch.Receive(msg); err != nil {
		t.Fatalf("dup receive: %v", err)
	}
	if ch.DedupCount() != 1 {
		t.Errorf("dedup count = %d, want 1", ch.DedupCount())
	}
	if ch.VerifiedCount() != 1 {
		t.Errorf("verified count = %d, want 1 (dup must not re-verify)", ch.VerifiedCount())
	}
}

// TestChannelRequiresIssuerKey covers the trivial boundary.
func TestChannelRequiresIssuerKey(t *testing.T) {
	ch := NewChannel(NewInbox())
	_, err := ch.Receive(sampleChat("m-1", "alice"))
	if err == nil {
		t.Fatal("receive without issuer key must fail")
	}
}

// TestMessageSigningBytesBoundAllFields pins the binding
// invariant.
func TestMessageSigningBytesBoundAllFields(t *testing.T) {
	primary := sampleChat("m-1", "alice").SigningBytes()
	mutations := []struct {
		name string
		fn   func(*Message)
	}{
		{"id", func(m *Message) { m.MessageID = "m-2" }},
		{"type", func(m *Message) { m.Type = MsgPresence }},
		{"sender", func(m *Message) { m.SenderID = "bob" }},
		{"conv", func(m *Message) { m.ConversationID = "c2" }},
		{"body", func(m *Message) { m.Body = "different" }},
		{"issued_at", func(m *Message) { m.IssuedAt = 1 }},
	}
	for _, m := range mutations {
		clone := sampleChat("m-1", "alice")
		m.fn(clone)
		if string(clone.SigningBytes()) == string(primary) {
			t.Errorf("signing bytes unchanged after %s mutation", m.name)
		}
	}
}

// TestChannelConcurrentReceive covers the lock boundary.
func TestChannelConcurrentReceive(t *testing.T) {
	inbox := NewInbox()
	ch := NewChannel(inbox)
	issuerPub, issuerPriv := newIssuerPair(t)
	ch.SetIssuerPubKey(issuerPub)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := sampleChat("m-"+string(rune('a'+i%26))+"-"+string(rune('0'+i%10)), "alice")
			signMessage(t, msg, issuerPriv)
			_, _ = ch.Receive(msg)
		}(i)
	}
	wg.Wait()
	if inbox.Count() == 0 {
		t.Error("inbox must have received messages")
	}
}

// _ keeps the strings import visible when tests evolve to use it.
var _ = strings.Contains
