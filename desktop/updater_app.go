package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"patty/desktop/internal/update"
	"patty/internal/installlayout"
	"patty/internal/repair"
)

// updater_app.go is the auto-updater's bound command surface — the App methods the
// frontend calls — mirroring settings_app.go's "one file per concern" split. The
// download progress as "updater:progress" events and routes macOS to the manual

var errUpdateManualRequired = errors.New("update: manual update required")
var errUpdateInProgress = errors.New("update: another download or install is already in progress")

var updaterRequestIDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

var (
	pendingUpdateExistsForInstall            = repair.PendingUpdateExists
	archiveSupersededPendingUpdateForInstall = archiveSupersededPendingUpdateAfterReady
	reconcilePendingUpdateForInstall         = repair.ReconcilePendingUpdate
	readPendingUpdateForHealth               = repair.ReadPendingUpdate
	markPendingUpdateHealthyAfterReady       = repair.MarkUpdateHealthyExact
)

func validateUpdaterRequest(requestID, selectedChannel, expectedVersion string) (string, string, string, error) {
	requestID = strings.TrimSpace(requestID)
	if !updaterRequestIDRE.MatchString(requestID) {
		return "", "", "", fmt.Errorf("update: invalid request id")
	}
	selectedChannel = targetUpdateChannel(selectedChannel)
	expectedVersion = strings.TrimSpace(expectedVersion)
	if !stableDesktopVersionRE.MatchString(expectedVersion) {
		return "", "", "", fmt.Errorf("update: invalid %s version %q", selectedChannel, expectedVersion)
	}
	return requestID, selectedChannel, expectedVersion, nil
}

func (a *App) beginUpdaterOperation(requestID string) (func(), error) {
	a.updaterOperationMu.Lock()
	defer a.updaterOperationMu.Unlock()
	if a.updaterOperationID != "" {
		return nil, errUpdateInProgress
	}
	a.updaterOperationID = requestID
	return func() {
		a.updaterOperationMu.Lock()
		if a.updaterOperationID == requestID {
			a.updaterOperationID = ""
		}
		a.updaterOperationMu.Unlock()
	}, nil
}

func ensureExpectedUpdateVersion(selectedChannel, expectedVersion, actualVersion string) error {
	if actualVersion == expectedVersion {
		return nil
	}
	return fmt.Errorf(
		"update: %s pointer changed from %s to %s; check again before downloading",
		selectedChannel,
		expectedVersion,
		actualVersion,
	)
}

func (a *App) Version() string { return version }

func (a *App) CheckUpdate(selectedChannel string) (*UpdateInfo, error) {
	selectedChannel = targetUpdateChannel(selectedChannel)
	profile := detectInstallProfile()
	c, err := httpClient()
	if err != nil {
		a.recordUpdateError(err)
		return &UpdateInfo{
			Current:           version,
			Channel:           selectedChannel,
			CanSelfUpdate:     profile.CanSelfUpdate && canSelfUpdate(),
			ManualOnly:        !(profile.CanSelfUpdate && canSelfUpdate()),
			ManualReason:      firstNonEmptyStr(profile.ManualReason, manualUpdateReason()),
			InstallMode:       profile.Mode,
			RequiresElevation: profile.RequiresElev,
			DownloadURL:       downloadPage(selectedChannel),
			Err:               err.Error(),
		}, nil
	}
	ctx, cancel := context.WithTimeout(a.reqCtx(), httpTimeout)
	defer cancel()
	v4, _ := httpClientIPv4()
	m, err := fetchManifest(ctx, c, v4, selectedChannel)
	if err != nil {
		a.recordUpdateError(err)
		return &UpdateInfo{
			Current:           version,
			Channel:           selectedChannel,
			CanSelfUpdate:     profile.CanSelfUpdate && canSelfUpdate(),
			ManualOnly:        !(profile.CanSelfUpdate && canSelfUpdate()),
			ManualReason:      firstNonEmptyStr(profile.ManualReason, manualUpdateReason()),
			InstallMode:       profile.Mode,
			RequiresElevation: profile.RequiresElev,
			DownloadURL:       downloadPage(selectedChannel),
			Err:               err.Error(),
		}, nil
	}
	info := evaluateForChannel(version, selectedChannel, m)
	return &info, nil
}

