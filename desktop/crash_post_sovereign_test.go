//go:build profile_sovereign

package main

import (
	"context"
	"net/http"
	"testing"
)

func TestSovereignDesktopCrashPostFailsClosed(t *testing.T) {
	err := postCrashReport(context.Background(), http.DefaultClient, crashEndpoint, crashReport{Kind: "crash"})
	if err != ErrCrashPostUnavailable {
		t.Fatalf("postCrashReport = %v, want ErrCrashPostUnavailable", err)
	}
	if crashEndpoint != "" {
		t.Fatalf("sovereign crashEndpoint = %q, want empty (no vendor endpoint string)", crashEndpoint)
	}
}

// TestSovereignFlushPendingCrashKeepsFile pins ADR G2's 'local list/show/
// delete kept' promise: flushPendingCrash must NOT wipe the queue directory
// when every postCrashReport call fails closed, so the operator can still
// list the captured panics via `patcode report list`.
func TestSovereignFlushPendingCrashKeepsFile(t *testing.T) {
	oldVersion := version
	t.Cleanup(func() {
		version = oldVersion
		removeAllPendingCrashes()
	})
	version = "v9.9.9"

	writePendingCrash("sovereign", "boom", []byte("stack"))
	NewApp().flushPendingCrash()

	if _, ok := readPending(t); !ok {
		t.Fatal("sovereign flushPendingCrash must keep the pending file so local list/show/delete remain available")
	}
}
