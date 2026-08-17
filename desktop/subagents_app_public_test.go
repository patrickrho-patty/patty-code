//go:build profile_public

// Public-surface desktop tests (ADR G4): every test here needs generic
// OpenAI/Anthropic-kind provider fixtures (BYOK flows, official-provider
// surfaces, model/effort/token-mode rebuilds against generic endpoints) or
// another public-only capability (e.g. balance fetch). The enterprise and
// sovereign tier locks refuse those configs at boot by design, so these
// assertions compile and run only in the public-profile leg (see Makefile
// test-profiles).
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/config"
)

// A try run must be cancellable (it is otherwise an unstoppable 12-step
// provider loop) and single-flight: a second concurrent try is refused
// instead of racing the first one's cancel handle.
func TestTrySubagentProfileCancelAbortsRunAndIsSingleFlight(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "PATTY_TEST_KEY", "sk-test")

	requestStarted := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-requestStarted:
		default:
			close(requestStarted)
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAll()

	cfg := config.Default()
	cfg.DefaultModel = "prov-t/model-t1"
	cfg.Providers = []config.ProviderEntry{
		{Name: "prov-t", Kind: "openai", BaseURL: srv.URL, Model: "model-t1", APIKeyEnv: "PATTY_TEST_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	a := NewApp()
	done := make(chan error, 1)
	go func() {
		_, err := a.TrySubagentProfile(SubagentProfileInput{SystemPrompt: "be helpful"}, "do something")
		done <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("try run never reached the provider")
	}
	if _, err := a.TrySubagentProfile(SubagentProfileInput{SystemPrompt: "p"}, "task"); err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("concurrent try error = %v, want the single-flight refusal", err)
	}

	a.CancelTrySubagentProfile()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled try run should return an error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled try run did not return")
	}

	// The slot frees up after the run settles: a fresh cancel is a no-op and
	// a new try is admitted. Release the handler first so this run fails
	// fast on the invalid empty response instead of blocking on the hang.
	releaseAll()
	a.CancelTrySubagentProfile()
	if _, err := a.TrySubagentProfile(SubagentProfileInput{SystemPrompt: "p"}, "task"); err != nil && strings.Contains(err.Error(), "in progress") {
		t.Fatalf("slot did not free after cancel: %v", err)
	}
}