// OpenDownloadPage opens the install page in the browser  the macOS manual-update
func (a *App) OpenDownloadPage() {
	a.openDownloadPage(targetUpdateChannel(""))
}

func (a *App) openDownloadPage(selectedChannel string) {
	selectedChannel = targetUpdateChannel(selectedChannel)
	page := downloadPage(selectedChannel)
	if c, err := httpClient(); err == nil {
		ctx, cancel := context.WithTimeout(a.reqCtx(), httpTimeout)
		defer cancel()
		v4, _ := httpClientIPv4()
		if m, err := fetchManifest(ctx, c, v4, selectedChannel); err == nil {
			page = manifestDownloadPage(selectedChannel, m.DownloadPage)
		}
	}
	if a.ctx != nil {
		wruntime.BrowserOpenURL(a.ctx, page)
	}
}

func (a *App) downloadUpdateRequest(selectedChannel, expectedVersion, requestID string) (*UpdateDownloadResult, error) {
	requestID, selectedChannel, expectedVersion, err := validateUpdaterRequest(requestID, selectedChannel, expectedVersion)
	if err != nil {
		return nil, err
	}
	profile := detectInstallProfile()
	if !profile.CanSelfUpdate || !canSelfUpdate() {
		return nil, a.requireManualUpdate(requestID, selectedChannel, expectedVersion, profile)
	}
	c, err := httpClient()
	if err != nil {
		return nil, a.failUpdate(requestID, selectedChannel, expectedVersion, err)
	}
	ctx, cancel := context.WithTimeout(a.reqCtx(), httpTimeout)
	defer cancel()
	v4, _ := httpClientIPv4()
	m, err := fetchManifest(ctx, c, v4, selectedChannel)
	if err != nil {
		return nil, a.failUpdate(requestID, selectedChannel, expectedVersion, err)
	}
	if err := ensureExpectedUpdateVersion(selectedChannel, expectedVersion, m.Version); err != nil {
		return nil, a.failUpdate(requestID, selectedChannel, expectedVersion, err)
	}
	profile = profileForManifest(profile, m)
	if !profile.CanSelfUpdate {
		return nil, a.requireManualUpdate(requestID, selectedChannel, expectedVersion, profile)
	}
	asset, kind, ok := selectUpdateAsset(m, profile)
	if !ok {
		return nil, a.failUpdate(requestID, selectedChannel, expectedVersion, fmt.Errorf("no update artifact for %s", update.CurrentPlatform()))
	}

	data, sig, err := a.downloadVerify(requestID, selectedChannel, expectedVersion, asset)
	if err != nil {
		return nil, a.failUpdate(requestID, selectedChannel, expectedVersion, err)
	}
	meta, err := saveCachedUpdateForChannel(selectedChannel, m.Version, asset, data, kind, sig)
	if err != nil {
		return nil, a.failUpdate(requestID, selectedChannel, expectedVersion, err)
	}
	a.emitProgress(requestID, selectedChannel, meta.Version, "downloaded", meta.Size, meta.Size, "")
	return &UpdateDownloadResult{
		RequestID: requestID,
		Version:   meta.Version,
		Channel:   meta.Channel,
		Path:      meta.Path,
		Size:      meta.Size,
		SHA256:    meta.SHA256,
	}, nil
}

