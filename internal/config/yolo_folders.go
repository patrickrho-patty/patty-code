package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// YoloAuthorizedFoldersPath is the per-user file recording workspace folders the
// user has explicitly approved for YOLO (no-approval) mode. It lives under the
// patty user-state dir so PATTY_HOME / PATTY_STATE_HOME isolation holds, and is
// deliberately separate from config.toml so the credentials/config schema is
// never touched by a folder-approval change.
func YoloAuthorizedFoldersPath() string {
	dir := userSupportDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "yolo_folders.json")
}

type yoloFoldersFile struct {
	Folders []string `json:"folders"`
}

// absCleanDir returns dir made absolute and cleaned, preserving its original
// casing. Empty when dir cannot be resolved.
func absCleanDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return filepath.Clean(dir)
}

// canonicalFolderKey is the comparison key for a workspace folder: absolute and
// cleaned, with case folded on Windows so varying spellings of one folder
// (drive-letter case, Explorer renames) match. NTFS is case-insensitive, so the
// folded key resolves to the same directory.
func canonicalFolderKey(dir string) string {
	key := absCleanDir(dir)
	if key == "" {
		return ""
	}
	if runtimeGOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

// IsYoloAuthorized reports whether dir (typically the process cwd) has been
// previously approved for YOLO mode. Any read/parse failure or empty dir is
// treated as "not authorized" so the disclaimer still shows.
func IsYoloAuthorized(dir string) bool {
	key := canonicalFolderKey(dir)
	if key == "" {
		return false
	}
	data, err := os.ReadFile(YoloAuthorizedFoldersPath())
	if err != nil {
		return false
	}
	var f yoloFoldersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return false
	}
	for _, existing := range f.Folders {
		if canonicalFolderKey(existing) == key {
			return true
		}
	}
	return false
}

// AuthorizeYoloFolder records dir as approved for YOLO mode. It is idempotent
// and de-duplicates by canonical key, so re-approving a folder never produces a
// duplicate entry even across path spellings.
func AuthorizeYoloFolder(dir string) error {
	abs := absCleanDir(dir)
	key := canonicalFolderKey(dir)
	if abs == "" || key == "" {
		return fmt.Errorf("authorize yolo folder: empty directory")
	}
	path := YoloAuthorizedFoldersPath()
	if path == "" {
		return fmt.Errorf("authorize yolo folder: user state dir unavailable")
	}
	var f yoloFoldersFile
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	for _, existing := range f.Folders {
		if canonicalFolderKey(existing) == key {
			return nil
		}
	}
	f.Folders = append(f.Folders, abs)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
