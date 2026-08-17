package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Online egress assertions (daily ping, flush round-trips) live in
// client_online_test.go behind !profile_sovereign; the tests below are
// profile-clean local-queue/identity behavior that must hold in every build.

func TestInstallIDRepairsMalformedOwnedFile(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "cli-telemetry-install-id")
	if err := os.WriteFile(path, []byte("truncated\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	id, err := installID(home)
	if err != nil {
		t.Fatalf("installID: %v", err)
	}
	if !validInstallID(id) {
		t.Fatalf("repaired install id = %q", id)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != id {
		t.Fatalf("persisted install id = %q, want %q", got, id)
	}
}

func TestPendingClaimsAreExclusiveAcrossFlushers(t *testing.T) {
	home := t.TempDir()
	if err := appendPending(home, pendingPayload{
		Version: "v1.20.0", OS: "linux", Counters: []Counter{{Signal: "turns", Bucket: "count", Count: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, pendingDirName)
	first, err := claimPendingFiles(dir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := claimPendingFiles(dir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 0 {
		t.Fatalf("claims: first=%v second=%v", first, second)
	}
}

func TestPendingValidationAcceptsAndroid(t *testing.T) {
	if !validPendingPayload(pendingPayload{
		Version: "v1.20.0", OS: "android", Counters: []Counter{{Signal: "turns", Bucket: "count", Count: 1}},
	}) {
		t.Fatal("Android CLI payload was rejected")
	}
}

func TestPendingQueueCountsActiveAndRecoversStaleClaims(t *testing.T) {
	dir := filepath.Join(t.TempDir(), pendingDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := range maxPending {
		path := filepath.Join(dir, strings.Repeat("a", 16)+"-"+time.Unix(int64(i), 0).Format("150405")+".json.uploading")
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := appendPending(filepath.Dir(dir), pendingPayload{
		Version: "v1.20.0", OS: "linux", Counters: []Counter{{Signal: "turns", Bucket: "count", Count: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != maxPending {
		t.Fatalf("bounded queue entries = %d, err = %v", len(entries), err)
	}

	staleDir := filepath.Join(t.TempDir(), pendingDirName)
	if err := os.MkdirAll(staleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	staleClaim := filepath.Join(staleDir, "sample.json.uploading")
	if err := os.WriteFile(staleClaim, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-3 * time.Minute)
	if err := os.Chtimes(staleClaim, stale, stale); err != nil {
		t.Fatal(err)
	}
	if !prunePending(staleDir, time.Now()) {
		t.Fatal("stale claim recovery did not make a queue slot")
	}
	if _, err := os.Stat(strings.TrimSuffix(staleClaim, ".uploading")); err != nil {
		t.Fatalf("stale claim was not recovered: %v", err)
	}
}