// request and then exitsrelaunches. Used only by ApplyUpdateRequest.
func (a *App) installUpdateRequest(selectedChannel, expectedVersion, requestID string) error {
	requestID, selectedChannel, expectedVersion, err := validateUpdaterRequest(requestID, selectedChannel, expectedVersion)
	if err != nil {
		return err
	}
	profile := detectInstallProfile()
	if !profile.CanSelfUpdate || !canSelfUpdate() {
		return a.requireManualUpdate(requestID, selectedChannel, expectedVersion, profile)
	}
	meta, data, err := readVerifiedCachedUpdateForChannel(selectedChannel)
	if err != nil {
		return a.failUpdate(requestID, selectedChannel, expectedVersion, err)
	}
	if meta.Version != expectedVersion {
		return a.failUpdate(requestID, selectedChannel, expectedVersion, fmt.Errorf(
			"update: cached version %s does not match checked version %s",
			meta.Version,
			expectedVersion,
		))
	}
	if c, err := httpClient(); err == nil {
		ctx, cancel := context.WithTimeout(a.reqCtx(), httpTimeout)
		defer cancel()
		v4, _ := httpClientIPv4()
		if m, err := fetchManifest(ctx, c, v4, selectedChannel); err == nil {
			profile = profileForManifest(detectInstallProfile(), m)
		} else {
			profile = detectInstallProfile()
		}
	} else {
		profile = detectInstallProfile()
	}
	if !profile.CanSelfUpdate {
		return a.requireManualUpdate(requestID, selectedChannel, expectedVersion, profile)
	}
	if err := ensureDebCacheMatchesProfile(meta, profile); err != nil {
		return a.failUpdate(requestID, selectedChannel, expectedVersion, err)
	}
	wantKind := profile.ArtifactKind
	if wantKind == "" {
		wantKind = artifactKindTarball
	}
	if artifactKindFromMeta(meta.ArtifactKind) != artifactKindFromMeta(wantKind) {
		return a.failUpdate(requestID, selectedChannel, expectedVersion, errUpdateCacheMismatch)
	}
	if err := a.reconcilePendingUpdateForRequest(requestID, meta); err != nil {
		return err
	}

	switch profile.Mode {
	case installModeDeb:
		return a.installDebUpdate(requestID, meta)
	default:
		return a.installPortableUpdate(requestID, meta, data)
	}
}

func (a *App) reconcilePendingUpdateForRequest(requestID string, meta *cachedUpdate) error {
	if pendingUpdateExistsForInstall() {
		a.emitProgress(requestID, meta.Channel, meta.Version, "recovering", meta.Size, meta.Size, "")
		if archived, archiveErr := archiveSupersededPendingUpdateForInstall(); archiveErr != nil {
			slog.Debug("desktop: superseded update was not eligible for automatic archival", "err", archiveErr)
		} else if archived {
			slog.Info("desktop: archived superseded update before install")
		}
// Visible UI at the pending target is health evidence  heal before
		refreshPendingUpdateHealthIdentity(a)
		if err := a.commitPendingUpdateHealth(); err != nil {
			slog.Debug("desktop: commit healthy update before install", "err", err)
		}
		if committed, err := repair.CommitProbationaryPendingUpdate(version); err != nil {
			slog.Debug("desktop: probationary update commit before install", "err", err)
		} else if committed {
			slog.Info("desktop: committed probationary update before install")
		}
	}
	if _, err := reconcilePendingUpdateForInstall(version); err != nil {
		if errors.Is(err, repair.ErrPendingUpdateAwaitingHealth) {
			refreshPendingUpdateHealthIdentity(a)
			if commitErr := a.commitPendingUpdateHealth(); commitErr != nil {
				slog.Debug("desktop: commit healthy update on awaiting-health retry", "err", commitErr)
			}
			if committed, commitErr := repair.CommitProbationaryPendingUpdate(version); commitErr != nil {
				slog.Debug("desktop: probationary update commit on awaiting-health retry", "err", commitErr)
			} else if committed {
				slog.Info("desktop: committed probationary update on awaiting-health retry")
			}
			if _, retryErr := reconcilePendingUpdateForInstall(version); retryErr == nil {
				return nil
			} else {
				err = retryErr
			}
		}
		if errors.Is(err, repair.ErrPendingUpdateAwaitingHealth) {
			err = fmt.Errorf("update recovery: the previous update is still completing its startup health check; wait briefly and try again, or discard the previous update")
		} else {
			err = fmt.Errorf("update recovery: could not safely finish the previous update: %w", err)
		}
		return a.failUpdate(requestID, meta.Channel, meta.Version, err)
	}
	return nil
}

func (a *App) AbandonPendingUpdate() error {
	if !pendingUpdateExistsForInstall() {
		return nil
	}
	refreshPendingUpdateHealthIdentity(a)
	if err := a.commitPendingUpdateHealth(); err != nil {
		slog.Debug("desktop: commit healthy update during abandon", "err", err)
	}
	if archived, err := archiveSupersededPendingUpdateForInstall(); err != nil {
		slog.Debug("desktop: archive superseded update during abandon", "err", err)
	} else if archived {
		return nil
	}
	if _, err := repair.AbandonPendingUpdate(version); err != nil {
		return fmt.Errorf("could not discard the previous update: %w", err)
	}
	return nil
}

