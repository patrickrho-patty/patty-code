//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"patty/internal/installlayout"
	"patty/internal/repair"
)

// activateVersionedWindowsFromStaging publishes the versioned-v1 layout from a
// staged NSIS payload:
//
//	InstallRoot/
//	  patty-code-launcher.exe
//	  PatCode.exe              (launcher alias when present or portable)
//	  patcode-cli.exe     (CLI entry; full binary for now)
//	  current.json
//	  versions/<version>/
//	    patty-desktop.exe
//	    patcode-cli.exe
//	    patty-code-update-helper.exe
//
// Any failure before the current.json pointer swap leaves the previous active
// version unchanged. The helper never counts crashes or selects prior versions.
func activateVersionedWindowsFromStaging(claimed *repair.UpdateTransaction, stagingDir string) error {
	if claimed == nil {
		return fmt.Errorf("versioned activate: transaction is nil")
	}
	installRoot := filepath.Clean(strings.TrimSpace(filepath.Dir(claimed.TargetPath)))
	// When the claimed primary is already under versions/<ver>/, climb to root.
	if root, err := installlayout.ResolveInstallRoot(claimed.TargetPath); err == nil && root != "" {
		installRoot = root
	}
	version := strings.TrimSpace(claimed.ToVersion)
	if err := installlayout.ValidateVersionName(version); err != nil {
		// Accept bare product versions from NSIS (1.20.0 → v1.20.0).
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		if err := installlayout.ValidateVersionName(version); err != nil {
			return fmt.Errorf("versioned activate: %w", err)
		}
	}
	stagingDir = filepath.Clean(strings.TrimSpace(stagingDir))

	desktopSrc := filepath.Join(stagingDir, "patty-desktop.exe")
	cliSrc := filepath.Join(stagingDir, "patcode-cli.exe")
	helperSrc := filepath.Join(stagingDir, "patty-code-update-helper.exe")
	launcherSrc := filepath.Join(stagingDir, "patty-code-launcher.exe")
	for _, path := range []string{desktopSrc, cliSrc, helperSrc, launcherSrc} {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("versioned activate: staged %s: %w", filepath.Base(path), err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("versioned activate: staged %s is not a regular file", filepath.Base(path))
		}
	}

	requestID := repair.UpdateTransactionID(claimed)
	if requestID == "" {
		requestID = "helper-" + version
	}
	if err := installlayout.ActivateVersion(installlayout.ActivationRequest{
		InstallRoot: installRoot,
		Version:     version,
		RequestID:   requestID,
		Members: []installlayout.Member{
			{Name: "patty-desktop.exe", Path: desktopSrc, Mode: 0o700},
			{Name: "patcode-cli.exe", Path: cliSrc, Mode: 0o700},
			{Name: "patty-code-update-helper.exe", Path: helperSrc, Mode: 0o700},
		},
		RootMembers: []installlayout.Member{
			{Name: "patty-code-launcher.exe", Path: launcherSrc, Mode: 0o700},
			{Name: "PatCode.exe", Path: launcherSrc, Mode: 0o700},
			{Name: "patcode-cli.exe", Path: cliSrc, Mode: 0o700},
		},
		RequiredRootNames: []string{"patty-code-launcher.exe", "PatCode.exe", "patcode-cli.exe"},
	}); err != nil {
		return err
	}

	// Remove flat release-unit leftovers so the install root is the thin layout.
	// Do not remove the launcher/CLI/alias we just wrote. Legacy flat names are
	// also removed so pre-rename 1.18–1.19.1 installs migrate cleanly.
	for _, name := range []string{
		"patty-desktop.exe",
		"patty-code-guard.exe",
		"patty-code-update-helper.exe", // helper lives only under versions/
		"patty-guard.exe",              // legacy flat guard
		"patty-update-helper.exe",      // legacy flat helper
	} {
		_ = os.Remove(filepath.Join(installRoot, name))
	}
	// Best-effort retention GC of older version trees.
	_ = installlayout.RetainPreviousVersions(installRoot, 0)
	_ = installlayout.CleanupStaleStaging(installRoot, 0)
	return nil
}

// preferVersionedWindowsActivation reports whether the staged payload is
// complete enough for versioned-v1 activation.
func preferVersionedWindowsActivation(stagingDir string) bool {
	for _, name := range []string{
		"patty-desktop.exe",
		"patcode-cli.exe",
		"patty-code-update-helper.exe",
		"patty-code-launcher.exe",
	} {
		info, err := os.Lstat(filepath.Join(stagingDir, name))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}
