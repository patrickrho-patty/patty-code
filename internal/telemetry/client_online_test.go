//go:build !profile_sovereign

// Online egress assertions (ADR G2): these exercise the real
// endpoint-roundtripping post path, which compiles only into
// public/enterprise/default builds. The sovereign no-op twin is covered by
// sovereign_test.go.

package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"patty/internal/testenv"
)

type roundTripFunc = testenv.RoundTripFunc

func telemetryResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

func testClient(home string, transport http.RoundTripper) *Client {
	return &Client{
		home:      home,
		version:   "v1.20.0",
		installID: strings.Repeat("a", 32),
		http:      &http.Client{Transport: transport},
	}
}

func TestDailyPingSendsOnceWithCLISurface(t *testing.T) {
	home := t.TempDir()
	var mu sync.Mutex
	var payloads []pingPayload
	client := testClient(home, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != endpoint+"/ping" {
			t.Fatalf("request URL = %q", req.URL)
		}
		var payload pingPayload
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		payloads = append(payloads, payload)
		mu.Unlock()
		return telemetryResponse(http.StatusAccepted), nil
	}))

	if err := client.sendDailyPing(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.sendDailyPing(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 1 {
		t.Fatalf("ping requests = %d, want 1", len(payloads))
	}
	if payloads[0].Surface != "cli" || payloads[0].InstallID != client.installID {
		t.Fatalf("ping payload = %+v", payloads[0])
	}
}

func TestFailedDailyPingRemovesClaimAndRetries(t *testing.T) {
	home := t.TempDir()
	calls := 0
	client := testClient(home, roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("offline")
		}
		return telemetryResponse(http.StatusAccepted), nil
	}))

	if err := client.sendDailyPing(context.Background()); err == nil {
		t.Fatal("first ping unexpectedly succeeded")
	}
	claim := filepath.Join(home, "cli-telemetry-ping-"+time.Now().UTC().Format("2006-01-02"))
	if _, err := os.Stat(claim); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed ping claim remains: %v", err)
	}
	if err := client.sendDailyPing(context.Background()); err != nil {
		t.Fatalf("retry ping: %v", err)
	}
	if calls != 2 {
		t.Fatalf("ping calls = %d, want 2", calls)
	}
}

func TestFlushPendingAggregatesAndDeletesOnlyAfterSuccess(t *testing.T) {
	home := t.TempDir()
	for _, counters := range [][]Counter{
		{{Signal: "turns", Bucket: "count", Count: 2}},
		{{Signal: "turns", Bucket: "count", Count: 3}, {Signal: "cli_exit", Bucket: "success", Count: 1}},
	} {
		if err := appendPending(home, pendingPayload{Version: "v1.20.0", OS: "android", Counters: counters}); err != nil {
			t.Fatal(err)
		}
	}
	requests := 0
	var client *Client
	client = testClient(home, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		var payload metricsPayload
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Surface != "cli" || payload.OS != "android" || payload.InstallID != client.installID {
			t.Fatalf("metrics payload = %+v", payload)
		}
		got := map[string]int{}
		for _, counter := range payload.Counters {
			got[counter.Signal+"/"+counter.Bucket] = counter.Count
		}
		if got["turns/count"] != 5 || got["cli_exit/success"] != 1 {
			t.Fatalf("aggregated counters = %#v", got)
		}
		return telemetryResponse(http.StatusAccepted), nil
	}))

	if err := client.flushPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("metrics requests = %d, want 1", requests)
	}
	entries, err := os.ReadDir(filepath.Join(home, pendingDirName))
	if err != nil || len(entries) != 0 {
		t.Fatalf("pending entries after success = %d, err = %v", len(entries), err)
	}
}

func TestFailedFlushRestoresClaimsForRetry(t *testing.T) {
	home := t.TempDir()
	if err := appendPending(home, pendingPayload{
		Version: "v1.20.0", OS: "linux", Counters: []Counter{{Signal: "turns", Bucket: "count", Count: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := testClient(home, roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return telemetryResponse(http.StatusServiceUnavailable), nil
		}
		return telemetryResponse(http.StatusAccepted), nil
	}))

	if err := client.flushPending(context.Background()); err == nil {
		t.Fatal("first flush unexpectedly succeeded")
	}
	entries, err := os.ReadDir(filepath.Join(home, pendingDirName))
	if err != nil || len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Fatalf("failed flush entries = %v, err = %v", entries, err)
	}
	if err := client.flushPending(context.Background()); err != nil {
		t.Fatalf("retry flush: %v", err)
	}
	entries, err = os.ReadDir(filepath.Join(home, pendingDirName))
	if err != nil || len(entries) != 0 {
		t.Fatalf("pending entries after retry = %d, err = %v", len(entries), err)
	}
}
