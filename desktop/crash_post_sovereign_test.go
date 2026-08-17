//go:build profile_sovereign

package main

import (
	"context"
	"net/http"
	"testing"
)

func TestSovereignDesktopCrashPostFailsClosed(t *testing.T) {
	err := postCrashReport(context.Background(), http.DefaultClient, crashEndpoint, crashReport{Kind: "crash"})
	if err != errSovereignCrashPost {
		t.Fatalf("postCrashReport = %v, want errSovereignCrashPost", err)
	}
	if crashEndpoint != "" {
		t.Fatalf("sovereign crashEndpoint = %q, want empty (no vendor endpoint string)", crashEndpoint)
	}
}
