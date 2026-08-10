package main

import (
	"context"
	goruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const windowsWebView2StartupFallbackDelay = 15 * time.Second

const windowsWebView2StartupFallbackMessage = "The desktop interface did not become ready within 15 seconds. " +
	"An unavailable Windows system proxy or a WebView2 failure may be blocking startup. " +
	"Check the system proxy, then restart Patty Code. If the problem continues, repair Microsoft Edge WebView2 Runtime or reinstall Patty Code.\n\n" +
	"데스크톱 인터페이스가 15초 내에 준비되지 않았습니다. 사용할 수 없는 Windows 시스템 프록시 또는 WebView2 오류로 시작이 차단되었을 수 있습니다." +
	"시스템 프록시를 확인한 후 Patty Code를 다시 시작하세요. 문제가 지속되면 Microsoft Edge WebView2 Runtime을 복구하거나 Patty Code를 다시 설치하세요."

func (a *App) startWindowsWebView2StartupFallback(ctx context.Context) {
	if !shouldStartWindowsWebView2StartupFallback(goruntime.GOOS, a.remoteWindowTicket != "") {
		return
	}
	go func() {
		timer := time.NewTimer(windowsWebView2StartupFallbackDelay)
		defer timer.Stop()
		if !awaitStartupFallback(ctx, timer.C, a.startupReady.Load) {
			return
		}

		runtime.WindowShow(ctx)
		if a.startupReady.Load() {
			return
		}
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:          runtime.WarningDialog,
			Title:         "Patty Code startup delayed / Patty Code 시작 지연",
			Message:       windowsWebView2StartupFallbackMessage,
			Buttons:       []string{"OK"},
			DefaultButton: "OK",
		})
	}()
}

func shouldStartWindowsWebView2StartupFallback(goos string, remoteWindow bool) bool {
	return goos == "windows" && !remoteWindow
}

func awaitStartupFallback(ctx context.Context, timeout <-chan time.Time, ready func() bool) bool {
	select {
	case <-ctx.Done():
		return false
	case <-timeout:
		return !ready()
	}
}