func (a *App) installDebUpdate(requestID string, meta *cachedUpdate) error {
	a.emitProgress(requestID, meta.Channel, meta.Version, "authorizing", meta.Size, meta.Size, "")
	err := applyDebLinux(meta.Path, meta.SignaturePath, func(phase string) {
		if phase == "installing" {
			a.emitProgress(requestID, meta.Channel, meta.Version, "installing", meta.Size, meta.Size, "")
		}
	})
	if isAuthCancelled(err) {
		a.recordUpdateEvent("authorization_cancelled")
		a.emitProgress(requestID, meta.Channel, meta.Version, "downloaded", meta.Size, meta.Size, "")
		return nil
	}
	if err != nil {
		if errors.Is(err, errUpdateAuthFailed) {
// Surface a manual-install hint without writing /usr/bin ourselves.
			return a.failUpdate(requestID, meta.Channel, meta.Version, fmt.Errorf("%w. %s", err, manualDebInstallHint()))
		}
		return a.failUpdate(requestID, meta.Channel, meta.Version, err)
	}
	a.emitProgress(requestID, meta.Channel, meta.Version, "installing", meta.Size, meta.Size, "")
	a.emitProgress(requestID, meta.Channel, meta.Version, "done", meta.Size, meta.Size, "")
	a.shutdown(a.ctx)
	_ = relaunchThroughLauncher()
	os.Exit(0)
	return nil
}

func (a *App) installPortableUpdate(requestID string, meta *cachedUpdate, data []byte) error {
	a.emitProgress(requestID, meta.Channel, meta.Version, "installing", meta.Size, meta.Size, "")
	var preparedUpdate *repair.UpdateTransaction
	versionedPortable := (runtime.GOOS == "windows" || runtime.GOOS == "linux") && installlayout.HasCurrent(currentInstallDir())
	if (runtime.GOOS == "windows" || runtime.GOOS == "linux") && !versionedPortable {
// state owns /usr/bin.
		var err error
		preparedUpdate, err = repair.PrepareFileUpdate(version, meta.Version, currentExecutablePath(), updateSiblingArtifacts()...)
		if err != nil {
			return a.failUpdate(requestID, meta.Channel, meta.Version, err)
		}
	}
	var err error
	switch runtime.GOOS {
	case "windows":
		err = applyWindowsFile(meta.Path, meta.SHA256, meta.Version, preparedUpdate)
	case "darwin":
		err = applyMac(meta.Path, meta.Version)
	case "linux":
		if versionedPortable {
			err = applyLinuxVersioned(data, meta.Version)
		} else {
			err = applyLinux(data, preparedUpdate)
		}
	default:
		err = fmt.Errorf("self-update unsupported on %s", runtime.GOOS)
	}
	if err != nil {
		if runtime.GOOS == "linux" {
// fails, retain the transaction for explicit repairreconciliation.
			if preparedUpdate != nil {
				if _, rollbackErr := repair.RollbackPendingUpdateExact(preparedUpdate); rollbackErr != nil {
					err = errors.Join(err, fmt.Errorf("restore prepared release unit: %w", rollbackErr))
				} else if clearErr := repair.ClearUpdateApplyFailureExact(preparedUpdate); clearErr != nil {
					err = errors.Join(err, fmt.Errorf("clear update recovery marker: %w", clearErr))
				}
			}
		} else if runtime.GOOS == "windows" {
			if preparedUpdate != nil {
				if cancelErr := repair.CancelPendingUpdateExact(preparedUpdate); cancelErr != nil {
					err = errors.Join(err, fmt.Errorf("cancel prepared update: %w", cancelErr))
				}
			}
		} else if runtime.GOOS != "darwin" {
			if preparedUpdate != nil {
				if cancelErr := repair.CancelPendingUpdateExact(preparedUpdate); cancelErr != nil {
					err = errors.Join(err, fmt.Errorf("cancel prepared update: %w", cancelErr))
				}
			}
		}
		return a.failUpdate(requestID, meta.Channel, meta.Version, err)
	}

	a.emitProgress(requestID, meta.Channel, meta.Version, "done", meta.Size, meta.Size, "")

// macOS the installerhelper we launched takes over once we exit.
	a.shutdown(a.ctx)
	if runtime.GOOS == "linux" {
		_ = relaunchThroughLauncher()
	}
	os.Exit(0)
	return nil
}

