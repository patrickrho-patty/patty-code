//go:build windows

package desktoplauncher

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveInstallRootThroughDirectoryJunction(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(root, "patty-launcher.exe")
	if err := os.WriteFile(launcher, []byte("launcher"), 0o755); err != nil {
		t.Fatal(err)
	}

	junction := filepath.Join(t.TempDir(), "current")
	output, err := exec.Command("cmd", "/c", "mklink", "/J", junction, root).CombinedOutput()
	if err != nil {
		t.Fatalf("create directory junction: %v: %s", err, output)
	}

	got, err := resolveInstallRoot(filepath.Join(junction, "patty-launcher.exe"))
	if err != nil {
		t.Fatal(err)
	}
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	wantInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("resolveInstallRoot() = %q, want %q", got, root)
	}
}

func TestNormalizeFinalWindowsPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "drive", path: `\\?\C:\Apps\Patty Code`, want: `C:\Apps\Patty Code`},
		{name: "UNC", path: `\\?\UNC\server\share\Patty Code`, want: `\\server\share\Patty Code`},
		{name: "volume GUID", path: `\\?\Volume{abc}\Patty Code`, want: `\\?\Volume{abc}\Patty Code`},
		{name: "ordinary", path: `C:\Apps\Patty Code`, want: `C:\Apps\Patty Code`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeFinalWindowsPath(test.path); got != test.want {
				t.Fatalf("normalizeFinalWindowsPath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}
