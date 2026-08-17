//go:build !profile_sovereign

package crashreport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"patty/internal/testenv"
)

type roundTripFunc = testenv.RoundTripFunc

func TestSendUsesSharedProtocolWithoutDeletingLocalReport(t *testing.T) {
	home := t.TempDir()
	if err := CapturePanic(home, "v1.20.0", "boom", []byte("goroutine 1 [running]:\npatty.run()\n\t/home/alice/patty/main.go:12")); err != nil {
		t.Fatal(err)
	}
	pending, err := Load(home, "")
	if err != nil {
		t.Fatal(err)
	}
	var uploaded Report
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != "https://example.invalid/v1/report" {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q", got)
		}
		if err := json.NewDecoder(req.Body).Decode(&uploaded); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})}
	if err := sendWithClient(context.Background(), client, "https://example.invalid/v1/report", pending.Report); err != nil {
		t.Fatal(err)
	}
	if uploaded.Source != "cli.go" || uploaded.Stack == "" || uploaded.TopFrame == "" {
		t.Fatalf("uploaded report = %+v", uploaded)
	}
	if _, err := Load(home, pending.ID); err != nil {
		t.Fatalf("Send removed local report: %v", err)
	}
	if err := Remove(home, pending.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(home, ""); !errors.Is(err, ErrNoReports) {
		t.Fatalf("Load after Remove = %v", err)
	}
}