// path (); there is no durable cross-restart pending state when the
// operation fails  the user simply retries.
func (a *App) ApplyUpdateRequest(selectedChannel, expectedVersion, requestID string) error {
	requestID, selectedChannel, expectedVersion, err := validateUpdaterRequest(requestID, selectedChannel, expectedVersion)
	if err != nil {
		return err
	}
	finish, err := a.beginUpdaterOperation(requestID)
	if err != nil {
		return err
	}
	defer finish()

	if err := a.reconcilePendingUpdateForRequest(requestID, &cachedUpdate{
		Channel: selectedChannel,
		Version: expectedVersion,
	}); err != nil {
		return err
	}
	if _, err := a.downloadUpdateRequest(selectedChannel, expectedVersion, requestID); err != nil {
		return err
	}
	a.emitProgress(requestID, selectedChannel, expectedVersion, "installing", 0, 0, "")
	if err := a.installUpdateRequest(selectedChannel, expectedVersion, requestID); err != nil {
		return err
	}
	a.emitProgress(requestID, selectedChannel, expectedVersion, "relaunching", 0, 0, "")
	return nil
}

func (a *App) downloadVerify(requestID, selectedChannel, expectedVersion string, asset update.Asset) (data, sig []byte, err error) {
	c, err := httpClient()
	if err != nil {
		return nil, nil, err
	}
	v4, _ := httpClientIPv4() // best-effort IPv4 fallback; nil just means retries reuse c
	data, err = downloadForChannel(a.reqCtx(), c, v4, selectedChannel, asset.URL, asset.Size, func(rcv, total int64) {
		a.emitProgress(requestID, selectedChannel, expectedVersion, "downloading", rcv, total, "")
	})
	if err != nil {
		return nil, nil, err
	}
	a.emitProgress(requestID, selectedChannel, expectedVersion, "verifying", asset.Size, asset.Size, "")
	sig, err = fetchBytesFallbackForChannelSized(
		a.reqCtx(),
		c,
		v4,
		selectedChannel,
		asset.Sig,
		maxDesktopSignatureSize,
	)
	if err != nil {
		return nil, nil, err
	}
	if err := update.Verify(data, sig); err != nil {
		return nil, nil, err
	}
	if err := checkSHA256(data, asset.SHA256); err != nil {
		return nil, nil, err
	}
	return data, sig, nil
}

// reqCtx is the context for updater HTTP calls  the Wails context once startup has
func (a *App) reqCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) emitProgress(requestID, selectedChannel, expectedVersion, phase string, received, total int64, errMsg string) {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "updater:progress", updateProgress{
		RequestID: requestID,
		Version:   expectedVersion,
		Channel:   normalizeUpdateChannel(selectedChannel),
		Phase:     phase, Received: received, Total: total, Err: errMsg,
	})
}

func (a *App) failUpdate(requestID, selectedChannel, expectedVersion string, err error) error {
	a.recordUpdateError(err)
	a.emitProgress(requestID, selectedChannel, expectedVersion, "error", 0, 0, err.Error())
	return err
}

func (a *App) requireManualUpdate(requestID, selectedChannel, expectedVersion string, profile installProfile) error {
	err := a.failUpdate(requestID, selectedChannel, expectedVersion, manualUpdateRequiredError(profile))
	a.openDownloadPage(selectedChannel)
	return err
}

func manualUpdateRequiredError(profile installProfile) error {
	reason := firstNonEmptyStr(profile.ManualReason, manualUpdateReason(), "automatic update is unavailable for this install")
	return fmt.Errorf("%w: %s", errUpdateManualRequired, reason)
}

func (a *App) recordUpdateError(err error) {
	if err == nil || version == "dev" {
		return
	}
	if isAuthCancelled(err) {
		return
	}
	if m := a.metrics.Load(); m != nil {
		m.inc("updater_error", errorClass(err.Error()))
	}
}

func (a *App) recordUpdateEvent(bucket string) {
	if version == "dev" {
		return
	}
	if m := a.metrics.Load(); m != nil {
		m.inc("updater_event", bucket)
	}
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
