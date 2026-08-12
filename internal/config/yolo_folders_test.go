package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestYoloFolderAuthorization(t *testing.T) {
	t.Setenv("PATTY_HOME", t.TempDir())
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	if IsYoloAuthorized(dir1) {
		t.Fatalf("fresh folder %q should not be YOLO-authorized", dir1)
	}
	if err := AuthorizeYoloFolder(dir1); err != nil {
		t.Fatalf("AuthorizeYoloFolder: %v", err)
	}
	if !IsYoloAuthorized(dir1) {
		t.Fatalf("approved folder %q should be YOLO-authorized", dir1)
	}
	if IsYoloAuthorized(dir2) {
		t.Fatalf("unrelated folder %q must not be authorized", dir2)
	}

	// Idempotent: re-approving the same folder must not duplicate the entry.
	if err := AuthorizeYoloFolder(filepath.Join(dir1, ".")); err != nil {
		t.Fatalf("re-approve: %v", err)
	}
	data := mustReadYoloFolders(t)
	if got, want := len(data), 1; got != want {
		t.Fatalf("expected %d authorized folder entry, got %d: %v", want, got, data)
	}

	// A second distinct folder appends without disturbing the first.
	if err := AuthorizeYoloFolder(dir2); err != nil {
		t.Fatalf("authorize second folder: %v", err)
	}
	if !IsYoloAuthorized(dir2) || !IsYoloAuthorized(dir1) {
		t.Fatalf("both folders should be authorized after approving the second")
	}
	if got := len(mustReadYoloFolders(t)); got != 2 {
		t.Fatalf("expected 2 entries, got %d", got)
	}
}

func mustReadYoloFolders(t *testing.T) []string {
	t.Helper()
	path := YoloAuthorizedFoldersPath()
	if path == "" {
		t.Fatal("YoloAuthorizedFoldersPath empty under PATTY_HOME")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var f yoloFoldersFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return f.Folders
}
