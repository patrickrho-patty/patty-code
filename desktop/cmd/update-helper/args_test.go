package main

import (
	"strings"
	"testing"
)

func TestInstallerCommandLineUsesVisibleUpdateModeAndLeavesDFlagLast(t *testing.T) {
	got := installerCommandLine(`C:\Temp\Patty Code Installer.exe`, `D:\Tools\Patty Code App`)
	want := `"C:\Temp\Patty Code Installer.exe" /PATTYCODEUPDATE=1 /PATTYCODESTAGE=1 /D=D:\Tools\Patty Code App`
	if got != want {
		t.Fatalf("installerCommandLine = %q, want %q", got, want)
	}
	if strings.Contains(got, " /S") {
		t.Fatalf("auto-update must expose progress instead of using silent mode, got %q", got)
	}
	if !strings.HasSuffix(got, `/D=D:\Tools\Patty Code App`) {
		t.Fatalf("/D= must be the final unquoted NSIS token, got %q", got)
	}
	if !strings.Contains(got, " /PATTYCODESTAGE=1") {
		t.Fatalf("auto-update must extract away from the live install, got %q", got)
	}
}
