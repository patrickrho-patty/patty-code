package dari

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"patty/internal/provider"
)

// Live one-turn walk against the RUNNING local stack (relay :8444, mock PIA).
// Run: go test ./internal/provider/dari/ -run TestLiveLocalStackTurn -v
func TestLiveLocalStackTurn(t *testing.T) {
	if os.Getenv("LIVE_STACK") != "1" {
		t.Skip("set LIVE_STACK=1 against the local dev stack")
	}
	os.Setenv("DARI_HARNESS_CREDENTIAL_FILE", "/tmp/pccp-live/ppc.cbor")
	os.Setenv("DARI_HARNESS_KEY_FILE", "/tmp/pccp-live/harness.key")
	os.Setenv("DARI_HARNESS_ID", "harness-patrick-3")

	prov, err := New(provider.Config{
		Name: "live-walk", BaseURL: "127.0.0.1:8444",
		Model: os.Getenv("LIVE_MODEL"), APIKey: "unused",
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	p := prov.(*Provider)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ch, err := p.Stream(ctx, provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "Reply with the single word: pong"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var text string
	var usage *provider.Usage
	for c := range ch {
		switch c.Type {
		case provider.ChunkText:
			text += c.Text
			fmt.Printf("TOKEN %q\n", c.Text)
		case provider.ChunkUsage:
			usage = c.Usage
		case provider.ChunkError:
			t.Fatalf("stream error: %v", c.Err)
		}
	}
	t.Logf("GOVERNED COMPLETION: %q usage=%+v", text, usage)

	// Receipt landed on disk (B3 durable store under the config home).
	if home, err := os.UserConfigDir(); err == nil {
		dir := home + "/patty/receipts"
		if entries, derr := os.ReadDir(dir); derr == nil && len(entries) > 0 {
			t.Logf("RECEIPTS ON DISK: %d", len(entries))
		}
	}
	time.Sleep(1500 * time.Millisecond) // let spine/audit settle
}
